package integration

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"warehouse/backend/internal/app"
	"warehouse/backend/internal/platform/config"
)

// Test-only protocol proxy: PostgreSQL commits normally, but the client never
// receives that COMMIT acknowledgement. Only a disposable fixture uses it.
type commitDropProxy struct {
	listener            net.Listener
	target              string
	commits             atomic.Int32
	acknowledgementLost chan struct{}
	once                sync.Once
	workers             sync.WaitGroup
	mu                  sync.Mutex
	connections         []net.Conn
}

func newCommitDropProxy(t *testing.T, target string) *commitDropProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxy := &commitDropProxy{listener: listener, target: target, acknowledgementLost: make(chan struct{})}
	proxy.workers.Add(1)
	go func() {
		defer proxy.workers.Done()
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			proxy.mu.Lock()
			proxy.connections = append(proxy.connections, client)
			proxy.mu.Unlock()
			proxy.workers.Add(1)
			go func() { defer proxy.workers.Done(); proxy.forward(client) }()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		proxy.mu.Lock()
		for _, conn := range proxy.connections {
			_ = conn.Close()
		}
		proxy.mu.Unlock()
		done := make(chan struct{})
		go func() { proxy.workers.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("commit proxy did not stop")
		}
	})
	return proxy
}
func readPostgresPacket(conn net.Conn) (byte, []byte, error) {
	var header [5]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return 0, nil, err
	}
	size := int(binary.BigEndian.Uint32(header[1:]))
	if size < 4 || size > 1<<24 {
		return 0, nil, fmt.Errorf("invalid PostgreSQL packet length")
	}
	packet := make([]byte, 1+size)
	copy(packet, header[:])
	_, err := io.ReadFull(conn, packet[5:])
	return header[0], packet, err
}
func (p *commitDropProxy) forward(client net.Conn) {
	defer client.Close()
	upstream, err := net.DialTimeout("tcp", p.target, time.Second)
	if err != nil {
		return
	}
	defer upstream.Close()
	// Startup packets have a length but no type byte. TLS is disabled for this fixture.
	var length [4]byte
	if _, err = io.ReadFull(client, length[:]); err != nil {
		return
	}
	size := int(binary.BigEndian.Uint32(length[:]))
	if size < 8 || size > 1<<20 {
		return
	}
	startup := make([]byte, size)
	copy(startup, length[:])
	if _, err = io.ReadFull(client, startup[4:]); err != nil {
		return
	}
	if _, err = upstream.Write(startup); err != nil {
		return
	}
	var commitPending atomic.Bool
	downstreamDone := make(chan struct{})
	go func() {
		defer close(downstreamDone)
		defer client.Close()
		defer upstream.Close()
		for {
			tag, packet, err := readPostgresPacket(upstream)
			if err != nil {
				return
			}
			if commitPending.Load() {
				if tag == 'C' && string(packet[5:]) == "COMMIT\x00" {
					p.once.Do(func() { close(p.acknowledgementLost) })
					return
				}
				continue
			}
			if _, err = client.Write(packet); err != nil {
				return
			}
		}
	}()
	for {
		tag, packet, err := readPostgresPacket(client)
		if err != nil {
			break
		}
		if tag == 'Q' && strings.EqualFold(strings.TrimSpace(strings.TrimSuffix(string(packet[5:]), "\x00")), "commit") {
			p.commits.Add(1)
			commitPending.Store(true)
		}
		if _, err = upstream.Write(packet); err != nil {
			break
		}
	}
	_ = upstream.Close()
	<-downstreamDone
}

func TestT23LostCommitAcknowledgementThroughActualHTTP(t *testing.T) {
	f := newItemFixture(t)
	item := f.create(t, "COMMIT-UNKNOWN", "Commit unknown", "Stock")
	locationID := f.locations(t)[0]
	original := f.pool.Config()
	target := net.JoinHostPort(original.ConnConfig.Host, strconv.Itoa(int(original.ConnConfig.Port)))
	proxy := newCommitDropProxy(t, target)
	host, portText, err := net.SplitHostPort(proxy.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portText)
	cfg := f.pool.Config()
	cfg.MaxConns = 1
	cfg.ConnConfig.Host = host
	cfg.ConnConfig.Port = uint16(port)
	cfg.ConnConfig.TLSConfig = nil
	cfg.ConnConfig.Fallbacks = nil
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	logs := &logBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	server := httptest.NewServer(app.NewApplicationRouter(config.Config{RequestTimeout: 3 * time.Second}, logger, pool))
	defer server.Close()
	test := &itemFixture{pool: pool, server: server, logs: logs}
	response, _ := test.request(t, "POST", "/api/v1/stock/receive", fmt.Sprintf(`{"item_id":%d,"location_id":%d,"qty":5}`, item.ID, locationID), 500)
	select {
	case <-proxy.acknowledgementLost:
	case <-ctx.Done():
		t.Fatal("COMMIT acknowledgement was not dropped")
	}
	if response.Message != "Stock receive outcome unknown" || !strings.Contains(response.Errors[0].Reason, "may have committed") {
		t.Fatal("claimed definitive failure", response)
	}
	if qty, count := f.quantity(t, item.ID, locationID); qty != 5 || count != 1 {
		t.Fatal("server commit was not retained", qty, count)
	}
	if proxy.commits.Load() != 1 {
		t.Fatal("automatic receive retry", proxy.commits.Load())
	}
	if len(logs.events(t, "stock.receive.committed")) != 0 {
		t.Fatal("unacknowledged commit emitted a committed-success log")
	}
	unknown := logs.events(t, "stock.receive.commit_unknown")
	if len(unknown) != 1 || unknown[0]["request_id"] != "phase05-test" || unknown[0]["item_id"] != float64(item.ID) || unknown[0]["location_id"] != float64(locationID) || unknown[0]["received_qty"] != float64(5) {
		t.Fatal("missing uncertainty log", unknown)
	}
	assertPoolReleased(t, pool)
	t.Log("PostgreSQL committed qty=5; COMMIT reply was dropped; HTTP 500 explicitly unknown; one uncertainty log; no retry or success log")
}

func TestEnterpriseTransferLostCommitAcknowledgement(t *testing.T) {
	f := newItemFixture(t)
	item := f.create(t, "COMMIT-UNKNOWN", "Commit unknown", "Stock")
	locations := f.locations(t)
	locationID := locations[0]
	f.receive(t, item.ID, locationID, 10, 200)
	original := f.pool.Config()
	target := net.JoinHostPort(original.ConnConfig.Host, strconv.Itoa(int(original.ConnConfig.Port)))
	proxy := newCommitDropProxy(t, target)
	host, portText, err := net.SplitHostPort(proxy.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portText)
	cfg := f.pool.Config()
	cfg.MaxConns = 1
	cfg.ConnConfig.Host = host
	cfg.ConnConfig.Port = uint16(port)
	cfg.ConnConfig.TLSConfig = nil
	cfg.ConnConfig.Fallbacks = nil
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	logs := &logBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	server := httptest.NewServer(app.NewApplicationRouter(config.Config{RequestTimeout: 3 * time.Second}, logger, pool))
	defer server.Close()
	test := &itemFixture{pool: pool, server: server, logs: logs}
	response, _ := test.request(t, "POST", "/api/v1/stock/transfer", transferBody(item.ID, locationID, locations[1], 5), 500)
	select {
	case <-proxy.acknowledgementLost:
	case <-ctx.Done():
		t.Fatal("COMMIT acknowledgement was not dropped")
	}
	if response.Message != "Stock transfer outcome unknown" || !strings.Contains(response.Errors[0].Reason, "may have committed") {
		t.Fatal("claimed definitive failure", response)
	}
	if qty, count := f.quantity(t, item.ID, locationID); qty != 5 || count != 1 {
		t.Fatal("server commit was not retained", qty, count)
	}
	if proxy.commits.Load() != 1 {
		t.Fatal("automatic receive retry", proxy.commits.Load())
	}
	if len(logs.events(t, "stock.transfer.committed")) != 0 {
		t.Fatal("unacknowledged commit emitted a committed-success log")
	}
	unknown := logs.events(t, "stock.transfer.commit_unknown")
	if len(unknown) != 1 || unknown[0]["request_id"] != "phase05-test" || unknown[0]["item_id"] != float64(item.ID) || unknown[0]["from_location_id"] != float64(locationID) || unknown[0]["qty"] != float64(5) {
		t.Fatal("missing uncertainty log", unknown)
	}
	assertPoolReleased(t, pool)
	t.Log("PostgreSQL committed qty=5; COMMIT reply was dropped; HTTP 500 explicitly unknown; one uncertainty log; no retry or success log")
}

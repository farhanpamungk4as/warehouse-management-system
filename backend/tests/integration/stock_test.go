package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"warehouse/backend/internal/app"
	stockdomain "warehouse/backend/internal/modules/stock/domain"
	stockpostgres "warehouse/backend/internal/modules/stock/repository/postgres"
	stockservice "warehouse/backend/internal/modules/stock/service"
	"warehouse/backend/internal/platform/config"
	"warehouse/backend/internal/platform/middleware"
)

// Capture slog safely while independent HTTP requests complete concurrently.
type logBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}
func (b *logBuffer) events(t *testing.T, name string) []map[string]any {
	t.Helper()
	b.mu.Lock()
	raw := b.data.String()
	b.mu.Unlock()
	var events []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event["event"] == name {
			events = append(events, event)
		}
	}
	return events
}
func (f *itemFixture) locations(t *testing.T) []int32 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := f.pool.Query(ctx, "SELECT id FROM locations ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil || len(ids) < 2 {
		t.Fatalf("seed locations: %v %v", ids, err)
	}
	return ids
}
func (f *itemFixture) receive(t *testing.T, itemID, locationID int32, qty int64, status int) stockdomain.Stock {
	t.Helper()
	v, _ := f.request(t, "POST", "/api/v1/stock/receive", fmt.Sprintf(`{"item_id":%d,"location_id":%d,"qty":%d}`, itemID, locationID, qty), status)
	if status >= 400 {
		return stockdomain.Stock{}
	}
	var stock stockdomain.Stock
	if err := json.Unmarshal(v.Data, &stock); err != nil {
		t.Fatal(err)
	}
	if stock.ID < 1 || stock.ItemID != itemID || stock.LocationID != locationID || stock.UpdatedAt.IsZero() || !strings.HasSuffix(stock.UpdatedAt.Format(time.RFC3339Nano), "Z") {
		t.Fatal(stock)
	}
	return stock
}
func (f *itemFixture) lookup(t *testing.T, itemID int32, status int) stockdomain.Lookup {
	t.Helper()
	v, _ := f.request(t, "GET", fmt.Sprintf("/api/v1/stock/%d", itemID), "", status)
	if status >= 400 {
		return stockdomain.Lookup{}
	}
	var view stockdomain.Lookup
	if err := json.Unmarshal(v.Data, &view); err != nil || view.ItemID != itemID || view.Stocks == nil {
		t.Fatalf("lookup: %s %v", v.Data, err)
	}
	return view
}
func (f *itemFixture) quantity(t *testing.T, itemID, locationID int32) (int32, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var qty int32
	var count int
	if err := f.pool.QueryRow(ctx, "SELECT coalesce(sum(qty),0),count(*) FROM stocks WHERE item_id=$1 AND location_id=$2", itemID, locationID).Scan(&qty, &count); err != nil {
		t.Fatal(err)
	}
	return qty, count
}
func beginTestTx(t *testing.T, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tx.Rollback(ctx)
	})
	return tx
}

// The primary synchronization signal is PostgreSQL reporting lock waits, not a sleep.
func waitForLockWaiters(t *testing.T, pool *pgxpool.Pool, queryPart string, want int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var count int
		err := pool.QueryRow(ctx, `SELECT count(DISTINCT pid) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND position($1 in query)>0`, queryPart).Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
		if count >= want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wanted %d independent PostgreSQL waiters for %q, got %d", want, queryPart, count)
		case <-ticker.C:
		}
	}
}
func receiveAsync(f *itemFixture, itemID, locationID int32, qty int64) <-chan asyncReceipt {
	done := make(chan asyncReceipt, 1)
	go func() {
		req, err := http.NewRequest("POST", f.server.URL+"/api/v1/stock/receive", strings.NewReader(fmt.Sprintf(`{"item_id":%d,"location_id":%d,"qty":%d}`, itemID, locationID, qty)))
		if err != nil {
			done <- asyncReceipt{err: err}
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", "concurrent-stock-test")
		client := &http.Client{Timeout: 6 * time.Second}
		res, err := client.Do(req)
		if err != nil {
			done <- asyncReceipt{err: err}
			return
		}
		defer res.Body.Close()
		var envelope itemEnvelope
		err = json.NewDecoder(res.Body).Decode(&envelope)
		result := asyncReceipt{status: res.StatusCode, body: envelope, err: err}
		if res.StatusCode == 200 {
			result.err = json.Unmarshal(envelope.Data, &result.stock)
		}
		done <- result
	}()
	return done
}

type asyncReceipt struct {
	status int
	stock  stockdomain.Stock
	body   itemEnvelope
	err    error
}

func awaitReceipt(t *testing.T, done <-chan asyncReceipt) asyncReceipt {
	t.Helper()
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.body.Success != (result.status == 200) {
			t.Fatal("invalid async envelope", result)
		}
		return result
	case <-time.After(7 * time.Second):
		t.Fatal("receipt did not finish")
		return asyncReceipt{}
	}
}
func assertPoolReleased(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal("transaction leaked its only connection", err)
	}
	conn.Release()
	if pool.Stat().AcquiredConns() != 0 {
		t.Fatal("connection not released")
	}
}
func singlePool(t *testing.T, f *itemFixture) *pgxpool.Pool {
	t.Helper()
	cfg := f.pool.Config()
	cfg.MaxConns = 1
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestStockReceivingLookupAndRetention(t *testing.T) {
	f := newItemFixture(t)
	ids := f.locations(t)
	item := f.create(t, "STOCK-BASE", "Stock box", "Stock")
	t.Run("T12_T13_T24_new_existing_and_identical_receipts", func(t *testing.T) {
		before := len(f.logs.events(t, "stock.receive.committed"))
		first := f.receive(t, item.ID, ids[0], 5, 200)
		second := f.receive(t, item.ID, ids[0], 5, 200)
		if first.Qty != 5 || second.Qty != 10 || first.ID != second.ID {
			t.Fatal(first, second)
		}
		if qty, count := f.quantity(t, item.ID, ids[0]); qty != 10 || count != 1 {
			t.Fatal(qty, count)
		}
		events := f.logs.events(t, "stock.receive.committed")[before:]
		if len(events) != 2 {
			t.Fatal("missing or duplicate mutation logs", events)
		}
		for i, event := range events {
			for field, want := range map[string]any{"request_id": "phase05-test", "item_id": float64(item.ID), "location_id": float64(ids[0]), "stock_id": float64(first.ID), "received_qty": float64(5), "qty_before": float64(i * 5), "qty_after": float64((i + 1) * 5)} {
				if event[field] != want {
					t.Fatalf("%s: %v want %v", field, event[field], want)
				}
			}
			timestamp, ok := event["timestamp"].(string)
			if !ok || !strings.HasSuffix(timestamp, "Z") {
				t.Fatal("not UTC", event)
			}
		}
	})
	t.Run("T14_verified_reference_errors_only", func(t *testing.T) {
		before := len(f.logs.events(t, "stock.receive.committed"))
		for _, tc := range []struct {
			item, location int32
			field          string
		}{{2147483647, ids[0], "item_id"}, {item.ID, 2147483647, "location_id"}, {2147483647, 2147483647, "item_id"}} {
			v, _ := f.request(t, "POST", "/api/v1/stock/receive", fmt.Sprintf(`{"item_id":%d,"location_id":%d,"qty":1}`, tc.item, tc.location), 404)
			if v.Errors[0].Field != tc.field {
				t.Fatal(v.Errors)
			}
		}
		if len(f.logs.events(t, "stock.receive.committed")) != before {
			t.Fatal("failed receipt logged as committed")
		}
		if qty, _ := f.quantity(t, item.ID, ids[0]); qty != 10 {
			t.Fatal("missing references changed stock")
		}
	})
	t.Run("T22_all_actual_locations_and_empty_item", func(t *testing.T) {
		empty := f.create(t, "STOCK-EMPTY", "Empty", "Stock")
		if stocks := f.lookup(t, empty.ID, 200).Stocks; len(stocks) != 0 {
			t.Fatal(stocks)
		}
		f.lookup(t, 2147483647, 404)
		// Existing zero balances are included, but absent locations are not invented.
		if _, err := f.pool.Exec(context.Background(), "INSERT INTO stocks(item_id,location_id,qty) VALUES($1,$2,0)", item.ID, ids[1]); err != nil {
			t.Fatal(err)
		}
		view := f.lookup(t, item.ID, 200)
		if len(view.Stocks) != 2 || view.Stocks[0].LocationID != ids[0] || view.Stocks[1].LocationID != ids[1] || view.Stocks[1].Qty != 0 {
			t.Fatal(view)
		}
		for _, balance := range view.Stocks {
			if balance.Location.ID != balance.LocationID || balance.Location.Code == "" || balance.Location.Zone == "" || balance.Location.Type != "rack" {
				t.Fatal(balance)
			}
		}
	})
	t.Run("T07_deleted_stock_hidden_but_retained", func(t *testing.T) {
		before := f.lookup(t, item.ID, 200)
		f.request(t, "DELETE", fmt.Sprintf("/api/v1/items/%d", item.ID), "", 200)
		f.lookup(t, item.ID, 404)
		f.receive(t, item.ID, ids[0], 5, 404)
		f.request(t, "GET", fmt.Sprintf("/api/v1/items/%d", item.ID), "", 404)
		for _, stock := range before.Stocks {
			var qty int32
			var updated time.Time
			if err := f.pool.QueryRow(context.Background(), "SELECT qty,updated_at FROM stocks WHERE id=$1", stock.ID).Scan(&qty, &updated); err != nil || qty != stock.Qty || !updated.Equal(stock.UpdatedAt) {
				t.Fatal("retained stock changed", err)
			}
		}
	})
}

func TestStockConcurrentReceipts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		initial  int32
		n        int
		qty      int64
		capacity bool
	}{{"T15_existing_pair", 10, 2, 5, false}, {"T16_absent_pair", 0, 2, 5, false}, {"T17_many_receipts", 0, 6, 5, false}, {"T20_capacity_competitors", 2147483646, 2, 1, true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newItemFixture(t)
			item := f.create(t, "CONCURRENT", "Concurrent", "Stock")
			locationID := f.locations(t)[0]
			if tc.initial > 0 {
				f.receive(t, item.ID, locationID, int64(tc.initial), 200)
			}
			blocker := beginTestTx(t, f.pool)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			waitQuery := "FROM locations"
			if tc.initial > 0 {
				if _, err := blocker.Exec(ctx, "SELECT id FROM stocks WHERE item_id=$1 AND location_id=$2 FOR UPDATE", item.ID, locationID); err != nil {
					t.Fatal(err)
				}
				waitQuery = "INSERT INTO stocks"
			} else {
				if _, err := blocker.Exec(ctx, "SELECT id FROM locations WHERE id=$1 FOR UPDATE", locationID); err != nil {
					t.Fatal(err)
				}
			}
			// A held row lock keeps every request on an independently acquired connection.
			before := len(f.logs.events(t, "stock.receive.committed"))
			var requests []<-chan asyncReceipt
			for i := 0; i < tc.n; i++ {
				requests = append(requests, receiveAsync(f, item.ID, locationID, tc.qty))
			}
			waitForLockWaiters(t, f.pool, waitQuery, tc.n)
			if err := blocker.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			var quantities []int
			successes, conflicts := 0, 0
			for _, done := range requests {
				result := awaitReceipt(t, done)
				switch result.status {
				case 200:
					successes++
					quantities = append(quantities, int(result.stock.Qty))
				case 409:
					conflicts++
					if len(result.body.Errors) != 1 || result.body.Errors[0].Field != "qty" {
						t.Fatal(result.body)
					}
				default:
					t.Fatal("unexpected receive", result.status, result.body)
				}
			}
			expectedSuccess := tc.n
			if tc.capacity {
				expectedSuccess = 1
			}
			if successes != expectedSuccess || conflicts != tc.n-expectedSuccess {
				t.Fatal(successes, conflicts)
			}
			expected := tc.initial + int32(int64(successes)*tc.qty)
			if qty, count := f.quantity(t, item.ID, locationID); qty != expected || count != 1 {
				t.Fatalf("qty=%d count=%d want %d", qty, count, expected)
			}
			if len(f.logs.events(t, "stock.receive.committed"))-before != successes {
				t.Fatal("mutation log count differs from successful commits")
			}
			sort.Ints(quantities)
			if tc.name == "T15_existing_pair" && fmt.Sprint(quantities) != "[15 20]" {
				t.Fatal(quantities)
			}
			if tc.name == "T16_absent_pair" && fmt.Sprint(quantities) != "[5 10]" {
				t.Fatal(quantities)
			}
			if tc.capacity {
				f.receive(t, item.ID, locationID, 1, 409)
			}
			t.Logf("%d independent PostgreSQL waiters; %d successful receipts; %d conflicts; final qty=%d; one balance row", tc.n, successes, conflicts, expected)
		})
	}
}

func TestStockSingleTransactionBindingAndLifetime(t *testing.T) {
	f := newItemFixture(t)
	item := f.create(t, "SINGLE-TX", "Single tx", "Stock")
	locationID := f.locations(t)[0]
	pool := singlePool(t, f)
	logs := &logBuffer{}
	server := httptest.NewServer(app.NewApplicationRouter(config.Config{RequestTimeout: time.Second}, slog.New(slog.NewJSONHandler(logs, nil)), pool))
	defer server.Close()
	one := &itemFixture{pool: pool, server: server, logs: logs}
	one.receive(t, item.ID, locationID, 1, 200) // Any guard acquiring a second connection would time out.
	runner := app.NewReceiveRunner(pool)
	var retained stockservice.ReceiveSession
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := runner.Run(ctx, func(session stockservice.ReceiveSession) (stockdomain.Stock, error) {
		retained = session
		if err := session.Item.EnsureActiveForReceive(ctx, item.ID); err != nil {
			return stockdomain.Stock{}, err
		}
		if err := session.Location.EnsureExistsForReceive(ctx, locationID); err != nil {
			return stockdomain.Stock{}, err
		}
		return session.Stock.Increment(ctx, item.ID, locationID, 1)
	})
	if err != nil {
		t.Fatal("guards and writer must share the one available connection", err)
	}
	if err = retained.Item.EnsureActiveForReceive(ctx, item.ID); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatal("item guard escaped transaction lifetime", err)
	}
	if err = retained.Location.EnsureExistsForReceive(ctx, locationID); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatal("location guard escaped transaction lifetime", err)
	}
	if _, err = retained.Stock.Increment(ctx, item.ID, locationID, 1); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatal("writer escaped transaction lifetime", err)
	}
	assertPoolReleased(t, pool)
}

func TestStockReceiveDeleteLockOrder(t *testing.T) {
	t.Run("T18_receive_first_delete_waits", func(t *testing.T) {
		f := newItemFixture(t)
		item := f.create(t, "RECEIVE-FIRST", "Receive first", "Stock")
		locationID := f.locations(t)[0]
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		locked, release := make(chan struct{}), make(chan struct{})
		done := make(chan error, 1)
		go func() {
			_, err := app.NewReceiveRunner(f.pool).Run(ctx, func(session stockservice.ReceiveSession) (stockdomain.Stock, error) {
				if err := session.Item.EnsureActiveForReceive(ctx, item.ID); err != nil {
					return stockdomain.Stock{}, err
				}
				if err := session.Location.EnsureExistsForReceive(ctx, locationID); err != nil {
					return stockdomain.Stock{}, err
				}
				stock, err := session.Stock.Increment(ctx, item.ID, locationID, 5)
				if err != nil {
					return stock, err
				}
				close(locked)
				select {
				case <-release:
					return stock, nil
				case <-ctx.Done():
					return stock, ctx.Err()
				}
			})
			done <- err
		}()
		select {
		case <-locked:
		case <-ctx.Done():
			t.Fatal("receive did not acquire locks")
		}
		deleteDone := make(chan int, 1)
		go func() {
			req, _ := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("%s/api/v1/items/%d", f.server.URL, item.ID), nil)
			response, err := f.server.Client().Do(req)
			if err != nil {
				deleteDone <- 0
				return
			}
			defer response.Body.Close()
			deleteDone <- response.StatusCode
		}()
		waitForLockWaiters(t, f.pool, "UPDATE items SET deleted_at", 1)
		close(release)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		select {
		case status := <-deleteDone:
			if status != 200 {
				t.Fatal(status)
			}
		case <-ctx.Done():
			t.Fatal("delete did not finish")
		}
		if qty, count := f.quantity(t, item.ID, locationID); qty != 5 || count != 1 {
			t.Fatal(qty, count)
		}
		f.lookup(t, item.ID, 404)
		f.receive(t, item.ID, locationID, 1, 404)
	})
	t.Run("T18_delete_first_receive_rechecks", func(t *testing.T) {
		f := newItemFixture(t)
		item := f.create(t, "DELETE-FIRST", "Delete first", "Stock")
		locationID := f.locations(t)[0]
		tx := beginTestTx(t, f.pool)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := tx.Exec(ctx, "UPDATE items SET deleted_at=clock_timestamp() WHERE id=$1 AND deleted_at IS NULL", item.ID); err != nil {
			t.Fatal(err)
		}
		before := len(f.logs.events(t, "stock.receive.committed"))
		done := receiveAsync(f, item.ID, locationID, 5)
		waitForLockWaiters(t, f.pool, "FROM items", 1)
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		result := awaitReceipt(t, done)
		if result.status != 404 || result.body.Errors[0].Field != "item_id" {
			t.Fatal(result)
		}
		if qty, count := f.quantity(t, item.ID, locationID); qty != 0 || count != 0 {
			t.Fatal("received after deleted item", qty, count)
		}
		if len(f.logs.events(t, "stock.receive.committed")) != before {
			t.Fatal("rejected receive logged as committed")
		}
	})
	t.Run("location_key_share_held_until_transaction_ends", func(t *testing.T) {
		f := newItemFixture(t)
		item := f.create(t, "LOCATION-LOCK", "Location lock", "Stock")
		locationID := f.locations(t)[0]
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		locked, release := make(chan struct{}), make(chan struct{})
		done := make(chan error, 1)
		sentinel := errors.New("stop before write")
		go func() {
			_, err := app.NewReceiveRunner(f.pool).Run(ctx, func(session stockservice.ReceiveSession) (stockdomain.Stock, error) {
				if err := session.Item.EnsureActiveForReceive(ctx, item.ID); err != nil {
					return stockdomain.Stock{}, err
				}
				if err := session.Location.EnsureExistsForReceive(ctx, locationID); err != nil {
					return stockdomain.Stock{}, err
				}
				close(locked)
				select {
				case <-release:
					return stockdomain.Stock{}, sentinel
				case <-ctx.Done():
					return stockdomain.Stock{}, ctx.Err()
				}
			})
			done <- err
		}()
		select {
		case <-locked:
		case <-ctx.Done():
			t.Fatal("reference locks not acquired")
		}
		deleted := make(chan error, 1)
		go func() { _, err := f.pool.Exec(ctx, "DELETE FROM locations WHERE id=$1", locationID); deleted <- err }()
		waitForLockWaiters(t, f.pool, "DELETE FROM locations", 1)
		close(release)
		if err := <-done; !errors.Is(err, sentinel) {
			t.Fatal(err)
		}
		if err := <-deleted; err != nil {
			t.Fatal(err)
		}
	})
}

// Inject only after the real app-bound transaction has executed its callback.
type afterWriteRunner struct {
	actual stockservice.ReceiveRunner
	after  func() error
}

func (r afterWriteRunner) Run(ctx context.Context, callback func(stockservice.ReceiveSession) (stockdomain.Stock, error)) (stockdomain.Stock, error) {
	return r.actual.Run(ctx, func(session stockservice.ReceiveSession) (stockdomain.Stock, error) {
		stock, err := callback(session)
		if err != nil {
			return stock, err
		}
		return stock, r.after()
	})
}
func TestStockRollbackCancellationAndSQLFailures(t *testing.T) {
	f := newItemFixture(t)
	item := f.create(t, "ROLLBACK", "Rollback", "Stock")
	locationID := f.locations(t)[0]
	f.receive(t, item.ID, locationID, 10, 200)
	pool := singlePool(t, f)
	t.Run("T19_precommit_rollback_through_HTTP", func(t *testing.T) {
		logs := &logBuffer{}
		logger := slog.New(slog.NewJSONHandler(logs, nil))
		sentinel := errors.New("private injected failure")
		runner := afterWriteRunner{actual: app.NewReceiveRunner(pool), after: func() error { return sentinel }}
		service := stockservice.New(stockpostgres.New(pool), runner, logger, middleware.ID)
		server := httptest.NewServer(app.NewRouter(config.Config{RequestTimeout: time.Second}, logger, pool, nil, service))
		defer server.Close()
		test := &itemFixture{pool: pool, server: server, logs: logs}
		test.receive(t, item.ID, locationID, 5, 500)
		if qty, _ := f.quantity(t, item.ID, locationID); qty != 10 {
			t.Fatal("failed receive persisted", qty)
		}
		if len(logs.events(t, "stock.receive.committed")) != 0 {
			t.Fatal("rollback logged as committed")
		}
		assertPoolReleased(t, pool)
	})
	t.Run("T23_cancel_after_write_rolls_back", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		logs := &logBuffer{}
		runner := afterWriteRunner{actual: app.NewReceiveRunner(pool), after: func() error { cancel(); return nil }}
		service := stockservice.New(stockpostgres.New(pool), runner, slog.New(slog.NewJSONHandler(logs, nil)), middleware.ID)
		if _, err := service.Receive(ctx, stockdomain.ReceiveInput{ItemID: int64(item.ID), LocationID: int64(locationID), Qty: 5}); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if qty, _ := f.quantity(t, item.ID, locationID); qty != 10 {
			t.Fatal("canceled receive persisted", qty)
		}
		if len(logs.events(t, "stock.receive.committed")) != 0 || len(logs.events(t, "stock.receive.commit_unknown")) != 0 {
			t.Fatal("precommit cancellation misclassified")
		}
		assertPoolReleased(t, pool)
	})
	t.Run("SQL_failure_is_500_not_404", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := f.pool.Exec(ctx, `CREATE FUNCTION fail_stock_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'private-database-detail'; END; $$; CREATE TRIGGER fail_stock_test BEFORE INSERT OR UPDATE ON stocks FOR EACH ROW EXECUTE FUNCTION fail_stock_test()`); err != nil {
			t.Fatal(err)
		}
		before := len(f.logs.events(t, "stock.receive.committed"))
		v, _ := f.request(t, "POST", "/api/v1/stock/receive", fmt.Sprintf(`{"item_id":%d,"location_id":%d,"qty":5}`, item.ID, locationID), 500)
		raw, _ := json.Marshal(v)
		if strings.Contains(string(raw), "private-database-detail") {
			t.Fatal("SQL leaked")
		}
		if qty, _ := f.quantity(t, item.ID, locationID); qty != 10 {
			t.Fatal(qty)
		}
		if len(f.logs.events(t, "stock.receive.committed")) != before {
			t.Fatal("SQL failure logged committed")
		}
		if _, err := f.pool.Exec(ctx, "DROP TRIGGER fail_stock_test ON stocks; DROP FUNCTION fail_stock_test()"); err != nil {
			t.Fatal(err)
		}
		// A guard's technical failure must retain technical identity, not become 404.
		if _, err := f.pool.Exec(ctx, "ALTER TABLE locations RENAME TO unavailable_locations"); err != nil {
			t.Fatal(err)
		}
		f.receive(t, item.ID, locationID, 5, 500)
		if _, err := f.pool.Exec(ctx, "ALTER TABLE unavailable_locations RENAME TO locations"); err != nil {
			t.Fatal(err)
		}
	})
}

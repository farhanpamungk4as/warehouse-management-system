package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"warehouse/backend/internal/app"
	inventory "warehouse/backend/internal/modules/inventory/domain"
	location "warehouse/backend/internal/modules/location/domain"
	locationpg "warehouse/backend/internal/modules/location/repository/postgres"
	locationsvc "warehouse/backend/internal/modules/location/service"
	stock "warehouse/backend/internal/modules/stock/domain"
	stocksvc "warehouse/backend/internal/modules/stock/service"
	"warehouse/backend/internal/platform/cache"
)

func transferBody(item, from, to int32, qty int64) string {
	return fmt.Sprintf(`{"item_id":%d,"from_location_id":%d,"to_location_id":%d,"qty":%d}`, item, from, to, qty)
}
func (f *itemFixture) transfer(t *testing.T, item, from, to int32, qty int64, status int) stock.Transfer {
	t.Helper()
	v, _ := f.request(t, "POST", "/api/v1/stock/transfer", transferBody(item, from, to, qty), status)
	var result stock.Transfer
	if status == 200 {
		if err := json.Unmarshal(v.Data, &result); err != nil {
			t.Fatal(err)
		}
	}
	return result
}
func transferAsync(f *itemFixture, item, from, to int32, qty int64) <-chan asyncReceipt {
	done := make(chan asyncReceipt, 1)
	go func() {
		req, _ := http.NewRequest("POST", f.server.URL+"/api/v1/stock/transfer", strings.NewReader(transferBody(item, from, to, qty)))
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 6 * time.Second}
		res, err := client.Do(req)
		if err != nil {
			done <- asyncReceipt{err: err}
			return
		}
		defer res.Body.Close()
		var v itemEnvelope
		err = json.NewDecoder(res.Body).Decode(&v)
		done <- asyncReceipt{status: res.StatusCode, body: v, err: err}
	}()
	return done
}
func TestEnterpriseReadsAndTransferRules(t *testing.T) {
	f := newItemFixture(t)
	locs := f.locations(t)
	a, b := locs[0], locs[1]
	summary := func() inventory.Summary {
		t.Helper()
		v, _ := f.request(t, "GET", "/api/v1/dashboard/summary", "", 200)
		var out inventory.Summary
		if err := json.Unmarshal(v.Data, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if s := summary(); s.ActiveItems != 0 || s.AvailableQuantity != 0 || s.Locations != int64(len(locs)) || s.OccupiedStockPairs != 0 {
		t.Fatal(s)
	}
	v, _ := f.request(t, "GET", "/api/v1/locations", "", 200)
	var locations []location.Location
	if err := json.Unmarshal(v.Data, &locations); err != nil || len(locations) != len(locs) {
		t.Fatal(string(v.Data), err)
	}
	item := f.create(t, "X%_TRANSFER", "Transfer case", "Operations")
	f.receive(t, item.ID, a, 10, 200)
	result := f.transfer(t, item.ID, a, b, 4, 200)
	if result.Source.Qty != 6 || result.Destination.Qty != 4 || result.Qty != 4 {
		t.Fatal(result)
	}
	if s := summary(); s.ActiveItems != 1 || s.AvailableQuantity != 10 || s.OccupiedStockPairs != 2 {
		t.Fatal(s)
	}
	v, _ = f.request(t, "GET", fmt.Sprintf("/api/v1/inventory/overview?q=%%25_&category=Operations&location_id=%d&limit=1", b), "", 200)
	var rows []inventory.Row
	_ = json.Unmarshal(v.Data, &rows)
	if v.Meta.Total != 1 || len(rows) != 1 || rows[0].Qty != 4 || rows[0].Location.ID != b {
		t.Fatal(string(v.Data), v.Meta)
	}
	f.transfer(t, item.ID, a, b, 7, 409)
	f.transfer(t, item.ID, a, a, 1, 400)
	f.transfer(t, item.ID, a, 2147483647, 1, 404)
	f.transfer(t, 2147483647, a, b, 1, 404)
	for _, body := range []string{`{}`, transferBody(item.ID, a, b, 0), transferBody(item.ID, a, b, 2147483648), strings.Replace(transferBody(item.ID, a, b, 1), "{", `{"qty":2,`, 1), `{"item_id":1,"from_location_id":1,"to_location_id":2,"qty":null}`} {
		f.request(t, "POST", "/api/v1/stock/transfer", body, 400)
	}
	f.receive(t, item.ID, b, stock.MaxQuantity-4, 200)
	f.transfer(t, item.ID, a, b, 1, 409)
	if qty, _ := f.quantity(t, item.ID, a); qty != 6 {
		t.Fatal("overflow did not rollback source", qty)
	}
	f.request(t, "DELETE", fmt.Sprintf("/api/v1/items/%d", item.ID), "", 200)
	f.transfer(t, item.ID, a, b, 1, 404)
	if s := summary(); s.ActiveItems != 0 || s.AvailableQuantity != 0 || s.OccupiedStockPairs != 0 {
		t.Fatal("deleted metrics", s)
	}
	v, _ = f.request(t, "GET", "/api/v1/inventory/overview", "", 200)
	if v.Meta.Total != 0 || string(v.Data) != "[]" {
		t.Fatal("deleted inventory", string(v.Data))
	}
	if len(f.logs.events(t, "stock.transfer.committed")) != 1 {
		t.Fatal("incorrect confirmed log count")
	}
	for _, query := range []string{"?page=0", "?limit=101", "?location_id=0", "?location_id=2147483648", "?q=a&q=b", "?unknown=x", "?page=999999999999999999"} {
		f.request(t, "GET", "/api/v1/inventory/overview"+query, "", 400)
	}
}
func TestEnterpriseConcurrentTransfers(t *testing.T) {
	f := newItemFixture(t)
	ids := f.locations(t)
	a, b := ids[0], ids[1]
	item := f.create(t, "CONCURRENT-TRANSFER", "Concurrent transfer", "Stock")
	f.receive(t, item.ID, a, 10, 200)
	blocker := beginTestTx(t, f.pool)
	if _, err := blocker.Exec(context.Background(), "SELECT id FROM locations WHERE id=$1 FOR UPDATE", a); err != nil {
		t.Fatal(err)
	}
	first := transferAsync(f, item.ID, a, b, 7)
	second := transferAsync(f, item.ID, a, b, 7)
	waitForLockWaiters(t, f.pool, "FROM locations", 2)
	if err := blocker.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	statuses := map[int]int{}
	statuses[awaitReceipt(t, first).status]++
	statuses[awaitReceipt(t, second).status]++
	if statuses[200] != 1 || statuses[409] != 1 {
		t.Fatal(statuses)
	}
	// Opposing transfers contend on the same lowest location, then serialize.
	blocker = beginTestTx(t, f.pool)
	_, err := blocker.Exec(context.Background(), "SELECT id FROM locations WHERE id=$1 FOR UPDATE", a)
	if err != nil {
		t.Fatal(err)
	}
	first = transferAsync(f, item.ID, a, b, 2)
	second = transferAsync(f, item.ID, b, a, 2)
	waitForLockWaiters(t, f.pool, "FROM locations", 2)
	_ = blocker.Commit(context.Background())
	if x, y := awaitReceipt(t, first), awaitReceipt(t, second); x.status != 200 || y.status != 200 {
		t.Fatal(x, y)
	}
	// Receive waits on a location before taking stock locks; transfers cannot form a cycle.
	blocker = beginTestTx(t, f.pool)
	_, err = blocker.Exec(context.Background(), "SELECT id FROM locations WHERE id=$1 FOR UPDATE", a)
	if err != nil {
		t.Fatal(err)
	}
	first = transferAsync(f, item.ID, a, b, 1)
	received := receiveAsync(f, item.ID, a, 5)
	waitForLockWaiters(t, f.pool, "FROM locations", 2)
	_ = blocker.Commit(context.Background())
	if x, y := awaitReceipt(t, first), awaitReceipt(t, received); x.status != 200 || y.status != 200 {
		t.Fatal(x, y)
	}
	x, _ := f.quantity(t, item.ID, a)
	y, _ := f.quantity(t, item.ID, b)
	if int64(x)+int64(y) != 15 || x < 0 || y < 0 {
		t.Fatal("conservation", x, y)
	}
}
func TestEnterpriseDeleteAndTransferOrdering(t *testing.T) {
	f := newItemFixture(t)
	ids := f.locations(t)
	a, b := ids[0], ids[1]
	item := f.create(t, "DELETE-TRANSFER", "Delete transfer", "Stock")
	f.receive(t, item.ID, a, 8, 200)
	blocker := beginTestTx(t, f.pool)
	_, err := blocker.Exec(context.Background(), "SELECT id FROM locations WHERE id=$1 FOR UPDATE", a)
	if err != nil {
		t.Fatal(err)
	}
	moved := transferAsync(f, item.ID, a, b, 3)
	waitForLockWaiters(t, f.pool, "FROM locations", 1)
	deleted := make(chan int, 1)
	go func() {
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/items/%d", f.server.URL, item.ID), nil)
		res, err := f.server.Client().Do(req)
		if err != nil {
			deleted <- 0
			return
		}
		defer res.Body.Close()
		deleted <- res.StatusCode
	}()
	waitForLockWaiters(t, f.pool, "UPDATE items", 1)
	_ = blocker.Commit(context.Background())
	if result := awaitReceipt(t, moved); result.status != 200 {
		t.Fatal(result)
	}
	select {
	case status := <-deleted:
		if status != 200 {
			t.Fatal(status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("delete blocked")
	}
	f.transfer(t, item.ID, a, b, 1, 404)
	x, _ := f.quantity(t, item.ID, a)
	y, _ := f.quantity(t, item.ID, b)
	if x != 5 || y != 3 {
		t.Fatal(x, y)
	}
	// Delete first: transfer waits on active item guard and then rejects after commit.
	item = f.create(t, "DELETE-FIRST-TRANSFER", "Delete first", "Stock")
	f.receive(t, item.ID, a, 8, 200)
	blocker = beginTestTx(t, f.pool)
	_, err = blocker.Exec(context.Background(), "UPDATE items SET deleted_at=clock_timestamp() WHERE id=$1", item.ID)
	if err != nil {
		t.Fatal(err)
	}
	moved = transferAsync(f, item.ID, a, b, 1)
	waitForLockWaiters(t, f.pool, "FROM items", 1)
	_ = blocker.Commit(context.Background())
	if result := awaitReceipt(t, moved); result.status != 404 {
		t.Fatal(result)
	}
}

type failingTransferRunner struct {
	actual stocksvc.TransferRunner
	after  func(context.Context) error
}

func (r failingTransferRunner) RunTransfer(ctx context.Context, fn func(stocksvc.TransferSession) (stock.Transfer, error)) (stock.Transfer, error) {
	return r.actual.RunTransfer(ctx, func(session stocksvc.TransferSession) (stock.Transfer, error) {
		v, err := fn(session)
		if err != nil {
			return v, err
		}
		return v, r.after(ctx)
	})
}
func TestEnterpriseTransferRollbackAndCancellation(t *testing.T) {
	f := newItemFixture(t)
	ids := f.locations(t)
	item := f.create(t, "TRANSFER-ROLLBACK", "Transfer rollback", "Stock")
	f.receive(t, item.ID, ids[0], 10, 200)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	in := stock.TransferInput{ItemID: int64(item.ID), FromLocationID: int64(ids[0]), ToLocationID: int64(ids[1]), Qty: 4}
	injected := errors.New("injected after both writes")
	service := stocksvc.NewTransfer(failingTransferRunner{app.NewTransferRunner(f.pool), func(context.Context) error { return injected }}, logger, func(context.Context) string { return "test" })
	if _, err := service.Transfer(context.Background(), in); !errors.Is(err, injected) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	service = stocksvc.NewTransfer(failingTransferRunner{app.NewTransferRunner(f.pool), func(context.Context) error { cancel(); return nil }}, logger, func(context.Context) string { return "test" })
	if _, err := service.Transfer(ctx, in); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if x, _ := f.quantity(t, item.ID, ids[0]); x != 10 {
		t.Fatal(x)
	}
	if y, n := f.quantity(t, item.ID, ids[1]); y != 0 || n != 0 {
		t.Fatal("partial transfer", y, n)
	}
	assertPoolReleased(t, f.pool)
}

type countedLocations struct {
	locationsvc.Store
	calls int
}

func (r *countedLocations) List(ctx context.Context) ([]location.Location, error) {
	r.calls++
	return r.Store.List(ctx)
}
func TestEnterpriseRealRedisReadThroughTTLAndOutage(t *testing.T) {
	raw := os.Getenv("TEST_REDIS_URL")
	if raw == "" {
		t.Skip("set TEST_REDIS_URL for real Redis integration")
	}
	f := newItemFixture(t)
	redis, err := cache.Open(raw)
	if err != nil {
		t.Fatal(err)
	}
	defer redis.Close()
	// Dedicated Redis instance/database only; overwrite this namespaced test key with short TTL.
	ctx := context.Background()
	_ = redis.Set(ctx, locationsvc.CacheKey, []byte("invalid"), time.Millisecond)
	repo := &countedLocations{Store: locationpg.New(f.pool)}
	s := locationsvc.New(repo, redis, 150*time.Millisecond)
	first, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.List(ctx)
	if err != nil || len(second) != len(first) || repo.calls != 1 {
		t.Fatal("cache hit missed", repo.calls, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, err = redis.Get(ctx, locationsvc.CacheKey)
		if err != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("TTL did not expire")
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, err = s.List(ctx)
	if err != nil || repo.calls != 2 {
		t.Fatal("TTL miss", repo.calls, err)
	}
	unavailable, err := cache.Open("redis://127.0.0.1:1/0")
	if err != nil {
		t.Fatal(err)
	}
	defer unavailable.Close()
	s = locationsvc.New(repo, unavailable, time.Second)
	started := time.Now()
	rows, err := s.List(ctx)
	if err != nil || len(rows) != len(first) || time.Since(started) > time.Second {
		t.Fatal("outage fallback", err)
	}
	f.create(t, "REDIS-OUTAGE", "Core functionality", "Stock")
}

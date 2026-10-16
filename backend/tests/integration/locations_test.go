package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	location "warehouse/backend/internal/modules/location/domain"
	locationpg "warehouse/backend/internal/modules/location/repository/postgres"
	locationsvc "warehouse/backend/internal/modules/location/service"
	"warehouse/backend/internal/platform/cache"
)

func createLocation(t *testing.T, f *itemFixture, code string) location.Location {
	t.Helper()
	v, h := f.request(t, "POST", "/api/v1/locations", fmt.Sprintf(`{"code":%q,"zone":"C","type":"rack"}`, code), 201)
	var out location.Location
	if err := json.Unmarshal(v.Data, &out); err != nil {
		t.Fatal(err)
	}
	if h.Get("Location") != fmt.Sprintf("/api/v1/locations/%d", out.ID) {
		t.Fatal("missing resource location header", h)
	}
	return out
}
func TestLocationCRUDValidationAndStockProtection(t *testing.T) {
	f := newItemFixture(t)
	for _, body := range []string{
		`{}`, `{"code":"A","zone":"A","type":null}`, `{"code":"A","zone":"A","type":"rack","id":1}`,
		`{"code":"A","code":"B","zone":"A","type":"rack"}`, `{"Code":"A","zone":"A","type":"rack"}`,
		`{"code":"A","zone":"A","type":"rack"} {}`, `{"code":"\u0085","zone":"A","type":"rack"}`,
		`{"code":"A\nB","zone":"A","type":"rack"}`, fmt.Sprintf(`{"code":%q,"zone":"A","type":"rack"}`, strings.Repeat("界", 65)),
	} {
		f.request(t, "POST", "/api/v1/locations", body, 400)
	}
	f.request(t, "POST", "/api/v1/locations?x=1", `{"code":"A","zone":"A","type":"rack"}`, 400)
	v := createLocation(t, f, "  RACK-C1  ")
	if v.Code != "RACK-C1" {
		t.Fatal("not normalized", v)
	}
	path := fmt.Sprintf("/api/v1/locations/%d", v.ID)
	f.request(t, "GET", path, "", 200)
	for _, id := range []string{"0", "-1", "2147483648", "1.2", "abc"} {
		f.request(t, "GET", "/api/v1/locations/"+id, "", 400)
	}
	f.request(t, "GET", path+"?x=1", "", 400)
	f.request(t, "POST", "/api/v1/locations", `{"code":"RACK-C1","zone":"C","type":"rack"}`, 409)
	other := createLocation(t, f, "RACK-D1")
	f.request(t, "PUT", path, `{"code":"RACK-D1","zone":"Z","type":"bin"}`, 409)
	f.request(t, "PUT", path, `{"code":"RACK-C2","zone":"Z","type":"bin"}`, 200)
	body, _ := f.request(t, "GET", path, "", 200)
	var updated location.Location
	_ = json.Unmarshal(body.Data, &updated)
	if updated.Code != "RACK-C2" || updated.Zone != "Z" || updated.Type != "bin" {
		t.Fatal(updated)
	}
	item := f.create(t, "LOCATION-ITEM", "Location stock protection", "Demo")
	f.receive(t, item.ID, v.ID, 3, 200)
	f.request(t, "DELETE", path, "", 409)
	if _, err := f.pool.Exec(context.Background(), "UPDATE stocks SET qty=0 WHERE location_id=$1", v.ID); err != nil {
		t.Fatal(err)
	}
	f.request(t, "DELETE", path, "", 409)
	otherPath := fmt.Sprintf("/api/v1/locations/%d", other.ID)
	f.request(t, "DELETE", otherPath, "", 200)
	f.request(t, "DELETE", otherPath, "", 404)
	f.request(t, "GET", otherPath, "", 404)
	f.request(t, "PUT", otherPath, `{"code":"GONE","zone":"A","type":"rack"}`, 404)
	_, h := f.request(t, "PATCH", path, `{}`, 405)
	if h.Get("Allow") != "GET, PUT, DELETE" {
		t.Fatal(h)
	}
}

func TestLocationDeleteSerializesAgainstReceive(t *testing.T) {
	f := newItemFixture(t)
	v := createLocation(t, f, "RACK-CONCURRENT")
	item := f.create(t, "LOC-CONCURRENT", "Concurrent location receipt", "Demo")
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT id FROM locations WHERE id=$1 FOR KEY SHARE", v.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() {
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/locations/%d", f.server.URL, v.ID), nil)
		res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if err != nil {
			done <- 0
			return
		}
		res.Body.Close()
		done <- res.StatusCode
	}()
	select {
	case status := <-done:
		t.Fatalf("delete did not wait for receiving location guard: %d", status)
	case <-time.After(100 * time.Millisecond):
	}
	if _, err = tx.Exec(ctx, "INSERT INTO stocks(item_id,location_id,qty) VALUES($1,$2,1)", item.ID, v.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if status := <-done; status != 409 {
		t.Fatal("delete should preserve committed receipt", status)
	}
	f.request(t, "GET", fmt.Sprintf("/api/v1/stock/%d", item.ID), "", 200)
	// Reverse order: a committed deletion makes the waiting receipt return 404.
	gone := createLocation(t, f, "RACK-DELETED-FIRST")
	deletion, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer deletion.Rollback(ctx)
	if _, err = deletion.Exec(ctx, "DELETE FROM locations WHERE id=$1", gone.ID); err != nil {
		t.Fatal(err)
	}
	receipt := receiveAsync(f, item.ID, gone.ID, 1)
	select {
	case result := <-receipt:
		t.Fatalf("receipt did not wait for deleting location: %+v", result)
	case <-time.After(100 * time.Millisecond):
	}
	if err = deletion.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if result := <-receipt; result.status != 404 || result.err != nil {
		t.Fatalf("receipt should see deleted location: %+v", result)
	}
}

type failedLocationPurge struct{ *cache.Redis }

func (f failedLocationPurge) Delete(context.Context, string) error {
	return errors.New("cache purge unavailable")
}

func TestLocationCRUDInvalidatesRealRedis(t *testing.T) {
	raw := os.Getenv("TEST_REDIS_URL")
	if raw == "" {
		t.Skip("set TEST_REDIS_URL")
	}
	f := newItemFixture(t)
	redis, err := cache.Open(raw)
	if err != nil {
		t.Fatal(err)
	}
	defer redis.Close()
	ctx := context.Background()
	if err = redis.Delete(ctx, locationsvc.CacheKey); err != nil {
		t.Fatal(err)
	}
	s := locationsvc.New(locationpg.New(f.pool), redis, time.Minute)
	original, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Create(ctx, location.Write{Code: "CACHE-NEW", Zone: "N", Type: "bin"})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.List(ctx)
	if err != nil || len(rows) != len(original)+1 {
		t.Fatal("create cache stale", rows, err)
	}
	_, err = s.Update(ctx, v.ID, location.Write{Code: "CACHE-UPDATED", Zone: "U", Type: "rack"})
	if err != nil {
		t.Fatal(err)
	}
	rows, err = s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.ID == v.ID {
			found = row.Code == "CACHE-UPDATED"
		}
	}
	if !found {
		t.Fatal("update cache stale", rows)
	}
	if _, err = s.Delete(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	rows, err = s.List(ctx)
	if err != nil || len(rows) != len(original) {
		t.Fatal("delete cache stale", rows, err)
	}
	// Even if DEL fails while GET still works, a committed write must not make
	// this process serve the old cached snapshot or report a false write failure.
	s = locationsvc.New(locationpg.New(f.pool), failedLocationPurge{redis}, time.Minute)
	v, err = s.Create(ctx, location.Write{Code: "CACHE-PURGE-FAIL", Zone: "F", Type: "bin"})
	if err != nil {
		t.Fatal(err)
	}
	rows, err = s.List(ctx)
	if err != nil || len(rows) != len(original)+1 {
		t.Fatal("failed purge served stale data", rows, err)
	}
	if _, err = s.Delete(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	rows, err = s.List(ctx)
	if err != nil || len(rows) != len(original) {
		t.Fatal("delete with failed purge stale", rows, err)
	}
}

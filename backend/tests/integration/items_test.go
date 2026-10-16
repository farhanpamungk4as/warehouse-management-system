package integration

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"warehouse/backend/internal/app"
	"warehouse/backend/internal/modules/item/domain"
	"warehouse/backend/internal/platform/config"
	"warehouse/backend/internal/platform/httpx"
)

type itemFixture struct {
	pool   *pgxpool.Pool
	server *httptest.Server
	logs   *logBuffer
}

func newItemFixture(t *testing.T) *itemFixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL and GOOSE_BIN for real PostgreSQL Item acceptance tests")
	}
	goose := os.Getenv("GOOSE_BIN")
	if goose == "" {
		var err error
		goose, err = exec.LookPath("goose")
		if err != nil {
			t.Fatal("GOOSE_BIN is required")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = admin.Close(ctx)
	})
	name := "warehouse_test_item_" + strings.ToLower(rand.Text())
	identifier := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.Exec(ctx, "DROP DATABASE "+identifier+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	})
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("TEST_DATABASE_URL must be PostgreSQL URL")
	}
	u.Path = "/" + name
	q := u.Query()
	q.Del("dbname")
	q.Del("options")
	u.RawQuery = q.Encode()
	cmd := exec.CommandContext(ctx, goose, "-dir", "../../migrations", "postgres", u.String(), "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("migration: %v %s", err, output)
	}
	cfg, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 8
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	logs := &logBuffer{}
	router := app.NewApplicationRouter(config.Config{RequestTimeout: 3 * time.Second}, slog.New(slog.NewJSONHandler(logs, nil)), pool)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return &itemFixture{pool: pool, server: server, logs: logs}
}

type itemEnvelope struct {
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Data    json.RawMessage    `json:"data"`
	Errors  []httpx.FieldError `json:"errors"`
	Meta    struct {
		Page, Limit, Total int64
		Categories         []string
	} `json:"meta"`
}

func (f *itemFixture) request(t *testing.T, method, path, body string, status int) (itemEnvelope, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, f.server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Request-ID", "phase05-test")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var v itemEnvelope
	if err = json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("invalid JSON: %s", raw)
	}
	if response.StatusCode != status || v.Success != (status < 400) || v.Message == "" {
		t.Fatalf("%s %s: want %d got %d %s", method, path, status, response.StatusCode, raw)
	}
	if status >= 400 && (len(v.Errors) == 0 || strings.Contains(string(raw), "SQLSTATE") || strings.Contains(string(raw), "warehouse_test")) {
		t.Fatalf("unsafe failure: %s", raw)
	}
	if response.Header.Get("X-Request-ID") == "" {
		t.Fatal("missing request ID")
	}
	return v, response.Header
}
func itemFrom(t *testing.T, v itemEnvelope) domain.Item {
	t.Helper()
	var item domain.Item
	if err := json.Unmarshal(v.Data, &item); err != nil {
		t.Fatal(err)
	}
	return item
}
func listFrom(t *testing.T, v itemEnvelope) []domain.Item {
	t.Helper()
	var items []domain.Item
	if err := json.Unmarshal(v.Data, &items); err != nil {
		t.Fatal(err)
	}
	if items == nil {
		t.Fatal("null item list")
	}
	return items
}
func writeBody(sku, name, category, unit string) string {
	b, _ := json.Marshal(map[string]string{"sku": sku, "name": name, "category": category, "unit": unit})
	return string(b)
}
func (f *itemFixture) create(t *testing.T, sku, name, category string) domain.Item {
	t.Helper()
	v, h := f.request(t, "POST", "/api/v1/items", writeBody(sku, name, category, "pcs"), 201)
	item := itemFrom(t, v)
	if h.Get("Location") != "/api/v1/items/"+strconv.Itoa(int(item.ID)) || item.ID < 1 || item.CreatedAt.IsZero() {
		t.Fatalf("create headers/item: %v %+v", h, item)
	}
	return item
}

func TestItemCRUDAndConcurrency(t *testing.T) {
	f := newItemFixture(t)
	ctx := context.Background()
	first := f.create(t, " abc ", " Box ", " Tools ")
	path := "/api/v1/items/" + strconv.Itoa(int(first.ID))
	if first.SKU != "abc" || first.Name != "Box" || first.Category != "Tools" {
		t.Fatal(first)
	}
	var count int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM items").Scan(&count); err != nil || count != 1 {
		t.Fatalf("create persistence %d %v", count, err)
	}
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM stocks").Scan(&count); err != nil || count != 0 {
		t.Fatal("create produced stocks")
	}
	t.Run("T01_T03_duplicate", func(t *testing.T) {
		f.request(t, "POST", "/api/v1/items", writeBody("abc", "Other", "Tools", "pcs"), 409)
		other := f.create(t, "ABC", "Other", "Other")
		otherPath := "/api/v1/items/" + strconv.Itoa(int(other.ID))
		v, _ := f.request(t, "PUT", otherPath, writeBody("abc", "Changed", "Changed", "kg"), 409)
		if v.Message != "SKU already exists" || v.Errors[0] != (httpx.FieldError{Field: "sku", Reason: "Duplicate entry"}) {
			t.Fatal(v)
		}
		v, _ = f.request(t, "GET", otherPath, "", 200)
		if itemFrom(t, v) != other {
			t.Fatal("duplicate update mutated original")
		}
		v, _ = f.request(t, "GET", path, "", 200)
		if itemFrom(t, v) != first {
			t.Fatal("duplicate create mutated original")
		}
	})
	t.Run("T04_concurrent_duplicate", func(t *testing.T) {
		// Two independent HTTP clients, with two pool connections held to ensure capacity.
		c1, err := f.pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		c2, err := f.pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if c1.Conn().PgConn().PID() == c2.Conn().PgConn().PID() {
			t.Fatal("same connection")
		}
		c1.Release()
		c2.Release()
		start := make(chan struct{})
		statuses := make(chan int, 2)
		failures := make(chan error, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				req, _ := http.NewRequest("POST", f.server.URL+"/api/v1/items", strings.NewReader(writeBody("RACE-SKU", "Race", "Race", "pcs")))
				req.Header.Set("Content-Type", "application/json")
				client := &http.Client{Timeout: 5 * time.Second}
				res, err := client.Do(req)
				if err != nil {
					failures <- err
					return
				}
				defer res.Body.Close()
				var v itemEnvelope
				if err = json.NewDecoder(res.Body).Decode(&v); err != nil {
					failures <- err
					return
				}
				if v.Success != (res.StatusCode == 201) || (res.StatusCode == 409 && (len(v.Errors) != 1 || v.Errors[0].Field != "sku")) {
					failures <- fmt.Errorf("invalid concurrent envelope: %+v", v)
					return
				}
				statuses <- res.StatusCode
			}()
		}
		close(start)
		wg.Wait()
		close(statuses)
		close(failures)
		for err := range failures {
			t.Fatal(err)
		}
		var got []int
		for status := range statuses {
			got = append(got, status)
		}
		sort.Ints(got)
		if fmt.Sprint(got) != "[201 409]" {
			t.Fatalf("concurrent statuses: %v", got)
		}
		if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM items WHERE sku='RACE-SKU'").Scan(&count); err != nil || count != 1 {
			t.Fatal("concurrent duplicate persisted")
		}
	})
	t.Run("T05_T06_detail_full_update", func(t *testing.T) {
		v, _ := f.request(t, "GET", path, "", 200)
		if itemFrom(t, v) != first {
			t.Fatal("detail mismatch")
		}
		f.request(t, "GET", "/api/v1/items/2147483647", "", 404)
		f.request(t, "PUT", path, `{"sku":"partial"}`, 400)
		v, _ = f.request(t, "PUT", path, writeBody("abc", "Large Box", "Packaging", "kg"), 200)
		updated := itemFrom(t, v)
		if updated.ID != first.ID || !updated.CreatedAt.Equal(first.CreatedAt) || updated.SKU != "abc" || updated.Name != "Large Box" || updated.Category != "Packaging" || updated.Unit != "kg" {
			t.Fatal(updated)
		}
		v, _ = f.request(t, "PUT", path, writeBody("new-abc", "Changed again", "Changed", "box"), 200)
		first = itemFrom(t, v)
		v, _ = f.request(t, "GET", path, "", 200)
		if itemFrom(t, v) != first {
			t.Fatal("update not persisted")
		}
		f.request(t, "PUT", "/api/v1/items/2147483647", writeBody("none", "none", "none", "pcs"), 404)
	})
	t.Run("T07_soft_delete_retains_stock_and_SKU", func(t *testing.T) {
		var stockID int32
		var stockTime time.Time
		if err := f.pool.QueryRow(ctx, "INSERT INTO stocks(item_id,location_id,qty) SELECT $1,id,7 FROM locations WHERE code='RACK-A1' RETURNING id,updated_at", first.ID).Scan(&stockID, &stockTime); err != nil {
			t.Fatal(err)
		}
		v, _ := f.request(t, "DELETE", path, "", 200)
		var deleted domain.Deleted
		if err := json.Unmarshal(v.Data, &deleted); err != nil || deleted.ID != first.ID || deleted.DeletedAt.IsZero() {
			t.Fatal(string(v.Data))
		}
		var physical domain.Item
		var deletedTime time.Time
		if err := f.pool.QueryRow(ctx, "SELECT id,sku,name,category,unit,created_at,deleted_at FROM items WHERE id=$1", first.ID).Scan(&physical.ID, &physical.SKU, &physical.Name, &physical.Category, &physical.Unit, &physical.CreatedAt, &deletedTime); err != nil || physical.ID != first.ID || physical.SKU != first.SKU || physical.Name != first.Name || physical.Category != first.Category || physical.Unit != first.Unit || !physical.CreatedAt.Equal(first.CreatedAt) || !deletedTime.Equal(deleted.DeletedAt) {
			t.Fatalf("physical item changed: %+v %v", physical, err)
		}
		var qty int
		var afterTime time.Time
		if err := f.pool.QueryRow(ctx, "SELECT qty,updated_at FROM stocks WHERE id=$1", stockID).Scan(&qty, &afterTime); err != nil || qty != 7 || !afterTime.Equal(stockTime) {
			t.Fatal("stock changed")
		}
		f.request(t, "GET", path, "", 404)
		f.request(t, "GET", fmt.Sprintf("/api/v1/stock/%d", first.ID), "", 404)
		f.request(t, "PUT", path, writeBody(first.SKU, "No revive", "Tools", "pcs"), 404)
		f.request(t, "DELETE", path, "", 404)
		f.request(t, "POST", "/api/v1/items", writeBody(first.SKU, "No reuse", "Tools", "pcs"), 409)
		v, _ = f.request(t, "GET", "/api/v1/items", "", 200)
		for _, item := range listFrom(t, v) {
			if item.ID == first.ID {
				t.Fatal("deleted item in list")
			}
		}
		for _, category := range v.Meta.Categories {
			if category == "Changed" {
				t.Fatal("deleted category in options")
			}
		}
	})
	t.Run("technical_failure_envelopes", func(t *testing.T) {
		locker, err := f.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = locker.Exec(ctx, "LOCK TABLE items IN ACCESS EXCLUSIVE MODE"); err != nil {
			t.Fatal(err)
		}
		f.request(t, "GET", "/api/v1/items", "", 503)
		if err = locker.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err = f.pool.Exec(ctx, `CREATE FUNCTION fail_item_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'private-database-detail'; END; $$; CREATE TRIGGER fail_item_test BEFORE INSERT ON items FOR EACH ROW EXECUTE FUNCTION fail_item_test()`); err != nil {
			t.Fatal(err)
		}
		v, _ := f.request(t, "POST", "/api/v1/items", writeBody("FAIL", "Failure", "Tools", "pcs"), 500)
		raw, _ := json.Marshal(v)
		if strings.Contains(string(raw), "private-database-detail") {
			t.Fatal("SQL leaked")
		}
	})
}

func TestItemListFilteringPaginationAndSearch(t *testing.T) {
	f := newItemFixture(t)
	v, _ := f.request(t, "GET", "/api/v1/items", "", 200)
	if len(listFrom(t, v)) != 0 || v.Meta.Total != 0 || v.Meta.Page != 1 || v.Meta.Limit != 10 || v.Meta.Categories == nil || len(v.Meta.Categories) != 0 {
		t.Fatalf("empty defaults: %+v", v)
	}
	var all []domain.Item
	for i := 1; i <= 25; i++ {
		category := "Tools"
		if i%2 == 0 {
			category = "Packaging"
		}
		name := fmt.Sprintf("Box %02d", i)
		sku := fmt.Sprintf("SKU-%02d", i)
		if i == 25 {
			name = "Off-page Needle"
			sku = "Special%_\\End"
			category = "Z-last"
		}
		all = append(all, f.create(t, sku, name, category))
	}
	deleted := f.create(t, "DELETED", "Needle", "Deleted-only")
	f.request(t, "DELETE", fmt.Sprintf("/api/v1/items/%d", deleted.ID), "", 200)
	t.Run("T08_ordered_disjoint_pages", func(t *testing.T) {
		var ids []int32
		for page := 1; page <= 3; page++ {
			v, _ := f.request(t, "GET", fmt.Sprintf("/api/v1/items?page=%d&limit=10", page), "", 200)
			items := listFrom(t, v)
			if v.Meta.Total != 25 || v.Meta.Page != int64(page) || v.Meta.Limit != 10 {
				t.Fatal(v.Meta)
			}
			for _, item := range items {
				ids = append(ids, item.ID)
			}
			if fmt.Sprint(v.Meta.Categories) != "[Packaging Tools Z-last]" {
				t.Fatal(v.Meta.Categories)
			}
		}
		if len(ids) != 25 {
			t.Fatal(ids)
		}
		for i, id := range ids {
			if id != all[i].ID {
				t.Fatal("unstable/overlapping ordering", ids)
			}
		}
		v, _ := f.request(t, "GET", "/api/v1/items?category=Packaging&page=2&limit=5", "", 200)
		items := listFrom(t, v)
		if v.Meta.Total != 12 || len(items) != 5 || items[0].ID != all[11].ID {
			t.Fatalf("filtered page: %+v %+v", items, v.Meta)
		}
		v, _ = f.request(t, "GET", "/api/v1/items?category=packaging", "", 200)
		if v.Meta.Total != 0 {
			t.Fatal("category not case sensitive")
		}
	})
	t.Run("T09_empty_out_of_range_and_overflow", func(t *testing.T) {
		for path, total := range map[string]int64{"/api/v1/items?page=4": 25, "/api/v1/items?category=Absent": 0, "/api/v1/items?q=missing": 0, "/api/v1/items?page=2147483647&limit=1": 25, "/api/v1/items?page=2147483648&limit=1": 25} {
			v, _ := f.request(t, "GET", path, "", 200)
			if len(listFrom(t, v)) != 0 || v.Meta.Total != total || len(v.Meta.Categories) != 3 {
				t.Fatal(path, v)
			}
		}
		f.request(t, "GET", "/api/v1/items?page=2147483647&limit=100", "", 400)
	})
	t.Run("T10_global_literal_search_and_categories", func(t *testing.T) {
		for _, q := range []string{"needle", "NEEDLE", "special", "%", "_", `\`, `%_\`, `' OR 1=1 --`} {
			path := "/api/v1/items?q=" + url.QueryEscape(q)
			v, _ := f.request(t, "GET", path, "", 200)
			items := listFrom(t, v)
			want := int64(1)
			if q == "' OR 1=1 --" {
				want = 0
			}
			if v.Meta.Total != want || int64(len(items)) != want {
				t.Fatalf("literal %q: %+v %+v", q, items, v.Meta)
			}
			if want == 1 && items[0].ID != all[24].ID {
				t.Fatal(items)
			}
			if len(v.Meta.Categories) != 3 {
				t.Fatal("categories filtered")
			}
		}
		v, _ := f.request(t, "GET", "/api/v1/items?q=needle&category=Tools", "", 200)
		if v.Meta.Total != 0 {
			t.Fatal("filters did not AND")
		}
		v, _ = f.request(t, "GET", "/api/v1/items?q=%20needle%20&category=%20Z-last%20&limit=1", "", 200)
		if v.Meta.Total != 1 || len(listFrom(t, v)) != 1 || len(v.Meta.Categories) != 3 {
			t.Fatal(v)
		}
		v, _ = f.request(t, "GET", "/api/v1/items?q=&category=&page=1&limit=100", "", 200)
		if v.Meta.Total != 25 || len(listFrom(t, v)) != 25 {
			t.Fatal("empty optional filters")
		}
	})
}

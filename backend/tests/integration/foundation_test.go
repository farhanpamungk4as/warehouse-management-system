package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"warehouse/backend/internal/app"
	"warehouse/backend/internal/platform/config"
)

// The supplied URL is an admin connection. Only a newly created random database is mutated/dropped.
func TestPostgresFoundation(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL and GOOSE_BIN to run real PostgreSQL integration tests")
	}
	goose := os.Getenv("GOOSE_BIN")
	if goose == "" {
		var err error
		goose, err = exec.LookPath("goose")
		if err != nil {
			t.Fatal("GOOSE_BIN or goose on PATH is required")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal("cannot connect to integration PostgreSQL")
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = admin.Close(cleanup)
	})
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	name := "warehouse_test_" + hex.EncodeToString(nonce[:])
	identifier := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		if _, err := admin.Exec(cleanup, "DROP DATABASE "+identifier+" WITH (FORCE)"); err != nil {
			t.Errorf("cleanup disposable database: %v", err)
		}
	})
	cfg := admin.Config().Copy()
	cfg.Database = name
	// ConnString preserves the original URL even if Config.Database is changed.
	// Build the migration URL explicitly so Goose cannot target the admin database.
	dsnURL, err := neturl.Parse(url)
	if err != nil || (dsnURL.Scheme != "postgres" && dsnURL.Scheme != "postgresql") {
		t.Fatal("TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	dsnURL.Path = "/" + name
	query := dsnURL.Query()
	query.Del("dbname")
	dsnURL.RawQuery = query.Encode()
	dsn := dsnURL.String()
	migrationConfig, err := pgx.ParseConfig(dsn)
	if err != nil || migrationConfig.Database != name {
		t.Fatal("migration URL must target the newly created disposable database")
	}
	db, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close(context.Background()) })
	// Run the actual pinned CLI, exercising Goose parsing/versioning as well as PostgreSQL SQL.
	migrate := func(command string, allow bool) error {
		runURL := *dsnURL
		options := runURL.Query()
		options.Del("options")
		if allow {
			options.Set("options", "-c warehouse.allow_destructive_rollback=on")
		}
		runURL.RawQuery = options.Encode()
		cmd := exec.CommandContext(ctx, goose, "-dir", "../../migrations", "postgres", runURL.String(), command)
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "PGOPTIONS=") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			return errors.New(string(output))
		}
		return nil
	}
	mustExec := func(t *testing.T, sql string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	version := func(t *testing.T) int64 {
		t.Helper()
		var n int64
		if err := db.QueryRow(ctx, "SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	checkViolation := func(t *testing.T, sql, state, constraint string, args ...any) {
		t.Helper()
		_, err := db.Exec(ctx, sql, args...)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != state || (constraint != "" && pg.ConstraintName != constraint) {
			t.Fatalf("expected %s/%s, got %v", state, constraint, err)
		}
	}
	if err := migrate("up", false); err != nil {
		t.Fatal(err)
	}
	if err := migrate("up", false); err != nil {
		t.Fatal(err)
	}
	if version(t) != 2 {
		t.Fatal("expected version 2")
	}
	var locationID int32
	if err := db.QueryRow(ctx, "SELECT id FROM locations WHERE code='RACK-A1' AND zone='A' AND type='rack'").Scan(&locationID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM locations").Scan(&count); err != nil || count != 2 {
		t.Fatalf("seed count: %d, %v", count, err)
	}
	var index string
	if err := db.QueryRow(ctx, "SELECT indexdef FROM pg_indexes WHERE tablename='items' AND indexname='items_active_category_id_idx'").Scan(&index); err != nil || !strings.Contains(index, "deleted_at IS NULL") {
		t.Fatal("active category index missing")
	}
	var itemID int32
	if err := db.QueryRow(ctx, "INSERT INTO items(sku,name,category,unit) VALUES('SKU-1','Box','Packaging','pcs') RETURNING id").Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	t.Run("named constraints", func(t *testing.T) {
		checkViolation(t, "INSERT INTO items(sku,name,category,unit) VALUES('SKU-1','Other','Tools','pcs')", "23505", "items_sku_key")
		mustExec(t, "INSERT INTO stocks(item_id,location_id) VALUES($1,$2)", itemID, locationID)
		var qty int32
		var updated time.Time
		if err := db.QueryRow(ctx, "SELECT qty,updated_at FROM stocks WHERE item_id=$1 AND location_id=$2", itemID, locationID).Scan(&qty, &updated); err != nil || qty != 0 || updated.IsZero() {
			t.Fatal("stock defaults invalid")
		}
		checkViolation(t, "INSERT INTO stocks(item_id,location_id,qty) VALUES($1,$2,1)", "23505", "stocks_item_location_key", itemID, locationID)
		checkViolation(t, "UPDATE stocks SET qty=-1 WHERE item_id=$1", "23514", "stocks_qty_nonnegative", itemID)
		checkViolation(t, "INSERT INTO stocks(item_id,location_id) VALUES(2147483647,$1)", "23503", "stocks_item_id_fkey", locationID)
		checkViolation(t, "INSERT INTO stocks(item_id,location_id) VALUES($1,2147483647)", "23503", "stocks_location_id_fkey", itemID)
		checkViolation(t, "DELETE FROM items WHERE id=$1", "23503", "stocks_item_id_fkey", itemID)
		for _, sku := range []string{"", " padded ", "tab\tvalue", "control\x7f"} {
			checkViolation(t, "INSERT INTO items(sku,name,category,unit) VALUES($1,'Box','Packaging','pcs')", "23514", "items_sku_valid", sku)
		}
		checkViolation(t, "INSERT INTO items(sku,name,category,unit) VALUES($1,'Box','Packaging','pcs')", "22001", "", strings.Repeat("x", 65))
		checkViolation(t, "INSERT INTO items(sku,name,category,unit) VALUES('NULL-NAME',NULL,'Packaging','pcs')", "23502", "")
	})
	t.Run("soft deleted SKU stays reserved", func(t *testing.T) {
		mustExec(t, "UPDATE items SET deleted_at=now() WHERE id=$1", itemID)
		checkViolation(t, "INSERT INTO items(sku,name,category,unit) VALUES('SKU-1','New','Packaging','pcs')", "23505", "items_sku_key")
		var deleted *time.Time
		if err := db.QueryRow(ctx, "SELECT deleted_at FROM items WHERE id=$1", itemID).Scan(&deleted); err != nil || deleted == nil {
			t.Fatal("soft-delete timestamp missing")
		}
	})
	t.Run("rollback refusal is atomic", func(t *testing.T) {
		if err := migrate("down", false); err == nil || !strings.Contains(err.Error(), "requires explicit disposable-database opt-in") {
			t.Fatalf("expected opt-in refusal, got %v", err)
		}
		if err := migrate("down", true); err == nil || !strings.Contains(err.Error(), "locations changed or referenced by stock") {
			t.Fatalf("expected referenced-seed refusal, got %v", err)
		}
		if version(t) != 2 {
			t.Fatal("failed rollback changed Goose version")
		}
		mustExec(t, "DELETE FROM stocks")
		mustExec(t, "DELETE FROM items")
		mustExec(t, "UPDATE locations SET zone='changed' WHERE code='RACK-A1'")
		if err := migrate("down", true); err == nil || !strings.Contains(err.Error(), "locations changed or referenced by stock") {
			t.Fatalf("expected altered-seed refusal, got %v", err)
		}
		if version(t) != 2 {
			t.Fatal("failed seed rollback changed version")
		}
		mustExec(t, "UPDATE locations SET zone='A' WHERE code='RACK-A1'")
	})
	t.Run("disposable rollback and reapply", func(t *testing.T) {
		if err := migrate("down", true); err != nil {
			t.Fatal(err)
		}
		mustExec(t, "INSERT INTO items(sku,name,category,unit) VALUES('KEEP','Box','Tools','pcs')")
		if err := migrate("down", true); err == nil || !strings.Contains(err.Error(), "database contains business data") {
			t.Fatalf("expected business-data refusal, got %v", err)
		}
		if version(t) != 1 {
			t.Fatal("failed schema rollback changed version")
		}
		mustExec(t, "DELETE FROM items")
		if err := migrate("down", true); err != nil {
			t.Fatal(err)
		}
		if version(t) != 0 {
			t.Fatal("rollback version not zero")
		}
		if err := migrate("up", false); err != nil {
			t.Fatal(err)
		}
		if version(t) != 2 {
			t.Fatal("reapply failed")
		}
	})
	t.Run("real server lifecycle", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := listener.Addr().String()
		_ = listener.Close()
		serverCtx, stop := context.WithCancel(ctx)
		defer stop()
		done := make(chan error, 1)
		go func() {
			done <- app.Run(serverCtx, config.Config{DatabaseURL: dsn, HTTPAddr: addr, DBMaxConns: 2, StartupTimeout: time.Second, RequestTimeout: time.Second, ShutdownTimeout: time.Second}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
		}()
		client := http.Client{Timeout: time.Second}
		deadline := time.Now().Add(5 * time.Second)
		ready := false
		for time.Now().Before(deadline) {
			resp, err := client.Get("http://" + addr + "/healthz")
			if err == nil {
				var body struct {
					Success bool `json:"success"`
				}
				err = json.NewDecoder(resp.Body).Decode(&body)
				_ = resp.Body.Close()
				if err == nil && resp.StatusCode == 200 && body.Success {
					ready = true
					break
				}
			}
			select {
			case err := <-done:
				t.Fatalf("server stopped before readiness: %v", err)
			default:
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !ready {
			t.Fatal("server readiness timed out")
		}
		stop()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("graceful shutdown timed out")
		}
	})
}

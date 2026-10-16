package config

import (
	"strings"
	"testing"
	"time"
)

func TestConfigDefaultsAndValidation(t *testing.T) {
	valid := "postgres://test:secret@localhost:5432/test?sslmode=disable"
	lookup := func(m map[string]string) func(string) (string, bool) {
		return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
	}
	cfg, err := load(lookup(map[string]string{"DATABASE_URL": valid}))
	if err != nil || cfg.HTTPAddr != ":8080" || cfg.DBMaxConns != 10 || cfg.RequestTimeout != 5*time.Second {
		t.Fatalf("defaults: %+v, %v", cfg, err)
	}
	for _, tc := range []struct{ key, value string }{
		{"DATABASE_URL", ""}, {"DATABASE_URL", "postgres://test:secret@localhost:invalid/test"},
		{"HTTP_ADDR", "8080"}, {"HTTP_ADDR", ":0"}, {"HTTP_ADDR", ":65536"}, {"LOG_LEVEL", "unknown"},
		{"DB_MAX_CONNS", "0"}, {"DB_MAX_CONNS", "101"}, {"DB_MAX_CONNS", "2147483648"},
		{"STARTUP_TIMEOUT", "0s"}, {"REQUEST_TIMEOUT", "-1s"}, {"SHUTDOWN_TIMEOUT", "invalid"}, {"REQUEST_TIMEOUT", "2m"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			_, err := load(lookup(map[string]string{"DATABASE_URL": valid, tc.key: tc.value}))
			if err == nil {
				t.Fatal("expected configuration rejection")
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "postgres://") {
				t.Fatal("configuration error leaked credentials")
			}
		})
	}
}

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"warehouse/backend/internal/platform/config"
	"warehouse/backend/internal/platform/httpx"
	"warehouse/backend/internal/platform/middleware"
)

type pingFunc func(context.Context) error

func (p pingFunc) Ping(ctx context.Context) error { return p(ctx) }

func TestRouterEnvelopesLogsAndRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, path, method string
		status             int
		unavailable, panic bool
	}{
		{"ready", "/healthz", "GET", 200, false, false},
		{"unavailable", "/healthz", "GET", 503, true, false},
		{"missing", "/api/v1/items?password=do-not-log", "GET", 404, false, false},
		{"method", "/healthz", "POST", 405, false, false},
		{"trailing slash", "/healthz/", "GET", 404, false, false},
		{"panic", "/panic", "GET", 500, false, true},
		{"internal", "/internal", "GET", 500, false, false},
		{"validation", "/validation", "GET", 400, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			r := NewRouter(config.Config{RequestTimeout: time.Second}, logger, pingFunc(func(ctx context.Context) error {
				if middleware.ID(ctx) != "test-id" {
					t.Error("request ID did not propagate into readiness context")
				}
				if tc.unavailable {
					return errors.New("SELECT secret password=do-not-log")
				}
				return nil
			}), nil, nil)
			r.GET("/panic", func(c *gin.Context) { panic("do-not-log") })
			r.GET("/internal", func(c *gin.Context) { httpx.Reject(c, errors.New("SQL password=do-not-log")) })
			r.GET("/validation", func(c *gin.Context) { httpx.Reject(c, httpx.NewFailure(400, "Validation failed", "Invalid value")) })
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-Request-ID", "test-id")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["success"] != (tc.status < 400) || body["message"] == nil {
				t.Fatalf("invalid envelope: %v", body)
			}
			if tc.status < 400 {
				if body["data"] == nil {
					t.Fatal("missing data")
				}
			} else {
				if _, ok := body["errors"].([]any); !ok {
					t.Fatal("missing errors array")
				}
			}
			if strings.Contains(w.Body.String(), "do-not-log") || strings.Contains(logs.String(), "do-not-log") {
				t.Fatal("secret leaked")
			}
			if w.Header().Get("X-Request-ID") != "test-id" || !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
				t.Fatal("missing headers")
			}
			if tc.status == 405 && w.Header().Get("Allow") != "GET" {
				t.Fatal("missing Allow")
			}
			lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
			var metadata map[string]any
			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &metadata); err != nil {
				t.Fatal(err)
			}
			if metadata["msg"] != "http.request" || metadata["status"] != float64(tc.status) || metadata["method"] != tc.method || metadata["request_id"] != "test-id" || metadata["duration_ms"] == nil {
				t.Fatalf("bad request log: %v", metadata)
			}
		})
	}
}

func TestRequestIDAndDeadline(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	r := NewRouter(config.Config{RequestTimeout: 20 * time.Millisecond}, logger, pingFunc(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }), nil, nil)
	for _, id := range []string{"", strings.Repeat("x", 65), "invalid/id"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/healthz", nil)
		req.Header.Set("X-Request-ID", id)
		r.ServeHTTP(w, req)
		got := w.Header().Get("X-Request-ID")
		if w.Code != 503 || len(got) < 16 || len(got) > 64 || got == id {
			t.Fatalf("deadline/request ID: %d %q", w.Code, got)
		}
	}
}

func TestBodyLimitUsesCentralEnvelope(t *testing.T) {
	r := NewRouter(config.Config{RequestTimeout: time.Second}, slog.New(slog.NewJSONHandler(io.Discard, nil)), pingFunc(func(context.Context) error { return nil }), nil, nil)
	r.POST("/parse", func(c *gin.Context) {
		var v any
		if err := c.ShouldBindJSON(&v); err != nil {
			httpx.Reject(c, err)
			return
		}
		httpx.Success(c, 200, "OK", v)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/parse", strings.NewReader(`{"value":"`+strings.Repeat("x", 65536)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 413 || !strings.Contains(w.Body.String(), `"success":false`) {
		t.Fatalf("expected 413 envelope: %d %s", w.Code, w.Body.String())
	}
}

package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"warehouse/backend/internal/app"
	"warehouse/backend/internal/modules/item/domain"
	"warehouse/backend/internal/modules/item/handler"
	"warehouse/backend/internal/platform/config"
)

type ping struct{}

func (ping) Ping(context.Context) error { return nil }

type spy struct {
	handler.UseCases
	calls int
	write domain.Write
}

func (s *spy) Create(_ context.Context, w domain.Write) (domain.Item, error) {
	s.calls++
	s.write = w
	return domain.Item{ID: 1}, nil
}
func (s *spy) Update(_ context.Context, _ int32, w domain.Write) (domain.Item, error) {
	s.calls++
	s.write = w
	return domain.Item{ID: 1}, nil
}
func (s *spy) Get(context.Context, int32) (domain.Item, error) { s.calls++; return domain.Item{}, nil }
func (s *spy) Delete(context.Context, int32) (domain.Deleted, error) {
	s.calls++
	return domain.Deleted{}, nil
}
func (s *spy) List(context.Context, domain.ListParams) (domain.ListResult, error) {
	s.calls++
	return domain.ListResult{}, nil
}

func TestValidationBeforeService(t *testing.T) {
	valid := `{"sku":"abc","name":"Box","category":"Tools","unit":"pcs"}`
	cases := []struct {
		name, method, path, body, media string
		status                          int
	}{
		{"empty", "POST", "/api/v1/items", "", "application/json", 400},
		{"null", "POST", "/api/v1/items", "null", "application/json", 400},
		{"array", "POST", "/api/v1/items", "[]", "application/json", 400},
		{"missing", "POST", "/api/v1/items", `{"sku":"abc"}`, "application/json", 400},
		{"null field", "POST", "/api/v1/items", strings.Replace(valid, `"Box"`, `null`, 1), "application/json", 400},
		{"wrong type", "POST", "/api/v1/items", strings.Replace(valid, `"Box"`, `123`, 1), "application/json", 400},
		{"blank", "POST", "/api/v1/items", strings.Replace(valid, `"Box"`, `"   "`, 1), "application/json", 400},
		{"unicode blank", "POST", "/api/v1/items", strings.Replace(valid, "Box", "\u2003\u00a0", 1), "application/json", 400},
		{"control", "POST", "/api/v1/items", strings.Replace(valid, "Box", `Box\t`, 1), "application/json", 400},
		{"delete control", "POST", "/api/v1/items", strings.Replace(valid, "Box", `Box\u007f`, 1), "application/json", 400},
		{"sku long", "POST", "/api/v1/items", strings.Replace(valid, "abc", strings.Repeat("界", 65), 1), "application/json", 400},
		{"name long", "POST", "/api/v1/items", strings.Replace(valid, "Box", strings.Repeat("界", 201), 1), "application/json", 400},
		{"category long", "POST", "/api/v1/items", strings.Replace(valid, "Tools", strings.Repeat("界", 65), 1), "application/json", 400},
		{"unit long", "POST", "/api/v1/items", strings.Replace(valid, "pcs", strings.Repeat("界", 33), 1), "application/json", 400},
		{"server ID", "POST", "/api/v1/items", strings.Replace(valid, "{", `{"id":1,`, 1), "application/json", 400},
		{"server time", "POST", "/api/v1/items", strings.Replace(valid, "{", `{"created_at":"2026-10-15T00:00:00Z",`, 1), "application/json", 400},
		{"deleted time", "POST", "/api/v1/items", strings.Replace(valid, "{", `{"deleted_at":null,`, 1), "application/json", 400},
		{"duplicate field", "POST", "/api/v1/items", strings.Replace(valid, "{", `{"sku":"first",`, 1), "application/json", 400},
		{"wrong key case", "POST", "/api/v1/items", strings.Replace(valid, "sku", "SKU", 1), "application/json", 400},
		{"trailing", "POST", "/api/v1/items", valid + " {}", "application/json", 400},
		{"garbage", "POST", "/api/v1/items", valid + " garbage", "application/json", 400},
		{"malformed", "POST", "/api/v1/items", valid[:len(valid)-1], "application/json", 400},
		{"invalid UTF8", "POST", "/api/v1/items", strings.Replace(valid, "Box", string([]byte{0xff}), 1), "application/json", 400},
		{"too big", "POST", "/api/v1/items", strings.Replace(valid, "Box", strings.Repeat("x", 65536), 1), "application/json", 413},
		{"media", "POST", "/api/v1/items", valid, "text/plain", 415},
		{"missing media", "POST", "/api/v1/items", valid, "", 415},
		{"partial put", "PUT", "/api/v1/items/1", `{"sku":"new"}`, "application/json", 400},
		{"post query", "POST", "/api/v1/items?foo=x", valid, "application/json", 400},
	}
	for _, id := range []string{"0", "-1", "abc", "1.5", "2147483648", "+1"} {
		for _, method := range []string{"GET", "PUT", "DELETE"} {
			cases = append(cases, struct {
				name, method, path, body, media string
				status                          int
			}{method + id, method, "/api/v1/items/" + id, valid, "application/json", 400})
		}
	}
	for _, query := range []string{"page=0", "page=-1", "page=", "page=1.1", "page=2147483648", "page=2147483647&limit=100", "limit=0", "limit=101", "limit=", "limit=abc", "page=1&page=2", "q=a&q=b", "category=a&category=b", "limit=1&limit=2", "sort=id", "q=%zz", "q=a;b", "category=%E2%80%83", "q=%09", "q=" + strings.Repeat("x", 201), "category=" + strings.Repeat("x", 65)} {
		cases = append(cases, struct {
			name, method, path, body, media string
			status                          int
		}{query, "GET", "/api/v1/items?" + query, "", "", 400})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := &spy{}
			router := app.NewRouter(config.Config{RequestTimeout: time.Second}, slog.New(slog.NewJSONHandler(io.Discard, nil)), ping{}, service, nil)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.media)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			var body struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
				Errors  []any  `json:"errors"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || body.Success || body.Message == "" || len(body.Errors) == 0 || service.calls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", w.Code, service.calls, w.Body.String())
			}
		})
	}
}
func TestValidNormalizationAndEmptyList(t *testing.T) {
	service := &spy{}
	router := app.NewRouter(config.Config{RequestTimeout: time.Second}, slog.New(slog.NewJSONHandler(io.Discard, nil)), ping{}, service, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/items", strings.NewReader(`{"sku":" abc ","name":" 箱 ","category":" Tools ","unit":" pcs "}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	router.ServeHTTP(w, req)
	if w.Code != 201 || service.write != (domain.Write{SKU: "abc", Name: "箱", Category: "Tools", Unit: "pcs"}) || w.Header().Get("Location") != "/api/v1/items/1" {
		t.Fatalf("%d %s %+v", w.Code, w.Body.String(), service.write)
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/items", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"data":[]`) || !strings.Contains(w.Body.String(), `"categories":[]`) {
		t.Fatal(w.Body.String())
	}
	for path, allow := range map[string]string{"/api/v1/items": "GET, POST", "/api/v1/items/1": "GET, PUT, DELETE"} {
		w = httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("PATCH", path, nil))
		if w.Code != 405 || w.Header().Get("Allow") != allow {
			t.Fatalf("%d %s", w.Code, w.Header())
		}
	}
}

package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"warehouse/backend/internal/app"
	itemdomain "warehouse/backend/internal/modules/item/domain"
	locationdomain "warehouse/backend/internal/modules/location/domain"
	"warehouse/backend/internal/modules/stock/domain"
	"warehouse/backend/internal/platform/config"
)

type ping struct{}

func (ping) Ping(context.Context) error { return nil }

type spy struct {
	calls int
	err   error
	input domain.ReceiveInput
}

func (s *spy) Receive(_ context.Context, input domain.ReceiveInput) (domain.Stock, error) {
	s.calls++
	s.input = input
	return domain.Stock{}, s.err
}
func (s *spy) Lookup(context.Context, int64) (domain.Lookup, error) {
	s.calls++
	return domain.Lookup{ItemID: 1}, s.err
}
func router(s *spy) *gin.Engine {
	return app.NewRouter(config.Config{RequestTimeout: time.Second}, slog.New(slog.NewJSONHandler(io.Discard, nil)), ping{}, nil, s)
}
func TestT11StrictValidationBeforeService(t *testing.T) {
	valid := `{"item_id":1,"location_id":2,"qty":5}`
	cases := []struct {
		method, path, body, media string
		status                    int
	}{}
	for _, body := range []string{"", "null", "[]", "{}", `{"item_id":1,"location_id":2}`, `{"item_id":1,"qty":5}`, `{"location_id":2,"qty":5}`, valid + " {}", valid + "x", valid[:len(valid)-1], strings.Replace(valid, "{", `{"qty":1,`, 1), strings.Replace(valid, "{", `{"id":9,`, 1), strings.Replace(valid, "item_id", "Item_id", 1)} {
		cases = append(cases, struct {
			method, path, body, media string
			status                    int
		}{"POST", "/api/v1/stock/receive", body, "application/json", 400})
	}
	for _, field := range []string{"item_id", "location_id", "qty"} {
		for _, value := range []string{"0", "-1", "1.5", "1.0", "1e0", "2147483648", "9223372036854775808", "null", `"5"`, "true", "[]", "{}"} {
			body := `{"item_id":1,"location_id":2,"qty":5}`
			old := `"` + field + `":`
			start := strings.Index(body, old) + len(old)
			end := start
			for end < len(body) && body[end] != ',' && body[end] != '}' {
				end++
			}
			body = body[:start] + value + body[end:]
			cases = append(cases, struct {
				method, path, body, media string
				status                    int
			}{"POST", "/api/v1/stock/receive", body, "application/json", 400})
		}
	}
	for _, id := range []string{"0", "-1", "+1", "1.2", "2147483648", "abc"} {
		cases = append(cases, struct {
			method, path, body, media string
			status                    int
		}{"GET", "/api/v1/stock/" + id, "", "", 400})
	}
	for _, query := range []string{"?q=x", "?item_id=1&item_id=2", "?q=%zz"} {
		cases = append(cases, struct {
			method, path, body, media string
			status                    int
		}{"POST", "/api/v1/stock/receive" + query, valid, "application/json", 400}, struct {
			method, path, body, media string
			status                    int
		}{"GET", "/api/v1/stock/1" + query, "", "", 400})
	}
	cases = append(cases, struct {
		method, path, body, media string
		status                    int
	}{"POST", "/api/v1/stock/receive", valid, "text/plain", 415}, struct {
		method, path, body, media string
		status                    int
	}{"POST", "/api/v1/stock/receive", valid, "", 415}, struct {
		method, path, body, media string
		status                    int
	}{"POST", "/api/v1/stock/receive", valid + strings.Repeat(" ", 65536), "application/json", 413})
	for _, tc := range cases {
		t.Run(tc.method+tc.path+tc.body[:min(len(tc.body), 60)], func(t *testing.T) {
			s := &spy{}
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.media)
			router(s).ServeHTTP(w, req)
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || s.calls != 0 || body["success"] != false || body["message"] == nil || body["errors"] == nil {
				t.Fatalf("%d calls=%d %s", w.Code, s.calls, w.Body.String())
			}
		})
	}
}
func TestCentralErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		field  string
	}{
		{&domain.ReferenceError{Field: "item_id", Cause: itemdomain.ErrNotFound}, 404, "item_id"},
		{&domain.ReferenceError{Field: "location_id", Cause: locationdomain.ErrNotFound}, 404, "location_id"},
		{&domain.ReferenceError{Field: "item_id", Cause: errors.New("private SQL")}, 500, "request"},
		{domain.ErrCapacityExceeded, 409, "qty"},
		{&domain.CommitOutcomeUnknown{Cause: context.DeadlineExceeded}, 500, "request"},
		{context.DeadlineExceeded, 503, "request"},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			s := &spy{err: tc.err}
			w := httptest.NewRecorder()
			router(s).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/stock/1", nil))
			var body struct {
				Success bool
				Errors  []struct{ Field string }
			}
			_ = json.Unmarshal(w.Body.Bytes(), &body)
			if w.Code != tc.status || body.Success || len(body.Errors) != 1 || body.Errors[0].Field != tc.field || strings.Contains(w.Body.String(), "private SQL") {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
}
func TestValidMaximumAndEmptyLookup(t *testing.T) {
	s := &spy{}
	r := router(s)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/stock/receive", strings.NewReader(`{"item_id":2147483647,"location_id":2147483647,"qty":2147483647}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	r.ServeHTTP(w, req)
	if w.Code != 200 || s.calls != 1 || s.input.Qty != domain.MaxQuantity {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/stock/1", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"stocks":[]`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/stock/1", nil))
	if w.Code != 405 || w.Header().Get("Allow") != "GET" {
		t.Fatal("missing Allow")
	}
}

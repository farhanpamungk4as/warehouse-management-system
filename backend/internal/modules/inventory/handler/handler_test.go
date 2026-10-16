package handler_test

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"warehouse/backend/internal/app"
	"warehouse/backend/internal/modules/inventory/domain"
	"warehouse/backend/internal/modules/inventory/handler"
	"warehouse/backend/internal/platform/config"
)

type spy struct{ calls int }

func (s *spy) Summary(context.Context) (domain.Summary, error) {
	s.calls++
	return domain.Summary{}, nil
}
func (s *spy) Overview(context.Context, domain.Params) (domain.Overview, error) {
	s.calls++
	return domain.Overview{}, nil
}
func TestInvalidInventoryInputNeverReachesUseCase(t *testing.T) {
	for _, query := range []string{"limit=101", "page=0", "page=2147483649&limit=1", "location_id=0", "location_id=2147483648", "q=x&q=y", "q=%zz", "q=%00", "category=%20%20%E2%80%83", "unknown=1", "page=1.0", "limit=+2"} {
		t.Run(query, func(t *testing.T) {
			s := &spy{}
			r := app.NewRouter(config.Config{RequestTimeout: time.Second}, slog.New(slog.NewJSONHandler(io.Discard, nil)), nil, nil, nil)
			handler.Register(r, s)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/inventory/overview?"+query, nil))
			if w.Code != 400 || s.calls != 0 {
				t.Fatalf("%d calls=%d: %s", w.Code, s.calls, w.Body.String())
			}
		})
	}
}

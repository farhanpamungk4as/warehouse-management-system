package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"warehouse/backend/internal/modules/stock/domain"
)

type runnerFunc func(context.Context, func(ReceiveSession) (domain.Stock, error)) (domain.Stock, error)

func (f runnerFunc) Run(ctx context.Context, callback func(ReceiveSession) (domain.Stock, error)) (domain.Stock, error) {
	return f(ctx, callback)
}

type readerFunc func(context.Context, int32) (domain.Lookup, error)

func (f readerFunc) Lookup(ctx context.Context, id int32) (domain.Lookup, error) { return f(ctx, id) }

type itemFunc func(context.Context, int32) error

func (f itemFunc) EnsureActiveForReceive(ctx context.Context, id int32) error { return f(ctx, id) }

type locationFunc func(context.Context, int32) error

func (f locationFunc) EnsureExistsForReceive(ctx context.Context, id int32) error { return f(ctx, id) }

type writerFunc func(context.Context, int32, int32, int32) (domain.Stock, error)

func (f writerFunc) Increment(ctx context.Context, i, l, q int32) (domain.Stock, error) {
	return f(ctx, i, l, q)
}

func TestT11IndependentValidation(t *testing.T) {
	var logs bytes.Buffer
	calls := 0
	runner := runnerFunc(func(context.Context, func(ReceiveSession) (domain.Stock, error)) (domain.Stock, error) {
		calls++
		return domain.Stock{}, nil
	})
	reader := readerFunc(func(context.Context, int32) (domain.Lookup, error) { calls++; return domain.Lookup{}, nil })
	s := New(reader, runner, slog.New(slog.NewJSONHandler(&logs, nil)), func(context.Context) string { return "service-test" })
	for _, input := range []domain.ReceiveInput{{ItemID: 0, LocationID: 1, Qty: 1}, {ItemID: -1, LocationID: 1, Qty: 1}, {ItemID: domain.MaxID + 1, LocationID: 1, Qty: 1}, {ItemID: 1, LocationID: 0, Qty: 1}, {ItemID: 1, LocationID: -1, Qty: 1}, {ItemID: 1, LocationID: domain.MaxID + 1, Qty: 1}, {ItemID: 1, LocationID: 1, Qty: 0}, {ItemID: 1, LocationID: 1, Qty: -1}, {ItemID: 1, LocationID: 1, Qty: domain.MaxQuantity + 1}} {
		if _, err := s.Receive(context.Background(), input); err == nil {
			t.Fatalf("accepted %+v", input)
		}
	}
	for _, id := range []int64{0, -1, domain.MaxID + 1} {
		if _, err := s.Lookup(context.Background(), id); err == nil {
			t.Fatal("accepted ID", id)
		}
	}
	if calls != 0 || logs.Len() != 0 {
		t.Fatal("invalid input reached persistence or logged success")
	}
}
func TestReceiveOrderCommitAndFailureLogs(t *testing.T) {
	for _, outcome := range []string{"success", "item failure", "location failure", "write failure", "rollback", "unknown"} {
		t.Run(outcome, func(t *testing.T) {
			var logs bytes.Buffer
			var order []string
			calls := 0
			failure := errors.New("private cause")
			committed := false
			session := ReceiveSession{
				Item: itemFunc(func(context.Context, int32) error {
					order = append(order, "item")
					if outcome == "item failure" {
						return failure
					}
					return nil
				}),
				Location: locationFunc(func(context.Context, int32) error {
					order = append(order, "location")
					if outcome == "location failure" {
						return failure
					}
					return nil
				}),
				Stock: writerFunc(func(_ context.Context, i, l, q int32) (domain.Stock, error) {
					order = append(order, "stock")
					if outcome == "write failure" {
						return domain.Stock{}, failure
					}
					return domain.Stock{ID: 3, ItemID: i, LocationID: l, Qty: q + 10}, nil
				}),
			}
			runner := runnerFunc(func(_ context.Context, callback func(ReceiveSession) (domain.Stock, error)) (domain.Stock, error) {
				calls++
				stock, err := callback(session)
				if err != nil {
					return stock, err
				}
				if outcome == "rollback" {
					return stock, failure
				}
				if outcome == "unknown" {
					return stock, &domain.CommitOutcomeUnknown{Cause: context.DeadlineExceeded}
				}
				committed = true
				return stock, nil
			})
			s := New(nil, runner, slog.New(slog.NewJSONHandler(&logs, nil)), func(context.Context) string { return "service-test" })
			stock, err := s.Receive(context.Background(), domain.ReceiveInput{ItemID: 1, LocationID: 2, Qty: 5})
			if calls != 1 {
				t.Fatal("automatic retry", calls)
			}
			if outcome == "success" {
				if err != nil || !committed || stock.Qty != 15 || len(order) != 3 || order[0] != "item" || order[1] != "location" || order[2] != "stock" {
					t.Fatalf("%+v %v %v", stock, err, order)
				}
				var event map[string]any
				if err = json.Unmarshal(logs.Bytes(), &event); err != nil {
					t.Fatal(err)
				}
				for field, want := range map[string]any{"event": "stock.receive.committed", "request_id": "service-test", "item_id": float64(1), "location_id": float64(2), "stock_id": float64(3), "received_qty": float64(5), "qty_before": float64(10), "qty_after": float64(15)} {
					if event[field] != want {
						t.Fatalf("%s=%v want %v", field, event[field], want)
					}
				}
				if event["timestamp"] == nil {
					t.Fatal("missing UTC timestamp")
				}
			} else {
				if err == nil || stock != (domain.Stock{}) || committed {
					t.Fatal("failure presented committed stock")
				}
				if outcome == "unknown" {
					var unknown *domain.CommitOutcomeUnknown
					if !errors.As(err, &unknown) || !bytes.Contains(logs.Bytes(), []byte("stock.receive.commit_unknown")) || bytes.Contains(logs.Bytes(), []byte("stock.receive.committed")) {
						t.Fatal("unknown logged as committed")
					}
				} else {
					if !errors.Is(err, failure) || logs.Len() != 0 {
						t.Fatal("failure lost identity or logged success")
					}
				}
				if outcome == "item failure" && len(order) != 1 {
					t.Fatal("checked location after missing item")
				}
				if outcome == "location failure" && len(order) != 2 {
					t.Fatal("wrote after missing location")
				}
			}
		})
	}
}

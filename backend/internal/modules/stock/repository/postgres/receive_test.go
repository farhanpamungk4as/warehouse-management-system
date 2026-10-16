package postgres

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"warehouse/backend/internal/modules/stock/domain"
	"warehouse/backend/internal/modules/stock/service"
)

type fakeTx struct {
	pgx.Tx
	commitErr          error
	commits, rollbacks int
	cleanupCanceled    bool
}

func (t *fakeTx) Commit(context.Context) error { t.commits++; return t.commitErr }
func (t *fakeTx) Rollback(ctx context.Context) error {
	t.rollbacks++
	t.cleanupCanceled = ctx.Err() != nil
	return nil
}

type fakeItem struct{}

func (fakeItem) EnsureActiveForReceive(context.Context, int32) error { return nil }

type fakeLocation struct{}

func (fakeLocation) EnsureExistsForReceive(context.Context, int32) error { return nil }
func TestRunnerFinalizationAndBinding(t *testing.T) {
	for _, outcome := range []string{"success", "callback failure", "item factory failure", "location factory failure", "cancellation", "commit rejected", "commit unknown"} {
		t.Run(outcome, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			tx := &fakeTx{}
			failure := errors.New("injected")
			if outcome == "commit rejected" {
				tx.commitErr = pgx.ErrTxCommitRollback
			}
			if outcome == "commit unknown" {
				tx.commitErr = io.ErrUnexpectedEOF
			}
			r := &ReceiveRunner{
				begin: func(_ context.Context, options pgx.TxOptions) (pgx.Tx, error) {
					if options.IsoLevel != pgx.ReadCommitted || options.AccessMode != pgx.ReadWrite {
						t.Fatal("wrong transaction options")
					}
					return tx, nil
				},
				itemFactory: func(bound pgx.Tx) (service.ItemGuard, error) {
					if bound != tx {
						t.Fatal("item bound to different tx")
					}
					if outcome == "item factory failure" {
						return nil, failure
					}
					return fakeItem{}, nil
				},
				locationFactory: func(bound pgx.Tx) (service.LocationLookup, error) {
					if bound != tx {
						t.Fatal("location bound to different tx")
					}
					if outcome == "location factory failure" {
						return nil, failure
					}
					return fakeLocation{}, nil
				},
			}
			stock, err := r.Run(ctx, func(session service.ReceiveSession) (domain.Stock, error) {
				if session.Stock.(*writer).tx != tx {
					t.Fatal("stock bound to different tx")
				}
				if outcome == "callback failure" {
					return domain.Stock{}, failure
				}
				if outcome == "cancellation" {
					cancel()
				}
				return domain.Stock{ID: 1}, nil
			})
			if tx.rollbacks != 1 || tx.cleanupCanceled {
				t.Fatal("cleanup was missing or used canceled context")
			}
			switch outcome {
			case "success":
				if err != nil || stock.ID != 1 || tx.commits != 1 {
					t.Fatal(stock, err)
				}
			case "commit unknown":
				var unknown *domain.CommitOutcomeUnknown
				if !errors.As(err, &unknown) || !errors.Is(err, io.ErrUnexpectedEOF) || stock.ID != 0 || tx.commits != 1 {
					t.Fatal("lost commit uncertainty", err)
				}
			case "commit rejected":
				var unknown *domain.CommitOutcomeUnknown
				if !errors.Is(err, pgx.ErrTxCommitRollback) || errors.As(err, &unknown) {
					t.Fatal("known rollback marked unknown", err)
				}
			default:
				if err == nil || stock.ID != 0 || tx.commits != 0 {
					t.Fatal("committed known failure", stock, err)
				}
			}
		})
	}
}
func TestCommitErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		err     error
		unknown bool
	}{{io.EOF, true}, {context.Canceled, true}, {&pgconn.PgError{Code: "08007"}, true}, {&pgconn.PgError{Code: "40003"}, true}, {&pgconn.PgError{Code: "57P01"}, true}, {&pgconn.PgError{Code: "40001"}, false}, {&pgconn.PgError{Code: "23503"}, false}, {pgx.ErrTxCommitRollback, false}} {
		err := classifyCommit(tc.err)
		var unknown *domain.CommitOutcomeUnknown
		if errors.As(err, &unknown) != tc.unknown {
			t.Fatalf("%v: %v", tc.err, err)
		}
	}
}
func TestRunnerRejectsMissingBindings(t *testing.T) {
	r := NewReceiveRunner(nil, nil, nil)
	if _, err := r.Run(context.Background(), nil); err == nil {
		t.Fatal("accepted missing dependencies")
	}
	for _, which := range []string{"item", "location"} {
		tx := &fakeTx{}
		r := &ReceiveRunner{begin: func(context.Context, pgx.TxOptions) (pgx.Tx, error) { return tx, nil }, itemFactory: func(pgx.Tx) (service.ItemGuard, error) {
			if which == "item" {
				return nil, nil
			}
			return fakeItem{}, nil
		}, locationFactory: func(pgx.Tx) (service.LocationLookup, error) { return nil, nil }}
		if _, err := r.Run(context.Background(), func(service.ReceiveSession) (domain.Stock, error) {
			t.Fatal("callback reached with nil guard")
			return domain.Stock{}, nil
		}); err == nil || tx.rollbacks != 1 {
			t.Fatal("nil guard was accepted")
		}
	}
}

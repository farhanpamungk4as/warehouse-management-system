package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"warehouse/backend/internal/modules/stock/domain"
	"warehouse/backend/internal/modules/stock/service"
	"warehouse/backend/internal/platform/database"
)

type ItemGuardFactory func(pgx.Tx) (service.ItemGuard, error)
type LocationLookupFactory func(pgx.Tx) (service.LocationLookup, error)

type ReceiveRunner struct {
	begin           func(context.Context, pgx.TxOptions) (pgx.Tx, error)
	itemFactory     ItemGuardFactory
	locationFactory LocationLookupFactory
}

func NewReceiveRunner(pool *pgxpool.Pool, itemFactory ItemGuardFactory, locationFactory LocationLookupFactory) *ReceiveRunner {
	r := &ReceiveRunner{itemFactory: itemFactory, locationFactory: locationFactory}
	if pool != nil {
		r.begin = pool.BeginTx
	}
	return r
}
func (r *ReceiveRunner) Run(ctx context.Context, callback func(service.ReceiveSession) (domain.Stock, error)) (domain.Stock, error) {
	if r.begin == nil || r.itemFactory == nil || r.locationFactory == nil || callback == nil {
		return domain.Stock{}, errors.New("invalid receive transaction dependencies")
	}
	tx, err := r.begin(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadWrite})
	if err != nil {
		return domain.Stock{}, database.Classify(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	item, err := r.itemFactory(tx)
	if err != nil {
		return domain.Stock{}, fmt.Errorf("bind item guard: %w", err)
	}
	if item == nil {
		return domain.Stock{}, errors.New("item guard factory returned nil")
	}
	location, err := r.locationFactory(tx)
	if err != nil {
		return domain.Stock{}, fmt.Errorf("bind location lookup: %w", err)
	}
	if location == nil {
		return domain.Stock{}, errors.New("location lookup factory returned nil")
	}
	stock, err := callback(service.ReceiveSession{Item: item, Location: location, Stock: &writer{tx: tx}})
	if err != nil {
		return domain.Stock{}, err
	}
	// A known cancellation before sending COMMIT is a rollback, not ambiguity.
	if err = ctx.Err(); err != nil {
		return domain.Stock{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Stock{}, classifyCommit(err)
	}
	return stock, nil
}
func classifyCommit(err error) error {
	if errors.Is(err, pgx.ErrTxCommitRollback) || errors.Is(err, pgx.ErrTxClosed) {
		return database.Classify(err)
	}
	var pg *pgconn.PgError
	// An explicit server rejection is known; transport/fatal/unknown-completion
	// failures after attempting COMMIT do not establish whether it committed.
	if errors.As(err, &pg) && !strings.HasPrefix(pg.Code, "08") && !strings.HasPrefix(pg.Code, "57") && pg.Code != "40003" {
		return database.Classify(err)
	}
	return &domain.CommitOutcomeUnknown{Cause: err}
}

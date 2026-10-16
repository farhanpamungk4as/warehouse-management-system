package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"warehouse/backend/internal/modules/stock/domain"
	"warehouse/backend/internal/modules/stock/service"
	"warehouse/backend/internal/platform/database"
)

type TransferRunner struct {
	begin           func(context.Context, pgx.TxOptions) (pgx.Tx, error)
	itemFactory     ItemGuardFactory
	locationFactory func(pgx.Tx) (service.TransferLocations, error)
}

func NewTransferRunner(pool *pgxpool.Pool, item ItemGuardFactory, locations func(pgx.Tx) (service.TransferLocations, error)) *TransferRunner {
	r := &TransferRunner{itemFactory: item, locationFactory: locations}
	if pool != nil {
		r.begin = pool.BeginTx
	}
	return r
}
func (r *TransferRunner) RunTransfer(ctx context.Context, callback func(service.TransferSession) (domain.Transfer, error)) (domain.Transfer, error) {
	if r.begin == nil || r.itemFactory == nil || r.locationFactory == nil || callback == nil {
		return domain.Transfer{}, errors.New("invalid transfer dependencies")
	}
	tx, err := r.begin(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadWrite})
	if err != nil {
		return domain.Transfer{}, database.Classify(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	item, err := r.itemFactory(tx)
	if err != nil {
		return domain.Transfer{}, err
	}
	locations, err := r.locationFactory(tx)
	if err != nil {
		return domain.Transfer{}, err
	}
	if item == nil || locations == nil {
		return domain.Transfer{}, errors.New("nil transfer binding")
	}
	result, err := callback(service.TransferSession{Item: item, Locations: locations, Stock: &transferWriter{tx}})
	if err != nil {
		return domain.Transfer{}, err
	}
	if err = ctx.Err(); err != nil {
		return domain.Transfer{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Transfer{}, classifyCommit(err)
	}
	return result, nil
}

type transferWriter struct{ tx pgx.Tx }

func (w *transferWriter) Move(ctx context.Context, in domain.TransferInput) (domain.Transfer, error) {
	result := domain.Transfer{ItemID: int32(in.ItemID), FromLocationID: int32(in.FromLocationID), ToLocationID: int32(in.ToLocationID), Qty: int32(in.Qty)}
	s := &result.Source
	err := w.tx.QueryRow(ctx, `UPDATE stocks SET qty=qty-$3,updated_at=clock_timestamp() WHERE item_id=$1 AND location_id=$2 AND qty >= $3 RETURNING id,item_id,location_id,qty,updated_at`, result.ItemID, result.FromLocationID, result.Qty).Scan(&s.ID, &s.ItemID, &s.LocationID, &s.Qty, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Transfer{}, domain.ErrInsufficientStock
	}
	if err != nil {
		return domain.Transfer{}, database.Classify(err)
	}
	dest, err := (&writer{tx: w.tx}).Increment(ctx, result.ItemID, result.ToLocationID, result.Qty)
	if err != nil {
		return domain.Transfer{}, err
	}
	result.Destination = dest
	return result, nil
}

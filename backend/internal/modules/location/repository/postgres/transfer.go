package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"warehouse/backend/internal/modules/location/domain"
	stock "warehouse/backend/internal/modules/stock/domain"
	"warehouse/backend/internal/platform/database"
)

type TransferLookup struct{ tx pgx.Tx }

func NewTransferLookup(tx pgx.Tx) (*TransferLookup, error) {
	if tx == nil {
		return nil, errors.New("transfer requires transaction")
	}
	return &TransferLookup{tx}, nil
}

// Lock both location rows in ascending ID order, before touching either stock row.
// FOR UPDATE conflicts with receiving's KEY SHARE: receive never holds a stock
// lock while waiting for a location. This prevents opposing-transfer cycles.
func (l *TransferLookup) LockForTransfer(ctx context.Context, from, to int32) error {
	ids := []int32{from, to}
	if from > to {
		ids[0], ids[1] = to, from
	}
	for _, id := range ids {
		var found int32
		err := l.tx.QueryRow(ctx, "SELECT id FROM locations WHERE id=$1 FOR UPDATE", id).Scan(&found)
		if errors.Is(err, pgx.ErrNoRows) {
			field := "from_location_id"
			if id == to {
				field = "to_location_id"
			}
			return &stock.ReferenceError{Field: field, Cause: domain.ErrNotFound}
		}
		if err != nil {
			return database.Classify(err)
		}
	}
	return nil
}

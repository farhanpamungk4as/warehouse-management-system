package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"warehouse/backend/internal/modules/location/domain"
	"warehouse/backend/internal/platform/database"
)

// Lookup is bound only to the receiving transaction supplied by the app.
type Lookup struct{ tx pgx.Tx }

func NewLookup(tx pgx.Tx) (*Lookup, error) {
	if tx == nil {
		return nil, errors.New("location lookup requires a transaction")
	}
	return &Lookup{tx: tx}, nil
}
func (l *Lookup) EnsureExistsForReceive(ctx context.Context, id int32) error {
	var found int32
	err := l.tx.QueryRow(ctx, "SELECT id FROM locations WHERE id=$1 FOR KEY SHARE", id).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return database.Classify(err)
}

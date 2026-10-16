package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"warehouse/backend/internal/modules/item/domain"
	"warehouse/backend/internal/platform/database"
)

// ReceiveGuard never acquires a pool connection or owns transaction lifetime.
type ReceiveGuard struct{ tx pgx.Tx }

func NewReceiveGuard(tx pgx.Tx) (*ReceiveGuard, error) {
	if tx == nil {
		return nil, errors.New("item receive guard requires a transaction")
	}
	return &ReceiveGuard{tx: tx}, nil
}
func (g *ReceiveGuard) EnsureActiveForReceive(ctx context.Context, id int32) error {
	var found int32
	err := g.tx.QueryRow(ctx, "SELECT id FROM items WHERE id=$1 AND deleted_at IS NULL FOR SHARE", id).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return database.Classify(err)
}

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	itemdomain "warehouse/backend/internal/modules/item/domain"
	locationdomain "warehouse/backend/internal/modules/location/domain"
	"warehouse/backend/internal/modules/stock/domain"
	"warehouse/backend/internal/platform/database"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

type writer struct{ tx pgx.Tx }

func (w *writer) Increment(ctx context.Context, itemID, locationID, qty int32) (domain.Stock, error) {
	var stock domain.Stock
	err := w.tx.QueryRow(ctx, `INSERT INTO stocks(item_id,location_id,qty,updated_at)
 VALUES($1,$2,$3,clock_timestamp())
 ON CONFLICT (item_id,location_id) DO UPDATE
 SET qty=stocks.qty+EXCLUDED.qty,updated_at=clock_timestamp()
 WHERE stocks.qty <= 2147483647-EXCLUDED.qty
 RETURNING id,item_id,location_id,qty,updated_at`, itemID, locationID, qty).Scan(&stock.ID, &stock.ItemID, &stock.LocationID, &stock.Qty, &stock.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Stock{}, domain.ErrCapacityExceeded
	}
	stock.UpdatedAt = stock.UpdatedAt.UTC()
	return stock, translate(err)
}
func translate(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" {
		switch pg.ConstraintName {
		case "stocks_item_id_fkey":
			return &domain.ReferenceError{Field: "item_id", Cause: itemdomain.ErrNotFound}
		case "stocks_location_id_fkey":
			return &domain.ReferenceError{Field: "location_id", Cause: locationdomain.ErrNotFound}
		}
	}
	return database.Classify(err)
}

func (r *Repository) Lookup(ctx context.Context, itemID int32) (domain.Lookup, error) {
	result := domain.Lookup{ItemID: itemID, Stocks: []domain.Balance{}}
	// This deliberate read projection has one active-item/stock/location snapshot.
	rows, err := r.pool.Query(ctx, `SELECT i.id,s.id,s.item_id,s.location_id,s.qty,s.updated_at,l.id,l.code,l.zone,l.type
 FROM items i LEFT JOIN stocks s ON s.item_id=i.id
 LEFT JOIN locations l ON l.id=s.location_id
 WHERE i.id=$1 AND i.deleted_at IS NULL ORDER BY s.location_id ASC`, itemID)
	if err != nil {
		return result, translate(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		found = true
		var root int32
		var stockID, stockItemID, locationID, qty, joinedID *int32
		var code, zone, kind *string
		var updated *time.Time
		if err = rows.Scan(&root, &stockID, &stockItemID, &locationID, &qty, &updated, &joinedID, &code, &zone, &kind); err != nil {
			return result, translate(err)
		}
		if stockID == nil {
			continue
		}
		if stockItemID == nil || locationID == nil || qty == nil || updated == nil || joinedID == nil || code == nil || zone == nil || kind == nil {
			return result, errors.New("incomplete stock projection")
		}
		result.Stocks = append(result.Stocks, domain.Balance{Stock: domain.Stock{ID: *stockID, ItemID: *stockItemID, LocationID: *locationID, Qty: *qty, UpdatedAt: updated.UTC()}, Location: locationdomain.Location{ID: *joinedID, Code: *code, Zone: *zone, Type: *kind}})
	}
	if err = rows.Err(); err != nil {
		return result, translate(err)
	}
	if !found {
		return result, &domain.ReferenceError{Field: "item_id", Cause: itemdomain.ErrNotFound}
	}
	return result, nil
}

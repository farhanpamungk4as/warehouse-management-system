package postgres

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"warehouse/backend/internal/modules/inventory/domain"
	"warehouse/backend/internal/platform/database"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool} }
func (r *Repository) Summary(ctx context.Context) (domain.Summary, error) {
	result := domain.Summary{Distribution: []domain.Distribution{}}
	var raw []byte
	err := r.pool.QueryRow(ctx, `WITH active AS (SELECT id FROM items WHERE deleted_at IS NULL), balances AS (SELECT s.* FROM stocks s JOIN active a ON a.id=s.item_id), distribution AS (SELECT l.id AS location_id,l.code,coalesce(sum(b.qty),0)::bigint AS qty FROM locations l LEFT JOIN balances b ON b.location_id=l.id GROUP BY l.id,l.code ORDER BY l.code,l.id)
 SELECT (SELECT count(*) FROM active),(SELECT coalesce(sum(qty),0)::bigint FROM balances),(SELECT count(*) FROM locations),(SELECT count(*) FROM balances WHERE qty>0),coalesce((SELECT json_agg(d ORDER BY d.code,d.location_id) FROM distribution d),'[]'::json)`).Scan(&result.ActiveItems, &result.AvailableQuantity, &result.Locations, &result.OccupiedStockPairs, &raw)
	if err != nil {
		return result, database.Classify(err)
	}
	err = json.Unmarshal(raw, &result.Distribution)
	return result, err
}

const joins = ` FROM stocks s JOIN items i ON i.id=s.item_id AND i.deleted_at IS NULL JOIN locations l ON l.id=s.location_id `
const filters = ` WHERE ($1='' OR i.sku ILIKE $1 ESCAPE '\' OR i.name ILIKE $1 ESCAPE '\') AND ($2='' OR i.category=$2) AND ($3=0 OR l.id=$3) `

func (r *Repository) Overview(ctx context.Context, p domain.Params) (domain.Overview, error) {
	result := domain.Overview{Rows: []domain.Row{}, Categories: []string{}}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, database.Classify(err)
	}
	defer tx.Rollback(context.Background())
	q := ""
	if p.Query != "" {
		q = "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(p.Query) + "%"
	}
	err = tx.QueryRow(ctx, "SELECT count(*)"+joins+filters, q, p.Category, p.LocationID).Scan(&result.Total)
	if err != nil {
		return result, database.Classify(err)
	}
	rows, err := tx.Query(ctx, `SELECT s.id,i.id,i.sku,i.name,i.category,i.unit,i.created_at,l.id,l.code,l.zone,l.type,s.qty,s.updated_at`+joins+filters+`ORDER BY i.id,l.id LIMIT $4 OFFSET $5`, q, p.Category, p.LocationID, p.Limit, (p.Page-1)*p.Limit)
	if err != nil {
		return result, database.Classify(err)
	}
	for rows.Next() {
		var v domain.Row
		err = rows.Scan(&v.ID, &v.Item.ID, &v.Item.SKU, &v.Item.Name, &v.Item.Category, &v.Item.Unit, &v.Item.CreatedAt, &v.Location.ID, &v.Location.Code, &v.Location.Zone, &v.Location.Type, &v.Qty, &v.UpdatedAt)
		if err != nil {
			rows.Close()
			return result, database.Classify(err)
		}
		result.Rows = append(result.Rows, v)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return result, database.Classify(err)
	}
	rows, err = tx.Query(ctx, `SELECT DISTINCT category FROM items WHERE deleted_at IS NULL ORDER BY category`)
	if err != nil {
		return result, database.Classify(err)
	}
	for rows.Next() {
		var cat string
		if err = rows.Scan(&cat); err != nil {
			rows.Close()
			return result, database.Classify(err)
		}
		result.Categories = append(result.Categories, cat)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return result, database.Classify(err)
	}
	return result, database.Classify(tx.Commit(ctx))
}

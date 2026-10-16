package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"warehouse/backend/internal/modules/item/domain"
	"warehouse/backend/internal/platform/database"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const columns = "id, sku, name, category, unit, created_at"

func scanItem(row pgx.Row) (domain.Item, error) {
	var v domain.Item
	err := row.Scan(&v.ID, &v.SKU, &v.Name, &v.Category, &v.Unit, &v.CreatedAt)
	v.CreatedAt = v.CreatedAt.UTC()
	return v, translate(err)
}

func translate(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == "items_sku_key" {
		return domain.ErrDuplicateSKU
	}
	return database.Classify(err)
}

func (r *Repository) Create(ctx context.Context, w domain.Write) (domain.Item, error) {
	return scanItem(r.pool.QueryRow(ctx, "INSERT INTO items(sku,name,category,unit) VALUES($1,$2,$3,$4) RETURNING "+columns, w.SKU, w.Name, w.Category, w.Unit))
}
func (r *Repository) Get(ctx context.Context, id int32) (domain.Item, error) {
	return scanItem(r.pool.QueryRow(ctx, "SELECT "+columns+" FROM items WHERE id=$1 AND deleted_at IS NULL", id))
}
func (r *Repository) Update(ctx context.Context, id int32, w domain.Write) (domain.Item, error) {
	return scanItem(r.pool.QueryRow(ctx, "UPDATE items SET sku=$2,name=$3,category=$4,unit=$5 WHERE id=$1 AND deleted_at IS NULL RETURNING "+columns, id, w.SKU, w.Name, w.Category, w.Unit))
}
func (r *Repository) Delete(ctx context.Context, id int32) (domain.Deleted, error) {
	var v domain.Deleted
	err := r.pool.QueryRow(ctx, "UPDATE items SET deleted_at=clock_timestamp() WHERE id=$1 AND deleted_at IS NULL RETURNING id,deleted_at", id).Scan(&v.ID, &v.DeletedAt)
	v.DeletedAt = v.DeletedAt.UTC()
	return v, translate(err)
}

const filtered = ` FROM items WHERE deleted_at IS NULL AND ($1::text='' OR category=$1) AND ($2::text='' OR sku ILIKE $2 ESCAPE '\' OR name ILIKE $2 ESCAPE '\')`

func (r *Repository) List(ctx context.Context, p domain.ListParams) (domain.ListResult, error) {
	result := domain.ListResult{Items: []domain.Item{}, Categories: []string{}}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, translate(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	pattern := ""
	if p.Query != "" {
		pattern = "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(p.Query) + "%"
	}
	rows, err := tx.Query(ctx, "SELECT "+columns+filtered+" ORDER BY id ASC LIMIT $3 OFFSET $4", p.Category, pattern, p.Limit, (p.Page-1)*p.Limit)
	if err != nil {
		return result, translate(err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Item, error) { return scanItem(row) })
	if err != nil {
		return result, translate(err)
	}
	result.Items = items
	if err = tx.QueryRow(ctx, "SELECT count(*)"+filtered, p.Category, pattern).Scan(&result.Total); err != nil {
		return result, translate(err)
	}
	rows, err = tx.Query(ctx, "SELECT DISTINCT category FROM items WHERE deleted_at IS NULL ORDER BY category ASC")
	if err != nil {
		return result, translate(err)
	}
	categories, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return result, translate(err)
	}
	result.Categories = categories
	if result.Items == nil {
		result.Items = []domain.Item{}
	}
	if result.Categories == nil {
		result.Categories = []string{}
	}
	if err = tx.Commit(ctx); err != nil {
		return result, translate(err)
	}
	return result, nil
}

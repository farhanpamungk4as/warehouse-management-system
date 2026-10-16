package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"warehouse/backend/internal/modules/location/domain"
	"warehouse/backend/internal/platform/database"
)

func translate(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		if pg.Code == "23505" && pg.ConstraintName == "locations_code_key" {
			return domain.ErrDuplicateCode
		}
		// PostgreSQL's RESTRICT FK serializes deletion against concurrent receipts
		// and transfers. No check-then-delete gap, and stock history is preserved.
		if pg.Code == "23503" && pg.ConstraintName == "stocks_location_id_fkey" {
			return domain.ErrInUse
		}
	}
	return database.Classify(err)
}
func scanLocation(row pgx.Row) (domain.Location, error) {
	var v domain.Location
	err := row.Scan(&v.ID, &v.Code, &v.Zone, &v.Type)
	return v, translate(err)
}
func (r *Repository) Get(ctx context.Context, id int32) (domain.Location, error) {
	return scanLocation(r.pool.QueryRow(ctx, "SELECT id,code,zone,type FROM locations WHERE id=$1", id))
}
func (r *Repository) Create(ctx context.Context, w domain.Write) (domain.Location, error) {
	return scanLocation(r.pool.QueryRow(ctx, "INSERT INTO locations(code,zone,type) VALUES($1,$2,$3) RETURNING id,code,zone,type", w.Code, w.Zone, w.Type))
}
func (r *Repository) Update(ctx context.Context, id int32, w domain.Write) (domain.Location, error) {
	return scanLocation(r.pool.QueryRow(ctx, "UPDATE locations SET code=$2,zone=$3,type=$4 WHERE id=$1 RETURNING id,code,zone,type", id, w.Code, w.Zone, w.Type))
}
func (r *Repository) Delete(ctx context.Context, id int32) (domain.Location, error) {
	return scanLocation(r.pool.QueryRow(ctx, "DELETE FROM locations WHERE id=$1 RETURNING id,code,zone,type", id))
}

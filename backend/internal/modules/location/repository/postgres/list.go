package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"warehouse/backend/internal/modules/location/domain"
	"warehouse/backend/internal/platform/database"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool} }
func (r *Repository) List(ctx context.Context) ([]domain.Location, error) {
	rows, err := r.pool.Query(ctx, "SELECT id,code,zone,type FROM locations ORDER BY code,id")
	if err != nil {
		return nil, database.Classify(err)
	}
	defer rows.Close()
	result := []domain.Location{}
	for rows.Next() {
		var v domain.Location
		if err := rows.Scan(&v.ID, &v.Code, &v.Zone, &v.Type); err != nil {
			return nil, database.Classify(err)
		}
		result = append(result, v)
	}
	return result, database.Classify(rows.Err())
}

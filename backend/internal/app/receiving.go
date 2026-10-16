package app

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	itempostgres "warehouse/backend/internal/modules/item/repository/postgres"
	locationpostgres "warehouse/backend/internal/modules/location/repository/postgres"
	stockpostgres "warehouse/backend/internal/modules/stock/repository/postgres"
	stockservice "warehouse/backend/internal/modules/stock/service"
)

// NewReceiveRunner is the application's actual cross-module transaction binding.
// The runner owns Begin/Commit/Rollback; these factories only use its exact tx.
func NewReceiveRunner(pool *pgxpool.Pool) *stockpostgres.ReceiveRunner {
	return stockpostgres.NewReceiveRunner(pool,
		func(tx pgx.Tx) (stockservice.ItemGuard, error) { return itempostgres.NewReceiveGuard(tx) },
		func(tx pgx.Tx) (stockservice.LocationLookup, error) { return locationpostgres.NewLookup(tx) },
	)
}

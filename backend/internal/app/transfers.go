package app

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	item "warehouse/backend/internal/modules/item/repository/postgres"
	location "warehouse/backend/internal/modules/location/repository/postgres"
	stock "warehouse/backend/internal/modules/stock/repository/postgres"
	"warehouse/backend/internal/modules/stock/service"
)

func NewTransferRunner(pool *pgxpool.Pool) *stock.TransferRunner {
	return stock.NewTransferRunner(pool, func(tx pgx.Tx) (service.ItemGuard, error) { return item.NewReceiveGuard(tx) }, func(tx pgx.Tx) (service.TransferLocations, error) { return location.NewTransferLookup(tx) })
}

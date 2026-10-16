package domain

import (
	"time"

	item "warehouse/backend/internal/modules/item/domain"
	location "warehouse/backend/internal/modules/location/domain"
)

type Distribution struct {
	LocationID int32  `json:"location_id"`
	Code       string `json:"code"`
	Qty        int64  `json:"qty"`
}
type Summary struct {
	ActiveItems        int64          `json:"active_items"`
	AvailableQuantity  int64          `json:"available_quantity"`
	Locations          int64          `json:"locations"`
	OccupiedStockPairs int64          `json:"occupied_stock_pairs"`
	Distribution       []Distribution `json:"distribution"`
}
type Row struct {
	ID        int32             `json:"id"`
	Item      item.Item         `json:"item"`
	Location  location.Location `json:"location"`
	Qty       int32             `json:"qty"`
	UpdatedAt time.Time         `json:"updated_at"`
}
type Params struct {
	item.ListParams
	LocationID int32
}
type Overview struct {
	Rows       []Row
	Total      int64
	Categories []string
}

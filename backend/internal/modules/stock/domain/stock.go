package domain

import (
	"time"

	locationdomain "warehouse/backend/internal/modules/location/domain"
)

const MaxQuantity int64 = 2147483647
const MaxID int64 = 2147483647

type Stock struct {
	ID         int32     `json:"id"`
	ItemID     int32     `json:"item_id"`
	LocationID int32     `json:"location_id"`
	Qty        int32     `json:"qty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ReceiveInput struct{ ItemID, LocationID, Qty int64 }

type Balance struct {
	Stock
	Location locationdomain.Location `json:"location"`
}

type Lookup struct {
	ItemID int32     `json:"item_id"`
	Stocks []Balance `json:"stocks"`
}

func ValidateID(field string, id int64) error {
	if id < 1 || id > MaxID {
		return &ValidationError{Fields: []FieldError{{field, "Must be a positive 32-bit integer"}}}
	}
	return nil
}

func ValidateReceive(input ReceiveInput) error {
	var fields []FieldError
	for _, field := range []struct {
		name       string
		value, max int64
	}{{"item_id", input.ItemID, MaxID}, {"location_id", input.LocationID, MaxID}, {"qty", input.Qty, MaxQuantity}} {
		if field.value < 1 || field.value > field.max {
			reason := "Must be a positive 32-bit integer"
			if field.name == "qty" {
				reason = "Must be a positive integer"
			}
			fields = append(fields, FieldError{field.name, reason})
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

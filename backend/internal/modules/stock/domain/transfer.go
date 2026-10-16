package domain

import "errors"

var ErrInsufficientStock = errors.New("insufficient source stock")

type TransferInput struct{ ItemID, FromLocationID, ToLocationID, Qty int64 }
type Transfer struct {
	ItemID         int32 `json:"item_id"`
	FromLocationID int32 `json:"from_location_id"`
	ToLocationID   int32 `json:"to_location_id"`
	Qty            int32 `json:"qty"`
	Source         Stock `json:"source"`
	Destination    Stock `json:"destination"`
}

func ValidateTransfer(in TransferInput) error {
	var fields []FieldError
	for _, f := range []struct {
		name  string
		value int64
	}{{"item_id", in.ItemID}, {"from_location_id", in.FromLocationID}, {"to_location_id", in.ToLocationID}, {"qty", in.Qty}} {
		if f.value < 1 || f.value > MaxQuantity {
			fields = append(fields, FieldError{f.name, "Must be a positive 32-bit integer"})
		}
	}
	if in.FromLocationID == in.ToLocationID {
		fields = append(fields, FieldError{"to_location_id", "Must differ from source location"})
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

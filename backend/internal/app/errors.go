package app

import (
	"context"
	"errors"
	"net/http"

	itemdomain "warehouse/backend/internal/modules/item/domain"
	locationdomain "warehouse/backend/internal/modules/location/domain"
	stockdomain "warehouse/backend/internal/modules/stock/domain"
	"warehouse/backend/internal/platform/database"
	"warehouse/backend/internal/platform/httpx"
)

func mapError(err error) *httpx.Failure {
	var unknown *stockdomain.CommitOutcomeUnknown
	if errors.As(err, &unknown) {
		if unknown.Operation == "transfer" {
			return httpx.NewFailure(500, "Stock transfer outcome unknown", "Reconcile stock before retrying; the transfer may have committed")
		}
		return httpx.NewFailure(500, "Stock receive outcome unknown", "Reconcile stock before retrying; the receipt may have committed")
	}
	var f *httpx.Failure
	if errors.As(err, &f) {
		return f
	}
	var stockValidation *stockdomain.ValidationError
	if errors.As(err, &stockValidation) {
		fields := make([]httpx.FieldError, 0, len(stockValidation.Fields))
		for _, field := range stockValidation.Fields {
			fields = append(fields, httpx.FieldError{Field: field.Field, Reason: field.Reason})
		}
		return &httpx.Failure{Status: 400, Message: "Validation failed", Errors: fields}
	}
	if errors.Is(err, stockdomain.ErrInsufficientStock) {
		return &httpx.Failure{Status: 409, Message: "Insufficient source stock", Errors: []httpx.FieldError{{Field: "qty", Reason: "Source balance is less than requested quantity"}}}
	}
	if errors.Is(err, stockdomain.ErrCapacityExceeded) {
		return &httpx.Failure{Status: 409, Message: "Stock quantity limit exceeded", Errors: []httpx.FieldError{{Field: "qty", Reason: "Cumulative quantity exceeds the storage limit"}}}
	}
	var reference *stockdomain.ReferenceError
	if errors.As(err, &reference) {
		if errors.Is(err, itemdomain.ErrNotFound) {
			return &httpx.Failure{Status: 404, Message: "Item not found", Errors: []httpx.FieldError{{Field: reference.Field, Reason: "Item does not exist"}}}
		}
		if errors.Is(err, locationdomain.ErrNotFound) {
			return &httpx.Failure{Status: 404, Message: "Location not found", Errors: []httpx.FieldError{{Field: reference.Field, Reason: "Location does not exist"}}}
		}
	}
	var validation *itemdomain.ValidationError
	var locationValidation *locationdomain.ValidationError
	if errors.As(err, &locationValidation) {
		fields := make([]httpx.FieldError, 0, len(locationValidation.Fields))
		for _, f := range locationValidation.Fields {
			fields = append(fields, httpx.FieldError{Field: f.Field, Reason: f.Reason})
		}
		return &httpx.Failure{Status: 400, Message: "Validation failed", Errors: fields}
	}
	if errors.Is(err, locationdomain.ErrNotFound) {
		return &httpx.Failure{Status: 404, Message: "Location not found", Errors: []httpx.FieldError{{Field: "id", Reason: "Location does not exist"}}}
	}
	if errors.Is(err, locationdomain.ErrDuplicateCode) {
		return &httpx.Failure{Status: 409, Message: "Location code already exists", Errors: []httpx.FieldError{{Field: "code", Reason: "Duplicate entry"}}}
	}
	if errors.Is(err, locationdomain.ErrInUse) {
		return &httpx.Failure{Status: 409, Message: "Location is in use", Errors: []httpx.FieldError{{Field: "id", Reason: "Locations with stock records cannot be deleted, including zero balances"}}}
	}
	if errors.As(err, &validation) {
		fields := make([]httpx.FieldError, 0, len(validation.Fields))
		for _, field := range validation.Fields {
			fields = append(fields, httpx.FieldError{Field: field.Field, Reason: field.Reason})
		}
		return &httpx.Failure{Status: 400, Message: "Validation failed", Errors: fields}
	}
	if errors.Is(err, itemdomain.ErrNotFound) {
		return &httpx.Failure{Status: 404, Message: "Item not found", Errors: []httpx.FieldError{{Field: "id", Reason: "Item does not exist"}}}
	}
	if errors.Is(err, itemdomain.ErrDuplicateSKU) {
		return &httpx.Failure{Status: 409, Message: "SKU already exists", Errors: []httpx.FieldError{{Field: "sku", Reason: "Duplicate entry"}}}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return httpx.NewFailure(413, "Request body too large", "Request exceeds the body limit")
	}
	if errors.Is(err, database.ErrUnavailable) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return httpx.NewFailure(503, "Service temporarily unavailable", "Operation could not be completed")
	}
	return httpx.NewFailure(500, "Internal server error", "An unexpected error occurred")
}

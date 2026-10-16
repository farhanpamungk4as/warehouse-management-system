package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/url"
	"strconv"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"warehouse/backend/internal/modules/stock/domain"
	"warehouse/backend/internal/platform/httpx"
)

func invalid(field, reason string) error {
	return &domain.ValidationError{Fields: []domain.FieldError{{Field: field, Reason: reason}}}
}
func noQuery(c *gin.Context) error {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil || len(values) > 0 {
		return invalid("request", "Query parameters are not supported")
	}
	return nil
}
func parseItemID(raw string) (int64, error) {
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, invalid("item_id", "Must be a positive 32-bit integer")
		}
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, invalid("item_id", "Must be a positive 32-bit integer")
	}
	return id, domain.ValidateID("item_id", id)
}
func parseReceive(c *gin.Context) (domain.ReceiveInput, error) {
	var input domain.ReceiveInput
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		return input, httpx.NewFailure(415, "Content type must be application/json", "Unsupported content type")
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return input, err
	}
	bad := func() (domain.ReceiveInput, error) {
		return input, invalid("request", "Expected one JSON object with exactly item_id, location_id and qty integers")
	}
	if !utf8.Valid(body) {
		return bad()
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return bad()
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return bad()
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return bad()
		}
		seen[key] = true
		var target *int64
		switch key {
		case "item_id":
			target = &input.ItemID
		case "location_id":
			target = &input.LocationID
		case "qty":
			target = &input.Qty
		default:
			return bad()
		}
		var value *int64
		if err = decoder.Decode(&value); err != nil || value == nil {
			return bad()
		}
		*target = *value
	}
	if _, err = decoder.Token(); err != nil {
		return bad()
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return bad()
	}
	return input, domain.ValidateReceive(input)
}

func parseTransfer(c *gin.Context) (domain.TransferInput, error) {
	var input domain.TransferInput
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		return input, httpx.NewFailure(415, "Content type must be application/json", "Unsupported content type")
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return input, err
	}
	bad := func() (domain.TransferInput, error) {
		return input, invalid("request", "Expected one JSON object with exactly item_id, from_location_id, to_location_id and qty integers")
	}
	if !utf8.Valid(body) {
		return bad()
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return bad()
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return bad()
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return bad()
		}
		seen[key] = true
		var target *int64
		switch key {
		case "item_id":
			target = &input.ItemID
		case "from_location_id":
			target = &input.FromLocationID
		case "to_location_id":
			target = &input.ToLocationID
		case "qty":
			target = &input.Qty
		default:
			return bad()
		}
		var value *int64
		if err = decoder.Decode(&value); err != nil || value == nil {
			return bad()
		}
		*target = *value
	}
	if _, err = decoder.Token(); err != nil {
		return bad()
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return bad()
	}
	return input, domain.ValidateTransfer(input)
}

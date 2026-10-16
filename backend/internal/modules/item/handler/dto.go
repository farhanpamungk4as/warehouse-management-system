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
	"warehouse/backend/internal/modules/item/domain"
	"warehouse/backend/internal/platform/httpx"
)

func invalid(field, reason string) error {
	return &domain.ValidationError{Fields: []domain.FieldError{{Field: field, Reason: reason}}}
}
func parseID(raw string) (int32, error) {
	n, err := positiveInteger(raw)
	if err != nil || n > domain.MaxID {
		return 0, invalid("id", "Must be a positive 32-bit integer")
	}
	return int32(n), nil
}
func positiveInteger(raw string) (int64, error) {
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, strconv.ErrSyntax
		}
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 1 {
		return 0, strconv.ErrRange
	}
	return n, nil
}
func parseQuery(c *gin.Context, list bool) (domain.ListParams, error) {
	p := domain.ListParams{Page: 1, Limit: 10}
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		return p, invalid("request", "Invalid query string")
	}
	for key, v := range values {
		if !list || len(v) != 1 {
			return p, invalid(key, "Unknown or duplicate query parameter")
		}
		switch key {
		case "page":
			p.Page, err = positiveInteger(v[0])
		case "limit":
			p.Limit, err = positiveInteger(v[0])
		case "category":
			p.Category = v[0]
		case "q":
			p.Query = v[0]
		default:
			return p, invalid(key, "Unknown query parameter")
		}
		if err != nil {
			return p, invalid(key, "Must be a positive integer")
		}
	}
	return domain.NormalizeList(p)
}
func parseWrite(c *gin.Context) (domain.Write, error) {
	var w domain.Write
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		return w, httpx.NewFailure(415, "Content type must be application/json", "Unsupported content type")
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return w, err
	}
	bad := func() (domain.Write, error) {
		return w, invalid("request", "Expected one JSON object with exactly sku, name, category and unit strings")
	}
	if !utf8.Valid(body) {
		return bad()
	}
	d := json.NewDecoder(bytes.NewReader(body))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return bad()
	}
	seen := map[string]bool{}
	for d.More() {
		token, err = d.Token()
		if err != nil {
			return bad()
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return bad()
		}
		seen[key] = true
		var target *string
		switch key {
		case "sku":
			target = &w.SKU
		case "name":
			target = &w.Name
		case "category":
			target = &w.Category
		case "unit":
			target = &w.Unit
		default:
			return bad()
		}
		var value *string
		if err = d.Decode(&value); err != nil || value == nil {
			return bad()
		}
		*target = *value
	}
	if _, err = d.Token(); err != nil {
		return bad()
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return bad()
	}
	return domain.NormalizeWrite(w)
}

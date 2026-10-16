package handler

import (
	"context"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"warehouse/backend/internal/modules/inventory/domain"
	item "warehouse/backend/internal/modules/item/domain"
	"warehouse/backend/internal/platform/httpx"
)

type UseCases interface {
	Summary(context.Context) (domain.Summary, error)
	Overview(context.Context, domain.Params) (domain.Overview, error)
}

func Register(r *gin.Engine, s UseCases) {
	r.GET("/api/v1/dashboard/summary", func(c *gin.Context) {
		if c.Request.URL.RawQuery != "" {
			httpx.Reject(c, httpx.NewFailure(400, "Validation failed", "Query parameters are not supported"))
			return
		}
		v, err := s.Summary(c.Request.Context())
		if err != nil {
			httpx.Reject(c, err)
			return
		}
		httpx.Success(c, 200, "Dashboard summary retrieved successfully", v)
	})
	r.GET("/api/v1/inventory/overview", func(c *gin.Context) {
		p := domain.Params{ListParams: item.ListParams{Page: 1, Limit: 10}}
		values, err := url.ParseQuery(c.Request.URL.RawQuery)
		if err != nil {
			httpx.Reject(c, httpx.NewFailure(400, "Validation failed", "Invalid query string"))
			return
		}
		for key, v := range values {
			bad := func() {
				httpx.Reject(c, &item.ValidationError{Fields: []item.FieldError{{Field: key, Reason: "Unknown, duplicate or invalid parameter"}}})
			}
			if len(v) != 1 {
				bad()
				return
			}
			switch key {
			case "q":
				p.Query = v[0]
			case "category":
				p.Category = v[0]
			case "page", "limit", "location_id":
				for _, ch := range v[0] {
					if ch < '0' || ch > '9' {
						bad()
						return
					}
				}
				n, e := strconv.ParseInt(v[0], 10, 64)
				if e != nil || n < 1 {
					bad()
					return
				}
				if key == "page" {
					p.Page = n
				} else if key == "limit" {
					p.Limit = n
				} else {
					if n > 2147483647 {
						bad()
						return
					}
					p.LocationID = int32(n)
				}
			default:
				bad()
				return
			}
		}
		normalized, err := item.NormalizeList(p.ListParams)
		if err != nil {
			httpx.Reject(c, err)
			return
		}
		p.ListParams = normalized
		result, err := s.Overview(c.Request.Context(), p)
		if err != nil {
			httpx.Reject(c, err)
			return
		}
		httpx.SuccessWithMeta(c, 200, "Inventory retrieved successfully", result.Rows, gin.H{"page": p.Page, "limit": p.Limit, "total": result.Total, "categories": result.Categories})
	})
}

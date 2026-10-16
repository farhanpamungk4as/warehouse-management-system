package handler

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"warehouse/backend/internal/modules/location/domain"
	"warehouse/backend/internal/platform/httpx"
)

type UseCase interface {
	List(context.Context) ([]domain.Location, error)
	Get(context.Context, int32) (domain.Location, error)
	Create(context.Context, domain.Write) (domain.Location, error)
	Update(context.Context, int32, domain.Write) (domain.Location, error)
	Delete(context.Context, int32) (domain.Location, error)
}

func Register(r *gin.Engine, s UseCase) {
	r.GET("/api/v1/locations", func(c *gin.Context) {
		if c.Request.URL.RawQuery != "" {
			httpx.Reject(c, httpx.NewFailure(400, "Validation failed", "Query parameters are not supported"))
			return
		}
		result, err := s.List(c.Request.Context())
		if err != nil {
			httpx.Reject(c, err)
			return
		}
		httpx.Success(c, 200, "Locations retrieved successfully", result)
	})
	pathID := func(c *gin.Context) (int32, error) {
		if c.Request.URL.RawQuery != "" {
			return 0, invalid("request", "Query parameters are not supported")
		}
		return parseID(c.Param("id"))
	}
	r.GET("/api/v1/locations/:id", func(c *gin.Context) {
		id, err := pathID(c)
		if err != nil {
			httpx.Reject(c, err)
			return
		}
		v, err := s.Get(c.Request.Context(), id)
		if err != nil {
			httpx.Reject(c, err)
			return
		}
		httpx.Success(c, 200, "Location retrieved successfully", v)
	})
	write := func(create bool) gin.HandlerFunc {
		return func(c *gin.Context) {
			if c.Request.URL.RawQuery != "" {
				httpx.Reject(c, invalid("request", "Query parameters are not supported"))
				return
			}
			var id int32
			var err error
			if !create {
				id, err = pathID(c)
				if err != nil {
					httpx.Reject(c, err)
					return
				}
			}
			w, err := parseWrite(c)
			if err != nil {
				httpx.Reject(c, err)
				return
			}
			var v domain.Location
			if create {
				v, err = s.Create(c.Request.Context(), w)
			} else {
				v, err = s.Update(c.Request.Context(), id, w)
			}
			if err != nil {
				httpx.Reject(c, err)
				return
			}
			status, message := 200, "Location updated successfully"
			if create {
				status, message = 201, "Location created successfully"
				c.Header("Location", "/api/v1/locations/"+strconv.FormatInt(int64(v.ID), 10))
			}
			httpx.Success(c, status, message, v)
		}
	}
	r.POST("/api/v1/locations", write(true))
	r.PUT("/api/v1/locations/:id", write(false))
	r.DELETE("/api/v1/locations/:id", func(c *gin.Context) {
		id, err := pathID(c)
		if err != nil {
			httpx.Reject(c, err)
			return
		}
		v, err := s.Delete(c.Request.Context(), id)
		if err != nil {
			httpx.Reject(c, err)
			return
		}
		httpx.Success(c, 200, "Location deleted successfully", v)
	})
}

package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	"warehouse/backend/internal/modules/stock/domain"
	"warehouse/backend/internal/platform/httpx"
)

type TransferUseCase interface {
	Transfer(context.Context, domain.TransferInput) (domain.Transfer, error)
}

func RegisterTransfer(r *gin.Engine, s TransferUseCase) {
	r.POST("/api/v1/stock/transfer", func(c *gin.Context) {
		if err := noQuery(c); err != nil {
			httpx.Reject(c, err)
			return
		}
		in, err := parseTransfer(c)
		if err != nil {
			httpx.Reject(c, err)
			return
		}
		result, err := s.Transfer(c.Request.Context(), in)
		if err != nil {
			httpx.Reject(c, err)
			return
		}
		httpx.Success(c, 200, "Stock transferred successfully", result)
	})
}

package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"warehouse/backend/internal/modules/stock/domain"
	"warehouse/backend/internal/platform/httpx"
)

type UseCases interface {
	Receive(context.Context, domain.ReceiveInput) (domain.Stock, error)
	Lookup(context.Context, int64) (domain.Lookup, error)
}
type Handler struct{ service UseCases }

func New(service UseCases) *Handler { return &Handler{service: service} }
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/receive", h.receive)
	r.GET("/:item_id", h.lookup)
}
func (h *Handler) receive(c *gin.Context) {
	if err := noQuery(c); err != nil {
		httpx.Reject(c, err)
		return
	}
	input, err := parseReceive(c)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	stock, err := h.service.Receive(c.Request.Context(), input)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Stock received successfully", stock)
}
func (h *Handler) lookup(c *gin.Context) {
	if err := noQuery(c); err != nil {
		httpx.Reject(c, err)
		return
	}
	id, err := parseItemID(c.Param("item_id"))
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	stock, err := h.service.Lookup(c.Request.Context(), id)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	if stock.Stocks == nil {
		stock.Stocks = []domain.Balance{}
	}
	httpx.Success(c, http.StatusOK, "Stock retrieved successfully", stock)
}

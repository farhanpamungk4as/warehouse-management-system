package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"warehouse/backend/internal/modules/item/domain"
	"warehouse/backend/internal/platform/httpx"
)

// UseCases allows HTTP validation to be tested independently of the service.
type UseCases interface {
	Create(context.Context, domain.Write) (domain.Item, error)
	List(context.Context, domain.ListParams) (domain.ListResult, error)
	Get(context.Context, int32) (domain.Item, error)
	Update(context.Context, int32, domain.Write) (domain.Item, error)
	Delete(context.Context, int32) (domain.Deleted, error)
}
type Handler struct{ service UseCases }

func New(service UseCases) *Handler { return &Handler{service: service} }
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("", h.create)
	r.GET("", h.list)
	r.GET("/:id", h.get)
	r.PUT("/:id", h.update)
	r.DELETE("/:id", h.delete)
}
func (h *Handler) create(c *gin.Context) {
	if _, err := parseQuery(c, false); err != nil {
		httpx.Reject(c, err)
		return
	}
	w, err := parseWrite(c)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	item, err := h.service.Create(c.Request.Context(), w)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	c.Header("Location", "/api/v1/items/"+strconv.FormatInt(int64(item.ID), 10))
	httpx.Success(c, http.StatusCreated, "Item created successfully", item)
}
func (h *Handler) list(c *gin.Context) {
	p, err := parseQuery(c, true)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	result, err := h.service.List(c.Request.Context(), p)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	if result.Items == nil {
		result.Items = []domain.Item{}
	}
	if result.Categories == nil {
		result.Categories = []string{}
	}
	httpx.SuccessWithMeta(c, http.StatusOK, "Items retrieved successfully", result.Items, listMeta{p.Page, p.Limit, result.Total, result.Categories})
}

type listMeta struct {
	Page       int64    `json:"page"`
	Limit      int64    `json:"limit"`
	Total      int64    `json:"total"`
	Categories []string `json:"categories"`
}

func pathID(c *gin.Context) (int32, error) {
	if _, err := parseQuery(c, false); err != nil {
		return 0, err
	}
	return parseID(c.Param("id"))
}
func (h *Handler) get(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Item retrieved successfully", item)
}
func (h *Handler) update(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	w, err := parseWrite(c)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, w)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Item updated successfully", item)
}
func (h *Handler) delete(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	item, err := h.service.Delete(c.Request.Context(), id)
	if err != nil {
		httpx.Reject(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Item deleted successfully", item)
}

package teachers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

// Handler adapts HTTP to the Service. It binds and validates input, calls the
// service, and renders the OpenAPI-shaped JSON — no business logic here, and the
// *gin.Context never leaves this file.
type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// RegisterRoutes mounts the teacher endpoints onto the given group (expected to
// be "/v1/teachers").
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("", h.List)
	rg.GET("/:slug", h.GetBySlug)
}

type listQuery struct {
	Language      string `form:"language"`
	Kind          string `form:"kind"`
	MaxPriceMinor *int64 `form:"max_price_minor"`
	Q             string `form:"q"`
	Sort          string `form:"sort"`
	Page          int    `form:"page"`
	PageSize      int    `form:"page_size"`
}

// List handles GET /v1/teachers.
func (h *Handler) List(c *gin.Context) {
	var q listQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		web.BadRequest(c, "Invalid query parameters.")
		return
	}

	if q.Kind != "" && !Kind(q.Kind).valid() {
		web.BadRequest(c, "kind must be 'professional' or 'community'.")
		return
	}
	if q.Sort != "" && !Sort(q.Sort).valid() {
		web.BadRequest(c, "sort must be one of recommended, price_asc, price_desc, rating_desc.")
		return
	}

	res, err := h.svc.List(c.Request.Context(), ListParams{
		Language:      q.Language,
		Kind:          Kind(q.Kind),
		MaxPriceMinor: q.MaxPriceMinor,
		Q:             q.Q,
		Sort:          Sort(q.Sort),
		Page:          q.Page,
		PageSize:      q.PageSize,
	})
	if err != nil {
		h.logger.Error("list teachers", slog.Any("error", err))
		web.Internal(c)
		return
	}

	c.JSON(http.StatusOK, toListResponse(res))
}

// GetBySlug handles GET /v1/teachers/:slug.
func (h *Handler) GetBySlug(c *gin.Context) {
	slug := c.Param("slug")

	t, err := h.svc.GetBySlug(c.Request.Context(), slug)
	switch {
	case errors.Is(err, ErrNotFound):
		web.NotFound(c, "No teacher with that slug.")
		return
	case err != nil:
		h.logger.Error("get teacher", slog.String("slug", slug), slog.Any("error", err))
		web.Internal(c)
		return
	}

	c.JSON(http.StatusOK, toProfile(*t))
}

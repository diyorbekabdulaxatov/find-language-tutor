package teachers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
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
// be "/v1/teachers"). The write routes and /me are gated by requireAuth
// (auth.RequireAuth); ownership is checked in the service.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	rg.GET("", h.List)
	rg.POST("", requireAuth, h.Create)
	rg.GET("/me", requireAuth, h.GetMine)
	rg.GET("/:slug", h.GetBySlug)
	rg.PATCH("/:slug", requireAuth, h.Update)
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

// GetMine handles GET /v1/teachers/me — the caller's own profile. RequireAuth
// has run. 404 (standard envelope) when the account has not claimed a profile.
func (h *Handler) GetMine(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}

	t, err := h.svc.GetOwnProfile(c.Request.Context(), uid)
	switch {
	case errors.Is(err, ErrNotFound):
		web.NotFound(c, "You have not created a teacher profile yet.")
		return
	case err != nil:
		h.logger.Error("get own teacher profile", slog.Any("error", err))
		web.Internal(c)
		return
	}

	c.JSON(http.StatusOK, toProfile(*t))
}

// Create handles POST /v1/teachers — claim/create the caller's profile.
// RequireAuth has run.
func (h *Handler) Create(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}

	var req createProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, "Request body is not valid JSON for a teacher profile.")
		return
	}

	t, err := h.svc.Create(c.Request.Context(), uid, req.toInput())
	var ve ValidationError
	switch {
	case errors.Is(err, ErrProfileExists):
		web.WriteError(c, http.StatusConflict, "profile_exists", "Your account already has a teacher profile.")
		return
	case errors.As(err, &ve):
		web.BadRequest(c, ve.Error())
		return
	case err != nil:
		h.logger.Error("create teacher profile", slog.Any("error", err))
		web.Internal(c)
		return
	}

	c.JSON(http.StatusCreated, toProfile(*t))
}

// Update handles PATCH /v1/teachers/:slug — edit the caller's own profile.
// RequireAuth has run; the service checks ownership.
func (h *Handler) Update(c *gin.Context) {
	slug := c.Param("slug")

	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}

	var req patchProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, "Request body is not valid JSON for a teacher profile patch.")
		return
	}

	t, err := h.svc.Update(c.Request.Context(), slug, uid, req.toPatch())
	var ve ValidationError
	switch {
	case errors.Is(err, ErrNotFound):
		web.NotFound(c, "No teacher with that slug.")
		return
	case errors.Is(err, ErrNotOwner):
		web.Forbidden(c, "You can only edit your own teacher profile.")
		return
	case errors.As(err, &ve):
		web.BadRequest(c, ve.Error())
		return
	case err != nil:
		h.logger.Error("update teacher profile", slog.String("slug", slug), slog.Any("error", err))
		web.Internal(c)
		return
	}

	c.JSON(http.StatusOK, toProfile(*t))
}

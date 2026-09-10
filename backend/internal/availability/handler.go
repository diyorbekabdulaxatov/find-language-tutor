package availability

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

// RegisterRoutes mounts the availability endpoints onto the given group
// (expected to be "/v1/teachers"), alongside the teacher-profile routes. The
// PUT route is gated by requireAuth (auth.RequireAuth) and an ownership check
// in the handler.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	rg.GET("/:slug/availability", h.Get)
	rg.PUT("/:slug/availability", requireAuth, h.Replace)
}

// Get handles GET /v1/teachers/:slug/availability.
func (h *Handler) Get(c *gin.Context) {
	slug := c.Param("slug")

	wa, err := h.svc.GetBySlug(c.Request.Context(), slug)
	switch {
	case errors.Is(err, ErrTeacherNotFound):
		web.NotFound(c, "No teacher with that slug.")
		return
	case err != nil:
		h.logger.Error("get availability", slog.String("slug", slug), slog.Any("error", err))
		web.Internal(c)
		return
	}

	c.JSON(http.StatusOK, toAvailabilityDTO(wa))
}

// Replace handles PUT /v1/teachers/:slug/availability — replace the full weekly
// set. RequireAuth has run; the service checks that the caller owns the profile.
func (h *Handler) Replace(c *gin.Context) {
	slug := c.Param("slug")

	userID, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}

	var req replaceRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"slots": [{"weekday", "start_minute", "end_minute"}, ...]}.`)
		return
	}

	wa, err := h.svc.Replace(c.Request.Context(), slug, userID, fromSlotDTOs(req.Slots))
	var ve ValidationError
	switch {
	case errors.Is(err, ErrTeacherNotFound):
		web.NotFound(c, "No teacher with that slug.")
		return
	case errors.Is(err, ErrNotOwner):
		web.Forbidden(c, "You can only edit your own availability.")
		return
	case errors.As(err, &ve):
		web.BadRequest(c, ve.Error())
		return
	case err != nil:
		h.logger.Error("replace availability", slog.String("slug", slug), slog.Any("error", err))
		web.Internal(c)
		return
	}

	c.JSON(http.StatusOK, toAvailabilityDTO(wa))
}

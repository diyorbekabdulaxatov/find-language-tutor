package availability

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

// RegisterRoutes mounts the availability endpoints onto the given group
// (expected to be "/v1/teachers"), alongside the teacher-profile routes.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/:slug/availability", h.Get)
	rg.PUT("/:slug/availability", h.Replace)
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

// Replace handles PUT /v1/teachers/:slug/availability — replace the full weekly set.
func (h *Handler) Replace(c *gin.Context) {
	slug := c.Param("slug")

	// TODO(auth): the backend README calls for a RequireAuth() middleware in
	// internal/httpapi guarding the mutating routes once Clerk/Auth0 is wired.
	// Until real JWT verification exists, a teacher proves ownership of the
	// profile by sending an X-Teacher-Slug header matching the slug being
	// edited. Replace this stand-in with the middleware + a caller-identity
	// check when auth lands.
	if c.GetHeader("X-Teacher-Slug") != slug {
		web.Forbidden(c, "You can only edit your own availability.")
		return
	}

	var req replaceRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"slots": [{"weekday", "start_minute", "end_minute"}, ...]}.`)
		return
	}

	wa, err := h.svc.Replace(c.Request.Context(), slug, fromSlotDTOs(req.Slots))
	var ve ValidationError
	switch {
	case errors.Is(err, ErrTeacherNotFound):
		web.NotFound(c, "No teacher with that slug.")
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

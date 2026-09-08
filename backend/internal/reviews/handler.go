package reviews

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

// Handler adapts HTTP to the Service: bind + validate, call the service, render
// the OpenAPI-shaped JSON. No business logic; the *gin.Context never leaves
// this file.
type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{svc: svc, logger: logger}
}

// RegisterBookingRoutes mounts POST /:id/review onto the "/v1/bookings" group.
// It requires a valid access token; the student-only check is in the service.
func RegisterBookingRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	rg.POST("/:id/review", requireAuth, h.Create)
}

// RegisterTeacherRoutes mounts GET /:slug/reviews onto the "/v1/teachers"
// group, next to the profile / availability / slots routes. Public.
func RegisterTeacherRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/:slug/reviews", h.ListForTeacher)
}

// Create handles POST /v1/bookings/:id/review. Body {"rating": 1-5, "comment"}.
func (h *Handler) Create(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The booking id must be a UUID.")
		return
	}

	var req createReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"rating": 1-5, "comment"?: string}.`)
		return
	}

	r, err := h.svc.Create(c.Request.Context(), uid, bookingID, req.Rating, req.Comment)
	if h.rendered(c, err, "create review", slog.String("booking_id", bookingID.String())) {
		return
	}
	c.JSON(http.StatusCreated, toCreatedReviewDTO(r))
}

// ListForTeacher handles GET /v1/teachers/:slug/reviews?page&page_size. Public.
func (h *Handler) ListForTeacher(c *gin.Context) {
	slug := c.Param("slug")

	page, err := optionalInt(c.Query("page"))
	if err != nil {
		web.BadRequest(c, "`page` must be an integer.")
		return
	}
	pageSize, err := optionalInt(c.Query("page_size"))
	if err != nil {
		web.BadRequest(c, "`page_size` must be an integer.")
		return
	}

	res, err := h.svc.ListForTeacher(c.Request.Context(), slug, page, pageSize)
	if h.rendered(c, err, "list teacher reviews", slog.String("slug", slug)) {
		return
	}
	c.JSON(http.StatusOK, toReviewListDTO(res))
}

// rendered maps a service error to an HTTP response. Returns true when it wrote
// one (the caller should stop).
func (h *Handler) rendered(c *gin.Context, err error, op string, attrs ...slog.Attr) bool {
	if err == nil {
		return false
	}

	var ve ValidationError
	switch {
	case errors.Is(err, ErrTeacherNotFound):
		web.NotFound(c, "No teacher with that slug.")
	case errors.Is(err, ErrBookingNotFound):
		web.NotFound(c, "No booking with that id.")
	case errors.Is(err, ErrNotStudent):
		web.Forbidden(c, "Only the student who took this lesson can review it.")
	case errors.Is(err, ErrBookingNotCompleted):
		web.WriteError(c, http.StatusConflict, "booking_not_completed", "You can only review a lesson that has been completed.")
	case errors.Is(err, ErrAlreadyReviewed):
		web.WriteError(c, http.StatusConflict, "already_reviewed", "You have already reviewed this lesson.")
	case errors.As(err, &ve):
		web.BadRequest(c, ve.Error())
	default:
		anys := make([]any, 0, len(attrs)+1)
		for _, a := range attrs {
			anys = append(anys, a)
		}
		anys = append(anys, slog.Any("error", err))
		h.logger.Error(op, anys...)
		web.Internal(c)
	}
	return true
}

func optionalInt(v string) (int, error) {
	if v == "" {
		return 0, nil
	}
	return strconv.Atoi(v)
}

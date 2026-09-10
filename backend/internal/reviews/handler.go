package reviews

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
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

// RegisterAdminRoutes mounts the moderation surface onto the given group
// (expected to be "/v1/admin", already behind auth.RequireAuth). Every route
// carries the `reviews.moderate` permission check.
func RegisterAdminRoutes(rg *gin.RouterGroup, h *Handler, guard *rbac.Guard) {
	rg.GET("/reviews", guard.Require(rbac.PermReviewsModerate), h.ModerationQueue)
	rg.POST("/reviews/:id/hide", guard.Require(rbac.PermReviewsModerate), h.Hide)
	rg.POST("/reviews/:id/unhide", guard.Require(rbac.PermReviewsModerate), h.Unhide)
	rg.DELETE("/reviews/:id", guard.Require(rbac.PermReviewsModerate), h.Remove)
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

// ModerationQueue handles GET /v1/admin/reviews?visibility&teacher&max_rating&page&page_size.
func (h *Handler) ModerationQueue(c *gin.Context) {
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
	maxRating, err := optionalInt(c.Query("max_rating"))
	if err != nil {
		web.BadRequest(c, "`max_rating` must be an integer.")
		return
	}

	res, err := h.svc.Moderate(c.Request.Context(), ModerationQuery{
		Visibility:  Visibility(c.Query("visibility")),
		TeacherSlug: c.Query("teacher"),
		MaxRating:   maxRating,
		Page:        page,
		PageSize:    pageSize,
	})
	if h.rendered(c, err, "list review moderation queue") {
		return
	}
	c.JSON(http.StatusOK, toAdminReviewListDTO(res))
}

// Hide handles POST /v1/admin/reviews/:id/hide.
func (h *Handler) Hide(c *gin.Context) { h.setHidden(c, true) }

// Unhide handles POST /v1/admin/reviews/:id/unhide.
func (h *Handler) Unhide(c *gin.Context) { h.setHidden(c, false) }

func (h *Handler) setHidden(c *gin.Context, hidden bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The review id must be a UUID.")
		return
	}
	var r AdminReview
	if hidden {
		r, err = h.svc.Hide(c.Request.Context(), id)
	} else {
		r, err = h.svc.Unhide(c.Request.Context(), id)
	}
	if h.rendered(c, err, "moderate review", slog.String("review_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toAdminReviewDTO(r))
}

// Remove handles DELETE /v1/admin/reviews/:id.
func (h *Handler) Remove(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The review id must be a UUID.")
		return
	}
	if h.rendered(c, h.svc.Remove(c.Request.Context(), id), "remove review", slog.String("review_id", id.String())) {
		return
	}
	c.Status(http.StatusNoContent)
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
	case errors.Is(err, ErrReviewNotFound):
		web.NotFound(c, "No review with that id.")
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

package courses

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

// RegisterReviewRoutes mounts the phase-D2 review surface onto "/v1/courses".
// The public list needs no auth; writing one does (enrollment is checked in
// the service).
func RegisterReviewRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	rg.GET("/:id/reviews", h.Reviews)
	g := rg.Group("", requireAuth)
	g.POST("/:id/review", h.CreateReview)
	g.PATCH("/:id/review", h.UpdateReview)
}

// Reviews handles GET /v1/courses/:id/reviews — public, paged, visible rows
// only, for a course that is actually on the storefront.
func (h *Handler) Reviews(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.NotFound(c, "No course with that id.")
		return
	}
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
	res, err := h.svc.Reviews(c.Request.Context(), id, page, pageSize)
	if h.rendered(c, err, "course reviews", slog.String("course_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toCourseReviewListDTO(res))
}

// CreateReview handles POST /v1/courses/:id/review. Enrolled buyers only;
// 409 when they have already reviewed it (PATCH instead).
func (h *Handler) CreateReview(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	var req createCourseReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"rating": 1-5, "comment"?: string}.`)
		return
	}
	r, err := h.svc.CreateReview(c.Request.Context(), uid, id, req.Rating, req.Comment)
	if h.rendered(c, err, "create course review", slog.String("course_id", id.String())) {
		return
	}
	c.JSON(http.StatusCreated, toCourseReviewDTO(r))
}

// UpdateReview handles PATCH /v1/courses/:id/review — the author revising
// their own standing opinion.
func (h *Handler) UpdateReview(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	var req createCourseReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"rating": 1-5, "comment"?: string}.`)
		return
	}
	r, err := h.svc.UpdateReview(c.Request.Context(), uid, id, req.Rating, req.Comment)
	if h.rendered(c, err, "update course review", slog.String("course_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toCourseReviewDTO(r))
}

// AdminReviews handles GET /v1/admin/course-reviews.
func (h *Handler) AdminReviews(c *gin.Context) {
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
	q := AdminReviewQuery{
		Visibility: ReviewVisibility(c.Query("visibility")),
		MaxRating:  maxRating,
		Page:       page,
		PageSize:   pageSize,
	}
	if v := c.Query("course_id"); v != "" {
		courseID, err := uuid.Parse(v)
		if err != nil {
			web.BadRequest(c, "`course_id` must be a UUID.")
			return
		}
		q.CourseID = &courseID
	}
	res, err := h.svc.AdminReviews(c.Request.Context(), q)
	if h.rendered(c, err, "admin course reviews") {
		return
	}
	c.JSON(http.StatusOK, toAdminCourseReviewListDTO(res))
}

// setReviewHidden backs POST /v1/admin/course-reviews/:id/{hide,unhide}.
func (h *Handler) setReviewHidden(hidden bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			web.NotFound(c, "No review with that id.")
			return
		}
		r, err := h.svc.SetReviewHidden(c.Request.Context(), id, hidden)
		if h.rendered(c, err, "set course review hidden", slog.String("review_id", id.String())) {
			return
		}
		c.JSON(http.StatusOK, toAdminCourseReviewDTO(r))
	}
}

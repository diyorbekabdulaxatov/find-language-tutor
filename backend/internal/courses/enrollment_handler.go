package courses

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

// RegisterCatalogRoutes mounts the public catalog + cover-image routes onto
// the given group (expected to be "/v1/courses", the same group RegisterRoutes
// uses for the authoring routes — static paths like "catalog" and "cover"
// coexist fine alongside the ":id" param routes in gin's tree). Catalog list
// and the cover image need no auth at all; the catalog detail page uses
// optionalAuth so it can personalise is_enrolled / is_owner for a logged-in
// viewer without requiring a login.
func RegisterCatalogRoutes(rg *gin.RouterGroup, h *Handler, optionalAuth gin.HandlerFunc) {
	rg.GET("/catalog", h.Catalog)
	rg.GET("/catalog/:id", optionalAuth, h.CatalogDetail)
	rg.GET("/:id/cover", h.Cover)
	rg.GET("/:id/items/:itemId/preview", h.Preview)
}

// RegisterLearnerRoutes mounts the purchase / player / progress routes onto
// the given group (expected to be "/v1/courses"). Every route requires a
// valid access token; enrollment/ownership is checked in the service.
func RegisterLearnerRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	g := rg.Group("", requireAuth)
	g.POST("/:id/purchase", h.Purchase)
	g.GET("/:id/learn", h.Learn)
	g.POST("/:id/items/:itemId/progress", h.RecordProgress)
}

// RegisterEnrollmentRoutes mounts the caller's "my learning" list onto its
// own top-level group (expected to be "/v1/enrollments").
func RegisterEnrollmentRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	rg.GET("", requireAuth, h.MyEnrollments)
}

// Catalog handles GET /v1/courses/catalog. Public.
func (h *Handler) Catalog(c *gin.Context) {
	var maxPrice *int64
	if v := c.Query("max_price_minor"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			web.BadRequest(c, "`max_price_minor` must be an integer.")
			return
		}
		maxPrice = &n
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
	res, err := h.svc.Catalog(c.Request.Context(), CatalogQuery{
		Q: c.Query("q"), MaxPriceMinor: maxPrice, Sort: c.Query("sort"), Page: page, PageSize: pageSize,
	})
	if h.rendered(c, err, "course catalog") {
		return
	}
	c.JSON(http.StatusOK, toCatalogListDTO(res))
}

// CatalogDetail handles GET /v1/courses/catalog/:id. Public with optional
// auth: a logged-in viewer's is_enrolled / is_owner are personalised; an
// anonymous viewer sees both as false. 404 unless the course is published.
func (h *Handler) CatalogDetail(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.NotFound(c, "No course with that id.")
		return
	}
	uid, _ := auth.UserID(c) // uuid.Nil when unauthenticated
	detail, err := h.svc.CatalogDetail(c.Request.Context(), uid, id)
	if h.rendered(c, err, "course catalog detail", slog.String("course_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toCatalogDetailDTO(detail))
}

// Cover handles GET /v1/courses/:id/cover. Public, no auth: 302/streams the
// cover image, or 404 when the course isn't published or has no cover.
func (h *Handler) Cover(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.NotFound(c, "No course with that id.")
		return
	}
	redirectURL, body, contentType, err := h.svc.CoverImage(c.Request.Context(), id)
	if h.rendered(c, err, "course cover", slog.String("course_id", id.String())) {
		return
	}
	if redirectURL != "" {
		c.Redirect(http.StatusFound, redirectURL)
		return
	}
	defer body.Close()
	c.DataFromReader(http.StatusOK, -1, contentType, body, nil)
}

// Preview handles GET /v1/courses/:id/items/:itemId/preview — a free sample
// lecture, no auth, so an anonymous shopper's <video> can load it straight
// from the landing page. Disk-backed files go through http.ServeContent so
// the player can seek (Range requests); R2 redirects to a signed URL. Same
// shape as the public teacher-media route.
//
// Every failure is a flat 404: the service already collapses "not a preview",
// "not on the storefront" and "no such item" into ErrItemNotFound so this
// route tells a prober nothing.
func (h *Handler) Preview(c *gin.Context) {
	courseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.NotFound(c, "No preview lesson with that id.")
		return
	}
	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		web.NotFound(c, "No preview lesson with that id.")
		return
	}

	redirectURL, body, contentType, err := h.svc.PreviewVideo(c.Request.Context(), courseID, itemID)
	switch {
	case errors.Is(err, ErrItemNotFound), errors.Is(err, ErrNotFound):
		web.NotFound(c, "No preview lesson with that id.")
		return
	case err != nil:
		h.logger.Error("course preview", slog.String("course_id", courseID.String()),
			slog.String("item_id", itemID.String()), slog.Any("error", err))
		web.Internal(c)
		return
	}
	if redirectURL != "" {
		c.Redirect(http.StatusFound, redirectURL)
		return
	}
	defer body.Close()

	c.Header("Cache-Control", "public, max-age=300")
	c.Header("Content-Type", contentType)
	if rs, ok := body.(io.ReadSeeker); ok {
		http.ServeContent(c.Writer, c.Request, "", time.Time{}, rs)
		return
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, body)
}

// Purchase handles POST /v1/courses/:id/purchase. Body
// {"method_token"?: string} — omit/ignore for a free course. 200 if already
// enrolled or the course is free; 201 on a new paid purchase.
func (h *Handler) Purchase(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	var req purchaseCourseRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			web.BadRequest(c, `Request body must be {"method_token"?: string}.`)
			return
		}
	}
	summary, justPurchased, err := h.svc.Purchase(c.Request.Context(), uid, id, req.MethodToken)
	if h.rendered(c, err, "purchase course", slog.String("course_id", id.String())) {
		return
	}
	status := http.StatusOK
	if justPurchased {
		status = http.StatusCreated
	}
	c.JSON(status, toCourseEnrollmentDTO(summary))
}

// MyEnrollments handles GET /v1/enrollments — the caller's "my learning" list.
func (h *Handler) MyEnrollments(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}
	rows, err := h.svc.MyEnrollments(c.Request.Context(), uid)
	if h.rendered(c, err, "my enrollments") {
		return
	}
	c.JSON(http.StatusOK, toCourseEnrollmentListDTO(rows))
}

// Learn handles GET /v1/courses/:id/learn. Enrolled-or-owner only.
func (h *Handler) Learn(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	detail, err := h.svc.Learn(c.Request.Context(), uid, id)
	if h.rendered(c, err, "learn course", slog.String("course_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toLearnDetailDTO(detail))
}

// RecordProgress handles POST /v1/courses/:id/items/:itemId/progress. Body
// {"position_seconds"?: int, "completed"?: bool}. Enrolled only, video items
// only (400 for a resource item — those complete through their submission).
func (h *Handler) RecordProgress(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		web.BadRequest(c, "The item id must be a UUID.")
		return
	}
	var req recordProgressRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			web.BadRequest(c, `Request body must be {"position_seconds"?: integer, "completed"?: boolean}.`)
			return
		}
	}
	p, err := h.svc.RecordProgress(c.Request.Context(), uid, id, itemID, req.PositionSeconds, req.Completed)
	if h.rendered(c, err, "record course item progress", slog.String("item_id", itemID.String())) {
		return
	}
	c.JSON(http.StatusOK, toItemProgressDTO(p))
}

package resources

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

// Handler adapts HTTP to the Service. The *gin.Context never leaves this file.
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

// RegisterRoutes mounts the library onto "/v1/resources". Every route needs a
// valid access token; the caller must own a teacher profile (checked in the
// service).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	g := rg.Group("", requireAuth)
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.PATCH("/:id", h.Update)
	g.POST("/:id/publish", h.publish(true))
	g.POST("/:id/unpublish", h.publish(false))
	g.POST("/:id/archive", h.archive(true))
	g.POST("/:id/unarchive", h.archive(false))
	g.DELETE("/:id", h.Delete)
}

// RegisterBookingRoutes mounts the lesson-attachment routes onto the given
// group (expected to be "/v1/bookings"), next to the booking lifecycle,
// review and dispute routes. Every route requires a valid access token; the
// participant / teacher-owner check happens in the service.
func RegisterBookingRoutes(bookingRoutes *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	bookingRoutes.POST("/:id/resources", requireAuth, h.Attach)
	bookingRoutes.GET("/:id/resources", requireAuth, h.ListForBooking)
	bookingRoutes.DELETE("/:id/resources/:attId", requireAuth, h.Detach)
}

// RegisterSubmissionRoutes mounts the submission routes onto their own
// top-level group (expected to be "/v1/submissions"). Every route requires a
// valid access token.
func RegisterSubmissionRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	g := rg.Group("", requireAuth)
	g.POST("", h.StartSubmission)
	g.PATCH("/:id", h.SaveAnswers)
	g.POST("/:id/submit", h.Submit)
	g.GET("/:id", h.GetSubmission)
	g.GET("", h.Inbox)
	g.POST("/:id/grade", h.Grade)
}

func (h *Handler) Create(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, "Request body is not valid JSON.")
		return
	}
	r, err := h.svc.Create(c.Request.Context(), uid, Type(req.Type), req.Title, req.Instructions, req.Content, req.Publish)
	if h.rendered(c, err, "create resource") {
		return
	}
	c.JSON(http.StatusCreated, toResourceDTO(r))
}

func (h *Handler) List(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
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

	res, err := h.svc.Library(c.Request.Context(), uid, ListQuery{
		Type:            Type(c.Query("type")),
		Status:          Status(c.Query("status")),
		IncludeArchived: c.Query("archived") == "true",
		Page:            page,
		PageSize:        pageSize,
	})
	if h.rendered(c, err, "list resources") {
		return
	}
	c.JSON(http.StatusOK, toResourceListDTO(res))
}

func (h *Handler) Get(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	r, err := h.svc.Get(c.Request.Context(), uid, id)
	if h.rendered(c, err, "get resource", slog.String("resource_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toResourceDTO(r))
}

func (h *Handler) Update(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, "Request body is not valid JSON.")
		return
	}
	r, err := h.svc.Update(c.Request.Context(), uid, id, req.Title, req.Instructions, req.Content)
	if h.rendered(c, err, "update resource", slog.String("resource_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toResourceDTO(r))
}

func (h *Handler) publish(published bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, id, ok := h.callerAndID(c)
		if !ok {
			return
		}
		r, err := h.svc.SetPublished(c.Request.Context(), uid, id, published)
		if h.rendered(c, err, "set resource published", slog.String("resource_id", id.String())) {
			return
		}
		c.JSON(http.StatusOK, toResourceDTO(r))
	}
}

func (h *Handler) archive(archived bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, id, ok := h.callerAndID(c)
		if !ok {
			return
		}
		r, err := h.svc.SetArchived(c.Request.Context(), uid, id, archived)
		if h.rendered(c, err, "set resource archived", slog.String("resource_id", id.String())) {
			return
		}
		c.JSON(http.StatusOK, toResourceDTO(r))
	}
}

func (h *Handler) Delete(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	if h.rendered(c, h.svc.Delete(c.Request.Context(), uid, id), "delete resource", slog.String("resource_id", id.String())) {
		return
	}
	c.Status(http.StatusNoContent)
}

// --- phase A2: booking attachment ---

// Attach handles POST /v1/bookings/:id/resources.
// Body {"resource_id", "kind": "material"|"homework", "due_at"?}.
func (h *Handler) Attach(c *gin.Context) {
	uid, bookingID, ok := h.callerAndBookingID(c)
	if !ok {
		return
	}
	var req attachResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"resource_id", "kind": "material"|"homework", "due_at"?}.`)
		return
	}
	resourceID, err := uuid.Parse(req.ResourceID)
	if err != nil {
		web.BadRequest(c, "`resource_id` must be a UUID.")
		return
	}
	br, err := h.svc.Attach(c.Request.Context(), uid, bookingID, resourceID, req.Kind, req.DueAt)
	if h.rendered(c, err, "attach booking resource", slog.String("booking_id", bookingID.String())) {
		return
	}
	c.JSON(http.StatusCreated, toAttachedResourceDTO(AttachedResource{BookingResource: br}))
}

// ListForBooking handles GET /v1/bookings/:id/resources. Participant-only.
func (h *Handler) ListForBooking(c *gin.Context) {
	uid, bookingID, ok := h.callerAndBookingID(c)
	if !ok {
		return
	}
	items, err := h.svc.ForBooking(c.Request.Context(), uid, bookingID)
	if h.rendered(c, err, "list booking resources", slog.String("booking_id", bookingID.String())) {
		return
	}
	c.JSON(http.StatusOK, toAttachedResourceListDTO(items))
}

// Detach handles DELETE /v1/bookings/:id/resources/:attId. Teacher-owner only.
func (h *Handler) Detach(c *gin.Context) {
	uid, bookingID, ok := h.callerAndBookingID(c)
	if !ok {
		return
	}
	attID, err := uuid.Parse(c.Param("attId"))
	if err != nil {
		web.BadRequest(c, "The attachment id must be a UUID.")
		return
	}
	err = h.svc.Detach(c.Request.Context(), uid, bookingID, attID)
	if h.rendered(c, err, "detach booking resource", slog.String("booking_id", bookingID.String())) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) callerAndBookingID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The booking id must be a UUID.")
		return uuid.Nil, uuid.Nil, false
	}
	return uid, id, true
}

// --- phase A3: submissions ---

// StartSubmission handles POST /v1/submissions. Body {"resource_id", "booking_id"}.
func (h *Handler) StartSubmission(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}
	var req startSubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"resource_id", "booking_id"}.`)
		return
	}
	resourceID, err := uuid.Parse(req.ResourceID)
	if err != nil {
		web.BadRequest(c, "`resource_id` must be a UUID.")
		return
	}
	bookingID, err := uuid.Parse(req.BookingID)
	if err != nil {
		web.BadRequest(c, "`booking_id` must be a UUID.")
		return
	}
	sub, err := h.svc.StartSubmission(c.Request.Context(), uid, resourceID, bookingID)
	if h.rendered(c, err, "start submission", slog.String("booking_id", bookingID.String())) {
		return
	}
	c.JSON(http.StatusCreated, toSubmissionDTO(sub))
}

// SaveAnswers handles PATCH /v1/submissions/:id. Body {"answers": object}.
func (h *Handler) SaveAnswers(c *gin.Context) {
	uid, id, ok := h.callerAndSubmissionID(c)
	if !ok {
		return
	}
	var req saveAnswersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"answers": object}.`)
		return
	}
	sub, err := h.svc.SaveAnswers(c.Request.Context(), uid, id, req.Answers)
	if h.rendered(c, err, "save submission answers", slog.String("submission_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toSubmissionDTO(sub))
}

// Submit handles POST /v1/submissions/:id/submit.
func (h *Handler) Submit(c *gin.Context) {
	uid, id, ok := h.callerAndSubmissionID(c)
	if !ok {
		return
	}
	sub, err := h.svc.Submit(c.Request.Context(), uid, id)
	if h.rendered(c, err, "submit submission", slog.String("submission_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toSubmissionDTO(sub))
}

// GetSubmission handles GET /v1/submissions/:id. The owning student or the
// booking's teacher-owner only.
func (h *Handler) GetSubmission(c *gin.Context) {
	uid, id, ok := h.callerAndSubmissionID(c)
	if !ok {
		return
	}
	sub, err := h.svc.GetSubmission(c.Request.Context(), uid, id)
	if h.rendered(c, err, "get submission", slog.String("submission_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toSubmissionDTO(sub))
}

// Inbox handles GET /v1/submissions?status&page&page_size. Teacher only (the
// caller's own resources' submissions).
func (h *Handler) Inbox(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
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
	res, err := h.svc.Inbox(c.Request.Context(), uid, SubmissionQuery{
		Status: c.Query("status"), Page: page, PageSize: pageSize,
	})
	if h.rendered(c, err, "submission inbox") {
		return
	}
	c.JSON(http.StatusOK, toSubmissionListDTO(res))
}

// Grade handles POST /v1/submissions/:id/grade. Body {"score"?, "feedback"}.
// Teacher-owner only, writing submissions only.
func (h *Handler) Grade(c *gin.Context) {
	uid, id, ok := h.callerAndSubmissionID(c)
	if !ok {
		return
	}
	var req gradeSubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"score"?: integer, "feedback": string}.`)
		return
	}
	sub, err := h.svc.Grade(c.Request.Context(), uid, id, req.Score, req.Feedback)
	if h.rendered(c, err, "grade submission", slog.String("submission_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toSubmissionDTO(sub))
}

func (h *Handler) callerAndSubmissionID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The submission id must be a UUID.")
		return uuid.Nil, uuid.Nil, false
	}
	return uid, id, true
}

func (h *Handler) callerAndID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The resource id must be a UUID.")
		return uuid.Nil, uuid.Nil, false
	}
	return uid, id, true
}

func (h *Handler) rendered(c *gin.Context, err error, op string, attrs ...slog.Attr) bool {
	if err == nil {
		return false
	}
	var ve ValidationError
	switch {
	case errors.As(err, &ve):
		web.BadRequest(c, ve.Error())
	case errors.Is(err, ErrNoTeacher):
		web.WriteError(c, http.StatusForbidden, "no_teacher_profile",
			"Create a teacher profile before building resources.")
	case errors.Is(err, ErrNotFound):
		web.NotFound(c, "No resource with that id.")
	case errors.Is(err, ErrForbidden):
		web.Forbidden(c, "This resource belongs to another teacher.")
	case errors.Is(err, ErrInUse):
		web.WriteError(c, http.StatusConflict, "resource_in_use",
			"This resource is attached to a lesson or course. Archive it instead.")
	case errors.Is(err, ErrBookingNotFound):
		web.NotFound(c, "No booking with that id.")
	case errors.Is(err, ErrAlreadyAttached):
		web.WriteError(c, http.StatusConflict, "resource_already_attached",
			"This resource is already attached to this booking.")
	case errors.Is(err, ErrAttachmentNotFound):
		web.WriteError(c, http.StatusNotFound, "attachment_not_found", "No attachment with that id on this booking.")
	case errors.Is(err, ErrHomeworkNotAssigned):
		web.WriteError(c, http.StatusConflict, "homework_not_assigned",
			"This resource is not assigned as homework on this booking.")
	case errors.Is(err, ErrSubmissionNotFound):
		web.WriteError(c, http.StatusNotFound, "submission_not_found", "No submission with that id.")
	case errors.Is(err, ErrInvalidSubmissionState):
		web.WriteError(c, http.StatusConflict, "invalid_state", "The submission is not in a state that allows this.")
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

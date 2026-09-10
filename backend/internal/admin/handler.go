package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/teachers"
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

// RegisterRoutes mounts the admin dashboard + teacher-moderation endpoints onto
// the given group (expected to be "/v1/admin", already behind auth.RequireAuth).
// Each route carries its own RBAC permission check via the guard.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, guard *rbac.Guard) {
	rg.GET("/metrics", guard.Require(rbac.PermMetricsView), h.Metrics)
	rg.GET("/users", guard.Require(rbac.PermUsersView), h.ListUsers)
	rg.GET("/users/:id", guard.Require(rbac.PermUsersView), h.GetUser)

	rg.GET("/teachers", guard.Require(rbac.PermTeachersView), h.ListTeachers)
	rg.GET("/teachers/:slug", guard.Require(rbac.PermTeachersView), h.GetTeacher)
	rg.POST("/teachers/:slug/approve", guard.Require(rbac.PermTeachersModerate), h.Approve)
	rg.POST("/teachers/:slug/reject", guard.Require(rbac.PermTeachersModerate), h.Reject)
	rg.POST("/teachers/:slug/suspend", guard.Require(rbac.PermTeachersModerate), h.Suspend)
	rg.POST("/teachers/:slug/verify", guard.Require(rbac.PermTeachersVerify), h.Verify)

	rg.GET("/bookings", guard.Require(rbac.PermBookingsView), h.ListBookings)
	rg.GET("/bookings/:id", guard.Require(rbac.PermBookingsView), h.GetBooking)
	rg.POST("/bookings/:id/force-cancel", guard.Require(rbac.PermBookingsForceCancel), h.ForceCancelBooking)
}

// Metrics handles GET /v1/admin/metrics.
func (h *Handler) Metrics(c *gin.Context) {
	m, err := h.svc.Metrics(c.Request.Context())
	if h.rendered(c, err, "admin metrics") {
		return
	}
	c.JSON(http.StatusOK, toMetricsDTO(m))
}

// ListUsers handles GET /v1/admin/users?q&page&page_size.
func (h *Handler) ListUsers(c *gin.Context) {
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
	res, err := h.svc.ListUsers(c.Request.Context(), c.Query("q"), page, pageSize)
	if h.rendered(c, err, "admin list users") {
		return
	}
	c.JSON(http.StatusOK, toUsersPageDTO(res))
}

// GetUser handles GET /v1/admin/users/{id}.
func (h *Handler) GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The user id must be a UUID.")
		return
	}
	res, err := h.svc.GetUser(c.Request.Context(), id)
	if h.rendered(c, err, "admin get user", slog.String("user_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toUserDetailDTO(res))
}

// ListTeachers handles GET /v1/admin/teachers?status&q&page&page_size.
func (h *Handler) ListTeachers(c *gin.Context) {
	status := c.Query("status")
	if status != "" && !validStatus(status) {
		web.BadRequest(c, "`status` must be one of pending, approved, rejected, suspended.")
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
	res, err := h.svc.ListTeachers(c.Request.Context(), status, c.Query("q"), page, pageSize)
	if h.rendered(c, err, "admin list teachers") {
		return
	}
	c.JSON(http.StatusOK, toTeachersPageDTO(res))
}

// GetTeacher handles GET /v1/admin/teachers/{slug}.
func (h *Handler) GetTeacher(c *gin.Context) {
	res, err := h.svc.GetTeacher(c.Request.Context(), c.Param("slug"))
	if h.rendered(c, err, "admin get teacher", slog.String("slug", c.Param("slug"))) {
		return
	}
	c.JSON(http.StatusOK, toTeacherDetailDTO(res))
}

type noteRequest struct {
	Note string `json:"note"`
}

type verifyRequest struct {
	Verified *bool `json:"verified"`
}

// Approve handles POST /v1/admin/teachers/{slug}/approve.
func (h *Handler) Approve(c *gin.Context) {
	res, err := h.svc.Approve(c.Request.Context(), c.Param("slug"))
	if h.rendered(c, err, "admin approve teacher", slog.String("slug", c.Param("slug"))) {
		return
	}
	c.JSON(http.StatusOK, toTeacherDetailDTO(res))
}

// Reject handles POST /v1/admin/teachers/{slug}/reject — body {"note"}.
func (h *Handler) Reject(c *gin.Context) {
	var req noteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"note": "..."}.`)
		return
	}
	res, err := h.svc.Reject(c.Request.Context(), c.Param("slug"), strings.TrimSpace(req.Note))
	if h.rendered(c, err, "admin reject teacher", slog.String("slug", c.Param("slug"))) {
		return
	}
	c.JSON(http.StatusOK, toTeacherDetailDTO(res))
}

// Suspend handles POST /v1/admin/teachers/{slug}/suspend — body {"note"}.
func (h *Handler) Suspend(c *gin.Context) {
	var req noteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"note": "..."}.`)
		return
	}
	res, err := h.svc.Suspend(c.Request.Context(), c.Param("slug"), strings.TrimSpace(req.Note))
	if h.rendered(c, err, "admin suspend teacher", slog.String("slug", c.Param("slug"))) {
		return
	}
	c.JSON(http.StatusOK, toTeacherDetailDTO(res))
}

// Verify handles POST /v1/admin/teachers/{slug}/verify — body {"verified": bool}.
func (h *Handler) Verify(c *gin.Context) {
	var req verifyRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Verified == nil {
		web.BadRequest(c, `Request body must be {"verified": true|false}.`)
		return
	}
	res, err := h.svc.SetVerified(c.Request.Context(), c.Param("slug"), *req.Verified)
	if h.rendered(c, err, "admin verify teacher", slog.String("slug", c.Param("slug"))) {
		return
	}
	c.JSON(http.StatusOK, toTeacherDetailDTO(res))
}

// --- phase D: bookings ---

// ListBookings handles GET /v1/admin/bookings?status&q&page&page_size.
func (h *Handler) ListBookings(c *gin.Context) {
	status := c.Query("status")
	if status != "" && !ValidBookingStatus(status) {
		web.BadRequest(c, "`status` must be one of pending_payment, confirmed, completed, cancelled.")
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

	res, err := h.svc.ListBookings(c.Request.Context(), status, c.Query("q"), page, pageSize)
	if h.rendered(c, err, "admin list bookings") {
		return
	}
	c.JSON(http.StatusOK, toBookingsPageDTO(res))
}

// GetBooking handles GET /v1/admin/bookings/{id}.
func (h *Handler) GetBooking(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The booking id must be a UUID.")
		return
	}
	res, err := h.svc.GetBooking(c.Request.Context(), id)
	if h.rendered(c, err, "admin get booking", slog.String("booking_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toBookingDetailDTO(res))
}

type forceCancelRequest struct {
	Reason string `json:"reason"`
	Refund bool   `json:"refund"`
}

// ForceCancelBooking handles POST /v1/admin/bookings/{id}/force-cancel —
// body {"reason": "...", "refund": bool}.
func (h *Handler) ForceCancelBooking(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The booking id must be a UUID.")
		return
	}

	var req forceCancelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"reason": string, "refund"?: bool}.`)
		return
	}

	res, err := h.svc.ForceCancelBooking(c.Request.Context(), id, strings.TrimSpace(req.Reason), req.Refund)
	if h.rendered(c, err, "admin force-cancel booking", slog.String("booking_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toBookingDetailDTO(res))
}

// rendered maps a service error to an HTTP response. Returns true when it wrote
// one (the caller should stop).
func (h *Handler) rendered(c *gin.Context, err error, op string, attrs ...slog.Attr) bool {
	if err == nil {
		return false
	}
	var ve ValidationError
	switch {
	case errors.Is(err, ErrUserNotFound):
		web.NotFound(c, "No user with that id.")
	case errors.Is(err, ErrTeacherNotFound), errors.Is(err, teachers.ErrNotFound):
		web.NotFound(c, "No teacher with that slug.")
	case errors.Is(err, ErrBookingNotFound):
		web.WriteError(c, http.StatusNotFound, "booking_not_found", "No booking with that id.")
	case errors.Is(err, ErrInvalidTransition):
		web.WriteError(c, http.StatusConflict, "invalid_transition", "The teacher is not in a state that allows this moderation action.")
	case errors.Is(err, ErrInvalidState):
		web.WriteError(c, http.StatusConflict, "invalid_state", "The booking is not in a state that allows this action.")
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

func validStatus(s string) bool {
	switch teachers.Status(s) {
	case teachers.StatusPending, teachers.StatusApproved, teachers.StatusRejected, teachers.StatusSuspended:
		return true
	default:
		return false
	}
}

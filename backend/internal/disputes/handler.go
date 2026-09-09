package disputes

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

// RegisterBookingRoutes mounts the participant-facing dispute routes onto the
// "/v1/bookings" group, next to the booking lifecycle and review routes. Both
// require a valid access token; the participant check is in the service.
func RegisterBookingRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	rg.POST("/:id/disputes", requireAuth, h.Raise)
	rg.GET("/:id/disputes", requireAuth, h.ListForBooking)
}

// RegisterAdminRoutes mounts the operator queue onto the given group (expected
// to be "/v1/admin", already behind auth.RequireAuth). Both routes carry the
// `disputes.resolve` permission check.
func RegisterAdminRoutes(rg *gin.RouterGroup, h *Handler, guard *rbac.Guard) {
	rg.GET("/disputes", guard.Require(rbac.PermDisputesResolve), h.ListQueue)
	rg.POST("/disputes/:id/resolve", guard.Require(rbac.PermDisputesResolve), h.Resolve)
}

// Raise handles POST /v1/bookings/:id/disputes. Body {"reason": "..."}.
func (h *Handler) Raise(c *gin.Context) {
	uid, bookingID, ok := h.callerAndID(c, "booking")
	if !ok {
		return
	}

	var req raiseDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"reason": string}.`)
		return
	}

	d, err := h.svc.Raise(c.Request.Context(), uid, bookingID, req.Reason)
	if h.rendered(c, err, "raise dispute", slog.String("booking_id", bookingID.String())) {
		return
	}
	c.JSON(http.StatusCreated, toDisputeDTO(d))
}

// ListForBooking handles GET /v1/bookings/:id/disputes. Participant-only.
func (h *Handler) ListForBooking(c *gin.Context) {
	uid, bookingID, ok := h.callerAndID(c, "booking")
	if !ok {
		return
	}

	ds, err := h.svc.ListForBooking(c.Request.Context(), uid, bookingID)
	if h.rendered(c, err, "list booking disputes", slog.String("booking_id", bookingID.String())) {
		return
	}
	c.JSON(http.StatusOK, toDisputeListDTO(ds))
}

// ListQueue handles GET /v1/admin/disputes?status&page&page_size.
func (h *Handler) ListQueue(c *gin.Context) {
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

	res, err := h.svc.ListQueue(c.Request.Context(), c.Query("status"), page, pageSize)
	if h.rendered(c, err, "list disputes") {
		return
	}
	c.JSON(http.StatusOK, toQueuePageDTO(res))
}

// Resolve handles POST /v1/admin/disputes/:id/resolve.
// Body {"outcome": "resolved"|"rejected", "resolution": string, "refund": bool}.
func (h *Handler) Resolve(c *gin.Context) {
	uid, disputeID, ok := h.callerAndID(c, "dispute")
	if !ok {
		return
	}

	var req resolveDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"outcome": "resolved"|"rejected", "resolution": string, "refund"?: bool}.`)
		return
	}

	d, err := h.svc.Resolve(c.Request.Context(), uid, disputeID, req.Outcome, req.Resolution, req.Refund)
	if h.rendered(c, err, "resolve dispute", slog.String("dispute_id", disputeID.String())) {
		return
	}
	c.JSON(http.StatusOK, toDisputeDTO(d))
}

// callerAndID pulls the authenticated user id and the :id path param, writing
// the appropriate error envelope and returning ok=false on failure. kind names
// the resource in the 400 message ("booking" / "dispute").
func (h *Handler) callerAndID(c *gin.Context, kind string) (uuid.UUID, uuid.UUID, bool) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The "+kind+" id must be a UUID.")
		return uuid.Nil, uuid.Nil, false
	}
	return uid, id, true
}

// rendered maps a service error to an HTTP response. Returns true when it wrote
// one (the caller should stop).
func (h *Handler) rendered(c *gin.Context, err error, op string, attrs ...slog.Attr) bool {
	if err == nil {
		return false
	}

	var ve ValidationError
	switch {
	case errors.Is(err, ErrBookingNotFound):
		web.NotFound(c, "No booking with that id.")
	case errors.Is(err, ErrDisputeNotFound):
		web.WriteError(c, http.StatusNotFound, "dispute_not_found", "No dispute with that id.")
	case errors.Is(err, ErrForbidden):
		web.Forbidden(c, "You are not a participant in this booking.")
	case errors.Is(err, ErrNotAllowed):
		web.WriteError(c, http.StatusConflict, "dispute_not_allowed",
			"Only a confirmed or completed lesson can be disputed.")
	case errors.Is(err, ErrDisputeExists):
		web.WriteError(c, http.StatusConflict, "dispute_exists",
			"This lesson already has an open dispute.")
	case errors.Is(err, ErrAlreadyResolved):
		web.WriteError(c, http.StatusConflict, "already_resolved", "This dispute has already been resolved.")
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

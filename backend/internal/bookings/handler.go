package bookings

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

// Handler adapts HTTP to the Service: bind + validate input, call the service,
// render the OpenAPI-shaped JSON. No business logic; the *gin.Context never
// leaves this file.
type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// RegisterRoutes mounts the booking endpoints onto the given group (expected to
// be "/v1/bookings"). Every route requires a valid access token.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	rg.POST("", requireAuth, h.Create)
	rg.GET("", requireAuth, h.List)
	rg.GET("/:id", requireAuth, h.Get)
	rg.POST("/:id/pay", requireAuth, h.Pay)
	rg.POST("/:id/complete", requireAuth, h.Complete)
	rg.POST("/:id/cancel", requireAuth, h.Cancel)
	rg.PUT("/:id/meeting-link", requireAuth, h.SetMeetingLink)
	rg.POST("/:id/no-show", requireAuth, h.NoShow)
}

// RegisterTeacherSlotRoute mounts GET /:slug/slots onto the "/v1/teachers"
// group, next to the teacher-profile and availability routes. Public.
func RegisterTeacherSlotRoute(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/:slug/slots", h.Slots)
}

// Slots handles GET /v1/teachers/:slug/slots?from&to&duration.
func (h *Handler) Slots(c *gin.Context) {
	slug := c.Param("slug")

	from, err := optionalTime(c.Query("from"))
	if err != nil {
		web.BadRequest(c, "`from` must be an RFC3339 timestamp.")
		return
	}
	to, err := optionalTime(c.Query("to"))
	if err != nil {
		web.BadRequest(c, "`to` must be an RFC3339 timestamp.")
		return
	}
	duration, err := optionalInt(c.Query("duration"))
	if err != nil {
		web.BadRequest(c, "`duration` must be an integer number of minutes.")
		return
	}

	res, err := h.svc.Slots(c.Request.Context(), slug, from, to, duration)
	if h.rendered(c, err, "list slots", slog.String("slug", slug)) {
		return
	}
	c.JSON(http.StatusOK, toSlotsResponseDTO(res))
}

// Create handles POST /v1/bookings.
func (h *Handler) Create(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}

	var req createBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"teacher_slug", "start_at" (RFC3339), "duration_minutes", "is_trial"?}.`)
		return
	}

	b, err := h.svc.Create(c.Request.Context(), uid, CreateInput{
		TeacherSlug:     req.TeacherSlug,
		StartAt:         req.StartAt,
		DurationMinutes: req.DurationMinutes,
		IsTrial:         req.IsTrial,
	})
	if h.rendered(c, err, "create booking", slog.String("slug", req.TeacherSlug)) {
		return
	}
	c.JSON(http.StatusCreated, h.annotate(c, toBookingDTO(b, uid), b, uid))
}

// List handles GET /v1/bookings?role&status.
func (h *Handler) List(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}

	var status *Status
	if s := c.Query("status"); s != "" {
		v := Status(s)
		status = &v
	}

	bs, err := h.svc.List(c.Request.Context(), uid, Role(c.Query("role")), status)
	if h.rendered(c, err, "list bookings") {
		return
	}
	reviews := make([]*BookingReview, len(bs))
	disputes := make([]*BookingDispute, len(bs))
	for i, b := range bs {
		if b.Status == StatusCompleted {
			reviews[i] = h.svc.ReviewFor(c.Request.Context(), b.ID)
		}
		if b.Status != StatusPendingPayment {
			disputes[i] = h.svc.OpenDisputeFor(c.Request.Context(), b.ID)
		}
	}
	c.JSON(http.StatusOK, toBookingListDTO(bs, uid, reviews, disputes))
}

// Get handles GET /v1/bookings/:id.
func (h *Handler) Get(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}

	b, snap, err := h.svc.GetWithPayment(c.Request.Context(), uid, id)
	if h.rendered(c, err, "get booking", slog.String("id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, h.annotate(c, toBookingDTOWithPayment(b, snap, uid), b, uid))
}

// Pay handles POST /v1/bookings/:id/pay. Body {"method_token": "..."}.
// Student-only: authorize payment, which confirms the booking on success.
func (h *Handler) Pay(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}

	var req payBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.MethodToken == "" {
		web.BadRequest(c, `Request body must be {"method_token": string}.`)
		return
	}

	b, snap, err := h.svc.Pay(c.Request.Context(), uid, id, req.MethodToken)
	if h.rendered(c, err, "pay booking", slog.String("id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, h.annotate(c, toBookingDTOWithPayment(b, snap, uid), b, uid))
}

// Complete handles POST /v1/bookings/:id/complete. Teacher-owner only: capture
// the payment and mark the lesson completed.
func (h *Handler) Complete(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}

	b, snap, err := h.svc.Complete(c.Request.Context(), uid, id)
	if h.rendered(c, err, "complete booking", slog.String("id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, h.annotate(c, toBookingDTOWithPayment(b, snap, uid), b, uid))
}

// Cancel handles POST /v1/bookings/:id/cancel.
func (h *Handler) Cancel(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}

	// Body is optional; an empty/missing body is a reason-less cancellation.
	var req cancelBookingRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			web.BadRequest(c, `Request body must be {"reason"?: string}.`)
			return
		}
	}

	b, err := h.svc.Cancel(c.Request.Context(), uid, id, req.Reason)
	if h.rendered(c, err, "cancel booking", slog.String("id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, h.annotate(c, toBookingDTO(b, uid), b, uid))
}

// SetMeetingLink handles PUT /v1/bookings/:id/meeting-link. Body {"url": "..."}.
// Teacher-owner only; an empty url clears the per-booking override.
func (h *Handler) SetMeetingLink(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}

	var req meetingLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"url": string}.`)
		return
	}

	b, err := h.svc.SetMeetingLink(c.Request.Context(), uid, id, req.URL)
	if h.rendered(c, err, "set meeting link", slog.String("id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, h.annotate(c, toBookingDTO(b, uid), b, uid))
}

// NoShow handles POST /v1/bookings/:id/no-show. Body {"party": "student"|"teacher"}.
// Teacher-owner only; allowed from confirmed once the lesson has started.
func (h *Handler) NoShow(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}

	var req noShowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"party": "student" | "teacher"}.`)
		return
	}

	b, snap, err := h.svc.NoShow(c.Request.Context(), uid, id, req.Party)
	if h.rendered(c, err, "record no-show", slog.String("id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, h.annotate(c, toBookingDTOWithPayment(b, snap, uid), b, uid))
}

// annotate fills the two read-model extras a participant sees on a booking:
// can_review / review (the reviews port) and can_raise_dispute / open_dispute
// (the disputes port). Both ports are optional and nil-safe, and a booking whose
// state rules the extra out skips the lookup entirely.
func (h *Handler) annotate(c *gin.Context, dto bookingDTO, b Booking, viewerID uuid.UUID) bookingDTO {
	ctx := c.Request.Context()

	var review *BookingReview
	if b.Status == StatusCompleted {
		review = h.svc.ReviewFor(ctx, b.ID)
	}
	// A dispute can only exist from `confirmed` on — but a confirmed booking
	// that was later cancelled (or force-cancelled) may still carry an open one,
	// so everything past pending_payment is checked.
	var dispute *BookingDispute
	if b.Status != StatusPendingPayment {
		dispute = h.svc.OpenDisputeFor(ctx, b.ID)
	}
	return withDispute(withReview(dto, b, viewerID, review), b, viewerID, dispute)
}

// callerAndID pulls the authenticated user id and the :id path param, writing
// the appropriate error envelope and returning ok=false on failure.
func (h *Handler) callerAndID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
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

// rendered maps a service error to an HTTP response. It returns true when it
// wrote a response (including on the nil-error fast path's opposite: caller
// should stop) — i.e. true means "stop, already handled".
func (h *Handler) rendered(c *gin.Context, err error, op string, attrs ...slog.Attr) bool {
	if err == nil {
		return false
	}

	var ve ValidationError
	var payFailed PaymentFailedError
	switch {
	case errors.Is(err, ErrTeacherNotFound):
		web.NotFound(c, "No teacher with that slug.")
	case errors.Is(err, ErrBookingNotFound):
		web.NotFound(c, "No booking with that id.")
	case errors.Is(err, ErrForbidden):
		web.Forbidden(c, "You are not a participant in this booking.")
	case errors.Is(err, ErrNotStudent):
		web.Forbidden(c, "Only the student who booked this lesson can pay for it.")
	case errors.Is(err, ErrNotTeacherOwner):
		web.Forbidden(c, "Only the teacher for this lesson can complete it.")
	case errors.Is(err, ErrCannotBookSelf):
		web.BadRequest(c, "You cannot book a lesson with your own teacher profile.")
	case errors.Is(err, ErrSlotUnavailable):
		web.WriteError(c, http.StatusConflict, "slot_unavailable", "That start time is not currently bookable for this teacher.")
	case errors.Is(err, ErrSlotTaken):
		web.WriteError(c, http.StatusConflict, "slot_taken", "That slot was just taken. Pick another time.")
	case errors.Is(err, ErrAlreadyPaid):
		web.WriteError(c, http.StatusConflict, "already_paid", "This booking has already been paid for.")
	case errors.Is(err, ErrTooEarly):
		web.WriteError(c, http.StatusConflict, "too_early", "The lesson has not ended yet.")
	case errors.Is(err, ErrLessonNotStarted):
		web.WriteError(c, http.StatusConflict, "too_early", "The lesson has not started yet, so a no-show cannot be recorded.")
	case errors.Is(err, ErrPaymentRequired):
		web.WriteError(c, http.StatusConflict, "payment_required", "This booking has not been paid for yet.")
	case errors.Is(err, ErrInvalidTransition):
		web.WriteError(c, http.StatusConflict, "invalid_state", "The booking is not in a state that allows this action.")
	case errors.As(err, &payFailed):
		web.WriteError(c, http.StatusPaymentRequired, "payment_failed", payFailed.Error())
	case errors.Is(err, ErrCaptureFailed):
		web.WriteError(c, http.StatusBadGateway, "capture_failed", "The payment could not be captured. Please try again.")
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

func optionalTime(v string) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func optionalInt(v string) (int, error) {
	if v == "" {
		return 0, nil
	}
	return strconv.Atoi(v)
}

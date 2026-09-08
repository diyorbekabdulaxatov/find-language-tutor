package payments

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

// Handler adapts HTTP to the Service: bind + validate, call the service, render
// JSON. No business logic; the *gin.Context never leaves this file.
type Handler struct {
	svc           *Service
	logger        *slog.Logger
	webhookSecret string // optional HMAC-SHA256 secret for provider webhooks
}

func NewHandler(svc *Service, webhookSecret string, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger, webhookSecret: webhookSecret}
}

// RegisterRoutes mounts the payment endpoints onto the given group (expected to
// be "/v1/payments"). The webhook is unauthenticated (the provider signs it);
// /me needs a Bearer token and is teacher-owner-gated in the service.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	rg.POST("/webhook", h.Webhook)
	rg.GET("/me", requireAuth, h.Earnings)
}

// Webhook handles POST /v1/payments/webhook. Idempotent by event_id: a
// well-formed duplicate returns 200. Real providers call this over HTTP; the
// in-process fake reaches the same Service.HandleWebhook via its sink.
func (h *Handler) Webhook(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil {
		web.BadRequest(c, "Could not read the request body.")
		return
	}

	if h.webhookSecret != "" && !validSignature(h.webhookSecret, c.GetHeader("X-Payment-Signature"), raw) {
		web.Unauthorized(c, "Invalid webhook signature.")
		return
	}

	var req webhookRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		web.BadRequest(c, "Body must be a JSON payment event.")
		return
	}
	paymentID, err := uuid.Parse(req.PaymentID)
	if err != nil {
		web.BadRequest(c, "`payment_id` must be a UUID.")
		return
	}
	if req.EventID == "" || !EventType(req.Type).valid() {
		web.BadRequest(c, "`event_id` is required and `type` must be a known event type.")
		return
	}

	applied, err := h.svc.HandleWebhook(c.Request.Context(), Event{
		ID:          req.EventID,
		Type:        EventType(req.Type),
		PaymentID:   paymentID,
		ProviderRef: req.ProviderRef,
		AmountMinor: req.AmountMinor,
		Currency:    req.Currency,
		Message:     req.Message,
	})
	if err != nil {
		if errors.Is(err, ErrPaymentNotFound) {
			web.NotFound(c, "No payment for that id.")
			return
		}
		h.logger.Error("handle webhook", slog.String("event_id", req.EventID), slog.Any("error", err))
		web.Internal(c)
		return
	}

	// 200 whether the event was newly applied or a recognised duplicate.
	c.JSON(http.StatusOK, webhookResponse{Received: true, Applied: applied})
}

// Earnings handles GET /v1/payments/me — the caller's teacher earnings summary.
func (h *Handler) Earnings(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}

	e, err := h.svc.Earnings(c.Request.Context(), uid)
	switch {
	case errors.Is(err, ErrNoTeacherProfile):
		web.NotFound(c, "You do not own a teacher profile.")
		return
	case err != nil:
		h.logger.Error("teacher earnings", slog.String("user_id", uid.String()), slog.Any("error", err))
		web.Internal(c)
		return
	}
	c.JSON(http.StatusOK, toEarningsDTO(e))
}

func validSignature(secret, header string, body []byte) bool {
	if header == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(want), []byte(header)) == 1
}

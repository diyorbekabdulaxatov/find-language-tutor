// Package payments is the money module for the booking flow. A booking has
// exactly one payment intent; the app drives it through a provider-agnostic
// port (Provider) so the deterministic fake shipped for the MVP can later be
// swapped for a real Payme / Click / Uzum adapter without touching callers.
// Stripe is deliberately not used — it does not operate in Uzbekistan.
//
// Layout mirrors the other modules: domain types + errors here, a Service with
// the rules, a Repository port (Postgres impl alongside, fake in tests), gin
// handlers, and a RegisterRoutes func. The Service never sees an *gin.Context.
//
// Boundary with internal/bookings: bookings defines the port it needs
// (bookings.PaymentGateway) and this package implements it (paymentGateway in
// gateway.go). The reverse direction — a successful authorization must move the
// booking pending_payment -> confirmed — is done inside the webhook transaction
// via a guarded UPDATE on the bookings row, so the two state changes commit
// atomically. Neither package imports the other's Go types; only cmd/api and
// internal/httpapi wire the concrete types together.
//
// Money is integer minor units everywhere. Never float.
package payments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Status is the payment intent lifecycle state. It is stored as text and
// CHECK-constrained in migration 000005.
type Status string

const (
	StatusRequiresPayment Status = "requires_payment"
	StatusAuthorized      Status = "authorized"
	StatusCaptured        Status = "captured"
	StatusRefunded        Status = "refunded"
	StatusFailed          Status = "failed"
)

// LedgerState is the payout_ledger row state.
type LedgerState string

const (
	LedgerHeld      LedgerState = "held"
	LedgerAvailable LedgerState = "available"
	LedgerReversed  LedgerState = "reversed"
)

// Money is an amount in integer minor units plus a currency code, matching the
// bookings module and openapi.yaml's Money schema.
type Money struct {
	AmountMinor int64
	Currency    string
}

// Payment is the domain aggregate: one intent per booking.
type Payment struct {
	ID           uuid.UUID
	BookingID    uuid.UUID
	Provider     string
	ProviderRef  string
	Status       Status
	Amount       Money
	LastError    string
	AuthorizedAt *time.Time
	CapturedAt   *time.Time
	RefundedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// EarningLine is one captured (or reversed) lesson in a teacher's earnings
// summary.
type EarningLine struct {
	BookingID          uuid.UUID
	StudentDisplayName string
	StartAt            time.Time
	AmountMinor        int64
	Currency           string
	State              LedgerState
}

// Earnings is the teacher earnings summary. Totals are derived from the lines.
type Earnings struct {
	TotalEarnedMinor int64 // captured, net of reversals (held + available)
	HeldMinor        int64
	AvailableMinor   int64
	Currency         string
	Lines            []EarningLine
}

// --- provider port -------------------------------------------------------

// AuthorizeInput is what the app hands the provider to place a hold.
type AuthorizeInput struct {
	PaymentID   uuid.UUID
	AmountMinor int64
	Currency    string
	// MethodToken is an opaque, client-supplied handle for the payment method.
	MethodToken string
}

// IntentStatus is the provider's view of an intent after an operation.
type IntentStatus string

const (
	IntentAuthorized IntentStatus = "authorized"
	IntentCaptured   IntentStatus = "captured"
	IntentRefunded   IntentStatus = "refunded"
	IntentFailed     IntentStatus = "failed"
)

// Intent is the provider's response: its own reference plus the resulting
// status and an optional human-readable message.
type Intent struct {
	ProviderRef string
	Status      IntentStatus
	Message     string
}

// Provider is the payment port. The MVP ships FakeProvider; a real adapter
// implements the same three calls.
type Provider interface {
	// Authorize places a hold for the amount on the customer's method.
	Authorize(ctx context.Context, in AuthorizeInput) (Intent, error)
	// Capture settles a previously authorized hold (full amount).
	Capture(ctx context.Context, providerRef string) (Intent, error)
	// Refund releases a hold (if only authorized) or refunds a capture.
	Refund(ctx context.Context, providerRef string, amountMinor int64) (Intent, error)
}

// Declined is returned by Provider.Authorize when the method is refused. It is
// an expected outcome, not a transport error: the handler renders it as 402.
type Declined struct{ Reason string }

func (e Declined) Error() string { return "payment declined: " + e.Reason }

// --- webhook events ----------------------------------------------------

// EventType is the provider-neutral event name.
type EventType string

const (
	EventAuthorized EventType = "payment.authorized"
	EventCaptured   EventType = "payment.captured"
	EventRefunded   EventType = "payment.refunded"
	EventFailed     EventType = "payment.failed"
)

func (t EventType) valid() bool {
	switch t {
	case EventAuthorized, EventCaptured, EventRefunded, EventFailed:
		return true
	default:
		return false
	}
}

// Event is one provider state-change notification. ID is stable for a given
// state change, so replaying an Event is a no-op (enforced by the
// payment_events primary key). A real provider delivers these over HTTP to
// POST /v1/payments/webhook; the in-process fake routes them through the same
// Service.HandleWebhook so the idempotent path is always exercised.
type Event struct {
	ID          string
	Type        EventType
	PaymentID   uuid.UUID
	ProviderRef string
	AmountMinor int64
	Currency    string
	Message     string
}

// eventID builds the stable id for a payment's state change.
func eventID(providerRef string, t EventType) string {
	return fmt.Sprintf("evt_%s_%s", providerRef, t)
}

// --- errors ------------------------------------------------------------

var (
	// ErrPaymentNotFound — no payment intent for the booking / id.
	ErrPaymentNotFound = errors.New("payments: payment not found")

	// ErrAlreadyPaid — the intent is already authorized or beyond; pay is a
	// one-shot action. Rendered as 409 already_paid.
	ErrAlreadyPaid = errors.New("payments: booking is already paid")

	// ErrNotPayable — the intent is in a state that cannot be authorized
	// (failed, refunded, captured). Rendered as 409.
	ErrNotPayable = errors.New("payments: payment is not in a payable state")

	// ErrCaptureFailed — the provider refused to settle the hold. Rendered as
	// 502 capture_failed; the booking stays confirmed so it can be retried.
	ErrCaptureFailed = errors.New("payments: provider failed to capture the hold")

	// ErrNoTeacherProfile — the caller owns no teacher profile (earnings).
	ErrNoTeacherProfile = errors.New("payments: caller owns no teacher profile")
)

// PaymentDeclined wraps a provider decline for the pay flow so the bookings
// handler can render 402 payment_failed with the provider's message.
type PaymentDeclined struct{ Reason string }

func (e PaymentDeclined) Error() string { return "payment failed: " + e.Reason }

// summarize folds ledger lines into the totals the earnings endpoint returns.
func summarize(lines []EarningLine, fallbackCurrency string) Earnings {
	e := Earnings{Lines: lines, Currency: fallbackCurrency}
	for _, l := range lines {
		if e.Currency == "" {
			e.Currency = l.Currency
		}
		switch l.State {
		case LedgerHeld:
			e.HeldMinor += l.AmountMinor
			e.TotalEarnedMinor += l.AmountMinor
		case LedgerAvailable:
			e.AvailableMinor += l.AmountMinor
			e.TotalEarnedMinor += l.AmountMinor
		case LedgerReversed:
			// reversed lessons contribute nothing
		}
	}
	return e
}

package bookings

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// PaymentGateway is the slice of the payments module the booking flow needs.
// internal/payments provides an adapter that satisfies it; this package never
// imports internal/payments, so the dependency direction is one-way (payments
// wiring depends on bookings' port, not the reverse). cmd/api / internal/httpapi
// construct the adapter and inject it with Service.SetPaymentGateway.
//
// All amounts are integer minor units.
type PaymentGateway interface {
	// InitiatePayment creates the requires_payment intent for a just-created
	// booking. Idempotent.
	InitiatePayment(ctx context.Context, bookingID uuid.UUID, amountMinor int64, currency string) error

	// Authorize places the hold for the booking's price. On a provider decline
	// it returns an error that satisfies errors.As(&PaymentFailedError{}); on an
	// already-authorized intent it returns ErrAlreadyPaid. On success the
	// payment webhook has already moved the booking pending_payment ->
	// confirmed.
	Authorize(ctx context.Context, bookingID uuid.UUID, methodToken string) (PaymentSnapshot, error)

	// Capture settles the authorized hold (lesson completed). Returns
	// ErrCaptureFailed if the provider refuses.
	Capture(ctx context.Context, bookingID uuid.UUID) (PaymentSnapshot, error)

	// Refund releases the hold / refunds the capture (booking cancelled). A
	// booking whose intent was never authorized is a no-op void.
	Refund(ctx context.Context, bookingID uuid.UUID) (PaymentSnapshot, error)

	// SnapshotForBooking returns the current payment state for embedding in a
	// BookingDTO. found is false when the booking has no intent.
	SnapshotForBooking(ctx context.Context, bookingID uuid.UUID) (snap PaymentSnapshot, found bool, err error)
}

// PaymentSnapshot is the read-model of a booking's payment embedded in booking
// responses.
type PaymentSnapshot struct {
	Status      string
	AmountMinor int64
	Currency    string
}

// Payment-flow domain errors (in addition to the ones in service.go).
var (
	// ErrAlreadyPaid — pay was called on a booking that is already
	// confirmed/paid. Rendered as 409 already_paid.
	ErrAlreadyPaid = errors.New("booking is already paid")

	// ErrCaptureFailed — the provider refused to settle the hold on complete.
	// Rendered as 502 capture_failed; the booking stays confirmed.
	ErrCaptureFailed = errors.New("could not capture payment")

	// ErrTooEarly — complete was called before the lesson's end_at. Rendered as
	// 409 too_early.
	ErrTooEarly = errors.New("the lesson has not ended yet")

	// ErrNotStudent — a non-student tried to pay. Rendered as 403.
	ErrNotStudent = errors.New("only the student can pay for this booking")

	// ErrNotTeacherOwner — a non-teacher-owner tried to complete. Rendered as
	// 403.
	ErrNotTeacherOwner = errors.New("only the teacher can complete this booking")

	// ErrPaymentRequired — pay has not happened yet for an action that needs it.
	ErrPaymentRequired = errors.New("booking has not been paid")
)

// PaymentFailedError carries the provider's decline reason so the handler can
// render 402 payment_failed with a useful message.
type PaymentFailedError struct{ Reason string }

func (e PaymentFailedError) Error() string {
	if e.Reason == "" {
		return "payment failed"
	}
	return "payment failed: " + e.Reason
}

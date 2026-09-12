package courses

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// PaymentGateway is the slice of the payments module the course-purchase flow
// needs. internal/payments provides an adapter (payments.CourseGateway) that
// satisfies it; this package never imports internal/payments, so the
// dependency direction is one-way, the same shape as bookings.PaymentGateway.
// cmd/api constructs the adapter and injects it with Service.SetPaymentGateway.
//
// All amounts are integer minor units.
type PaymentGateway interface {
	// Purchase authorizes and captures the course's price in one call —
	// unlike the booking flow's separate pay / complete steps, a course
	// purchase is a single one-time charge with no later lesson-completion
	// event to capture against. On a provider decline it returns an error
	// satisfying errors.As(&PaymentFailedError{}); on a capture failure
	// (after a successful authorize) it returns ErrCaptureFailed, retryable
	// by calling Purchase again with the same arguments.
	Purchase(ctx context.Context, courseID, studentID uuid.UUID, amountMinor int64, currency, methodToken string) (PurchaseSnapshot, error)
}

// PurchaseSnapshot is the read-model of a course payment's outcome.
type PurchaseSnapshot struct {
	Status      string
	AmountMinor int64
	Currency    string
}

// ErrCaptureFailed — the provider refused to settle the hold after a
// successful authorize. Rendered 502 capture_failed; retry by purchasing
// again.
var ErrCaptureFailed = errors.New("courses: could not capture payment")

// PaymentFailedError carries the provider's decline reason so the handler can
// render 402 payment_failed with a useful message.
type PaymentFailedError struct{ Reason string }

func (e PaymentFailedError) Error() string {
	if e.Reason == "" {
		return "payment failed"
	}
	return "payment failed: " + e.Reason
}

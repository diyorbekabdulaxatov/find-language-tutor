package payments

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the persistence port. The Postgres implementation lives in
// repository_postgres.go; tests use fakeRepo.
type Repository interface {
	// EnsurePayment creates the requires_payment intent for a booking, or
	// returns the existing one. Idempotent (called eagerly after booking
	// creation and again lazily on the first pay attempt).
	EnsurePayment(ctx context.Context, bookingID uuid.UUID, provider string, amount Money) (Payment, error)

	// PaymentByBooking / PaymentByID load one intent, or ErrPaymentNotFound.
	PaymentByBooking(ctx context.Context, bookingID uuid.UUID) (Payment, error)
	PaymentByID(ctx context.Context, id uuid.UUID) (Payment, error)

	// ApplyEvent processes one webhook event exactly once, transactionally.
	//
	// It inserts the payment_events row first; a duplicate event_id (the
	// primary-key unique violation — never a check-then-insert) means the event
	// was already handled, and ApplyEvent returns applied=false, err=nil
	// without touching any other row. Otherwise it applies the type-specific
	// state change (payment status + timestamps, plus the guarded booking
	// confirm on authorize, the `held` payout-ledger row opening the clearing
	// window on capture, and its reversal on refund) and commits, returning
	// applied=true.
	ApplyEvent(ctx context.Context, e Event) (applied bool, err error)

	// TeacherIDByOwner resolves the teacher profile owned by an account.
	TeacherIDByOwner(ctx context.Context, ownerID uuid.UUID) (id uuid.UUID, ok bool, err error)

	// EarningLines returns every payout-ledger line for a teacher, newest
	// lesson first, with the STORED state and the clearing deadline — the
	// effective held/available split is resolved above the repository.
	EarningLines(ctx context.Context, teacherID uuid.UUID) ([]EarningLine, error)
}

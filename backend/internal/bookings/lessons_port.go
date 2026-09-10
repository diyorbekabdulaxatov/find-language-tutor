package bookings

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// This file holds the two Phase-5 ports the booking flow depends on but does not
// implement: scheduled lesson reminders and transactional email. Both follow the
// same rule as PaymentGateway — bookings defines the interface, another package
// implements it, and cmd/api / cmd/worker inject it with a Service.SetX method.
// A nil implementation is always a safe no-op (guarded at every call), so tests
// and a cmd/api without Redis / Resend keep working.

// No-show parties. "" is the normal (no no-show) value.
const (
	NoShowStudent = "student"
	NoShowTeacher = "teacher"
)

// ReminderScheduler schedules (and cancels) the "your lesson is coming up"
// reminder jobs for a booking. The implementation (internal/lessons) wraps an
// asynq client + inspector and enqueues two deterministic tasks per booking
// (24h before and 1h before start_at); Cancel deletes both. Rescheduling is
// "Cancel then Schedule".
type ReminderScheduler interface {
	// Schedule enqueues the 24h and 1h reminders for a booking. A run time
	// already in the past is skipped. Idempotent for a given booking id.
	Schedule(ctx context.Context, bookingID uuid.UUID, startAt time.Time) error

	// Cancel removes any pending reminders for a booking. A not-found task is
	// not an error.
	Cancel(ctx context.Context, bookingID uuid.UUID) error
}

// Notifier sends the booking-lifecycle emails. The implementation
// (internal/lessons) renders the templates in internal/email and fans each
// message out to the student and the teacher as appropriate. Methods are
// fire-and-forget from the service's point of view: they log their own failures
// and never return an error that could fail the HTTP request.
type Notifier interface {
	// BookingConfirmed is sent to BOTH participants right after payment
	// succeeds. Includes when, price, and the meeting link.
	BookingConfirmed(ctx context.Context, b Booking)

	// BookingCancelled is sent to the OTHER party on a cancellation / teacher
	// no-show. refunded says whether the student's payment was returned.
	BookingCancelled(ctx context.Context, b Booking, cancelledBy uuid.UUID, refunded bool)
}

// Package disputes is the phase-D lesson-dispute domain module: a participant
// (the student or the teacher-owner) contests a confirmed or completed lesson,
// and an operator holding `disputes.resolve` closes it as resolved or rejected,
// optionally refunding the student.
//
// Layout mirrors the other modules: domain types + errors here, a Service with
// the rules, a Repository port (Postgres impl alongside, fake in tests), gin
// handlers, and RegisterRoutes funcs. The Service never sees a *gin.Context.
//
// Boundaries:
//
//   - The participant routes (POST/GET /v1/bookings/{id}/disputes) are mounted
//     on the /v1/bookings group from here, exactly like the reviews module's
//     POST /v1/bookings/{id}/review. The participant check needs the booking's
//     student + teacher-owner + status, which this module reads with its own SQL
//     (internal/db/queries/disputes.sql) rather than importing the bookings
//     service — the same call reviews made, and it keeps the packages decoupled.
//   - The admin routes (/v1/admin/disputes...) mount on the admin group behind
//     the RBAC guard, like rbac.RegisterAdminRoutes.
//   - The reverse direction (a BookingDTO carrying `can_raise_dispute` /
//     `open_dispute`) goes through bookings.DisputeReader: bookings defines the
//     port, BookingGateway here implements it, cmd/api injects it. bookings
//     never imports this package.
//   - Refunding a resolved dispute goes through the Refunder port defined in
//     this package (refund_port.go), satisfied by *bookings.Service.
package disputes

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MaxReasonLen / MaxResolutionLen cap the free-text fields (characters).
// Longer -> 400.
const (
	MaxReasonLen     = 2000
	MaxResolutionLen = 2000
)

// Status is the dispute lifecycle state: open is the only non-terminal one.
type Status string

const (
	StatusOpen     Status = "open"
	StatusResolved Status = "resolved"
	StatusRejected Status = "rejected"
)

func (s Status) valid() bool {
	switch s {
	case StatusOpen, StatusResolved, StatusRejected:
		return true
	default:
		return false
	}
}

// Booking statuses a dispute may be raised against. Kept as strings so this
// package does not import internal/bookings for a two-value check (the same
// call internal/reviews makes).
const (
	bookingConfirmed = "confirmed"
	bookingCompleted = "completed"
)

// UserRef is the light account summary embedded in a dispute.
type UserRef struct {
	ID          uuid.UUID
	DisplayName string
}

// Dispute is the domain aggregate. ResolvedBy / ResolvedAt are nil while the
// dispute is open.
type Dispute struct {
	ID         uuid.UUID
	BookingID  uuid.UUID
	Status     Status
	Reason     string
	Resolution string
	RaisedBy   UserRef
	ResolvedBy *UserRef
	CreatedAt  time.Time
	ResolvedAt *time.Time
}

// BookingRef is the slice of a booking the dispute flow needs to authorize the
// caller and check the lesson is disputable.
type BookingRef struct {
	ID     uuid.UUID
	Status string
	// StudentID / TeacherOwnerID are the two participant accounts.
	// TeacherOwnerID is uuid.Nil for an unclaimed teacher profile.
	StudentID      uuid.UUID
	TeacherOwnerID uuid.UUID
}

// Money is an amount in integer minor units plus a currency code, matching the
// rest of the API.
type Money struct {
	AmountMinor int64
	Currency    string
}

// TeacherRef / StudentRef are the parties shown next to a dispute in the
// operator queue.
type TeacherRef struct {
	Slug        string
	DisplayName string
}

type StudentRef struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
}

// BookingContext is the booking a queued dispute is about.
type BookingContext struct {
	ID      uuid.UUID
	Status  string
	StartAt time.Time
	Price   Money
	Teacher TeacherRef
	Student StudentRef
}

// QueueItem is one row of GET /v1/admin/disputes: the dispute plus the booking
// and parties an operator needs to triage it without opening the detail page.
type QueueItem struct {
	Dispute Dispute
	Booking BookingContext
}

// Page is a page of the operator queue plus the total match count.
type Page struct {
	Disputes []QueueItem
	Total    int
}

// Domain errors. The handler maps each to an HTTP status; anything else is 500.
var (
	// ErrBookingNotFound — no booking has the requested id.
	ErrBookingNotFound = errors.New("disputes: booking not found")

	// ErrForbidden — the caller is neither the student nor the teacher-owner.
	ErrForbidden = errors.New("disputes: not a participant in this booking")

	// ErrNotAllowed — the booking is not confirmed / completed, so it cannot be
	// disputed. Rendered 409 dispute_not_allowed.
	ErrNotAllowed = errors.New("disputes: this booking cannot be disputed")

	// ErrDisputeExists — the booking already has an OPEN dispute (the partial
	// unique index rejected the insert). Rendered 409 dispute_exists.
	ErrDisputeExists = errors.New("disputes: this booking already has an open dispute")

	// ErrDisputeNotFound — no dispute has the requested id. Rendered 404
	// dispute_not_found.
	ErrDisputeNotFound = errors.New("disputes: dispute not found")

	// ErrAlreadyResolved — the dispute is no longer open. Rendered 409
	// already_resolved.
	ErrAlreadyResolved = errors.New("disputes: this dispute is already resolved")
)

// ValidationError is a client-fixable problem with a request. The handler
// renders it as 400.
type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return ValidationError{msg: fmt.Sprintf(format, args...)}
}

// Pagination defaults, matching the other admin list endpoints.
const (
	defaultPageSize = 20
	maxPageSize     = 100
)

func normalizePage(page, pageSize int) (limit, offset int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return pageSize, (page - 1) * pageSize
}

// Package admin is the operator surface for find-language-tutor: a metrics
// dashboard (phase A) and the teacher-moderation workflow (phase B). Everything
// it exposes is mounted under /v1/admin behind auth.RequireAdmin.
//
// Layout mirrors the other modules: domain types + errors here, a Service with
// the rules, a Repository port (Postgres impl alongside, fake in tests), gin
// handlers, and a RegisterRoutes func. The Service never sees an *gin.Context.
//
// Unlike the domain modules, the admin repo reads other modules' tables
// directly (internal/db/queries/admin.sql) — it is an internal ops tool, not a
// public API, so it does not route reads through teachers / bookings / payments
// services. The one exception is the full teacher profile on
// GET /v1/admin/teachers/{slug}, which reuses the teachers read model via the
// TeacherProfiles port so the profile shape stays in one place.
package admin

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Domain errors. The handler maps each to an HTTP status; anything else is 500.
var (
	// ErrUserNotFound — GET /v1/admin/users/{id} for an unknown id.
	ErrUserNotFound = errors.New("admin: user not found")

	// ErrTeacherNotFound — a moderation action or detail read for an unknown slug.
	ErrTeacherNotFound = errors.New("admin: teacher not found")

	// ErrInvalidTransition — a moderation action from a state that does not allow
	// it (e.g. approve an already-approved teacher). Rendered 409 invalid_transition.
	ErrInvalidTransition = errors.New("admin: invalid moderation transition")

	// ErrBookingNotFound — a booking read or force-cancel for an unknown id.
	ErrBookingNotFound = errors.New("admin: booking not found")

	// ErrInvalidState — a booking action from a state that does not allow it
	// (force-cancelling an already completed / cancelled booking). Rendered 409
	// invalid_state, matching the participant cancel endpoint's code.
	ErrInvalidState = errors.New("admin: booking is not in a state that allows this")
)

// ValidationError is a client-fixable problem with a request (e.g. a missing
// moderation note). The handler renders it as 400.
type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return ValidationError{msg: fmt.Sprintf(format, args...)}
}

// --- phase A: dashboard ---

// Metrics is the answer to GET /v1/admin/metrics — the operator dashboard
// counters. All money is integer minor units in Currency ("UZS" for the MVP).
type Metrics struct {
	Currency string

	// People
	UsersTotal       int64
	UsersThisWeek    int64 // signed up in the last 7 days
	TeachersTotal    int64
	TeachersPending  int64 // awaiting moderation
	TeachersApproved int64
	TeachersVerified int64
	ActiveStudents   int64 // distinct accounts that have booked at least once

	// Bookings
	BookingsTotal     int64
	BookingsThisWeek  int64 // created in the last 7 days
	BookingsUpcoming  int64 // confirmed and not yet started
	BookingsCompleted int64
	BookingsCancelled int64

	// Money
	GMVMinor         int64 // booking price over confirmed + completed bookings
	CapturedMinor    int64 // collected from students
	RefundedMinor    int64 // returned to students
	PayoutsOwedMinor int64 // earned by teachers, not yet disbursed
	PayoutsPaidMinor int64 // disbursed by past payout runs

	// Reviews + moderation
	ReviewsVisible int64
	AverageRating  float64 // over visible reviews, one decimal; 0 when none
	DisputesOpen   int64
}

// UserRow is one row of GET /v1/admin/users.
type UserRow struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	CreatedAt    time.Time
	IsTeacher    bool // owns a teachers row
	BookingCount int64
}

// UsersPage is a page of users plus the total match count.
type UsersPage struct {
	Users []UserRow
	Total int
}

// UserCore is the account header on GET /v1/admin/users/{id}.
type UserCore struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	CreatedAt   time.Time
}

// RoleRef is the {id, name} of a role a user holds.
type RoleRef struct {
	ID   uuid.UUID
	Name string
}

// UserTeacherProfile is the light teacher-profile summary on a user detail
// (nil when the account owns no profile).
type UserTeacherProfile struct {
	Slug     string
	Status   string
	Verified bool
}

// BookingRole is the acting user's side of a booking on their detail view.
type BookingRole string

const (
	BookingRoleStudent BookingRole = "student"
	BookingRoleTeacher BookingRole = "teacher"
)

// UserBookingRow is one row of the (capped) bookings list on a user detail.
type UserBookingRow struct {
	ID             uuid.UUID
	Status         string
	StartAt        time.Time
	RoleInBooking  BookingRole
	OtherPartyName string
	PriceMinor     int64
	Currency       string
}

// PaymentsSummary buckets a user's payments (as the paying student) by current
// payment state.
type PaymentsSummary struct {
	AuthorizedMinor int64
	CapturedMinor   int64
	RefundedMinor   int64
	Currency        string
}

// UserDetail is the whole answer to GET /v1/admin/users/{id}. The bookings list
// is capped at the 50 newest.
type UserDetail struct {
	User           UserCore
	Roles          []RoleRef
	TeacherProfile *UserTeacherProfile
	Bookings       []UserBookingRow
	Payments       PaymentsSummary
}

// --- phase B: teacher moderation ---

// Owner is the account that owns a teacher profile.
type Owner struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
}

// TeacherRow is one row of GET /v1/admin/teachers.
type TeacherRow struct {
	Slug        string
	DisplayName string
	Status      string
	Verified    bool
	Headline    string
	CountryName string
	CreatedAt   time.Time
	Owner       Owner // ID == uuid.Nil / empty fields when the profile is unclaimed
}

// TeachersPage is a page of teachers plus the total match count.
type TeachersPage struct {
	Teachers []TeacherRow
	Total    int
}

// Moderation is the moderation slice of a teacher: status + badge + note + owner.
type Moderation struct {
	Status         string
	Verified       bool
	ModerationNote string
	Owner          Owner
}

// --- phase D: bookings ---

// Booking lifecycle states, as strings: the admin module reads and filters them
// but owns none of the transitions, so it does not import internal/bookings for
// a four-value enum (the same call internal/reviews makes for `completed`).
const (
	BookingPendingPayment = "pending_payment"
	BookingConfirmed      = "confirmed"
	BookingCompleted      = "completed"
	BookingCancelled      = "cancelled"
)

// ValidBookingStatus reports whether s is a booking status the list filter
// accepts.
func ValidBookingStatus(s string) bool {
	switch s {
	case BookingPendingPayment, BookingConfirmed, BookingCompleted, BookingCancelled:
		return true
	default:
		return false
	}
}

// BookingTeacher / BookingStudent are the parties on an admin booking row.
type BookingTeacher struct {
	Slug        string
	DisplayName string
}

type BookingStudent struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
}

// BookingRow is one row of GET /v1/admin/bookings.
type BookingRow struct {
	ID         uuid.UUID
	Status     string
	StartAt    time.Time
	EndAt      time.Time
	Teacher    BookingTeacher
	Student    BookingStudent
	PriceMinor int64
	Currency   string
	// PaymentStatus is "" when the booking has no payment intent yet.
	PaymentStatus  string
	CreatedAt      time.Time
	HasOpenDispute bool
}

// BookingsPage is a page of bookings plus the total match count.
type BookingsPage struct {
	Bookings []BookingRow
	Total    int
}

// BookingPayment is the payment intent on a booking detail (nil when there is
// none).
type BookingPayment struct {
	Status      string
	AmountMinor int64
	Currency    string
}

// BookingDispute is one entry of the dispute thread on a booking detail. The
// admin surface reads these straight from the disputes table (it is an ops tool
// — see the package doc), so it carries its own read model rather than
// importing internal/disputes.
type BookingDispute struct {
	ID         uuid.UUID
	Status     string
	Reason     string
	Resolution string
	RaisedBy   UserRef
	ResolvedBy *UserRef
	CreatedAt  time.Time
	ResolvedAt *time.Time
}

// UserRef is the light account summary on a dispute entry.
type UserRef struct {
	ID          uuid.UUID
	DisplayName string
}

// BookingDetail is the whole answer to GET /v1/admin/bookings/{id}: the list-row
// fields plus the lifecycle extras, the payment, and the full dispute thread.
type BookingDetail struct {
	BookingRow

	DurationMinutes int
	IsTrial         bool

	Payment *BookingPayment
	// MeetingURL is the effective link (per-booking override, else the teacher's
	// default). Operators see it regardless of booking status — unlike the
	// participant-facing Booking DTO, which hides it before confirmation.
	MeetingURL  string
	NoShowParty string

	CancelledAt        *time.Time
	CancelledBy        string
	CancellationReason string

	Disputes []BookingDispute
}

// ModerationListParams / UsersListParams are the validated list inputs.
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

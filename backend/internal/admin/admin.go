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
)

// ValidationError is a client-fixable problem with a request (e.g. a missing
// moderation note). The handler renders it as 400.
type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return ValidationError{msg: fmt.Sprintf(format, args...)}
}

// --- phase A: dashboard ---

// Metrics is the answer to GET /v1/admin/metrics.
type Metrics struct {
	UsersTotal       int64
	TeachersTotal    int64
	TeachersPending  int64
	BookingsTotal    int64
	BookingsThisWeek int64
	GMVMinor         int64  // sum of price_minor over confirmed + completed bookings
	GMVCurrency      string // "UZS" for the MVP (only supported currency)
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

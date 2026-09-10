// Package reviews is the Phase-6 lesson-review domain module: a student rates a
// completed lesson 1–5 with an optional comment, and the teacher's display
// aggregate (teachers.rating / review_count) is nudged in the same transaction.
//
// Layout mirrors the other modules: domain types + errors here, a Service with
// the rules, a Repository port (Postgres impl alongside, fake in tests), gin
// handlers, and RegisterRoutes funcs. The Service never sees a *gin.Context.
//
// Boundary with internal/bookings: bookings defines the port it needs
// (bookings.ReviewReader) and this package implements it (BookingGateway in
// booking_gateway.go). bookings never imports this package; cmd/api and
// internal/httpapi wire the concrete types together.
package reviews

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MaxCommentLen caps a review comment (characters). Longer -> 400.
const MaxCommentLen = 2000

// completedStatus is the one booking status a review is allowed for. Kept as a
// string so this package does not import internal/bookings.
const completedStatus = "completed"

// Review is the domain aggregate. BookingID is nil for the seeded sample rows.
type Review struct {
	ID          uuid.UUID
	TeacherID   uuid.UUID
	TeacherSlug string
	StudentID   uuid.UUID
	StudentName string
	BookingID   *uuid.UUID
	Rating      int
	Comment     string
	CreatedAt   time.Time
}

// BookingRef is the slice of a booking the review flow needs to authorize the
// caller and build the review.
type BookingRef struct {
	ID          uuid.UUID
	TeacherID   uuid.UUID
	TeacherSlug string
	StudentID   uuid.UUID
	StudentName string
	Status      string
}

// CreateParams is the repository's insert payload for a real, booking-tied
// review. The teacher aggregate is bumped in the same transaction.
type CreateParams struct {
	TeacherID   uuid.UUID
	TeacherSlug string
	StudentID   uuid.UUID
	StudentName string
	BookingID   uuid.UUID
	Rating      int
	Comment     string
}

// Page is a page of a teacher's reviews plus the total count.
type Page struct {
	Reviews []Review
	Total   int
}

// --- phase F: moderation ---

// AdminReview is one row of the operator moderation queue: the review plus the
// teacher it is about and its current visibility.
type AdminReview struct {
	ID          uuid.UUID
	TeacherSlug string
	TeacherName string
	StudentName string
	BookingID   *uuid.UUID // nil for a seeded sample review
	Rating      int
	Comment     string
	Hidden      bool
	CreatedAt   time.Time
}

// AdminPage is a page of the moderation queue plus the total match count.
type AdminPage struct {
	Reviews []AdminReview
	Total   int
}

// Visibility is the moderation-queue visibility filter.
type Visibility string

const (
	VisibilityAll     Visibility = ""        // both hidden and visible
	VisibilityVisible Visibility = "visible" // shown on the teacher profile
	VisibilityHidden  Visibility = "hidden"  // removed from display by an operator
)

// ModerationQuery is the validated input to the moderation list.
type ModerationQuery struct {
	Visibility  Visibility
	TeacherSlug string // "" = any teacher
	MaxRating   int    // 0 = no ceiling; else 1..5
	Page        int
	PageSize    int
}

// Domain errors. The handler maps each to an HTTP status; anything else is 500.
var (
	// ErrTeacherNotFound — no teacher has the requested slug.
	ErrTeacherNotFound = errors.New("reviews: teacher not found")

	// ErrBookingNotFound — no booking has the requested id.
	ErrBookingNotFound = errors.New("reviews: booking not found")

	// ErrNotStudent — the caller is not the student on the booking. Rendered 403.
	ErrNotStudent = errors.New("reviews: only the student on this booking can review it")

	// ErrBookingNotCompleted — the booking has not reached `completed`. Rendered
	// 409 booking_not_completed.
	ErrBookingNotCompleted = errors.New("reviews: booking is not completed")

	// ErrAlreadyReviewed — a review already exists for this booking (the
	// partial-unique index rejected the insert). Rendered 409 already_reviewed.
	ErrAlreadyReviewed = errors.New("reviews: this booking has already been reviewed")

	// ErrReviewNotFound — a moderation action for an unknown review id. Rendered 404.
	ErrReviewNotFound = errors.New("reviews: review not found")
)

// ValidationError is a client-fixable problem with a request. The handler
// renders it as 400.
type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return ValidationError{msg: fmt.Sprintf(format, args...)}
}

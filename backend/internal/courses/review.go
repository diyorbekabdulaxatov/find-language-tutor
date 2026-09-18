// Phase D2 domain types: course ratings and reviews.
//
// This lives inside internal/courses rather than in internal/reviews (which
// owns lesson reviews) for one structural reason: a course review's
// eligibility gate is the enrollment and its side effect is the course's own
// display aggregate, and courses already owns both tables. Putting it next
// door would mean a cross-module port for the eligibility read plus a second
// one back for the aggregate write, to share a rating scale and nothing else.
package courses

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// MaxReviewCommentLen caps a review comment (runes). Longer -> 400. Matches
// internal/reviews' cap so the two review surfaces feel the same.
const MaxReviewCommentLen = 2000

// Review is one buyer's standing opinion of one course. StudentName is joined
// in for display and is empty on the write paths that don't need it.
type Review struct {
	ID           uuid.UUID
	CourseID     uuid.UUID
	EnrollmentID uuid.UUID
	StudentID    uuid.UUID
	StudentName  string
	Rating       int
	Comment      string
	// Hidden is an operator takedown: the row stays (so its author still
	// counts as having reviewed and can't slip a second one past the UNIQUE)
	// but drops out of the public list and the rating aggregate.
	Hidden    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ReviewPage is a page of a course's public reviews plus the total count and
// the star histogram (index 0 = 1★ … index 4 = 5★) over every visible review,
// not just this page.
type ReviewPage struct {
	Reviews   []Review
	Total     int
	Breakdown [5]int
}

// AdminReview is one row of the operator moderation queue.
type AdminReview struct {
	ID          uuid.UUID
	CourseID    uuid.UUID
	CourseTitle string
	StudentName string
	Rating      int
	Comment     string
	Hidden      bool
	CreatedAt   time.Time
}

// AdminReviewPage is a page of the moderation queue plus the total match count.
type AdminReviewPage struct {
	Reviews []AdminReview
	Total   int
}

// ReviewVisibility filters the moderation queue.
type ReviewVisibility string

const (
	ReviewVisibilityAll     ReviewVisibility = ""        // hidden and visible
	ReviewVisibilityVisible ReviewVisibility = "visible" // shown on the course page
	ReviewVisibilityHidden  ReviewVisibility = "hidden"  // pulled by an operator
)

// AdminReviewQuery is the validated input to the moderation list.
type AdminReviewQuery struct {
	Visibility ReviewVisibility
	CourseID   *uuid.UUID // nil = any course
	MaxRating  int        // 0 = no ceiling; else 1..5
	Page       int
	PageSize   int
}

// Domain errors (phase D2).
var (
	// ErrNotEnrolled — the caller hasn't bought the course, so has no
	// standing to review it. Rendered 403.
	ErrNotEnrolled = errors.New("courses: you must be enrolled to review this course")

	// ErrAlreadyReviewed — this enrollment already has a review (the UNIQUE
	// constraint rejected the insert). Rendered 409; the client should PATCH
	// the existing review instead.
	ErrAlreadyReviewed = errors.New("courses: you have already reviewed this course")

	// ErrReviewNotFound — no review for this course/caller, or an unknown
	// review id in the moderation queue. Rendered 404.
	ErrReviewNotFound = errors.New("courses: review not found")

	// ErrReviewsUnavailable — no ReviewRepository was wired
	// (SetReviewRepository never called). Rendered 503, matching how a missing
	// PaymentGateway is handled.
	ErrReviewsUnavailable = errors.New("courses: course reviews aren't available right now")

	// ErrCannotReviewOwnCourse — the caller's own teacher profile owns the
	// course. Rendered 403.
	ErrCannotReviewOwnCourse = errors.New("courses: you can't review your own course")
)

// CreateReviewParams is the repository's review-insert payload.
type CreateReviewParams struct {
	CourseID     uuid.UUID
	EnrollmentID uuid.UUID
	StudentID    uuid.UUID
	Rating       int
	Comment      string
}

package reviews

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"
)

// Repository is the persistence port. The concrete implementation
// (repositoryPostgres) lives alongside; tests use a fake.
type Repository interface {
	// BookingForReview returns the fields needed to authorize + build a review,
	// or ErrBookingNotFound.
	BookingForReview(ctx context.Context, bookingID uuid.UUID) (BookingRef, error)

	// CreateReview inserts the review and bumps the teacher aggregate in one
	// transaction. It maps the reviews_booking_uniq violation (SQLSTATE 23505)
	// to ErrAlreadyReviewed — race-safe, never a check-then-insert.
	CreateReview(ctx context.Context, p CreateParams) (Review, error)

	// ReviewByBooking returns the review for a booking; ok is false when none.
	ReviewByBooking(ctx context.Context, bookingID uuid.UUID) (Review, bool, error)

	// TeacherIDBySlug resolves a slug, or ErrTeacherNotFound.
	TeacherIDBySlug(ctx context.Context, slug string) (uuid.UUID, error)

	// ListByTeacher returns a page of a teacher's reviews (newest first) and the
	// total count.
	ListByTeacher(ctx context.Context, teacherID uuid.UUID, limit, offset int) ([]Review, int, error)

	// --- phase F: moderation ---

	// ListForModeration returns a page of the moderation queue (newest first)
	// and the total match count for the query's filters.
	ListForModeration(ctx context.Context, q ModerationQuery, limit, offset int) ([]AdminReview, int, error)

	// SetReviewHidden sets a review's `hidden` flag and recomputes the teacher's
	// aggregate in one transaction, returning the updated row. ErrReviewNotFound
	// when the id is unknown. Idempotent: hiding an already-hidden review is a
	// no-op that still returns it.
	SetReviewHidden(ctx context.Context, reviewID uuid.UUID, hidden bool) (AdminReview, error)

	// RemoveReview permanently deletes a review and recomputes the teacher's
	// aggregate in one transaction. ErrReviewNotFound when the id is unknown.
	RemoveReview(ctx context.Context, reviewID uuid.UUID) error
}

// Service holds the review business rules. Handlers call it; it never sees a
// *gin.Context.
type Service struct {
	repo   Repository
	logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{repo: repo, logger: logger}
}

const (
	defaultPageSize = 10
	maxPageSize     = 50
)

// Create records the caller's review of a completed booking and nudges the
// teacher aggregate. Student-only; the booking must be `completed`; one review
// per booking (enforced by the DB unique index, surfaced as ErrAlreadyReviewed).
func (s *Service) Create(ctx context.Context, callerID, bookingID uuid.UUID, rating int, comment string) (Review, error) {
	comment = strings.TrimSpace(comment)
	if rating < 1 || rating > 5 {
		return Review{}, invalid("`rating` must be between 1 and 5.")
	}
	if len([]rune(comment)) > MaxCommentLen {
		return Review{}, invalid("`comment` must be at most %d characters.", MaxCommentLen)
	}

	b, err := s.repo.BookingForReview(ctx, bookingID)
	if err != nil {
		return Review{}, err
	}
	if b.StudentID != callerID {
		return Review{}, ErrNotStudent
	}
	if b.Status != completedStatus {
		return Review{}, ErrBookingNotCompleted
	}

	return s.repo.CreateReview(ctx, CreateParams{
		TeacherID:   b.TeacherID,
		TeacherSlug: b.TeacherSlug,
		StudentID:   b.StudentID,
		StudentName: b.StudentName,
		BookingID:   bookingID,
		Rating:      rating,
		Comment:     comment,
	})
}

// ListForTeacher returns a page of a teacher's reviews, newest first. Public.
// page defaults to 1, pageSize to 10 (capped at 50). 404 for an unknown slug.
func (s *Service) ListForTeacher(ctx context.Context, slug string, page, pageSize int) (Page, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	teacherID, err := s.repo.TeacherIDBySlug(ctx, slug)
	if err != nil {
		return Page{}, err
	}

	items, total, err := s.repo.ListByTeacher(ctx, teacherID, pageSize, (page-1)*pageSize)
	if err != nil {
		return Page{}, err
	}
	if items == nil {
		items = []Review{}
	}
	return Page{Reviews: items, Total: total}, nil
}

// ReviewForBooking returns the review attached to a booking (ok=false when the
// booking has not been reviewed). Backs the bookings.ReviewReader port.
func (s *Service) ReviewForBooking(ctx context.Context, bookingID uuid.UUID) (Review, bool, error) {
	return s.repo.ReviewByBooking(ctx, bookingID)
}

// --- phase F: moderation ---

const (
	moderationDefaultPageSize = 20
	moderationMaxPageSize     = 100
)

// Moderate returns a page of the review moderation queue. Operator-only (the
// handler enforces `reviews.moderate`). page defaults to 1, pageSize to 20
// (capped at 100); an out-of-range MaxRating or unknown Visibility is a 400.
func (s *Service) Moderate(ctx context.Context, q ModerationQuery) (AdminPage, error) {
	switch q.Visibility {
	case VisibilityAll, VisibilityVisible, VisibilityHidden:
	default:
		return AdminPage{}, invalid("`visibility` must be one of visible, hidden.")
	}
	if q.MaxRating < 0 || q.MaxRating > 5 {
		return AdminPage{}, invalid("`max_rating` must be between 1 and 5.")
	}

	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = moderationDefaultPageSize
	}
	if q.PageSize > moderationMaxPageSize {
		q.PageSize = moderationMaxPageSize
	}

	items, total, err := s.repo.ListForModeration(ctx, q, q.PageSize, (q.Page-1)*q.PageSize)
	if err != nil {
		return AdminPage{}, err
	}
	if items == nil {
		items = []AdminReview{}
	}
	return AdminPage{Reviews: items, Total: total}, nil
}

// Hide removes a review from public display; Unhide restores it. Both recompute
// the teacher's rating aggregate. Idempotent.
func (s *Service) Hide(ctx context.Context, reviewID uuid.UUID) (AdminReview, error) {
	return s.repo.SetReviewHidden(ctx, reviewID, true)
}

func (s *Service) Unhide(ctx context.Context, reviewID uuid.UUID) (AdminReview, error) {
	return s.repo.SetReviewHidden(ctx, reviewID, false)
}

// Remove permanently deletes a review and recomputes the teacher aggregate.
func (s *Service) Remove(ctx context.Context, reviewID uuid.UUID) error {
	return s.repo.RemoveReview(ctx, reviewID)
}

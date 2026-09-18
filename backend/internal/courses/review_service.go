package courses

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// ReviewRepository is the phase-D2 persistence port. It is declared separately
// from Repository (rather than appended to it) so the review surface can be
// read, and faked in tests, without dragging in the whole authoring port.
type ReviewRepository interface {
	// CreateReview inserts the review and recomputes the course aggregate in
	// one transaction. A second review for the same enrollment raises the
	// UNIQUE violation, mapped to ErrAlreadyReviewed — race-safe, never a
	// check-then-insert.
	CreateReview(ctx context.Context, p CreateReviewParams) (Review, error)
	// UpdateReview edits the author's own review and recomputes the aggregate
	// in one transaction. Scoped by studentID, so it can never touch someone
	// else's row. ErrReviewNotFound when there is no such review for them.
	UpdateReview(ctx context.Context, reviewID, studentID uuid.UUID, rating int, comment string) (Review, error)
	// ReviewByEnrollment returns the review written against one enrollment.
	// ok is false when the buyer hasn't reviewed yet.
	ReviewByEnrollment(ctx context.Context, enrollmentID uuid.UUID) (Review, bool, error)
	// ListReviews returns a page of a course's VISIBLE reviews (newest first),
	// the total visible count, and the star histogram over all of them.
	ListReviews(ctx context.Context, courseID uuid.UUID, limit, offset int) (ReviewPage, error)

	// --- moderation ---

	// AdminListReviews returns a page of the moderation queue and the total
	// match count.
	AdminListReviews(ctx context.Context, q AdminReviewQuery, limit, offset int) ([]AdminReview, int, error)
	// SetReviewHidden flips a review's visibility and recomputes its course's
	// aggregate in one transaction. Idempotent. ErrReviewNotFound for an
	// unknown id.
	SetReviewHidden(ctx context.Context, reviewID uuid.UUID, hidden bool) (AdminReview, error)
}

// SetReviewRepository wires the phase-D2 review store in. Optional: without
// it every review endpoint fails closed with ErrReviewsUnavailable, the same
// guarded-port discipline the rest of this module uses.
func (s *Service) SetReviewRepository(r ReviewRepository) { s.reviews = r }

// CreateReview records the caller's review of a course they bought, and
// nudges the course's display aggregate in the same transaction.
//
// Eligibility is the enrollment: 403 if the caller hasn't bought the course,
// 403 if they own it, 409 if they have already reviewed it (PATCH instead).
func (s *Service) CreateReview(ctx context.Context, callerID, courseID uuid.UUID, rating int, comment string) (Review, error) {
	e, c, err := s.reviewerEnrollment(ctx, callerID, courseID)
	if err != nil {
		return Review{}, err
	}
	if err := validateReviewInput(rating, &comment); err != nil {
		return Review{}, err
	}
	return s.reviews.CreateReview(ctx, CreateReviewParams{
		CourseID: c.ID, EnrollmentID: e.ID, StudentID: callerID,
		Rating: rating, Comment: comment,
	})
}

// UpdateReview edits the caller's own review. A course review is a standing
// opinion of something the student keeps using, so changing your mind is a
// first-class action rather than a moderator's job.
func (s *Service) UpdateReview(ctx context.Context, callerID, courseID uuid.UUID, rating int, comment string) (Review, error) {
	e, _, err := s.reviewerEnrollment(ctx, callerID, courseID)
	if err != nil {
		return Review{}, err
	}
	if err := validateReviewInput(rating, &comment); err != nil {
		return Review{}, err
	}
	existing, ok, err := s.reviews.ReviewByEnrollment(ctx, e.ID)
	if err != nil {
		return Review{}, err
	}
	if !ok {
		return Review{}, ErrReviewNotFound
	}
	return s.reviews.UpdateReview(ctx, existing.ID, callerID, rating, comment)
}

// MyReview returns the caller's own review of a course, if any. Unlike the
// create/update paths it does NOT require a live enrollment — a hidden review
// is still returned to its author, so the landing page can show them what
// they wrote rather than silently offering a "write a review" button that
// will 409.
func (s *Service) MyReview(ctx context.Context, callerID, courseID uuid.UUID) (Review, bool, error) {
	if s.reviews == nil || callerID == uuid.Nil {
		return Review{}, false, nil
	}
	e, ok, err := s.repo.EnrollmentByCourseAndStudent(ctx, courseID, callerID)
	if err != nil || !ok {
		return Review{}, false, err
	}
	return s.reviews.ReviewByEnrollment(ctx, e.ID)
}

// Reviews returns a page of a course's public review list. Unauthenticated —
// but only for a course that is actually on the storefront, so an unpublished
// or suspended course's reviews never leak.
func (s *Service) Reviews(ctx context.Context, courseID uuid.UUID, page, pageSize int) (ReviewPage, error) {
	if s.reviews == nil {
		return ReviewPage{}, ErrReviewsUnavailable
	}
	c, err := s.repo.ByID(ctx, courseID)
	if err != nil {
		return ReviewPage{}, err
	}
	if c.Status != StatusPublished || c.ArchivedAt != nil || c.SuspendedAt != nil {
		return ReviewPage{}, ErrNotFound
	}
	teacher, err := s.repo.TeacherSummaryByID(ctx, c.TeacherID)
	if err != nil {
		return ReviewPage{}, err
	}
	if !teacher.Approved {
		return ReviewPage{}, ErrNotFound
	}

	page, pageSize = clampPage(page, pageSize)
	res, err := s.reviews.ListReviews(ctx, courseID, pageSize, (page-1)*pageSize)
	if err != nil {
		return ReviewPage{}, err
	}
	if res.Reviews == nil {
		res.Reviews = []Review{}
	}
	return res, nil
}

// --- moderation ---

// AdminReviews returns a page of the course-review moderation queue.
func (s *Service) AdminReviews(ctx context.Context, q AdminReviewQuery) (AdminReviewPage, error) {
	if s.reviews == nil {
		return AdminReviewPage{}, ErrReviewsUnavailable
	}
	switch q.Visibility {
	case ReviewVisibilityAll, ReviewVisibilityVisible, ReviewVisibilityHidden:
	default:
		return AdminReviewPage{}, invalid("`visibility` must be one of visible, hidden.")
	}
	if q.MaxRating < 0 || q.MaxRating > 5 {
		return AdminReviewPage{}, invalid("`max_rating` must be between 1 and 5.")
	}
	q.Page, q.PageSize = clampPage(q.Page, q.PageSize)

	rows, total, err := s.reviews.AdminListReviews(ctx, q, q.PageSize, (q.Page-1)*q.PageSize)
	if err != nil {
		return AdminReviewPage{}, err
	}
	if rows == nil {
		rows = []AdminReview{}
	}
	return AdminReviewPage{Reviews: rows, Total: total}, nil
}

// SetReviewHidden hides or restores one course review, recomputing the
// course's rating in the same transaction.
func (s *Service) SetReviewHidden(ctx context.Context, reviewID uuid.UUID, hidden bool) (AdminReview, error) {
	if s.reviews == nil {
		return AdminReview{}, ErrReviewsUnavailable
	}
	return s.reviews.SetReviewHidden(ctx, reviewID, hidden)
}

// --- helpers ---

// reviewerEnrollment resolves the caller's standing to review a course: the
// course must exist and be reviewable, the caller must not own it, and they
// must be enrolled.
func (s *Service) reviewerEnrollment(ctx context.Context, callerID, courseID uuid.UUID) (Enrollment, Course, error) {
	if s.reviews == nil {
		return Enrollment{}, Course{}, ErrReviewsUnavailable
	}
	c, err := s.repo.ByID(ctx, courseID)
	if err != nil {
		return Enrollment{}, Course{}, err
	}
	// A suspended or unpublished course is still reviewable by someone who
	// already bought it — they paid for it and their opinion of it did not
	// stop being true. Only *new* buyers are turned away (see Purchase).
	if s.isCourseOwner(ctx, callerID, c.TeacherID) {
		return Enrollment{}, Course{}, ErrCannotReviewOwnCourse
	}
	e, ok, err := s.repo.EnrollmentByCourseAndStudent(ctx, courseID, callerID)
	if err != nil {
		return Enrollment{}, Course{}, err
	}
	if !ok {
		return Enrollment{}, Course{}, ErrNotEnrolled
	}
	return e, c, nil
}

func validateReviewInput(rating int, comment *string) error {
	*comment = strings.TrimSpace(*comment)
	if rating < 1 || rating > 5 {
		return invalid("`rating` must be between 1 and 5.")
	}
	if len([]rune(*comment)) > MaxReviewCommentLen {
		return invalid("`comment` must be at most %d characters.", MaxReviewCommentLen)
	}
	return nil
}

// clampPage applies the module's shared paging bounds.
func clampPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

package courses

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

type reviewRepositoryPostgres struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// NewPostgresReviewRepository builds a ReviewRepository backed by the pool.
func NewPostgresReviewRepository(pool *pgxpool.Pool) ReviewRepository {
	return &reviewRepositoryPostgres{pool: pool, q: sqlc.New(pool)}
}

// CreateReview inserts the review and rebuilds the course aggregate in one
// transaction. The insert goes first, so a duplicate is rejected before the
// rating moves.
func (r *reviewRepositoryPostgres) CreateReview(ctx context.Context, p CreateReviewParams) (Review, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Review{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op
	qtx := r.q.WithTx(tx)

	row, err := qtx.InsertCourseReview(ctx, sqlc.InsertCourseReviewParams{
		CourseID: p.CourseID, EnrollmentID: p.EnrollmentID, StudentID: p.StudentID,
		Rating: int16(p.Rating), Comment: p.Comment,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return Review{}, ErrAlreadyReviewed
		}
		return Review{}, fmt.Errorf("insert course review: %w", err)
	}
	if err := qtx.RecomputeCourseRating(ctx, p.CourseID); err != nil {
		return Review{}, fmt.Errorf("recompute course rating: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Review{}, fmt.Errorf("commit: %w", err)
	}
	return toReview(row), nil
}

func (r *reviewRepositoryPostgres) UpdateReview(ctx context.Context, reviewID, studentID uuid.UUID, rating int, comment string) (Review, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Review{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := r.q.WithTx(tx)

	row, err := qtx.UpdateCourseReview(ctx, sqlc.UpdateCourseReviewParams{
		ID: reviewID, StudentID: studentID, Rating: int16(rating), Comment: comment,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Review{}, ErrReviewNotFound
		}
		return Review{}, fmt.Errorf("update course review: %w", err)
	}
	if err := qtx.RecomputeCourseRating(ctx, row.CourseID); err != nil {
		return Review{}, fmt.Errorf("recompute course rating: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Review{}, fmt.Errorf("commit: %w", err)
	}
	return toReview(row), nil
}

func (r *reviewRepositoryPostgres) ReviewByEnrollment(ctx context.Context, enrollmentID uuid.UUID) (Review, bool, error) {
	row, err := r.q.GetCourseReviewByEnrollment(ctx, enrollmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Review{}, false, nil
		}
		return Review{}, false, fmt.Errorf("get course review by enrollment: %w", err)
	}
	return toReview(row), true, nil
}

func (r *reviewRepositoryPostgres) ListReviews(ctx context.Context, courseID uuid.UUID, limit, offset int) (ReviewPage, error) {
	rows, err := r.q.ListCourseReviews(ctx, sqlc.ListCourseReviewsParams{
		CourseID: courseID, PageLimit: int32(limit), PageOffset: int32(offset),
	})
	if err != nil {
		return ReviewPage{}, fmt.Errorf("list course reviews: %w", err)
	}
	total, err := r.q.CountCourseReviews(ctx, courseID)
	if err != nil {
		return ReviewPage{}, fmt.Errorf("count course reviews: %w", err)
	}
	breakdownRows, err := r.q.CourseRatingBreakdown(ctx, courseID)
	if err != nil {
		return ReviewPage{}, fmt.Errorf("course rating breakdown: %w", err)
	}

	out := make([]Review, len(rows))
	for i, row := range rows {
		out[i] = Review{
			ID: row.ID, CourseID: courseID, StudentName: row.StudentDisplayName,
			Rating: int(row.Rating), Comment: row.Comment,
			CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
		}
	}
	// The query returns only the star values that actually occur; fill the
	// rest with zeros so the histogram always has all five buckets.
	var breakdown [5]int
	for _, b := range breakdownRows {
		if b.Rating >= 1 && b.Rating <= 5 {
			breakdown[b.Rating-1] = int(b.N)
		}
	}
	return ReviewPage{Reviews: out, Total: int(total), Breakdown: breakdown}, nil
}

func (r *reviewRepositoryPostgres) AdminListReviews(ctx context.Context, q AdminReviewQuery, limit, offset int) ([]AdminReview, int, error) {
	hidden := pgtype.Bool{}
	switch q.Visibility {
	case ReviewVisibilityVisible:
		hidden = pgtype.Bool{Bool: false, Valid: true}
	case ReviewVisibilityHidden:
		hidden = pgtype.Bool{Bool: true, Valid: true}
	}
	courseID := uuid.NullUUID{}
	if q.CourseID != nil {
		courseID = uuid.NullUUID{UUID: *q.CourseID, Valid: true}
	}
	maxRating := pgtype.Int4{}
	if q.MaxRating > 0 {
		maxRating = pgtype.Int4{Int32: int32(q.MaxRating), Valid: true}
	}

	rows, err := r.q.AdminListCourseReviews(ctx, sqlc.AdminListCourseReviewsParams{
		Hidden: hidden, CourseID: courseID, MaxRating: maxRating,
		PageLimit: int32(limit), PageOffset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("admin list course reviews: %w", err)
	}
	total, err := r.q.AdminCountCourseReviews(ctx, sqlc.AdminCountCourseReviewsParams{
		Hidden: hidden, CourseID: courseID, MaxRating: maxRating,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("admin count course reviews: %w", err)
	}

	out := make([]AdminReview, len(rows))
	for i, row := range rows {
		out[i] = AdminReview{
			ID: row.ID, CourseID: row.CourseID, CourseTitle: row.CourseTitle,
			StudentName: row.StudentDisplayName, Rating: int(row.Rating),
			Comment: row.Comment, Hidden: row.Hidden, CreatedAt: row.CreatedAt.Time.UTC(),
		}
	}
	return out, int(total), nil
}

// SetReviewHidden flips visibility and rebuilds the course aggregate in one
// transaction — a hidden review must leave the rating in the same statement
// it leaves the list, or the two disagree for as long as the gap lasts.
func (r *reviewRepositoryPostgres) SetReviewHidden(ctx context.Context, reviewID uuid.UUID, hidden bool) (AdminReview, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return AdminReview{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := r.q.WithTx(tx)

	row, err := qtx.SetCourseReviewHidden(ctx, sqlc.SetCourseReviewHiddenParams{ID: reviewID, Hidden: hidden})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AdminReview{}, ErrReviewNotFound
		}
		return AdminReview{}, fmt.Errorf("set course review hidden: %w", err)
	}
	if err := qtx.RecomputeCourseRating(ctx, row.CourseID); err != nil {
		return AdminReview{}, fmt.Errorf("recompute course rating: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return AdminReview{}, fmt.Errorf("commit: %w", err)
	}

	// The RETURNING row can't carry the course title / student name (the
	// UPDATE touches one table), so re-read the queue row by id for a
	// complete response.
	full, err := r.q.AdminGetCourseReview(ctx, row.ID)
	if err != nil {
		return AdminReview{}, fmt.Errorf("admin get course review: %w", err)
	}
	return AdminReview{
		ID: full.ID, CourseID: full.CourseID, CourseTitle: full.CourseTitle,
		StudentName: full.StudentDisplayName, Rating: int(full.Rating),
		Comment: full.Comment, Hidden: full.Hidden, CreatedAt: full.CreatedAt.Time.UTC(),
	}, nil
}

func toReview(row sqlc.CourseReview) Review {
	return Review{
		ID: row.ID, CourseID: row.CourseID, EnrollmentID: row.EnrollmentID,
		StudentID: row.StudentID, Rating: int(row.Rating), Comment: row.Comment,
		Hidden: row.Hidden, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

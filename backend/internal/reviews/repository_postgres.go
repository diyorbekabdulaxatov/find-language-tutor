package reviews

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

// pgUniqueViolation is SQLSTATE 23505 — raised by reviews_booking_uniq when a
// booking is reviewed twice. It is the race-safe "already reviewed" gate.
const pgUniqueViolation = "23505"

type repositoryPostgres struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// NewPostgresRepository builds a Repository backed by the given pgx pool.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &repositoryPostgres{pool: pool, q: sqlc.New(pool)}
}

func (r *repositoryPostgres) BookingForReview(ctx context.Context, bookingID uuid.UUID) (BookingRef, error) {
	row, err := r.q.GetReviewBookingContext(ctx, bookingID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BookingRef{}, ErrBookingNotFound
		}
		return BookingRef{}, fmt.Errorf("get review booking context: %w", err)
	}
	return BookingRef{
		ID:          row.ID,
		TeacherID:   row.TeacherID,
		TeacherSlug: row.TeacherSlug,
		StudentID:   row.StudentID,
		StudentName: row.StudentDisplayName,
		Status:      row.Status,
	}, nil
}

// CreateReview inserts the review and bumps the teacher aggregate in one
// transaction. The insert is first, so a duplicate is rejected before the
// aggregate moves.
func (r *repositoryPostgres) CreateReview(ctx context.Context, p CreateParams) (Review, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Review{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	qtx := r.q.WithTx(tx)

	row, err := qtx.InsertReview(ctx, sqlc.InsertReviewParams{
		TeacherID: p.TeacherID,
		StudentID: p.StudentID,
		BookingID: uuid.NullUUID{UUID: p.BookingID, Valid: true},
		Rating:    int16(p.Rating),
		Comment:   p.Comment,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return Review{}, ErrAlreadyReviewed
		}
		return Review{}, fmt.Errorf("insert review: %w", err)
	}

	if err := qtx.BumpTeacherRatingForReview(ctx, sqlc.BumpTeacherRatingForReviewParams{
		NewRating: int32(p.Rating),
		TeacherID: p.TeacherID,
	}); err != nil {
		return Review{}, fmt.Errorf("bump teacher rating: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Review{}, fmt.Errorf("commit: %w", err)
	}

	bid := p.BookingID
	return Review{
		ID:          row.ID,
		TeacherID:   p.TeacherID,
		TeacherSlug: p.TeacherSlug,
		StudentID:   p.StudentID,
		StudentName: p.StudentName,
		BookingID:   &bid,
		Rating:      int(row.Rating),
		Comment:     row.Comment,
		CreatedAt:   row.CreatedAt.Time.UTC(),
	}, nil
}

func (r *repositoryPostgres) ReviewByBooking(ctx context.Context, bookingID uuid.UUID) (Review, bool, error) {
	row, err := r.q.GetReviewByBooking(ctx, uuid.NullUUID{UUID: bookingID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Review{}, false, nil
		}
		return Review{}, false, fmt.Errorf("get review by booking: %w", err)
	}
	rev := Review{
		ID:          row.ID,
		TeacherID:   row.TeacherID,
		TeacherSlug: row.TeacherSlug,
		StudentID:   row.StudentID,
		StudentName: row.StudentDisplayName,
		Rating:      int(row.Rating),
		Comment:     row.Comment,
		CreatedAt:   row.CreatedAt.Time.UTC(),
	}
	if row.BookingID.Valid {
		b := row.BookingID.UUID
		rev.BookingID = &b
	}
	return rev, true, nil
}

func (r *repositoryPostgres) TeacherIDBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
	ref, err := r.q.TeacherRefBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrTeacherNotFound
		}
		return uuid.Nil, fmt.Errorf("teacher ref by slug: %w", err)
	}
	return ref.ID, nil
}

func (r *repositoryPostgres) ListByTeacher(ctx context.Context, teacherID uuid.UUID, limit, offset int) ([]Review, int, error) {
	rows, err := r.q.ListTeacherReviews(ctx, sqlc.ListTeacherReviewsParams{
		TeacherID:  teacherID,
		PageLimit:  int32(limit),
		PageOffset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list teacher reviews: %w", err)
	}
	total, err := r.q.CountTeacherReviews(ctx, teacherID)
	if err != nil {
		return nil, 0, fmt.Errorf("count teacher reviews: %w", err)
	}

	out := make([]Review, len(rows))
	for i, row := range rows {
		out[i] = Review{
			ID:          row.ID,
			TeacherID:   teacherID,
			StudentName: row.StudentDisplayName,
			Rating:      int(row.Rating),
			Comment:     row.Comment,
			CreatedAt:   row.CreatedAt.Time.UTC(),
		}
	}
	return out, int(total), nil
}

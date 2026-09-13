package payments

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

type courseRepositoryPostgres struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries

	// clearingDays is the payout clearing window (PAYOUTS_CLEARING_DAYS),
	// mirroring repositoryPostgres.clearingDays for the booking flow.
	clearingDays int
}

// NewCoursePostgresRepository builds a CourseRepository backed by the given
// pgx pool. clearingDays is the payout clearing window in days (0 = payable
// immediately) — see InsertCourseLedgerHeld.
func NewCoursePostgresRepository(pool *pgxpool.Pool, clearingDays int) CourseRepository {
	if clearingDays < 0 {
		clearingDays = 0
	}
	return &courseRepositoryPostgres{pool: pool, q: sqlc.New(pool), clearingDays: clearingDays}
}

func rowToCoursePayment(r sqlc.CoursePayment) CoursePayment {
	p := CoursePayment{
		ID:        r.ID,
		CourseID:  r.CourseID,
		StudentID: r.StudentID,
		Provider:  r.Provider,
		Status:    Status(r.Status),
		Amount:    Money{AmountMinor: r.AmountMinor, Currency: string(r.Currency)},
		LastError: r.LastError,
		CreatedAt: r.CreatedAt.Time.UTC(),
		UpdatedAt: r.UpdatedAt.Time.UTC(),
	}
	if r.ProviderRef.Valid {
		p.ProviderRef = r.ProviderRef.String
	}
	p.AuthorizedAt = tsPtr(r.AuthorizedAt)
	p.CapturedAt = tsPtr(r.CapturedAt)
	p.RefundedAt = tsPtr(r.RefundedAt)
	return p
}

func (r *courseRepositoryPostgres) EnsureCoursePayment(ctx context.Context, courseID, studentID uuid.UUID, provider string, amount Money) (CoursePayment, error) {
	row, err := r.q.CreateCoursePayment(ctx, sqlc.CreateCoursePaymentParams{
		CourseID:    courseID,
		StudentID:   studentID,
		Provider:    provider,
		AmountMinor: amount.AmountMinor,
		Currency:    sqlc.CurrencyCode(amount.Currency),
	})
	if err != nil {
		return CoursePayment{}, fmt.Errorf("ensure course payment: %w", err)
	}
	return rowToCoursePayment(row), nil
}

func (r *courseRepositoryPostgres) CoursePaymentByCourse(ctx context.Context, courseID, studentID uuid.UUID) (CoursePayment, error) {
	row, err := r.q.GetCoursePaymentByCourseAndStudent(ctx, sqlc.GetCoursePaymentByCourseAndStudentParams{
		CourseID: courseID, StudentID: studentID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CoursePayment{}, ErrPaymentNotFound
		}
		return CoursePayment{}, fmt.Errorf("get course payment by course and student: %w", err)
	}
	return rowToCoursePayment(row), nil
}

// ApplyCourseEvent — see the CourseRepository interface doc. Mirrors
// repositoryPostgres.ApplyEvent exactly, minus the payout-ledger write on
// capture and the booking-confirm on authorize (a course purchase has no
// booking to move, and course revenue-share is a future phase).
func (r *courseRepositoryPostgres) ApplyCourseEvent(ctx context.Context, e Event) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	qtx := r.q.WithTx(tx)

	if err := qtx.InsertCoursePaymentEvent(ctx, sqlc.InsertCoursePaymentEventParams{
		EventID:   e.ID,
		PaymentID: e.PaymentID,
		Type:      string(e.Type),
	}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case pgUniqueViolation:
				return false, nil // already processed — no-op
			case pgForeignKeyViolation:
				return false, ErrPaymentNotFound // event names a payment we don't have
			}
		}
		return false, fmt.Errorf("insert course payment event: %w", err)
	}

	pmt, err := qtx.GetCoursePaymentByID(ctx, e.PaymentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrPaymentNotFound
		}
		return false, fmt.Errorf("load course payment: %w", err)
	}
	p := rowToCoursePayment(pmt)

	switch e.Type {
	case EventAuthorized:
		if err := qtx.MarkCoursePaymentAuthorized(ctx, sqlc.MarkCoursePaymentAuthorizedParams{
			ID:          p.ID,
			ProviderRef: pgtype.Text{String: e.ProviderRef, Valid: e.ProviderRef != ""},
		}); err != nil {
			return false, fmt.Errorf("mark authorized: %w", err)
		}

	case EventCaptured:
		if err := qtx.MarkCoursePaymentCaptured(ctx, p.ID); err != nil {
			return false, fmt.Errorf("mark captured: %w", err)
		}

	case EventRefunded:
		if err := qtx.MarkCoursePaymentRefunded(ctx, p.ID); err != nil {
			return false, fmt.Errorf("mark refunded: %w", err)
		}

	case EventFailed:
		if err := qtx.MarkCoursePaymentFailed(ctx, sqlc.MarkCoursePaymentFailedParams{
			ID:        p.ID,
			LastError: e.Message,
		}); err != nil {
			return false, fmt.Errorf("mark failed: %w", err)
		}

	default:
		return false, fmt.Errorf("unknown event type %q", e.Type)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}

// InsertCourseLedgerHeld — see the CourseRepository interface doc and
// CreditCourseSale's doc comment for why this is called outside
// ApplyCourseEvent, unlike the booking flow's InsertLedgerHeld.
func (r *courseRepositoryPostgres) InsertCourseLedgerHeld(ctx context.Context, enrollmentID uuid.UUID, teacherShareMinor int64, currency string) error {
	if err := r.q.InsertCourseLedgerHeld(ctx, sqlc.InsertCourseLedgerHeldParams{
		CourseEnrollmentID: enrollmentID,
		AmountMinor:        teacherShareMinor,
		Currency:           currency,
		ClearingDays:       int32(r.clearingDays),
	}); err != nil {
		return fmt.Errorf("insert course ledger held: %w", err)
	}
	return nil
}

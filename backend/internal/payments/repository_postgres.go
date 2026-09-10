package payments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

// pgUniqueViolation is SQLSTATE 23505 — raised by the payment_events primary
// key when an event_id is replayed. It is the race-safe idempotency gate.
const pgUniqueViolation = "23505"

type repositoryPostgres struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
	// clearingDays is the payout clearing window (PAYOUTS_CLEARING_DAYS): a
	// captured lesson's ledger row is written with available_at = now() + this
	// many days and is not payable until then. Stored per row rather than
	// recomputed at read time, so changing the setting never moves money that
	// has already been promised.
	clearingDays int
}

// NewPostgresRepository builds a Repository backed by the given pgx pool.
// clearingDays is the payout clearing window in days (0 = payable immediately).
func NewPostgresRepository(pool *pgxpool.Pool, clearingDays int) Repository {
	if clearingDays < 0 {
		clearingDays = 0
	}
	return &repositoryPostgres{pool: pool, q: sqlc.New(pool), clearingDays: clearingDays}
}

func rowToPayment(r sqlc.Payment) Payment {
	p := Payment{
		ID:        r.ID,
		BookingID: r.BookingID,
		Provider:  r.Provider,
		Status:    Status(r.Status),
		Amount:    Money{AmountMinor: r.AmountMinor, Currency: r.Currency},
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

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	u := t.Time.UTC()
	return &u
}

func (r *repositoryPostgres) EnsurePayment(ctx context.Context, bookingID uuid.UUID, provider string, amount Money) (Payment, error) {
	row, err := r.q.CreatePayment(ctx, sqlc.CreatePaymentParams{
		BookingID:   bookingID,
		Provider:    provider,
		AmountMinor: amount.AmountMinor,
		Currency:    amount.Currency,
	})
	if err != nil {
		return Payment{}, fmt.Errorf("ensure payment: %w", err)
	}
	return rowToPayment(row), nil
}

func (r *repositoryPostgres) PaymentByBooking(ctx context.Context, bookingID uuid.UUID) (Payment, error) {
	row, err := r.q.GetPaymentByBooking(ctx, bookingID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Payment{}, ErrPaymentNotFound
		}
		return Payment{}, fmt.Errorf("get payment by booking: %w", err)
	}
	return rowToPayment(row), nil
}

func (r *repositoryPostgres) PaymentByID(ctx context.Context, id uuid.UUID) (Payment, error) {
	row, err := r.q.GetPaymentByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Payment{}, ErrPaymentNotFound
		}
		return Payment{}, fmt.Errorf("get payment by id: %w", err)
	}
	return rowToPayment(row), nil
}

func (r *repositoryPostgres) TeacherIDByOwner(ctx context.Context, ownerID uuid.UUID) (uuid.UUID, bool, error) {
	id, err := r.q.GetTeacherIDByOwner(ctx, uuid.NullUUID{UUID: ownerID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, fmt.Errorf("teacher id by owner: %w", err)
	}
	return id, true, nil
}

func (r *repositoryPostgres) EarningLines(ctx context.Context, teacherID uuid.UUID) ([]EarningLine, error) {
	rows, err := r.q.ListTeacherEarnings(ctx, teacherID)
	if err != nil {
		return nil, fmt.Errorf("list teacher earnings: %w", err)
	}
	out := make([]EarningLine, len(rows))
	for i, row := range rows {
		out[i] = EarningLine{
			BookingID:          row.BookingID,
			StudentDisplayName: row.StudentDisplayName,
			StartAt:            row.StartAt.Time.UTC(),
			AmountMinor:        row.AmountMinor,
			Currency:           row.Currency,
			State:              LedgerState(row.State),
			AvailableAt:        row.AvailableAt.Time.UTC(),
		}
	}
	return out, nil
}

// ApplyEvent — see the Repository interface doc. The whole recipe runs in one
// transaction; the payment_events insert is the first statement so a replay is
// rejected before anything else changes.
func (r *repositoryPostgres) ApplyEvent(ctx context.Context, e Event) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	qtx := r.q.WithTx(tx)

	if err := qtx.InsertPaymentEvent(ctx, sqlc.InsertPaymentEventParams{
		EventID:   e.ID,
		PaymentID: e.PaymentID,
		Type:      string(e.Type),
	}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return false, nil // already processed — no-op
		}
		return false, fmt.Errorf("insert payment event: %w", err)
	}

	pmt, err := qtx.GetPaymentByID(ctx, e.PaymentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrPaymentNotFound
		}
		return false, fmt.Errorf("load payment: %w", err)
	}
	p := rowToPayment(pmt)

	switch e.Type {
	case EventAuthorized:
		if err := qtx.MarkPaymentAuthorized(ctx, sqlc.MarkPaymentAuthorizedParams{
			ID:          p.ID,
			ProviderRef: pgtype.Text{String: e.ProviderRef, Valid: e.ProviderRef != ""},
		}); err != nil {
			return false, fmt.Errorf("mark authorized: %w", err)
		}
		if err := qtx.ConfirmBookingForPayment(ctx, p.BookingID); err != nil {
			return false, fmt.Errorf("confirm booking: %w", err)
		}

	case EventCaptured:
		if err := qtx.MarkPaymentCaptured(ctx, p.ID); err != nil {
			return false, fmt.Errorf("mark captured: %w", err)
		}
		// The row opens the clearing window and stays `held`; nothing flips it
		// to `available` — that state is derived from available_at at read time,
		// and a payout run (internal/payouts) settles it as `paid`.
		if err := qtx.InsertLedgerHeld(ctx, sqlc.InsertLedgerHeldParams{
			BookingID:    p.BookingID,
			AmountMinor:  p.Amount.AmountMinor,
			Currency:     p.Amount.Currency,
			ClearingDays: int32(r.clearingDays),
		}); err != nil {
			return false, fmt.Errorf("insert ledger: %w", err)
		}

	case EventRefunded:
		if err := qtx.MarkPaymentRefunded(ctx, p.ID); err != nil {
			return false, fmt.Errorf("mark refunded: %w", err)
		}
		if err := qtx.MarkLedgerReversed(ctx, p.BookingID); err != nil {
			return false, fmt.Errorf("reverse ledger: %w", err)
		}

	case EventFailed:
		if err := qtx.MarkPaymentFailed(ctx, sqlc.MarkPaymentFailedParams{
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

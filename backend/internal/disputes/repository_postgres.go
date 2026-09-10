package disputes

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

// pgUniqueViolation is SQLSTATE 23505 — raised by
// disputes_one_open_per_booking when a booking is disputed while an earlier
// dispute is still open. It is the race-safe "already disputed" gate.
const pgUniqueViolation = "23505"

type repositoryPostgres struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// NewPostgresRepository builds a Repository backed by the given pgx pool.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &repositoryPostgres{pool: pool, q: sqlc.New(pool)}
}

func nullText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func (r *repositoryPostgres) BookingForDispute(ctx context.Context, bookingID uuid.UUID) (BookingRef, error) {
	row, err := r.q.GetDisputeBookingContext(ctx, bookingID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BookingRef{}, ErrBookingNotFound
		}
		return BookingRef{}, fmt.Errorf("get dispute booking context: %w", err)
	}
	ref := BookingRef{ID: row.ID, Status: row.Status, StudentID: row.StudentID}
	if row.TeacherUserID.Valid {
		ref.TeacherOwnerID = row.TeacherUserID.UUID
	}
	return ref, nil
}

func (r *repositoryPostgres) Create(ctx context.Context, bookingID, raisedBy uuid.UUID, reason string) (Dispute, error) {
	row, err := r.q.InsertDispute(ctx, sqlc.InsertDisputeParams{
		BookingID: bookingID,
		RaisedBy:  raisedBy,
		Reason:    reason,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return Dispute{}, ErrDisputeExists
		}
		return Dispute{}, fmt.Errorf("insert dispute: %w", err)
	}

	// The INSERT ... RETURNING cannot join the raiser's display name, and the
	// caller is the raiser, so one small follow-up read keeps the response shape
	// identical to every other dispute payload.
	d, err := r.byID(ctx, row.ID)
	if err != nil {
		return Dispute{}, err
	}
	return d, nil
}

func (r *repositoryPostgres) byID(ctx context.Context, id uuid.UUID) (Dispute, error) {
	row, err := r.q.GetDisputeByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Dispute{}, ErrDisputeNotFound
		}
		return Dispute{}, fmt.Errorf("get dispute: %w", err)
	}
	return Dispute{
		ID:         row.ID,
		BookingID:  row.BookingID,
		Status:     Status(row.Status),
		Reason:     row.Reason,
		Resolution: row.Resolution,
		RaisedBy:   UserRef{ID: row.RaisedBy, DisplayName: row.RaisedByDisplayName},
		ResolvedBy: userRef(row.ResolvedBy, row.ResolvedByDisplayName),
		CreatedAt:  row.CreatedAt.Time.UTC(),
		ResolvedAt: utcPtr(row.ResolvedAt),
	}, nil
}

func (r *repositoryPostgres) ListForBooking(ctx context.Context, bookingID uuid.UUID) ([]Dispute, error) {
	rows, err := r.q.ListDisputesForBooking(ctx, bookingID)
	if err != nil {
		return nil, fmt.Errorf("list disputes for booking: %w", err)
	}
	out := make([]Dispute, len(rows))
	for i, row := range rows {
		out[i] = Dispute{
			ID:         row.ID,
			BookingID:  row.BookingID,
			Status:     Status(row.Status),
			Reason:     row.Reason,
			Resolution: row.Resolution,
			RaisedBy:   UserRef{ID: row.RaisedBy, DisplayName: row.RaisedByDisplayName},
			ResolvedBy: userRef(row.ResolvedBy, row.ResolvedByDisplayName),
			CreatedAt:  row.CreatedAt.Time.UTC(),
			ResolvedAt: utcPtr(row.ResolvedAt),
		}
	}
	return out, nil
}

func (r *repositoryPostgres) OpenForBooking(ctx context.Context, bookingID uuid.UUID) (Dispute, bool, error) {
	row, err := r.q.GetOpenDisputeForBooking(ctx, bookingID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Dispute{}, false, nil
		}
		return Dispute{}, false, fmt.Errorf("get open dispute: %w", err)
	}
	return Dispute{
		ID:         row.ID,
		BookingID:  row.BookingID,
		Status:     Status(row.Status),
		Reason:     row.Reason,
		Resolution: row.Resolution,
		RaisedBy:   UserRef{ID: row.RaisedBy, DisplayName: row.RaisedByDisplayName},
		CreatedAt:  row.CreatedAt.Time.UTC(),
	}, true, nil
}

func (r *repositoryPostgres) ListQueue(ctx context.Context, status string, limit, offset int) ([]QueueItem, int, error) {
	rows, err := r.q.AdminListDisputes(ctx, sqlc.AdminListDisputesParams{
		Status:     nullText(status),
		PageLimit:  int32(limit),
		PageOffset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list disputes: %w", err)
	}
	total, err := r.q.AdminCountDisputes(ctx, nullText(status))
	if err != nil {
		return nil, 0, fmt.Errorf("count disputes: %w", err)
	}

	out := make([]QueueItem, len(rows))
	for i, row := range rows {
		out[i] = QueueItem{
			Dispute: Dispute{
				ID:         row.ID,
				BookingID:  row.BookingID,
				Status:     Status(row.Status),
				Reason:     row.Reason,
				Resolution: row.Resolution,
				RaisedBy:   UserRef{ID: row.RaisedBy, DisplayName: row.RaisedByDisplayName},
				ResolvedBy: userRef(row.ResolvedBy, row.ResolvedByDisplayName),
				CreatedAt:  row.CreatedAt.Time.UTC(),
				ResolvedAt: utcPtr(row.ResolvedAt),
			},
			Booking: BookingContext{
				ID:      row.BookingID,
				Status:  row.BookingStatus,
				StartAt: row.BookingStartAt.Time.UTC(),
				Price:   Money{AmountMinor: row.BookingPriceMinor, Currency: row.BookingCurrency},
				Teacher: TeacherRef{Slug: row.TeacherSlug, DisplayName: row.TeacherDisplayName},
				Student: StudentRef{ID: row.StudentID, Email: row.StudentEmail, DisplayName: row.StudentDisplayName},
			},
		}
	}
	return out, int(total), nil
}

// Resolve runs the status-guarded UPDATE. No rows back means the dispute is
// either unknown or already closed — a second read tells the two apart, so a
// lost race renders 409 rather than silently overwriting another operator's
// resolution.
func (r *repositoryPostgres) Resolve(ctx context.Context, disputeID uuid.UUID, status Status, resolution string, resolvedBy uuid.UUID) (Dispute, error) {
	_, err := r.q.ResolveDispute(ctx, sqlc.ResolveDisputeParams{
		ID:         disputeID,
		Status:     string(status),
		Resolution: resolution,
		ResolvedBy: resolvedBy,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, gerr := r.byID(ctx, disputeID); gerr != nil {
				return Dispute{}, gerr // ErrDisputeNotFound (or a real failure)
			}
			return Dispute{}, ErrAlreadyResolved
		}
		return Dispute{}, fmt.Errorf("resolve dispute: %w", err)
	}
	return r.byID(ctx, disputeID)
}

func userRef(id uuid.NullUUID, displayName string) *UserRef {
	if !id.Valid {
		return nil
	}
	return &UserRef{ID: id.UUID, DisplayName: displayName}
}

func utcPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	u := t.Time.UTC()
	return &u
}

package bookings

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

// pgExclusionViolation is SQLSTATE 23P01 — raised by the bookings EXCLUDE
// constraints when a lesson would double-book a teacher or student.
const pgExclusionViolation = "23P01"

type repositoryPostgres struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// NewPostgresRepository builds a Repository backed by the given pgx pool.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &repositoryPostgres{pool: pool, q: sqlc.New(pool)}
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }

func (r *repositoryPostgres) TeacherContextBySlug(ctx context.Context, slug string) (TeacherContext, error) {
	row, err := r.q.GetBookingTeacherContext(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TeacherContext{}, ErrTeacherNotFound
		}
		return TeacherContext{}, fmt.Errorf("get teacher context: %w", err)
	}

	tc := TeacherContext{
		ID:                row.ID,
		Slug:              row.Slug,
		DisplayName:       row.DisplayName,
		Timezone:          row.Timezone,
		AvatarURL:         row.AvatarUrl,
		Currency:          string(row.Currency),
		PricePerHourMinor: row.PricePerHourMinor,
		MeetingURL:        row.MeetingUrl,
	}
	if row.TrialPriceMinor.Valid {
		v := row.TrialPriceMinor.Int64
		tc.TrialPriceMinor = &v
	}
	if row.UserID.Valid {
		tc.OwnerID = row.UserID.UUID
	}
	return tc, nil
}

func (r *repositoryPostgres) TeacherIDOwnedBy(ctx context.Context, ownerID uuid.UUID) (uuid.UUID, bool, error) {
	id, err := r.q.GetTeacherIDByOwner(ctx, uuid.NullUUID{UUID: ownerID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, fmt.Errorf("teacher id by owner: %w", err)
	}
	return id, true, nil
}

func (r *repositoryPostgres) WeeklyAvailability(ctx context.Context, teacherID uuid.UUID) ([]AvailabilitySpan, error) {
	rows, err := r.q.ListAvailabilitySlots(ctx, teacherID)
	if err != nil {
		return nil, fmt.Errorf("list availability: %w", err)
	}
	spans := make([]AvailabilitySpan, len(rows))
	for i, row := range rows {
		spans[i] = AvailabilitySpan{
			Weekday:     int(row.Weekday),
			StartMinute: int(row.StartMinute),
			EndMinute:   int(row.EndMinute),
		}
	}
	return spans, nil
}

func (r *repositoryPostgres) BookedIntervals(ctx context.Context, teacherID uuid.UUID, from, to time.Time) ([]Interval, error) {
	rows, err := r.q.ListTeacherBookingIntervals(ctx, sqlc.ListTeacherBookingIntervalsParams{
		TeacherID:   teacherID,
		WindowEnd:   ts(to),
		WindowStart: ts(from),
	})
	if err != nil {
		return nil, fmt.Errorf("list booked intervals: %w", err)
	}
	out := make([]Interval, len(rows))
	for i, row := range rows {
		out[i] = Interval{Start: row.StartAt.Time.UTC(), End: row.EndAt.Time.UTC()}
	}
	return out, nil
}

func (r *repositoryPostgres) CreateBooking(ctx context.Context, p CreateBookingParams) (Booking, error) {
	id, err := r.q.CreateBooking(ctx, sqlc.CreateBookingParams{
		TeacherID:       p.TeacherID,
		StudentID:       p.StudentID,
		StartAt:         ts(p.StartAt),
		EndAt:           ts(p.EndAt),
		DurationMinutes: int32(p.DurationMinutes),
		Status:          string(StatusPendingPayment),
		PriceMinor:      p.PriceMinor,
		Currency:        p.Currency,
		IsTrial:         p.IsTrial,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgExclusionViolation {
			return Booking{}, ErrSlotTaken
		}
		return Booking{}, fmt.Errorf("create booking: %w", err)
	}
	return r.GetBooking(ctx, id)
}

func (r *repositoryPostgres) GetBooking(ctx context.Context, id uuid.UUID) (Booking, error) {
	row, err := r.q.GetBookingByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Booking{}, ErrBookingNotFound
		}
		return Booking{}, fmt.Errorf("get booking: %w", err)
	}
	return rowToBooking(row), nil
}

func (r *repositoryPostgres) ListBookings(ctx context.Context, f ListFilter) ([]Booking, error) {
	arg := sqlc.ListBookingsParams{
		StudentFilter: f.StudentFilter,
		TeacherFilter: f.TeacherFilter,
	}
	if f.Status != nil {
		arg.Status = pgtype.Text{String: string(*f.Status), Valid: true}
	}
	rows, err := r.q.ListBookings(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("list bookings: %w", err)
	}
	out := make([]Booking, len(rows))
	for i, row := range rows {
		// GetBookingByIDRow and ListBookingsRow are generated with an identical
		// field layout, so the conversion is exact.
		out[i] = rowToBooking(sqlc.GetBookingByIDRow(row))
	}
	return out, nil
}

func (r *repositoryPostgres) SetStatus(ctx context.Context, id uuid.UUID, status Status) (Booking, error) {
	if err := r.q.SetBookingStatus(ctx, sqlc.SetBookingStatusParams{ID: id, Status: string(status)}); err != nil {
		return Booking{}, fmt.Errorf("set booking status: %w", err)
	}
	return r.GetBooking(ctx, id)
}

func (r *repositoryPostgres) Cancel(ctx context.Context, id uuid.UUID, reason, by string) (Booking, error) {
	if err := r.q.CancelBooking(ctx, sqlc.CancelBookingParams{
		ID:                 id,
		CancellationReason: reason,
		CancelledBy:        by,
	}); err != nil {
		return Booking{}, fmt.Errorf("cancel booking: %w", err)
	}
	return r.GetBooking(ctx, id)
}

func (r *repositoryPostgres) SetMeetingLinkOverride(ctx context.Context, id uuid.UUID, url string) (Booking, error) {
	if err := r.q.SetBookingMeetingLinkOverride(ctx, sqlc.SetBookingMeetingLinkOverrideParams{ID: id, MeetingUrlOverride: url}); err != nil {
		return Booking{}, fmt.Errorf("set meeting link: %w", err)
	}
	return r.GetBooking(ctx, id)
}

func (r *repositoryPostgres) SetNoShowParty(ctx context.Context, id uuid.UUID, party string) error {
	if err := r.q.SetBookingNoShowParty(ctx, sqlc.SetBookingNoShowPartyParams{ID: id, NoShowParty: party}); err != nil {
		return fmt.Errorf("set no-show party: %w", err)
	}
	return nil
}

func rowToBooking(row sqlc.GetBookingByIDRow) Booking {
	b := Booking{
		ID:                 row.ID,
		Status:             Status(row.Status),
		StartAt:            row.StartAt.Time.UTC(),
		EndAt:              row.EndAt.Time.UTC(),
		DurationMinutes:    int(row.DurationMinutes),
		IsTrial:            row.IsTrial,
		Price:              Money{AmountMinor: row.PriceMinor, Currency: row.Currency},
		CreatedAt:          row.CreatedAt.Time.UTC(),
		CancellationReason: row.CancellationReason,
		CancelledBy:        row.CancelledBy,
		MeetingURLOverride: row.MeetingUrlOverride,
		TeacherMeetingURL:  row.TeacherMeetingUrl,
		NoShowParty:        row.NoShowParty,
		StudentEmail:       row.StudentEmail,
		TeacherEmail:       row.TeacherEmail,
		Teacher: TeacherSummary{
			Slug:        row.TeacherSlug,
			DisplayName: row.TeacherDisplayName,
			Timezone:    row.TeacherTimezone,
			AvatarURL:   row.TeacherAvatarUrl,
		},
		Student: StudentSummary{
			ID:          row.StudentID,
			DisplayName: row.StudentDisplayName,
		},
	}
	if row.CancelledAt.Valid {
		t := row.CancelledAt.Time.UTC()
		b.CancelledAt = &t
	}
	if row.TeacherUserID.Valid {
		b.TeacherOwnerID = row.TeacherUserID.UUID
	}
	return b
}

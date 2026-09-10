package admin

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/teachers"
)

// repositoryPostgres implements Repository over the sqlc-generated admin
// queries. It reads other modules' tables directly — see the package doc.
type repositoryPostgres struct {
	q *sqlc.Queries
}

// NewPostgresRepository builds a Repository backed by the given pgx pool.
func NewPostgresRepository(db sqlc.DBTX) Repository {
	return &repositoryPostgres{q: sqlc.New(db)}
}

func nullText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func (r *repositoryPostgres) Metrics(ctx context.Context) (Metrics, error) {
	users, err := r.q.AdminUserCount(ctx)
	if err != nil {
		return Metrics{}, fmt.Errorf("user count: %w", err)
	}
	tc, err := r.q.AdminTeacherCounts(ctx)
	if err != nil {
		return Metrics{}, fmt.Errorf("teacher counts: %w", err)
	}
	bs, err := r.q.AdminBookingStats(ctx)
	if err != nil {
		return Metrics{}, fmt.Errorf("booking stats: %w", err)
	}
	return Metrics{
		UsersTotal:       users,
		TeachersTotal:    tc.Total,
		TeachersPending:  tc.Pending,
		BookingsTotal:    bs.Total,
		BookingsThisWeek: bs.ThisWeek,
		GMVMinor:         bs.GmvMinor,
		GMVCurrency:      string(teachers.CurrencyUZS),
	}, nil
}

func (r *repositoryPostgres) ListUsers(ctx context.Context, q string, limit, offset int) ([]UserRow, int, error) {
	rows, err := r.q.AdminListUsers(ctx, sqlc.AdminListUsersParams{
		Q:          nullText(q),
		PageLimit:  int32(limit),
		PageOffset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	total, err := r.q.AdminCountUsers(ctx, nullText(q))
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}
	out := make([]UserRow, len(rows))
	for i, row := range rows {
		out[i] = UserRow{
			ID:           row.ID,
			Email:        row.Email,
			DisplayName:  row.DisplayName,
			CreatedAt:    row.CreatedAt.Time.UTC(),
			IsTeacher:    row.IsTeacher,
			BookingCount: row.BookingCount,
		}
	}
	return out, int(total), nil
}

func (r *repositoryPostgres) GetUserDetail(ctx context.Context, id uuid.UUID) (UserDetail, error) {
	u, err := r.q.AdminGetUser(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserDetail{}, ErrUserNotFound
		}
		return UserDetail{}, fmt.Errorf("get user: %w", err)
	}

	detail := UserDetail{
		User: UserCore{
			ID:          u.ID,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			CreatedAt:   u.CreatedAt.Time.UTC(),
		},
	}

	roleRows, err := r.q.AdminUserRoles(ctx, id)
	if err != nil {
		return UserDetail{}, fmt.Errorf("user roles: %w", err)
	}
	detail.Roles = make([]RoleRef, len(roleRows))
	for i, rr := range roleRows {
		detail.Roles[i] = RoleRef{ID: rr.ID, Name: rr.Name}
	}

	prof, err := r.q.AdminGetUserTeacherProfile(ctx, uuid.NullUUID{UUID: id, Valid: true})
	switch {
	case err == nil:
		detail.TeacherProfile = &UserTeacherProfile{Slug: prof.Slug, Status: prof.Status, Verified: prof.Verified}
	case errors.Is(err, pgx.ErrNoRows):
		// account owns no teacher profile
	default:
		return UserDetail{}, fmt.Errorf("get user teacher profile: %w", err)
	}

	brows, err := r.q.AdminListUserBookings(ctx, id)
	if err != nil {
		return UserDetail{}, fmt.Errorf("list user bookings: %w", err)
	}
	detail.Bookings = make([]UserBookingRow, len(brows))
	for i, b := range brows {
		role := BookingRoleStudent
		other := b.TeacherDisplayName
		if b.TeacherUserID.Valid && b.TeacherUserID.UUID == id && b.StudentID != id {
			role = BookingRoleTeacher
			other = b.StudentDisplayName
		}
		detail.Bookings[i] = UserBookingRow{
			ID:             b.ID,
			Status:         b.Status,
			StartAt:        b.StartAt.Time.UTC(),
			RoleInBooking:  role,
			OtherPartyName: other,
			PriceMinor:     b.PriceMinor,
			Currency:       b.Currency,
		}
	}

	ps, err := r.q.AdminUserPaymentsSummary(ctx, id)
	if err != nil {
		return UserDetail{}, fmt.Errorf("user payments summary: %w", err)
	}
	detail.Payments = PaymentsSummary{
		AuthorizedMinor: ps.AuthorizedMinor,
		CapturedMinor:   ps.CapturedMinor,
		RefundedMinor:   ps.RefundedMinor,
		Currency:        ps.Currency,
	}
	return detail, nil
}

func (r *repositoryPostgres) ListTeachers(ctx context.Context, status, q string, limit, offset int) ([]TeacherRow, int, error) {
	rows, err := r.q.AdminListTeachers(ctx, sqlc.AdminListTeachersParams{
		Status:     nullText(status),
		Q:          nullText(q),
		PageLimit:  int32(limit),
		PageOffset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list teachers: %w", err)
	}
	total, err := r.q.AdminCountTeachers(ctx, sqlc.AdminCountTeachersParams{
		Status: nullText(status),
		Q:      nullText(q),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count teachers: %w", err)
	}
	out := make([]TeacherRow, len(rows))
	for i, row := range rows {
		tr := TeacherRow{
			Slug:        row.Slug,
			DisplayName: row.DisplayName,
			Status:      row.Status,
			Verified:    row.Verified,
			Headline:    row.Headline,
			CountryName: row.CountryName,
			CreatedAt:   row.CreatedAt.Time.UTC(),
		}
		if row.OwnerID.Valid {
			tr.Owner = Owner{ID: row.OwnerID.UUID, Email: row.OwnerEmail.String}
		}
		out[i] = tr
	}
	return out, int(total), nil
}

func (r *repositoryPostgres) GetModeration(ctx context.Context, slug string) (Moderation, error) {
	row, err := r.q.AdminGetTeacherModeration(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Moderation{}, ErrTeacherNotFound
		}
		return Moderation{}, fmt.Errorf("get moderation: %w", err)
	}
	m := Moderation{
		Status:         row.Status,
		Verified:       row.Verified,
		ModerationNote: row.ModerationNote,
	}
	if row.OwnerID.Valid {
		m.Owner = Owner{
			ID:          row.OwnerID.UUID,
			Email:       row.OwnerEmail.String,
			DisplayName: row.OwnerDisplayName.String,
		}
	}
	return m, nil
}

func (r *repositoryPostgres) SetTeacherStatus(ctx context.Context, slug, status, note string) error {
	if err := r.q.AdminSetTeacherStatus(ctx, sqlc.AdminSetTeacherStatusParams{
		Slug:           slug,
		Status:         status,
		ModerationNote: note,
	}); err != nil {
		return fmt.Errorf("set teacher status: %w", err)
	}
	return nil
}

func (r *repositoryPostgres) ListBookings(ctx context.Context, status, q string, limit, offset int) ([]BookingRow, int, error) {
	rows, err := r.q.AdminListBookings(ctx, sqlc.AdminListBookingsParams{
		Status:     nullText(status),
		Q:          nullText(q),
		PageLimit:  int32(limit),
		PageOffset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list bookings: %w", err)
	}
	total, err := r.q.AdminCountBookings(ctx, sqlc.AdminCountBookingsParams{
		Status: nullText(status),
		Q:      nullText(q),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count bookings: %w", err)
	}

	out := make([]BookingRow, len(rows))
	for i, row := range rows {
		out[i] = BookingRow{
			ID:             row.ID,
			Status:         row.Status,
			StartAt:        row.StartAt.Time.UTC(),
			EndAt:          row.EndAt.Time.UTC(),
			Teacher:        BookingTeacher{Slug: row.TeacherSlug, DisplayName: row.TeacherDisplayName},
			Student:        BookingStudent{ID: row.StudentID, Email: row.StudentEmail, DisplayName: row.StudentDisplayName},
			PriceMinor:     row.PriceMinor,
			Currency:       row.Currency,
			PaymentStatus:  row.PaymentStatus,
			CreatedAt:      row.CreatedAt.Time.UTC(),
			HasOpenDispute: row.HasOpenDispute,
		}
	}
	return out, int(total), nil
}

func (r *repositoryPostgres) GetBookingDetail(ctx context.Context, id uuid.UUID) (BookingDetail, error) {
	row, err := r.q.AdminGetBooking(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BookingDetail{}, ErrBookingNotFound
		}
		return BookingDetail{}, fmt.Errorf("get booking: %w", err)
	}

	d := BookingDetail{
		BookingRow: BookingRow{
			ID:             row.ID,
			Status:         row.Status,
			StartAt:        row.StartAt.Time.UTC(),
			EndAt:          row.EndAt.Time.UTC(),
			Teacher:        BookingTeacher{Slug: row.TeacherSlug, DisplayName: row.TeacherDisplayName},
			Student:        BookingStudent{ID: row.StudentID, Email: row.StudentEmail, DisplayName: row.StudentDisplayName},
			PriceMinor:     row.PriceMinor,
			Currency:       row.Currency,
			PaymentStatus:  row.PaymentStatus,
			CreatedAt:      row.CreatedAt.Time.UTC(),
			HasOpenDispute: row.HasOpenDispute,
		},
		DurationMinutes:    int(row.DurationMinutes),
		IsTrial:            row.IsTrial,
		MeetingURL:         row.MeetingUrl,
		NoShowParty:        row.NoShowParty,
		CancelledBy:        row.CancelledBy,
		CancellationReason: row.CancellationReason,
	}
	if row.CancelledAt.Valid {
		t := row.CancelledAt.Time.UTC()
		d.CancelledAt = &t
	}
	// payment_status is '' exactly when the LEFT JOIN found no intent.
	if row.PaymentStatus != "" {
		d.Payment = &BookingPayment{
			Status:      row.PaymentStatus,
			AmountMinor: row.PaymentAmountMinor,
			Currency:    row.PaymentCurrency,
		}
	}

	drows, err := r.q.AdminListBookingDisputes(ctx, id)
	if err != nil {
		return BookingDetail{}, fmt.Errorf("list booking disputes: %w", err)
	}
	d.Disputes = make([]BookingDispute, len(drows))
	for i, dr := range drows {
		entry := BookingDispute{
			ID:         dr.ID,
			Status:     dr.Status,
			Reason:     dr.Reason,
			Resolution: dr.Resolution,
			RaisedBy:   UserRef{ID: dr.RaisedBy, DisplayName: dr.RaisedByDisplayName},
			CreatedAt:  dr.CreatedAt.Time.UTC(),
		}
		if dr.ResolvedBy.Valid {
			entry.ResolvedBy = &UserRef{ID: dr.ResolvedBy.UUID, DisplayName: dr.ResolvedByDisplayName}
		}
		if dr.ResolvedAt.Valid {
			t := dr.ResolvedAt.Time.UTC()
			entry.ResolvedAt = &t
		}
		d.Disputes[i] = entry
	}
	return d, nil
}

func (r *repositoryPostgres) SetTeacherVerified(ctx context.Context, slug string, verified bool) error {
	if err := r.q.AdminSetTeacherVerified(ctx, sqlc.AdminSetTeacherVerifiedParams{
		Slug:     slug,
		Verified: verified,
	}); err != nil {
		return fmt.Errorf("set teacher verified: %w", err)
	}
	return nil
}

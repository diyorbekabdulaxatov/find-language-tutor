package admin

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/teachers"
)

// Repository is the persistence port. The Postgres implementation reads across
// tables directly (internal/db/queries/admin.sql); tests use a fake.
type Repository interface {
	Metrics(ctx context.Context) (Metrics, error)

	ListUsers(ctx context.Context, q string, limit, offset int) (rows []UserRow, total int, err error)
	// GetUserDetail returns ErrUserNotFound for an unknown id.
	GetUserDetail(ctx context.Context, id uuid.UUID) (UserDetail, error)

	ListTeachers(ctx context.Context, status, q string, limit, offset int) (rows []TeacherRow, total int, err error)
	// GetModeration returns ErrTeacherNotFound for an unknown slug.
	GetModeration(ctx context.Context, slug string) (Moderation, error)

	SetTeacherStatus(ctx context.Context, slug, status, note string) error
	SetTeacherVerified(ctx context.Context, slug string, verified bool) error

	ListBookings(ctx context.Context, status, q string, limit, offset int) (rows []BookingRow, total int, err error)
	// GetBookingDetail returns ErrBookingNotFound for an unknown id.
	GetBookingDetail(ctx context.Context, id uuid.UUID) (BookingDetail, error)
}

// BookingModerator is the write port back into the bookings module for the one
// admin action that changes a booking: the force-cancel override. The admin
// module deliberately owns none of the cancellation logic — the refund, the
// reminder teardown and the cancellation mail all live in bookings and are
// reused verbatim. *bookings.Service satisfies this; cmd/api injects it with
// Service.SetBookingModerator.
//
// The state rule (only pending_payment / confirmed can be cancelled) is checked
// HERE too, off the detail this service already read, so no bookings-domain
// error has to cross the port.
type BookingModerator interface {
	// AdminForceCancel cancels the booking regardless of who is asking,
	// recording cancelled_by = 'admin'. refund=true releases / refunds the
	// booking's payment.
	AdminForceCancel(ctx context.Context, bookingID uuid.UUID, reason string, refund bool) error
}

// TeacherProfiles is the read port back into the teachers module for the one
// place the admin surface reuses a domain read model: the full profile on
// GET /v1/admin/teachers/{slug}. *teachers.Service satisfies it.
type TeacherProfiles interface {
	// GetForAdmin returns the full profile regardless of moderation status.
	GetForAdmin(ctx context.Context, slug string) (*teachers.Teacher, error)
}

// Service holds the admin business rules. Handlers call it; it never sees an
// *gin.Context.
type Service struct {
	repo     Repository
	teachers TeacherProfiles
	bookings BookingModerator // nil until SetBookingModerator
}

func NewService(repo Repository, tp TeacherProfiles) *Service {
	return &Service{repo: repo, teachers: tp}
}

// SetBookingModerator wires the bookings force-cancel port in. Called once at
// startup (cmd/api). Without it, force-cancel fails loudly (500) rather than
// silently pretending to cancel.
func (s *Service) SetBookingModerator(m BookingModerator) { s.bookings = m }

// --- phase A ---

// Metrics returns the dashboard counters.
func (s *Service) Metrics(ctx context.Context) (Metrics, error) {
	return s.repo.Metrics(ctx)
}

// ListUsers returns one page of the user directory. q matches email or
// display_name (case-insensitive substring).
func (s *Service) ListUsers(ctx context.Context, q string, page, pageSize int) (UsersPage, error) {
	limit, offset := normalizePage(page, pageSize)
	rows, total, err := s.repo.ListUsers(ctx, q, limit, offset)
	if err != nil {
		return UsersPage{}, err
	}
	if rows == nil {
		rows = []UserRow{}
	}
	return UsersPage{Users: rows, Total: total}, nil
}

// GetUser returns the full user detail, or ErrUserNotFound.
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (UserDetail, error) {
	return s.repo.GetUserDetail(ctx, id)
}

// --- phase B ---

// ListTeachers returns one page of the moderation queue. status is an optional
// exact filter ("" = all); q matches display_name / slug / owner email.
func (s *Service) ListTeachers(ctx context.Context, status, q string, page, pageSize int) (TeachersPage, error) {
	limit, offset := normalizePage(page, pageSize)
	rows, total, err := s.repo.ListTeachers(ctx, status, q, limit, offset)
	if err != nil {
		return TeachersPage{}, err
	}
	if rows == nil {
		rows = []TeacherRow{}
	}
	return TeachersPage{Teachers: rows, Total: total}, nil
}

// TeacherDetail is the full profile plus its moderation slice.
type TeacherDetail struct {
	Profile    *teachers.Teacher
	Moderation Moderation
}

// GetTeacher returns the full profile + moderation for one slug, or
// ErrTeacherNotFound.
func (s *Service) GetTeacher(ctx context.Context, slug string) (TeacherDetail, error) {
	mod, err := s.repo.GetModeration(ctx, slug)
	if err != nil {
		return TeacherDetail{}, err
	}
	profile, err := s.teachers.GetForAdmin(ctx, slug)
	if err != nil {
		return TeacherDetail{}, err
	}
	return TeacherDetail{Profile: profile, Moderation: mod}, nil
}

// Approve moves a teacher to `approved` and clears the moderation note. Allowed
// from pending, rejected, or suspended (the last is a reinstate). Approving an
// already-approved teacher is ErrInvalidTransition.
func (s *Service) Approve(ctx context.Context, slug string) (TeacherDetail, error) {
	return s.transition(ctx, slug, teachers.StatusApproved, "", func(cur string) bool {
		return cur == string(teachers.StatusPending) ||
			cur == string(teachers.StatusRejected) ||
			cur == string(teachers.StatusSuspended)
	})
}

// Reject moves a `pending` teacher to `rejected` with a required note.
func (s *Service) Reject(ctx context.Context, slug, note string) (TeacherDetail, error) {
	if note == "" {
		return TeacherDetail{}, invalid("a note is required when rejecting a teacher.")
	}
	return s.transition(ctx, slug, teachers.StatusRejected, note, func(cur string) bool {
		return cur == string(teachers.StatusPending)
	})
}

// Suspend moves an `approved` teacher to `suspended` with a required note. The
// profile drops out of search immediately; existing bookings are untouched.
func (s *Service) Suspend(ctx context.Context, slug, note string) (TeacherDetail, error) {
	if note == "" {
		return TeacherDetail{}, invalid("a note is required when suspending a teacher.")
	}
	return s.transition(ctx, slug, teachers.StatusSuspended, note, func(cur string) bool {
		return cur == string(teachers.StatusApproved)
	})
}

// SetVerified toggles the verified badge. Independent of moderation status.
func (s *Service) SetVerified(ctx context.Context, slug string, verified bool) (TeacherDetail, error) {
	if _, err := s.repo.GetModeration(ctx, slug); err != nil {
		return TeacherDetail{}, err
	}
	if err := s.repo.SetTeacherVerified(ctx, slug, verified); err != nil {
		return TeacherDetail{}, err
	}
	return s.GetTeacher(ctx, slug)
}

// --- phase D ---

// ListBookings returns one page of every booking on the platform. status is an
// optional exact filter (""= all); q matches the teacher's display name / slug
// and the student's email / display name.
func (s *Service) ListBookings(ctx context.Context, status, q string, page, pageSize int) (BookingsPage, error) {
	limit, offset := normalizePage(page, pageSize)
	rows, total, err := s.repo.ListBookings(ctx, status, q, limit, offset)
	if err != nil {
		return BookingsPage{}, err
	}
	if rows == nil {
		rows = []BookingRow{}
	}
	return BookingsPage{Bookings: rows, Total: total}, nil
}

// GetBooking returns the full operator view of one booking, or
// ErrBookingNotFound.
func (s *Service) GetBooking(ctx context.Context, id uuid.UUID) (BookingDetail, error) {
	return s.repo.GetBookingDetail(ctx, id)
}

// ForceCancelBooking is the operator override: it cancels a booking neither
// participant asked to cancel. Allowed from pending_payment / confirmed only
// (ErrInvalidState otherwise); a reason is required and is stored on the
// booking. refund=true releases / refunds the payment.
//
// The cancellation itself is delegated to the bookings module through the
// BookingModerator port, so the refund, the reminder teardown and the
// "your lesson was cancelled" mail are exactly the ones a participant cancel
// triggers — this module adds only the override.
func (s *Service) ForceCancelBooking(ctx context.Context, id uuid.UUID, reason string, refund bool) (BookingDetail, error) {
	if reason == "" {
		return BookingDetail{}, invalid("a reason is required when force-cancelling a booking.")
	}
	if s.bookings == nil {
		return BookingDetail{}, fmt.Errorf("admin: no booking moderator wired")
	}

	current, err := s.repo.GetBookingDetail(ctx, id)
	if err != nil {
		return BookingDetail{}, err
	}
	if current.Status != BookingPendingPayment && current.Status != BookingConfirmed {
		return BookingDetail{}, ErrInvalidState
	}

	if err := s.bookings.AdminForceCancel(ctx, id, reason, refund); err != nil {
		return BookingDetail{}, err
	}
	return s.repo.GetBookingDetail(ctx, id)
}

// transition validates the current status against allow, writes the new status
// + note, and returns the refreshed detail.
func (s *Service) transition(ctx context.Context, slug string, to teachers.Status, note string, allow func(cur string) bool) (TeacherDetail, error) {
	mod, err := s.repo.GetModeration(ctx, slug)
	if err != nil {
		return TeacherDetail{}, err
	}
	if !allow(mod.Status) {
		return TeacherDetail{}, ErrInvalidTransition
	}
	if err := s.repo.SetTeacherStatus(ctx, slug, string(to), note); err != nil {
		return TeacherDetail{}, err
	}
	return s.GetTeacher(ctx, slug)
}

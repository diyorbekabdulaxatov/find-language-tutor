package admin

import (
	"context"

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
}

func NewService(repo Repository, tp TeacherProfiles) *Service {
	return &Service{repo: repo, teachers: tp}
}

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

package availability

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrTeacherNotFound is returned when no teacher has the requested slug.
var ErrTeacherNotFound = errors.New("teacher not found")

// TeacherRef is the slice of a teacher the availability endpoints need.
type TeacherRef struct {
	ID       uuid.UUID
	Slug     string
	Timezone string
}

// Repository is the persistence port for this module. The concrete
// implementation (repositoryPostgres) lives alongside; tests use a fake.
type Repository interface {
	// TeacherContext resolves a slug to the teacher id and timezone, or
	// ErrTeacherNotFound.
	TeacherContext(ctx context.Context, slug string) (TeacherRef, error)

	// ListSlots returns the stored weekly slots for a teacher.
	ListSlots(ctx context.Context, teacherID uuid.UUID) ([]Slot, error)

	// ReplaceSlots atomically replaces a teacher's entire weekly set.
	ReplaceSlots(ctx context.Context, teacherID uuid.UUID, slots []Slot) error
}

// Service holds the availability business logic. Handlers call it; it never
// sees an *gin.Context.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetBySlug returns a teacher's full weekly availability. Propagates
// ErrTeacherNotFound.
func (s *Service) GetBySlug(ctx context.Context, slug string) (WeeklyAvailability, error) {
	ref, err := s.repo.TeacherContext(ctx, slug)
	if err != nil {
		return WeeklyAvailability{}, err
	}

	slots, err := s.repo.ListSlots(ctx, ref.ID)
	if err != nil {
		return WeeklyAvailability{}, err
	}
	sortSlots(slots)

	return WeeklyAvailability{TeacherSlug: ref.Slug, Timezone: ref.Timezone, Slots: slots}, nil
}

// Replace validates a proposed weekly set and stores it as the teacher's full
// availability, returning the stored result. A ValidationError means the input
// was client-fixable; ErrTeacherNotFound means the slug is unknown.
func (s *Service) Replace(ctx context.Context, slug string, slots []Slot) (WeeklyAvailability, error) {
	ref, err := s.repo.TeacherContext(ctx, slug)
	if err != nil {
		return WeeklyAvailability{}, err
	}

	if err := validateSlots(slots); err != nil {
		return WeeklyAvailability{}, err
	}
	sortSlots(slots)

	if err := s.repo.ReplaceSlots(ctx, ref.ID, slots); err != nil {
		return WeeklyAvailability{}, err
	}

	return WeeklyAvailability{TeacherSlug: ref.Slug, Timezone: ref.Timezone, Slots: slots}, nil
}

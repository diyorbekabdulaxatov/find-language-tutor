package teachers

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// The service rules around a teacher's 1-on-1 offerings. Ownership is resolved
// from the caller's own profile — a handler never passes a teacher id in.

const maxLessonTitle = 80

// LessonTypes returns a teacher's live offerings by slug — the public list
// behind a profile's "what you can book" cards.
func (s *Service) LessonTypes(ctx context.Context, slug string) ([]LessonType, error) {
	ref, err := s.repo.RefBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	return s.repo.ListLessonTypes(ctx, ref.ID, false)
}

// OwnLessonTypes returns the caller's own offerings, archived ones included so
// the dashboard can restore them.
func (s *Service) OwnLessonTypes(ctx context.Context, ownerID uuid.UUID) ([]LessonType, error) {
	ref, err := s.repo.RefByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListLessonTypes(ctx, ref.ID, true)
}

// CreateLessonType adds an offering to the caller's own profile.
func (s *Service) CreateLessonType(ctx context.Context, ownerID uuid.UUID, in LessonTypeInput) (LessonType, error) {
	ref, err := s.repo.RefByOwner(ctx, ownerID)
	if err != nil {
		return LessonType{}, err
	}
	if err := validateLessonType(&in); err != nil {
		return LessonType{}, err
	}

	live, err := s.repo.ListLessonTypes(ctx, ref.ID, false)
	if err != nil {
		return LessonType{}, err
	}
	if len(live) >= MaxLessonTypes {
		return LessonType{}, ErrTooManyLessonTypes
	}
	if in.IsTrial {
		for _, lt := range live {
			if lt.IsTrial {
				return LessonType{}, ErrTrialExists
			}
		}
	}
	// A trial sorts above the rest; everything else lands at the end.
	if in.IsTrial {
		in.Position = -1
	} else if in.Position == 0 {
		in.Position = len(live)
	}
	return s.repo.CreateLessonType(ctx, ref.ID, in)
}

// UpdateLessonType edits one of the caller's own offerings. The trial flag is
// fixed at creation — a teacher archives the trial and makes a new one instead.
func (s *Service) UpdateLessonType(ctx context.Context, ownerID, id uuid.UUID, in LessonTypeInput) (LessonType, error) {
	if _, err := s.ownedLessonType(ctx, ownerID, id); err != nil {
		return LessonType{}, err
	}
	if err := validateLessonType(&in); err != nil {
		return LessonType{}, err
	}
	return s.repo.UpdateLessonType(ctx, id, in)
}

// SetLessonTypeArchived hides or restores one of the caller's own offerings.
// Past bookings keep pointing at it, so nothing is deleted.
func (s *Service) SetLessonTypeArchived(ctx context.Context, ownerID, id uuid.UUID, archived bool) (LessonType, error) {
	if _, err := s.ownedLessonType(ctx, ownerID, id); err != nil {
		return LessonType{}, err
	}
	return s.repo.SetLessonTypeArchived(ctx, id, archived)
}

// ownedLessonType resolves an offering and checks the caller owns the profile
// it hangs off. A type owned by someone else reads as "not found", not 403, so
// the endpoint never confirms another teacher's ids.
func (s *Service) ownedLessonType(ctx context.Context, ownerID, id uuid.UUID) (LessonType, error) {
	ref, err := s.repo.RefByOwner(ctx, ownerID)
	if err != nil {
		return LessonType{}, err
	}
	lt, err := s.repo.GetLessonType(ctx, id)
	if err != nil {
		return LessonType{}, err
	}
	if lt.TeacherID != ref.ID {
		return LessonType{}, ErrLessonTypeNotFound
	}
	return lt, nil
}

func validateLessonType(in *LessonTypeInput) error {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)

	if in.Title == "" {
		return invalid("A lesson needs a title.")
	}
	if len([]rune(in.Title)) > maxLessonTitle {
		return invalid("A lesson title can be at most %d characters.", maxLessonTitle)
	}
	if len(in.Prices) == 0 {
		return invalid("A lesson needs at least one length and price.")
	}

	seen := make(map[int]bool, len(in.Prices))
	for _, p := range in.Prices {
		if !allowedLessonDuration(p.DurationMinutes) {
			return invalid("A lesson length must be one of %s minutes.", durationList())
		}
		if seen[p.DurationMinutes] {
			return invalid("The %d-minute length is listed twice.", p.DurationMinutes)
		}
		seen[p.DurationMinutes] = true
		if p.Price.AmountMinor < 0 {
			return invalid("A price cannot be negative.")
		}
	}
	return nil
}

func allowedLessonDuration(minutes int) bool {
	for _, d := range LessonDurations {
		if d == minutes {
			return true
		}
	}
	return false
}

func durationList() string {
	parts := make([]string, len(LessonDurations))
	for i, d := range LessonDurations {
		parts[i] = fmt.Sprintf("%d", d)
	}
	return strings.Join(parts, " / ")
}

// LessonTypeByID returns one offering by id regardless of owner — the read
// behind cmd/api's booking adapter, which needs to price a lesson the student
// picked. Returns ErrLessonTypeNotFound for an unknown id.
func (s *Service) LessonTypeByID(ctx context.Context, id uuid.UUID) (LessonType, error) {
	return s.repo.GetLessonType(ctx, id)
}

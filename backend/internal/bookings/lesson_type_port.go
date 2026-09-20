package bookings

import (
	"context"

	"github.com/google/uuid"
)

// LessonTypeReader is how the booking flow prices a lesson against one of the
// teacher's offerings. Declared here, implemented by the teachers module and
// injected in cmd/api — bookings never imports teachers.
//
// A nil reader (SetLessonTypes never called) leaves the pre-L1 behaviour
// intact: a booking with no lesson_type_id is priced from the teacher's hourly
// rate, and one that names a type is refused rather than mispriced.
type LessonTypeReader interface {
	// Offering returns the named lesson type priced for durationMinutes.
	// ok is false when the type is unknown, archived, belongs to another
	// teacher, or is not offered at that length.
	Offering(ctx context.Context, lessonTypeID uuid.UUID, durationMinutes int) (Offering, bool, error)
}

// Offering is the slice of a lesson type the booking flow needs.
type Offering struct {
	ID        uuid.UUID
	TeacherID uuid.UUID
	Title     string
	IsTrial   bool
	Price     Money
}

// SetLessonTypes injects the reader. Guarded no-op when never called.
func (s *Service) SetLessonTypes(r LessonTypeReader) { s.lessonTypes = r }

// offering resolves the chosen lesson type and holds it to the teacher being
// booked, so a type id from another profile can never price this lesson.
func (s *Service) offering(ctx context.Context, teacherID, lessonTypeID uuid.UUID, durationMinutes int) (Offering, error) {
	if s.lessonTypes == nil {
		return Offering{}, invalid("Choosing a lesson isn't available right now.")
	}
	o, ok, err := s.lessonTypes.Offering(ctx, lessonTypeID, durationMinutes)
	if err != nil {
		return Offering{}, err
	}
	if !ok || o.TeacherID != teacherID {
		return Offering{}, invalid("That lesson isn't offered at that length.")
	}
	return o, nil
}

func lessonTypeRef(o *Offering) uuid.NullUUID {
	if o == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: o.ID, Valid: true}
}

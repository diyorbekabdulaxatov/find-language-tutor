package main

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/teachers"
)

// lessonTypeGateway adapts the teachers module to bookings.LessonTypeReader:
// the booking flow prices a lesson against one of the teacher's offerings
// without importing teachers, and teachers knows nothing about bookings. Both
// halves are typed, so this conversion lives here — wiring glue, like
// multiAssigneeChecker.
type lessonTypeGateway struct{ svc *teachers.Service }

var _ bookings.LessonTypeReader = (*lessonTypeGateway)(nil)

// Offering returns the offering priced for durationMinutes. An unknown,
// archived, or differently-priced type is ok=false rather than an error, so the
// booking service can answer "that isn't offered at that length" with a 400.
func (g *lessonTypeGateway) Offering(ctx context.Context, lessonTypeID uuid.UUID, durationMinutes int) (bookings.Offering, bool, error) {
	lt, err := g.svc.LessonTypeByID(ctx, lessonTypeID)
	if errors.Is(err, teachers.ErrLessonTypeNotFound) {
		return bookings.Offering{}, false, nil
	}
	if err != nil {
		return bookings.Offering{}, false, err
	}
	if lt.Archived {
		return bookings.Offering{}, false, nil
	}
	for _, p := range lt.Prices {
		if p.DurationMinutes != durationMinutes {
			continue
		}
		return bookings.Offering{
			ID:        lt.ID,
			TeacherID: lt.TeacherID,
			Title:     lt.Title,
			IsTrial:   lt.IsTrial,
			Price: bookings.Money{
				AmountMinor: p.Price.AmountMinor,
				Currency:    string(p.Price.Currency),
			},
		}, true, nil
	}
	return bookings.Offering{}, false, nil
}

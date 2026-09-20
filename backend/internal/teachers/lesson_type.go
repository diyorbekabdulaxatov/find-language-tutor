package teachers

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// A lesson type is one of a teacher's 1-on-1 offerings — italki's shape, where
// "IELTS Speaking prep, 30/60 min" and "General conversation, 30/60/90 min" are
// separate things a student can buy, rather than one hourly rate.
//
// Prices are per duration and set by the teacher; nothing is prorated. The
// trial is a lesson type with IsTrial set, and a teacher has at most one live
// one (enforced by a partial unique index).

// LessonDurations are the lengths an offering may be priced at. 45 is possible
// because availability is stored on a 15-minute grid.
var LessonDurations = []int{30, 45, 60, 90, 120}

// MaxLessonTypes caps how many live offerings one teacher may list, so a
// profile stays readable.
const MaxLessonTypes = 8

var (
	// ErrLessonTypeNotFound — no such offering, or it belongs to someone else.
	ErrLessonTypeNotFound = errors.New("teachers: lesson type not found")

	// ErrTrialExists — a teacher may list only one live trial offering.
	ErrTrialExists = errors.New("teachers: a trial lesson already exists")

	// ErrTooManyLessonTypes — MaxLessonTypes reached.
	ErrTooManyLessonTypes = errors.New("teachers: too many lesson types")
)

// LessonPrice is one duration a lesson type can be booked for.
type LessonPrice struct {
	DurationMinutes int
	Price           Money
}

// LessonType is one offering with its price list.
type LessonType struct {
	ID          uuid.UUID
	TeacherID   uuid.UUID
	Title       string
	Description string
	IsTrial     bool
	Archived    bool
	Position    int
	Prices      []LessonPrice
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// From returns the cheapest price across the offering's durations — the "from
// 45,000 so'm" line on a card. Zero Money when the type has no prices.
func (lt LessonType) From() Money {
	var out Money
	for i, p := range lt.Prices {
		if i == 0 || p.Price.AmountMinor < out.AmountMinor {
			out = p.Price
		}
	}
	return out
}

// LessonTypeInput is the writable shape of an offering.
type LessonTypeInput struct {
	Title       string
	Description string
	IsTrial     bool
	Position    int
	Prices      []LessonPrice
}

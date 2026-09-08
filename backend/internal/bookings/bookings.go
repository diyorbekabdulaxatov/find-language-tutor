// Package bookings is the lesson-booking domain module. A booking is a concrete
// scheduled lesson between a student account and a teacher profile, stored as an
// absolute UTC instant. This file owns the domain types, the status lifecycle,
// and the pure slot-generation logic that projects a teacher's recurring weekly
// availability (UTC minutes-from-midnight on a UTC weekday) onto real datetimes.
//
// Timezone model: the weekly availability is already in UTC — start_minute /
// end_minute are offsets from 00:00 UTC on the given UTC weekday. Projecting to
// concrete UTC datetimes is therefore direct: for each UTC calendar day in the
// window whose EXTRACT(DOW)-equivalent weekday matches, the span runs from
// (midnight UTC + start_minute) to (midnight UTC + end_minute). The teacher's
// IANA timezone is echoed in responses for display only; it never enters the
// math. Spans the frontend split across the 00:00 boundary (e.g. a teacher whose
// local hours wrap midnight) are stitched back together here by merging touching
// concrete intervals before slots are stepped, so a lesson may legitimately
// straddle UTC midnight.
package bookings

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Status is the booking lifecycle state.
type Status string

const (
	StatusPendingPayment Status = "pending_payment"
	StatusConfirmed      Status = "confirmed"
	StatusCompleted      Status = "completed"
	StatusCancelled      Status = "cancelled"
)

func (s Status) valid() bool {
	switch s {
	case StatusPendingPayment, StatusConfirmed, StatusCompleted, StatusCancelled:
		return true
	default:
		return false
	}
}

const (
	// slotStepMinutes is the grid candidate start times are stepped on, measured
	// from the start of each (merged) availability window.
	slotStepMinutes = 30

	// trialDurationMinutes is the fixed length of a trial lesson. is_trial forces
	// this regardless of the requested duration_minutes.
	trialDurationMinutes = 30

	// DefaultDurationMinutes is used when the slots query omits duration.
	DefaultDurationMinutes = 60

	// defaultWindowDays / maxWindowDays bound the slots query window.
	defaultWindowDays = 14
	maxWindowDays     = 21
)

// allowedDurations is the set of accepted lesson lengths (minutes).
var allowedDurations = map[int]bool{30: true, 60: true, 90: true, 120: true}

// AllowedDuration reports whether d is an accepted (non-trial) lesson length.
func AllowedDuration(d int) bool { return allowedDurations[d] }

// Money is an amount in integer minor units plus a currency code, matching the
// teachers module and openapi.yaml's Money schema.
type Money struct {
	AmountMinor int64
	Currency    string
}

// AvailabilitySpan is one recurring weekly availability span, in UTC minutes
// from midnight on the given weekday (0 = Sunday .. 6 = Saturday).
type AvailabilitySpan struct {
	Weekday     int
	StartMinute int
	EndMinute   int
}

// Interval is a concrete [Start, End) instant range in UTC.
type Interval struct {
	Start time.Time
	End   time.Time
}

// TeacherContext is the slice of a teacher the booking flow needs.
type TeacherContext struct {
	ID                uuid.UUID
	Slug              string
	DisplayName       string
	Timezone          string
	AvatarURL         string
	Currency          string
	PricePerHourMinor int64
	TrialPriceMinor   *int64 // nil when the teacher offers no trial lesson
	OwnerID           uuid.UUID
	MeetingURL        string // teacher's default video room ("" when unset)
}

// TeacherSummary / StudentSummary are the light participant summaries embedded
// in every BookingDTO so list views avoid N+1 lookups.
type TeacherSummary struct {
	Slug        string
	DisplayName string
	Timezone    string
	AvatarURL   string
}

type StudentSummary struct {
	ID          uuid.UUID
	DisplayName string
}

// Booking is the full domain aggregate.
type Booking struct {
	ID                 uuid.UUID
	Status             Status
	StartAt            time.Time
	EndAt              time.Time
	DurationMinutes    int
	IsTrial            bool
	Price              Money
	CreatedAt          time.Time
	CancelledAt        *time.Time
	CancellationReason string

	// MeetingURLOverride is an optional per-booking video link. When set it wins
	// over the teacher's default; "" means "use the teacher default".
	MeetingURLOverride string
	// TeacherMeetingURL is the teacher's default video room, carried on the
	// booking so the effective link can be computed without a second lookup.
	TeacherMeetingURL string
	// NoShowParty is "" normally, or "student" / "teacher" once a no-show has
	// been recorded.
	NoShowParty string

	Teacher TeacherSummary
	Student StudentSummary

	// TeacherOwnerID is the account that owns the teacher profile. Used for the
	// participant authorization check; never serialized.
	TeacherOwnerID uuid.UUID

	// StudentEmail / TeacherEmail are the participants' account emails, used to
	// address transactional mail. Never serialized.
	StudentEmail string
	TeacherEmail string
}

// EffectiveMeetingURL is the per-booking override if present, else the teacher's
// default room. It is NOT an authorization decision — the DTO layer decides
// whether the caller may see it (participant + confirmed/completed only).
func (b Booking) EffectiveMeetingURL() string {
	if b.MeetingURLOverride != "" {
		return b.MeetingURLOverride
	}
	return b.TeacherMeetingURL
}

// Slot is one concrete bookable start time with its price.
type Slot struct {
	StartAt time.Time
	EndAt   time.Time
	Price   Money
}

// SlotQuery is a validated request for a teacher's bookable slots.
type SlotQuery struct {
	From            time.Time
	To              time.Time
	DurationMinutes int
}

// SlotResult is the answer to a SlotQuery.
type SlotResult struct {
	TeacherSlug     string
	Timezone        string
	From            time.Time
	To              time.Time
	DurationMinutes int
	Slots           []Slot
}

// ValidationError is a client-fixable problem with a request. The handler
// renders it as 400; anything else unexpected from the service is a 500.
type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return ValidationError{msg: fmt.Sprintf(format, args...)}
}

// hourlyPrice = round(perHourMinor * durationMinutes / 60), half-up, in integer
// minor units. Documented in openapi.yaml.
func hourlyPrice(perHourMinor int64, durationMinutes int, currency string) Money {
	amount := (perHourMinor*int64(durationMinutes) + 30) / 60
	return Money{AmountMinor: amount, Currency: currency}
}

// --- pure slot generation -------------------------------------------------

// projectSpans turns the recurring weekly spans into concrete UTC intervals
// covering [from, to], padded by a day on each side so a window that stitches
// across UTC midnight is fully represented before merging.
func projectSpans(spans []AvailabilitySpan, from, to time.Time) []Interval {
	if len(spans) == 0 {
		return nil
	}
	day0 := from.UTC().Truncate(24*time.Hour).AddDate(0, 0, -1)
	dayN := to.UTC().Truncate(24*time.Hour).AddDate(0, 0, 1)

	var out []Interval
	for d := day0; !d.After(dayN); d = d.AddDate(0, 0, 1) {
		wd := int(d.Weekday()) // time.Weekday: Sunday = 0, matching EXTRACT(DOW)
		for _, s := range spans {
			if s.Weekday != wd {
				continue
			}
			out = append(out, Interval{
				Start: d.Add(time.Duration(s.StartMinute) * time.Minute),
				End:   d.Add(time.Duration(s.EndMinute) * time.Minute),
			})
		}
	}
	return out
}

// mergeIntervals collapses overlapping or exactly-touching intervals into
// maximal windows. Touching at UTC midnight is what re-joins a span the frontend
// split across the day boundary.
func mergeIntervals(ivs []Interval) []Interval {
	if len(ivs) == 0 {
		return nil
	}
	sort.Slice(ivs, func(i, j int) bool { return ivs[i].Start.Before(ivs[j].Start) })

	merged := []Interval{ivs[0]}
	for _, iv := range ivs[1:] {
		last := &merged[len(merged)-1]
		if !iv.Start.After(last.End) { // overlap or touch
			if iv.End.After(last.End) {
				last.End = iv.End
			}
			continue
		}
		merged = append(merged, iv)
	}
	return merged
}

func overlapsAny(start, end time.Time, booked []Interval) bool {
	for _, b := range booked {
		if start.Before(b.End) && b.Start.Before(end) {
			return true
		}
	}
	return false
}

// generateSlots is the single source of truth for "is this a bookable start
// time": both the slots endpoint and the create re-check call it. A start is
// bookable when [start, start+duration] fits entirely inside one merged
// availability window, sits on the 30-minute step from that window's start, is
// not in the past, lies within [from, to], and overlaps no non-cancelled
// booking.
func generateSlots(spans []AvailabilitySpan, booked []Interval, q SlotQuery, priceFn func() Money, now time.Time) []Slot {
	from := q.From.UTC()
	to := q.To.UTC()
	dur := time.Duration(q.DurationMinutes) * time.Minute
	step := time.Duration(slotStepMinutes) * time.Minute

	lower := from
	if now.After(lower) {
		lower = now
	}

	windows := mergeIntervals(projectSpans(spans, from, to))

	var slots []Slot
	seen := make(map[int64]struct{})
	for _, w := range windows {
		for s := w.Start; !s.Add(dur).After(w.End); s = s.Add(step) {
			e := s.Add(dur)
			if s.Before(lower) || e.After(to) {
				continue
			}
			if overlapsAny(s, e, booked) {
				continue
			}
			key := s.Unix()
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			slots = append(slots, Slot{StartAt: s.UTC(), EndAt: e.UTC(), Price: priceFn()})
		}
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].StartAt.Before(slots[j].StartAt) })
	return slots
}

// isBookableStart re-runs slot generation for a one-slot window and reports
// whether exactly start is produced.
func isBookableStart(spans []AvailabilitySpan, booked []Interval, start time.Time, durationMinutes int, now time.Time) bool {
	start = start.UTC()
	q := SlotQuery{
		From:            start,
		To:              start.Add(time.Duration(durationMinutes) * time.Minute),
		DurationMinutes: durationMinutes,
	}
	for _, sl := range generateSlots(spans, booked, q, func() Money { return Money{} }, now) {
		if sl.StartAt.Equal(start) {
			return true
		}
	}
	return false
}

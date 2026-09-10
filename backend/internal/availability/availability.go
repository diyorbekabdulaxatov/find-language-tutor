// Package availability is the teacher weekly-availability domain module. A
// teacher's recurring availability is a set of weekly slots stored in UTC; the
// booking module converts them to concrete times later using the teacher's IANA
// timezone (teachers.timezone). It owns its domain types, a service with the
// validation rules, a repository port, and gin handlers.
package availability

import (
	"fmt"
	"sort"
)

const (
	// SlotGranularityMinutes is the grid every slot boundary must align to.
	// 15 minutes lets teachers offer 30/45/60-minute lessons and back-to-back
	// slots without forcing an artificial rounding.
	SlotGranularityMinutes = 15

	minuteOfDayMin = 0
	minuteOfDayMax = 24 * 60 // 1440, exclusive upper bound for a start time

	// maxSlotsPerRequest caps a single PUT so a bad client can't insert an
	// unbounded number of rows. 7 days × 96 quarter-hours is the theoretical
	// ceiling; 100 is a generous practical limit.
	maxSlotsPerRequest = 100
)

// Weekday is 0 (Sunday) .. 6 (Saturday), matching JavaScript Date.getDay() and
// Postgres EXTRACT(DOW) so the frontend grid maps straight through.
type Weekday int

const (
	Sunday Weekday = iota
	Monday
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
)

func (d Weekday) valid() bool { return d >= Sunday && d <= Saturday }

// Slot is one recurring availability span on a given weekday, expressed as
// minutes from 00:00 UTC. EndMinute is exclusive and must exceed StartMinute.
type Slot struct {
	Weekday     Weekday
	StartMinute int
	EndMinute   int
}

// WeeklyAvailability is a teacher's full recurring availability.
type WeeklyAvailability struct {
	TeacherSlug string
	Timezone    string // IANA name; booking uses it to localise the UTC slots
	Slots       []Slot
}

// ValidationError is a client-fixable problem with a proposed weekly set. The
// handler renders it as a 400; anything else from the service is a 500.
type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return ValidationError{msg: fmt.Sprintf(format, args...)}
}

// validateSlots checks a proposed weekly set: every slot well-formed and aligned
// to the granularity grid, and no two slots on the same weekday overlapping.
// Touching slots (one ends exactly where the next starts) are allowed.
func validateSlots(slots []Slot) error {
	if len(slots) > maxSlotsPerRequest {
		return invalid("too many slots: %d (max %d)", len(slots), maxSlotsPerRequest)
	}

	byDay := make(map[Weekday][]Slot, 7)
	for i, s := range slots {
		if !s.Weekday.valid() {
			return invalid("slot %d: weekday %d out of range (0–6)", i, int(s.Weekday))
		}
		if s.StartMinute < minuteOfDayMin || s.StartMinute >= minuteOfDayMax {
			return invalid("slot %d: start_minute %d out of range (0–%d)", i, s.StartMinute, minuteOfDayMax-1)
		}
		if s.EndMinute <= minuteOfDayMin || s.EndMinute > minuteOfDayMax {
			return invalid("slot %d: end_minute %d out of range (1–%d)", i, s.EndMinute, minuteOfDayMax)
		}
		if s.StartMinute >= s.EndMinute {
			return invalid("slot %d: start_minute %d must be before end_minute %d", i, s.StartMinute, s.EndMinute)
		}
		if s.StartMinute%SlotGranularityMinutes != 0 || s.EndMinute%SlotGranularityMinutes != 0 {
			return invalid("slot %d: start_minute and end_minute must be multiples of %d", i, SlotGranularityMinutes)
		}
		byDay[s.Weekday] = append(byDay[s.Weekday], s)
	}

	for day, daySlots := range byDay {
		sort.Slice(daySlots, func(a, b int) bool { return daySlots[a].StartMinute < daySlots[b].StartMinute })
		for i := 1; i < len(daySlots); i++ {
			if daySlots[i].StartMinute < daySlots[i-1].EndMinute {
				return invalid("weekday %d: slots %d–%d and %d–%d overlap",
					int(day),
					daySlots[i-1].StartMinute, daySlots[i-1].EndMinute,
					daySlots[i].StartMinute, daySlots[i].EndMinute)
			}
		}
	}
	return nil
}

// sortSlots orders a weekly set canonically: by weekday, then start time.
func sortSlots(slots []Slot) {
	sort.Slice(slots, func(a, b int) bool {
		if slots[a].Weekday != slots[b].Weekday {
			return slots[a].Weekday < slots[b].Weekday
		}
		return slots[a].StartMinute < slots[b].StartMinute
	})
}

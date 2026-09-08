package bookings

import (
	"testing"
	"time"
)

// span is a tiny constructor for readability in the table below.
func span(weekday, startMin, endMin int) AvailabilitySpan {
	return AvailabilitySpan{Weekday: weekday, StartMinute: startMin, EndMinute: endMin}
}

func utc(y int, mo time.Month, d, h, mi int) time.Time {
	return time.Date(y, mo, d, h, mi, 0, 0, time.UTC)
}

// TestGenerateSlots_Projection covers the tz / weekday / midnight-wrap behaviour
// that is the review focus: the weekly availability is UTC minutes-from-midnight
// on a UTC weekday, so projection is direct, and spans the frontend split at
// 00:00 UTC are stitched back so a lesson may straddle midnight.
func TestGenerateSlots_Projection(t *testing.T) {
	// now = Monday 2026-01-05 00:00 UTC.
	now := fixedNow
	price := func() Money { return Money{AmountMinor: 100, Currency: "UZS"} }

	// Availability:
	//   Mon 23:00–24:00 UTC  (weekday 1, [1380,1440])   -- split half
	//   Tue 00:00–02:00 UTC  (weekday 2, [0,120])        -- split half + more
	//   Wed 09:00–10:00 UTC  (weekday 3, [540,600])      -- ordinary
	spans := []AvailabilitySpan{
		span(1, 1380, 1440),
		span(2, 0, 120),
		span(3, 540, 600),
	}

	tests := []struct {
		name     string
		booked   []Interval
		duration int
		want     []time.Time
	}{
		{
			name:     "60m slots stitch across UTC midnight",
			duration: 60,
			want: []time.Time{
				utc(2026, 1, 5, 23, 0),  // Mon 23:00 -> Tue 00:00
				utc(2026, 1, 5, 23, 30), // Mon 23:30 -> Tue 00:30  (crosses midnight)
				utc(2026, 1, 6, 0, 0),   // Tue 00:00 -> 01:00
				utc(2026, 1, 6, 0, 30),  // Tue 00:30 -> 01:30
				utc(2026, 1, 6, 1, 0),   // Tue 01:00 -> 02:00 (last that fits)
				utc(2026, 1, 7, 9, 0),   // Wed 09:00 -> 10:00
			},
		},
		{
			name:     "existing Tue 00:00-01:00 booking removes the overlapping starts",
			duration: 60,
			booked:   []Interval{{Start: utc(2026, 1, 6, 0, 0), End: utc(2026, 1, 6, 1, 0)}},
			want: []time.Time{
				utc(2026, 1, 5, 23, 0), // ends exactly at 00:00 — half-open, no overlap
				utc(2026, 1, 6, 1, 0),  // starts exactly at 01:00 — half-open, no overlap
				utc(2026, 1, 7, 9, 0),
			},
		},
		{
			name:     "90m lessons only fit the stitched Mon/Tue window",
			duration: 90,
			want: []time.Time{
				utc(2026, 1, 5, 23, 0),  // -> Tue 00:30
				utc(2026, 1, 5, 23, 30), // -> Tue 01:00
				utc(2026, 1, 6, 0, 0),   // -> Tue 01:30
				utc(2026, 1, 6, 0, 30),  // -> Tue 02:00 (last that fits)
			},
		},
	}

	// window: now .. now+3d (Thu 2026-01-08 00:00 UTC).
	to := now.AddDate(0, 0, 3)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := SlotQuery{From: now, To: to, DurationMinutes: tc.duration}
			got := generateSlots(spans, tc.booked, q, price, now)

			if len(got) != len(tc.want) {
				t.Fatalf("got %d slots, want %d\n got:  %v\n want: %v", len(got), len(tc.want), starts(got), tc.want)
			}
			for i := range tc.want {
				if !got[i].StartAt.Equal(tc.want[i]) {
					t.Errorf("slot %d start = %s, want %s", i, got[i].StartAt, tc.want[i])
				}
				if got[i].EndAt.Sub(got[i].StartAt) != time.Duration(tc.duration)*time.Minute {
					t.Errorf("slot %d duration = %s, want %dm", i, got[i].EndAt.Sub(got[i].StartAt), tc.duration)
				}
				if got[i].StartAt.Location() != time.UTC {
					t.Errorf("slot %d not in UTC: %s", i, got[i].StartAt.Location())
				}
			}
		})
	}
}

func TestGenerateSlots_NeverInThePast(t *testing.T) {
	// now sits in the middle of a Monday span; only starts at/after now survive.
	now := utc(2026, 1, 5, 9, 15)
	spans := []AvailabilitySpan{span(1, 480, 720)} // Mon 08:00–12:00 UTC
	q := SlotQuery{From: utc(2026, 1, 5, 0, 0), To: utc(2026, 1, 6, 0, 0), DurationMinutes: 60}

	got := generateSlots(spans, nil, q, func() Money { return Money{} }, now)
	if len(got) == 0 {
		t.Fatal("expected some slots")
	}
	for _, s := range got {
		if s.StartAt.Before(now) {
			t.Errorf("slot %s is before now %s", s.StartAt, now)
		}
	}
	// First surviving start steps from the span start (08:00) by 30m: 09:30.
	if !got[0].StartAt.Equal(utc(2026, 1, 5, 9, 30)) {
		t.Errorf("first slot = %s, want 09:30", got[0].StartAt)
	}
}

func TestIsBookableStart(t *testing.T) {
	now := fixedNow
	spans := []AvailabilitySpan{span(1, 1380, 1440), span(2, 0, 120)} // stitched Mon 23:00 -> Tue 02:00

	cases := []struct {
		name     string
		start    time.Time
		duration int
		booked   []Interval
		want     bool
	}{
		{"aligned start inside stitched window", utc(2026, 1, 5, 23, 30), 60, nil, true},
		{"misaligned (10 past)", utc(2026, 1, 5, 23, 10), 60, nil, false},
		{"does not fit window", utc(2026, 1, 6, 1, 30), 60, nil, false},
		{"in the past", utc(2026, 1, 4, 23, 30), 60, nil, false},
		{"blocked by existing booking", utc(2026, 1, 5, 23, 30), 60,
			[]Interval{{Start: utc(2026, 1, 6, 0, 0), End: utc(2026, 1, 6, 1, 0)}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isBookableStart(spans, c.booked, c.start, c.duration, now); got != c.want {
				t.Errorf("isBookableStart = %v, want %v", got, c.want)
			}
		})
	}
}

func TestHourlyPrice_Rounding(t *testing.T) {
	// 90,000 so'm/hour = 9_000_000 minor. 90 min -> 13_500_000. 45 min not
	// allowed as a duration, but the rounding formula is half-up:
	cases := []struct {
		perHour  int64
		duration int
		want     int64
	}{
		{9_000_000, 60, 9_000_000},
		{9_000_000, 90, 13_500_000},
		{9_000_000, 30, 4_500_000},
		{100, 90, 150},
		{101, 90, 152}, // 101*90/60 = 151.5 -> 152
	}
	for _, c := range cases {
		if got := hourlyPrice(c.perHour, c.duration, "UZS"); got.AmountMinor != c.want {
			t.Errorf("hourlyPrice(%d, %d) = %d, want %d", c.perHour, c.duration, got.AmountMinor, c.want)
		}
	}
}

func starts(slots []Slot) []time.Time {
	out := make([]time.Time, len(slots))
	for i, s := range slots {
		out[i] = s.StartAt
	}
	return out
}

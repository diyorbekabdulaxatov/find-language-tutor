package main

import (
	"sort"
	"strings"
	"testing"
)

// TestSeedDemoEmailsAreUniqueAndWellFormed guards the derived demo logins: one
// per teacher, lowercase ASCII, collision-free (the seed inserts them into the
// unique users.email column).
func TestSeedDemoEmailsAreUniqueAndWellFormed(t *testing.T) {
	seen := map[string]string{}
	for _, tr := range seedTeachers {
		email := demoEmail(tr.DisplayName)
		if !strings.HasSuffix(email, "@example.com") || strings.ContainsAny(email, " \t") {
			t.Errorf("%s: malformed demo email %q", tr.Slug, email)
		}
		if email != strings.ToLower(email) {
			t.Errorf("%s: demo email not lowercase: %q", tr.Slug, email)
		}
		for _, r := range email {
			if r > 127 {
				t.Errorf("%s: non-ASCII demo email %q", tr.Slug, email)
			}
		}
		if prev, ok := seen[email]; ok {
			t.Errorf("demo email collision: %q from both %q and %q", email, prev, tr.Slug)
		}
		seen[email] = tr.Slug
	}
}

// TestSeedBookingsAreWellFormed guards the demo bookings against the rules the
// database enforces (valid status, positive duration, no double-booking of a
// teacher or a student) and against a teacher booking their own profile, so
// `make seed` cannot be broken by an edit to the data.
func TestSeedBookingsAreWellFormed(t *testing.T) {
	knownSlugs := map[string]string{} // slug -> owning demo first name
	for _, tr := range seedTeachers {
		knownSlugs[tr.Slug] = strings.ToLower(firstName(tr.DisplayName))
	}
	knownStudents := map[string]bool{}
	for _, tr := range seedTeachers {
		knownStudents[strings.ToLower(firstName(tr.DisplayName))] = true
	}

	validStatus := map[string]bool{
		"pending_payment": true, "confirmed": true, "completed": true, "cancelled": true,
	}
	validDuration := map[int]bool{30: true, 60: true, 90: true, 120: true}

	type span struct{ start, end int }
	byTeacher := map[string][]span{}
	byStudent := map[string][]span{}

	for _, b := range seedBookings {
		owner, ok := knownSlugs[b.TeacherSlug]
		if !ok {
			t.Errorf("seedBookings has slug %q with no matching teacher", b.TeacherSlug)
			continue
		}
		if !knownStudents[b.StudentFirstName] {
			t.Errorf("seedBookings %s: no demo account for %q", b.TeacherSlug, b.StudentFirstName)
		}
		if owner == b.StudentFirstName {
			t.Errorf("seedBookings %s: %q would be booking their own profile", b.TeacherSlug, owner)
		}
		if !validStatus[b.Status] {
			t.Errorf("seedBookings %s: unknown status %q", b.TeacherSlug, b.Status)
		}
		if !validDuration[b.DurationMinutes] {
			t.Errorf("seedBookings %s: duration %d is not one of 30/60/90/120", b.TeacherSlug, b.DurationMinutes)
		}
		if b.Dispute != "" && b.Status != "confirmed" && b.Status != "completed" {
			t.Errorf("seedBookings %s: a %s booking cannot carry a dispute", b.TeacherSlug, b.Status)
		}
		switch b.Payout {
		case "":
		case "available", "paid":
			// Only a completed lesson has been captured, so only a completed
			// lesson can carry a payout-ledger row.
			if b.Status != "completed" {
				t.Errorf("seedBookings %s: a %s booking cannot carry a payout", b.TeacherSlug, b.Status)
			}
			if b.PayoutClearedDaysAgo <= 0 {
				t.Errorf("seedBookings %s: a seeded payout must have cleared in the past", b.TeacherSlug)
			}
		default:
			t.Errorf("seedBookings %s: unknown payout %q", b.TeacherSlug, b.Payout)
		}

		s := span{start: b.StartOffsetHours * 60, end: b.StartOffsetHours*60 + b.DurationMinutes}
		for _, prev := range byTeacher[b.TeacherSlug] {
			if s.start < prev.end && prev.start < s.end {
				t.Errorf("seedBookings: two lessons overlap for teacher %s", b.TeacherSlug)
			}
		}
		for _, prev := range byStudent[b.StudentFirstName] {
			if s.start < prev.end && prev.start < s.end {
				t.Errorf("seedBookings: two lessons overlap for student %s", b.StudentFirstName)
			}
		}
		byTeacher[b.TeacherSlug] = append(byTeacher[b.TeacherSlug], s)
		byStudent[b.StudentFirstName] = append(byStudent[b.StudentFirstName], s)
	}
}

// TestSeedAvailabilityIsWellFormed guards the demo availability against the
// same rules the database (CHECK constraints + the no-overlap exclusion
// constraint) and the availability service enforce, so `make seed` can't be
// broken by an edit to the data.
func TestSeedAvailabilityIsWellFormed(t *testing.T) {
	knownSlugs := map[string]bool{}
	for _, tr := range seedTeachers {
		knownSlugs[tr.Slug] = true
	}

	for slug, slots := range seedAvailability {
		if !knownSlugs[slug] {
			t.Errorf("seedAvailability has slug %q with no matching teacher", slug)
		}

		byDay := map[int][]seedSlot{}
		for _, s := range slots {
			if s.Weekday < 0 || s.Weekday > 6 {
				t.Errorf("%s: weekday %d out of range", slug, s.Weekday)
			}
			if s.Start < 0 || s.Start >= 1440 || s.End <= 0 || s.End > 1440 {
				t.Errorf("%s: slot %d-%d out of the 0..1440 range", slug, s.Start, s.End)
			}
			if s.Start >= s.End {
				t.Errorf("%s: slot start %d not before end %d", slug, s.Start, s.End)
			}
			if s.Start%15 != 0 || s.End%15 != 0 {
				t.Errorf("%s: slot %d-%d not aligned to the 15-minute grid", slug, s.Start, s.End)
			}
			byDay[s.Weekday] = append(byDay[s.Weekday], s)
		}

		for day, ds := range byDay {
			sort.Slice(ds, func(a, b int) bool { return ds[a].Start < ds[b].Start })
			for i := 1; i < len(ds); i++ {
				if ds[i].Start < ds[i-1].End {
					t.Errorf("%s weekday %d: slots %d-%d and %d-%d overlap",
						slug, day, ds[i-1].Start, ds[i-1].End, ds[i].Start, ds[i].End)
				}
			}
		}
	}
}

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

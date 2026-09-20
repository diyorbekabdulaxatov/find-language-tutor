package teachers

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// newLessonTypeFixture builds a service whose repo has one teacher profile
// owned by the returned owner id.
func newLessonTypeFixture(t *testing.T) (*Service, uuid.UUID, *fakeRepo) {
	t.Helper()
	owner := uuid.New()
	teacherID := uuid.New()
	repo := &fakeRepo{
		bySlug:      map[string]*Teacher{"nodira": {ID: teacherID, Slug: "nodira"}},
		ownerBySlug: map[string]uuid.UUID{"nodira": owner},
		lessonTypes: map[uuid.UUID]LessonType{},
	}
	return NewService(repo), owner, repo
}

func price(minutes int, minor int64) LessonPrice {
	return LessonPrice{DurationMinutes: minutes, Price: Money{AmountMinor: minor, Currency: CurrencyUZS}}
}

func TestCreateLessonType_ValidatesInput(t *testing.T) {
	svc, owner, _ := newLessonTypeFixture(t)
	ctx := context.Background()

	cases := map[string]LessonTypeInput{
		"no title":         {Prices: []LessonPrice{price(60, 9_000_000)}},
		"blank title":      {Title: "   ", Prices: []LessonPrice{price(60, 9_000_000)}},
		"no prices":        {Title: "IELTS"},
		"unknown duration": {Title: "IELTS", Prices: []LessonPrice{price(37, 9_000_000)}},
		"duplicate length": {Title: "IELTS", Prices: []LessonPrice{price(60, 9_000_000), price(60, 8_000_000)}},
		"negative price":   {Title: "IELTS", Prices: []LessonPrice{price(60, -1)}},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			var ve ValidationError
			if _, err := svc.CreateLessonType(ctx, owner, in); !errors.As(err, &ve) {
				t.Fatalf("want ValidationError, got %v", err)
			}
		})
	}
}

func TestCreateLessonType_TrimsAndPositions(t *testing.T) {
	svc, owner, _ := newLessonTypeFixture(t)
	ctx := context.Background()

	first, err := svc.CreateLessonType(ctx, owner, LessonTypeInput{
		Title:       "  IELTS Speaking  ",
		Description: "  Mock Part 2.  ",
		Prices:      []LessonPrice{price(60, 9_000_000), price(45, 7_000_000)},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if first.Title != "IELTS Speaking" || first.Description != "Mock Part 2." {
		t.Fatalf("not trimmed: %q / %q", first.Title, first.Description)
	}
	if first.Position != 0 {
		t.Fatalf("first offering position = %d, want 0", first.Position)
	}
	// From() is the cheapest price, not the first listed.
	if got := first.From().AmountMinor; got != 7_000_000 {
		t.Fatalf("From() = %d, want the 45-minute price", got)
	}

	second, err := svc.CreateLessonType(ctx, owner, LessonTypeInput{
		Title:  "Conversation",
		Prices: []LessonPrice{price(30, 4_000_000)},
	})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if second.Position != 1 {
		t.Fatalf("second offering position = %d, want 1", second.Position)
	}

	// A trial always sorts above the rest.
	trial, err := svc.CreateLessonType(ctx, owner, LessonTypeInput{
		Title: "Trial lesson", IsTrial: true, Prices: []LessonPrice{price(30, 3_000_000)},
	})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	if trial.Position != -1 {
		t.Fatalf("trial position = %d, want -1", trial.Position)
	}
}

func TestCreateLessonType_OneTrialPerTeacher(t *testing.T) {
	svc, owner, _ := newLessonTypeFixture(t)
	ctx := context.Background()

	in := LessonTypeInput{Title: "Trial lesson", IsTrial: true, Prices: []LessonPrice{price(30, 3_000_000)}}
	if _, err := svc.CreateLessonType(ctx, owner, in); err != nil {
		t.Fatalf("first trial: %v", err)
	}
	if _, err := svc.CreateLessonType(ctx, owner, in); !errors.Is(err, ErrTrialExists) {
		t.Fatalf("second trial: want ErrTrialExists, got %v", err)
	}
}

func TestCreateLessonType_CapsTheList(t *testing.T) {
	svc, owner, _ := newLessonTypeFixture(t)
	ctx := context.Background()

	for i := 0; i < MaxLessonTypes; i++ {
		if _, err := svc.CreateLessonType(ctx, owner, LessonTypeInput{
			Title: "Lesson", Prices: []LessonPrice{price(60, 9_000_000)},
		}); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if _, err := svc.CreateLessonType(ctx, owner, LessonTypeInput{
		Title: "One too many", Prices: []LessonPrice{price(60, 9_000_000)},
	}); !errors.Is(err, ErrTooManyLessonTypes) {
		t.Fatalf("want ErrTooManyLessonTypes, got %v", err)
	}
}

func TestLessonTypes_HideArchivedFromThePublicList(t *testing.T) {
	svc, owner, _ := newLessonTypeFixture(t)
	ctx := context.Background()

	lt, err := svc.CreateLessonType(ctx, owner, LessonTypeInput{
		Title: "Conversation", Prices: []LessonPrice{price(30, 4_000_000)},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.SetLessonTypeArchived(ctx, owner, lt.ID, true); err != nil {
		t.Fatalf("archive: %v", err)
	}

	public, err := svc.LessonTypes(ctx, "nodira")
	if err != nil {
		t.Fatalf("public list: %v", err)
	}
	if len(public) != 0 {
		t.Fatalf("archived offering still public: %+v", public)
	}

	own, err := svc.OwnLessonTypes(ctx, owner)
	if err != nil {
		t.Fatalf("own list: %v", err)
	}
	if len(own) != 1 || !own[0].Archived {
		t.Fatalf("own list should keep the archived offering: %+v", own)
	}
}

func TestLessonTypeWrites_AreScopedToTheOwner(t *testing.T) {
	svc, owner, repo := newLessonTypeFixture(t)
	ctx := context.Background()

	mine, err := svc.CreateLessonType(ctx, owner, LessonTypeInput{
		Title: "Mine", Prices: []LessonPrice{price(60, 9_000_000)},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// A second teacher, with their own profile and offering.
	otherOwner := uuid.New()
	otherTeacher := uuid.New()
	repo.bySlug["bekzod"] = &Teacher{ID: otherTeacher, Slug: "bekzod"}
	repo.ownerBySlug["bekzod"] = otherOwner

	// Someone else's offering reads as "not found", never 403 — the endpoint
	// must not confirm another teacher's ids.
	if _, err := svc.UpdateLessonType(ctx, otherOwner, mine.ID, LessonTypeInput{
		Title: "Hijacked", Prices: []LessonPrice{price(60, 1)},
	}); !errors.Is(err, ErrLessonTypeNotFound) {
		t.Fatalf("cross-owner update: want ErrLessonTypeNotFound, got %v", err)
	}
	if _, err := svc.SetLessonTypeArchived(ctx, otherOwner, mine.ID, true); !errors.Is(err, ErrLessonTypeNotFound) {
		t.Fatalf("cross-owner archive: want ErrLessonTypeNotFound, got %v", err)
	}
}

func TestDefaultLessonTypes(t *testing.T) {
	// 120,000 so'm/hour, matching migration 000025's ((hourly*mins)+30)/60.
	const hourly int64 = 12_000_000
	got := DefaultLessonTypes(hourly, nil, CurrencyUZS)

	if len(got) != 1 {
		t.Fatalf("without a trial price: got %d offerings, want 1", len(got))
	}
	regular := got[0]
	if regular.IsTrial {
		t.Error("the default offering must not be the trial")
	}
	want := map[int]int64{30: 6_000_000, 60: 12_000_000, 90: 18_000_000, 120: 24_000_000}
	if len(regular.Prices) != len(want) {
		t.Fatalf("prices = %d, want %d", len(regular.Prices), len(want))
	}
	for _, p := range regular.Prices {
		if w, ok := want[p.DurationMinutes]; !ok {
			t.Errorf("unexpected duration %d", p.DurationMinutes)
		} else if p.Price.AmountMinor != w {
			t.Errorf("%d min = %d, want %d", p.DurationMinutes, p.Price.AmountMinor, w)
		}
		if p.Price.Currency != CurrencyUZS {
			t.Errorf("%d min currency = %q", p.DurationMinutes, p.Price.Currency)
		}
	}

	// A named trial price adds a second offering, sorted above the rest.
	trial := int64(3_000_000)
	got = DefaultLessonTypes(hourly, &trial, CurrencyUZS)
	if len(got) != 2 {
		t.Fatalf("with a trial price: got %d offerings, want 2", len(got))
	}
	if !got[1].IsTrial || got[1].Position >= got[0].Position {
		t.Errorf("trial = %+v, want is_trial with a lower position than %d", got[1], got[0].Position)
	}
	if len(got[1].Prices) != 1 || got[1].Prices[0].Price.AmountMinor != trial {
		t.Errorf("trial prices = %+v, want one row at %d", got[1].Prices, trial)
	}
}

func TestProRateRoundsToNearest(t *testing.T) {
	// 100,001 minor over 30 min is 50,000.5 → 50,001, the same rounding the
	// migration's integer arithmetic does. Drifting here would reprice every
	// new teacher relative to the backfilled ones.
	for _, tc := range []struct {
		hourly  int64
		minutes int
		want    int64
	}{
		{100_001, 30, 50_001},
		{100_000, 30, 50_000},
		{100_000, 45, 75_000},
		{1, 30, 1},
		{0, 60, 0},
	} {
		if got := proRate(tc.hourly, tc.minutes); got != tc.want {
			t.Errorf("proRate(%d, %d) = %d, want %d", tc.hourly, tc.minutes, got, tc.want)
		}
	}
}

func TestSummaryFromPriceFallback(t *testing.T) {
	hourly := Money{AmountMinor: 9_000_000, Currency: CurrencyUZS}

	// Off the search path FromPrice is never populated, so the card still has a
	// price to show: the hourly rate.
	if got := fromPrice(Teacher{PricePerHour: hourly}); got != hourly {
		t.Errorf("underived FromPrice: got %+v, want the hourly %+v", got, hourly)
	}

	// Derived by the search query: that value wins.
	derived := Money{AmountMinor: 4_000_000, Currency: CurrencyUZS}
	if got := fromPrice(Teacher{PricePerHour: hourly, FromPrice: derived}); got != derived {
		t.Errorf("derived FromPrice: got %+v, want %+v", got, derived)
	}

	// A free lesson is a real price, not a missing one — lesson_type_prices
	// allows price_minor = 0, so zero must not fall back to the hourly rate.
	free := Money{AmountMinor: 0, Currency: CurrencyUZS}
	if got := fromPrice(Teacher{PricePerHour: hourly, FromPrice: free}); got != free {
		t.Errorf("free lesson: got %+v, want %+v", got, free)
	}
}

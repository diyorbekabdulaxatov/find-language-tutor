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

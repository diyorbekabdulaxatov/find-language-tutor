package availability

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// fakeRepo is an in-memory Repository for service tests.
type fakeRepo struct {
	ref        TeacherRef
	refErr     error
	stored     []Slot
	replaced   []Slot
	replaceErr error
	replaceHit bool
}

func (f *fakeRepo) TeacherContext(_ context.Context, _ string) (TeacherRef, error) {
	return f.ref, f.refErr
}

func (f *fakeRepo) ListSlots(_ context.Context, _ uuid.UUID) ([]Slot, error) {
	return f.stored, nil
}

func (f *fakeRepo) ReplaceSlots(_ context.Context, _ uuid.UUID, slots []Slot) error {
	f.replaceHit = true
	if f.replaceErr != nil {
		return f.replaceErr
	}
	f.replaced = slots
	return nil
}

// demoOwnerID is the account that owns the fake teacher in these tests.
var demoOwnerID = uuid.New()

func teacherRef() TeacherRef {
	return TeacherRef{ID: uuid.New(), Slug: "nodira-karimova", Timezone: "Asia/Tashkent", OwnerID: demoOwnerID}
}

func TestService_GetBySlug_TeacherNotFound(t *testing.T) {
	svc := NewService(&fakeRepo{refErr: ErrTeacherNotFound})

	_, err := svc.GetBySlug(context.Background(), "ghost")
	if !errors.Is(err, ErrTeacherNotFound) {
		t.Fatalf("err = %v, want ErrTeacherNotFound", err)
	}
}

func TestService_GetBySlug_ReturnsSortedSlotsAndTimezone(t *testing.T) {
	repo := &fakeRepo{
		ref: teacherRef(),
		stored: []Slot{
			{Weekday: Wednesday, StartMinute: 600, EndMinute: 660},
			{Weekday: Monday, StartMinute: 540, EndMinute: 600},
			{Weekday: Monday, StartMinute: 240, EndMinute: 300},
		},
	}
	svc := NewService(repo)

	wa, err := svc.GetBySlug(context.Background(), "nodira-karimova")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wa.Timezone != "Asia/Tashkent" || wa.TeacherSlug != "nodira-karimova" {
		t.Errorf("context not propagated: %+v", wa)
	}
	want := []Slot{
		{Weekday: Monday, StartMinute: 240, EndMinute: 300},
		{Weekday: Monday, StartMinute: 540, EndMinute: 600},
		{Weekday: Wednesday, StartMinute: 600, EndMinute: 660},
	}
	if len(wa.Slots) != len(want) {
		t.Fatalf("got %d slots, want %d", len(wa.Slots), len(want))
	}
	for i := range want {
		if wa.Slots[i] != want[i] {
			t.Errorf("slot %d = %+v, want %+v", i, wa.Slots[i], want[i])
		}
	}
}

func TestService_Replace_Valid_SortsAndPersists(t *testing.T) {
	repo := &fakeRepo{ref: teacherRef()}
	svc := NewService(repo)

	in := []Slot{
		{Weekday: Tuesday, StartMinute: 600, EndMinute: 690},
		{Weekday: Monday, StartMinute: 540, EndMinute: 600},
		{Weekday: Monday, StartMinute: 600, EndMinute: 660}, // touches previous — allowed
	}
	wa, err := svc.Replace(context.Background(), "nodira-karimova", demoOwnerID, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.replaceHit {
		t.Fatal("ReplaceSlots was not called")
	}
	if repo.replaced[0].Weekday != Monday || repo.replaced[0].StartMinute != 540 {
		t.Errorf("slots not sorted before persist: %+v", repo.replaced)
	}
	if wa.Timezone != "Asia/Tashkent" {
		t.Errorf("timezone = %q", wa.Timezone)
	}
}

func TestService_Replace_TeacherNotFound(t *testing.T) {
	svc := NewService(&fakeRepo{refErr: ErrTeacherNotFound})

	_, err := svc.Replace(context.Background(), "ghost", demoOwnerID, nil)
	if !errors.Is(err, ErrTeacherNotFound) {
		t.Fatalf("err = %v, want ErrTeacherNotFound", err)
	}
}

func TestService_Replace_RejectsNonOwner(t *testing.T) {
	repo := &fakeRepo{ref: teacherRef()}
	svc := NewService(repo)

	_, err := svc.Replace(context.Background(), "nodira-karimova", uuid.New(), []Slot{})
	if !errors.Is(err, ErrNotOwner) {
		t.Fatalf("err = %v, want ErrNotOwner", err)
	}
	if repo.replaceHit {
		t.Error("ReplaceSlots should not run for a non-owner")
	}
}

func TestService_Replace_RejectsUnclaimedProfile(t *testing.T) {
	ref := teacherRef()
	ref.OwnerID = uuid.Nil
	repo := &fakeRepo{ref: ref}
	svc := NewService(repo)

	_, err := svc.Replace(context.Background(), "nodira-karimova", uuid.New(), []Slot{})
	if !errors.Is(err, ErrNotOwner) {
		t.Fatalf("err = %v, want ErrNotOwner", err)
	}
}

func TestService_Replace_RejectsInvalidSets(t *testing.T) {
	cases := map[string][]Slot{
		"weekday out of range": {{Weekday: 7, StartMinute: 60, EndMinute: 120}},
		"start after end":      {{Weekday: Monday, StartMinute: 600, EndMinute: 540}},
		"start equals end":     {{Weekday: Monday, StartMinute: 600, EndMinute: 600}},
		"off the grid":         {{Weekday: Monday, StartMinute: 605, EndMinute: 665}},
		"past midnight":        {{Weekday: Monday, StartMinute: 1380, EndMinute: 1500}},
		"overlap same weekday": {
			{Weekday: Monday, StartMinute: 540, EndMinute: 660},
			{Weekday: Monday, StartMinute: 600, EndMinute: 720},
		},
	}

	for name, slots := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{ref: teacherRef()}
			svc := NewService(repo)

			_, err := svc.Replace(context.Background(), "nodira-karimova", demoOwnerID, slots)
			var ve ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
			if repo.replaceHit {
				t.Error("ReplaceSlots should not be called for an invalid set")
			}
		})
	}
}

func TestService_Replace_AllowsEmptySet(t *testing.T) {
	repo := &fakeRepo{ref: teacherRef()}
	svc := NewService(repo)

	wa, err := svc.Replace(context.Background(), "nodira-karimova", demoOwnerID, []Slot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.replaceHit || len(wa.Slots) != 0 {
		t.Errorf("empty set should clear availability: hit=%v slots=%d", repo.replaceHit, len(wa.Slots))
	}
}

package bookings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeLessonTypes is an in-memory LessonTypeReader keyed by (type id, minutes).
type fakeLessonTypes struct {
	offerings map[uuid.UUID]map[int]Offering
	err       error
	calls     int
}

func (f *fakeLessonTypes) Offering(_ context.Context, id uuid.UUID, minutes int) (Offering, bool, error) {
	f.calls++
	if f.err != nil {
		return Offering{}, false, f.err
	}
	o, ok := f.offerings[id][minutes]
	return o, ok, nil
}

// bookableStart is Monday 23:30 UTC — the start the shared stitchedSpans
// fixture makes bookable for any length up to two hours.
func bookableStart() time.Time { return fixedNow.Add(23*time.Hour + 30*time.Minute) }

func TestCreate_PricesFromTheOffering(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()

	id := uuid.New()
	svc := newService(repo)
	svc.SetLessonTypes(&fakeLessonTypes{offerings: map[uuid.UUID]map[int]Offering{
		id: {45: {ID: id, TeacherID: repo.tc.ID, Title: "IELTS Speaking", Price: Money{AmountMinor: 7_500_000, Currency: "UZS"}}},
	}})

	b, err := svc.Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: bookableStart(), DurationMinutes: 45, LessonTypeID: id,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// The offering's price wins over the teacher's hourly rate, and 45 minutes
	// is bookable even though it is not in the legacy duration list.
	if got := repo.created.PriceMinor; got != 7_500_000 {
		t.Fatalf("price = %d, want the offering's 7,500,000", got)
	}
	if repo.created.DurationMinutes != 45 {
		t.Fatalf("duration = %d, want 45", repo.created.DurationMinutes)
	}
	if !repo.created.LessonTypeID.Valid || repo.created.LessonTypeID.UUID != id {
		t.Fatalf("booking did not record the offering: %+v", repo.created.LessonTypeID)
	}
	_ = b
}

func TestCreate_OfferingSetsTheTrialFlag(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()

	id := uuid.New()
	svc := newService(repo)
	svc.SetLessonTypes(&fakeLessonTypes{offerings: map[uuid.UUID]map[int]Offering{
		id: {30: {ID: id, TeacherID: repo.tc.ID, Title: "Trial lesson", IsTrial: true, Price: Money{AmountMinor: 3_000_000, Currency: "UZS"}}},
	}})

	// The client says nothing about a trial; the offering decides.
	if _, err := svc.Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: bookableStart(), DurationMinutes: 30, LessonTypeID: id,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if !repo.created.IsTrial {
		t.Fatal("a trial offering must mark the booking as a trial")
	}
}

func TestCreate_RejectsAnotherTeachersOffering(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()

	id := uuid.New()
	svc := newService(repo)
	svc.SetLessonTypes(&fakeLessonTypes{offerings: map[uuid.UUID]map[int]Offering{
		// same id, but it belongs to a different teacher
		id: {60: {ID: id, TeacherID: uuid.New(), Title: "Someone else's", Price: Money{AmountMinor: 1, Currency: "UZS"}}},
	}})

	_, err := svc.Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: bookableStart(), DurationMinutes: 60, LessonTypeID: id,
	})
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestCreate_RejectsALengthTheOfferingIsNotPricedAt(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()

	id := uuid.New()
	svc := newService(repo)
	svc.SetLessonTypes(&fakeLessonTypes{offerings: map[uuid.UUID]map[int]Offering{
		id: {60: {ID: id, TeacherID: repo.tc.ID, Price: Money{AmountMinor: 9_000_000, Currency: "UZS"}}},
	}})

	_, err := svc.Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: bookableStart(), DurationMinutes: 90, LessonTypeID: id,
	})
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestCreate_WithoutAReaderRefusesAnOffering(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()

	// No SetLessonTypes: the port is nil. Naming a type must fail closed
	// rather than silently fall back to the hourly rate.
	_, err := newService(repo).Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: bookableStart(), DurationMinutes: 60, LessonTypeID: uuid.New(),
	})
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestSlots_PricedFromTheOffering(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()

	id := uuid.New()
	svc := newService(repo)
	svc.SetLessonTypes(&fakeLessonTypes{offerings: map[uuid.UUID]map[int]Offering{
		id: {45: {ID: id, TeacherID: repo.tc.ID, Price: Money{AmountMinor: 7_500_000, Currency: "UZS"}}},
	}})

	res, err := svc.Slots(ctx(), "nodira-karimova", nil, nil, 45, id)
	if err != nil {
		t.Fatalf("slots: %v", err)
	}
	if len(res.Slots) == 0 {
		t.Fatal("no slots generated")
	}
	if got := res.Slots[0].Price.AmountMinor; got != 7_500_000 {
		t.Fatalf("slot price = %d, want the offering's", got)
	}
}

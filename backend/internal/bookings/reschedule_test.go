package bookings

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Rescheduling: the student moves a live lesson to another bookable slot of
// the same teacher, until the free-cancellation deadline, at most
// MaxReschedules times. Length, price and payment ride along.

// weekAhead is next Monday 23:30 UTC — inside the stitchedSpans window and
// comfortably outside the 24h free-cancellation window from fixedNow.
var weekAhead = fixedNow.Add(7*24*time.Hour + 23*time.Hour + 30*time.Minute)

func movableBooking(repo *fakeRepo, student, owner uuid.UUID, status Status, start time.Time) uuid.UUID {
	id := uuid.New()
	repo.store[id] = Booking{
		ID:              id,
		Status:          status,
		StartAt:         start,
		EndAt:           start.Add(time.Hour),
		DurationMinutes: 60,
		Price:           Money{AmountMinor: 9_000_000, Currency: "UZS"},
		Student:         StudentSummary{ID: student, DisplayName: "Student"},
		TeacherOwnerID:  owner,
		Teacher:         TeacherSummary{Slug: "nodira-karimova", DisplayName: "Nodira", Timezone: "Asia/Tashkent"},
	}
	// The teacher's calendar as the repo reports it: the lesson's own slot.
	repo.booked = []Interval{{Start: start, End: start.Add(time.Hour)}}
	return id
}

func rescheduleService(repo *fakeRepo) (*Service, *fakeScheduler, *fakeNotifier) {
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()
	svc := newService(repo)
	sch := &fakeScheduler{}
	n := newFakeNotifier()
	svc.SetReminderScheduler(sch)
	svc.SetNotifier(n)
	return svc, sch, n
}

func TestReschedule_MovesTheLessonAndItsReminders(t *testing.T) {
	repo := newFakeRepo()
	svc, sch, n := rescheduleService(repo)
	student, owner := uuid.New(), uuid.New()
	id := movableBooking(repo, student, owner, StatusConfirmed, weekAhead)

	// 00:00 overlaps the lesson's own 23:30–00:30 slot — that must not count
	// as taken, since the lesson is the thing being moved.
	newStart := weekAhead.Add(30 * time.Minute)
	b, err := svc.Reschedule(ctx(), student, id, newStart)
	if err != nil {
		t.Fatalf("reschedule: %v", err)
	}
	if !b.StartAt.Equal(newStart) || !b.EndAt.Equal(newStart.Add(time.Hour)) || b.DurationMinutes != 60 {
		t.Fatalf("moved to %s–%s (%d min)", b.StartAt, b.EndAt, b.DurationMinutes)
	}
	if b.RescheduleCount != 1 || b.RescheduledFrom == nil || !b.RescheduledFrom.Equal(weekAhead) {
		t.Fatalf("history: count=%d from=%v", b.RescheduleCount, b.RescheduledFrom)
	}
	if b.Price.AmountMinor != 9_000_000 || b.Status != StatusConfirmed {
		t.Fatalf("price/status must not change: %+v", b)
	}
	// Reminders were torn down and re-armed; the teacher was told.
	if len(sch.cancelled) != 1 || len(sch.scheduled) != 1 {
		t.Fatalf("reminders: cancelled=%v scheduled=%v", sch.cancelled, sch.scheduled)
	}
	if prev, ok := n.rescheduled[id]; !ok || !prev.Equal(weekAhead) {
		t.Fatalf("notifier previousStart = %v, want %s", prev, weekAhead)
	}
}

func TestReschedule_UnpaidBookingHasNoRemindersToMove(t *testing.T) {
	repo := newFakeRepo()
	svc, sch, _ := rescheduleService(repo)
	student := uuid.New()
	id := movableBooking(repo, student, uuid.New(), StatusPendingPayment, weekAhead)

	if _, err := svc.Reschedule(ctx(), student, id, weekAhead.Add(time.Hour)); err != nil {
		t.Fatalf("reschedule: %v", err)
	}
	if len(sch.cancelled)+len(sch.scheduled) != 0 {
		t.Fatalf("no reminders should be touched for pending_payment: %v %v", sch.cancelled, sch.scheduled)
	}
}

func TestReschedule_RefusesWhatABookingWouldRefuse(t *testing.T) {
	repo := newFakeRepo()
	svc, _, _ := rescheduleService(repo)
	student := uuid.New()
	id := movableBooking(repo, student, uuid.New(), StatusConfirmed, weekAhead)

	cases := map[string]struct {
		start time.Time
		want  error
	}{
		"outside availability": {weekAhead.Add(4 * time.Hour), ErrSlotUnavailable}, // Tue 03:30, past the 02:00 end
		"misaligned":           {weekAhead.Add(7 * time.Minute), ErrSlotUnavailable},
		"in the past":          {fixedNow.Add(-time.Hour), nil}, // ValidationError
		"same time":            {weekAhead, nil},                // ValidationError
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := svc.Reschedule(ctx(), student, id, tc.start)
			var ve ValidationError
			if tc.want == nil {
				if !errors.As(err, &ve) {
					t.Fatalf("err = %v, want ValidationError", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
	// Another lesson holding the target slot → unavailable.
	repo.booked = append(repo.booked, Interval{Start: weekAhead.Add(time.Hour), End: weekAhead.Add(2 * time.Hour)})
	if _, err := svc.Reschedule(ctx(), student, id, weekAhead.Add(time.Hour)); !errors.Is(err, ErrSlotUnavailable) {
		t.Fatalf("taken slot: err = %v", err)
	}
	if repo.store[id].RescheduleCount != 0 {
		t.Fatal("a refused move must not touch the booking")
	}
}

func TestReschedule_TeacherCannot(t *testing.T) {
	repo := newFakeRepo()
	svc, _, _ := rescheduleService(repo)
	student, owner := uuid.New(), uuid.New()
	id := movableBooking(repo, student, owner, StatusConfirmed, weekAhead)

	if _, err := svc.Reschedule(ctx(), owner, id, weekAhead.Add(time.Hour)); !errors.Is(err, ErrOnlyStudentReschedules) {
		t.Fatalf("teacher: err = %v", err)
	}
	if _, err := svc.Reschedule(ctx(), uuid.New(), id, weekAhead.Add(time.Hour)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("stranger: err = %v", err)
	}
}

func TestReschedule_LateIsRefusedWithThePolicy(t *testing.T) {
	repo := newFakeRepo()
	svc, _, _ := rescheduleService(repo)
	student := uuid.New()
	// Tonight 23:30 is inside the 24h window.
	id := movableBooking(repo, student, uuid.New(), StatusConfirmed, fixedNow.Add(23*time.Hour+30*time.Minute))

	_, err := svc.Reschedule(ctx(), student, id, weekAhead)
	var late LateRescheduleError
	if !errors.As(err, &late) || !late.Policy.Late {
		t.Fatalf("err = %v, want LateRescheduleError", err)
	}
	if svc.CanReschedule(repo.store[id], student) {
		t.Fatal("CanReschedule must agree with Reschedule")
	}
}

func TestReschedule_CapsTheNumberOfMoves(t *testing.T) {
	repo := newFakeRepo()
	svc, _, _ := rescheduleService(repo)
	student := uuid.New()
	id := movableBooking(repo, student, uuid.New(), StatusConfirmed, weekAhead)
	b := repo.store[id]
	b.RescheduleCount = MaxReschedules
	repo.store[id] = b

	if _, err := svc.Reschedule(ctx(), student, id, weekAhead.Add(time.Hour)); !errors.Is(err, ErrRescheduleLimit) {
		t.Fatalf("err = %v, want ErrRescheduleLimit", err)
	}
	if svc.CanReschedule(b, student) {
		t.Fatal("CanReschedule must be false at the cap")
	}
}

func TestReschedule_CompletedOrCancelledCannotMove(t *testing.T) {
	repo := newFakeRepo()
	svc, _, _ := rescheduleService(repo)
	student := uuid.New()
	for _, st := range []Status{StatusCompleted, StatusCancelled} {
		id := movableBooking(repo, student, uuid.New(), st, weekAhead)
		if _, err := svc.Reschedule(ctx(), student, id, weekAhead.Add(time.Hour)); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("%s: err = %v", st, err)
		}
	}
}

func TestReschedule_RaceLostOnTheUpdateIsSlotTaken(t *testing.T) {
	repo := newFakeRepo()
	svc, _, _ := rescheduleService(repo)
	student := uuid.New()
	id := movableBooking(repo, student, uuid.New(), StatusConfirmed, weekAhead)
	repo.rescheduleErr = ErrSlotTaken // the EXCLUDE constraint fired

	if _, err := svc.Reschedule(ctx(), student, id, weekAhead.Add(time.Hour)); !errors.Is(err, ErrSlotTaken) {
		t.Fatalf("err = %v, want ErrSlotTaken", err)
	}
}

func TestHandler_Reschedule(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()
	tm := testTokenManager()
	student, owner := uuid.New(), uuid.New()
	id := movableBooking(repo, student, owner, StatusConfirmed, weekAhead)
	r := newTestRouter(repo, tm)

	// The DTO advertises the move to the student only.
	for _, tc := range []struct {
		who  uuid.UUID
		want bool
	}{{student, true}, {owner, false}} {
		w := do(r, http.MethodGet, "/v1/bookings/"+id.String(), "", bearerFor(tm, tc.who))
		var dto bookingDTO
		if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
			t.Fatal(err)
		}
		if dto.CanReschedule != tc.want {
			t.Fatalf("can_reschedule for %v = %v, want %v", tc.who == student, dto.CanReschedule, tc.want)
		}
	}

	newStart := weekAhead.Add(time.Hour).Format(time.RFC3339)
	w := do(r, http.MethodPost, "/v1/bookings/"+id.String()+"/reschedule", `{"start_at":"`+newStart+`"}`, bearerFor(tm, owner))
	if w.Code != http.StatusForbidden {
		t.Fatalf("teacher: %d %s", w.Code, w.Body.String())
	}
	w = do(r, http.MethodPost, "/v1/bookings/"+id.String()+"/reschedule", `{}`, bearerFor(tm, student))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("no start_at: %d %s", w.Code, w.Body.String())
	}
	w = do(r, http.MethodPost, "/v1/bookings/"+id.String()+"/reschedule", `{"start_at":"`+newStart+`"}`, bearerFor(tm, student))
	if w.Code != http.StatusOK {
		t.Fatalf("student: %d %s", w.Code, w.Body.String())
	}
	var dto bookingDTO
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto.RescheduleCount != 1 || dto.RescheduledFrom == nil || !dto.StartAt.Equal(weekAhead.Add(time.Hour)) {
		t.Fatalf("dto after move: %+v", dto)
	}
}

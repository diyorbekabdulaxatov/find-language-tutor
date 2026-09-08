package bookings

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

const testMeetingURL = "https://meet.example/room-abc"

func confirmedBooking(student, owner uuid.UUID) Booking {
	return Booking{
		ID:                uuid.New(),
		Status:            StatusConfirmed,
		StartAt:           fixedNow.Add(48 * time.Hour),
		EndAt:             fixedNow.Add(49 * time.Hour),
		Student:           StudentSummary{ID: student, DisplayName: "Student"},
		TeacherOwnerID:    owner,
		TeacherMeetingURL: testMeetingURL,
	}
}

// --- meeting-link visibility (the review focus) ---

func TestBookingDTO_MeetingLinkHiddenBeforePayment(t *testing.T) {
	student := uuid.New()
	b := confirmedBooking(student, uuid.New())
	b.Status = StatusPendingPayment // teacher link is set, but not payable yet

	dto := toBookingDTO(b, student)
	if dto.MeetingURL != "" {
		t.Fatalf("meeting_url leaked on a pending_payment booking: %q", dto.MeetingURL)
	}
	// And it must not even appear in the JSON.
	raw, _ := json.Marshal(dto)
	if strings.Contains(string(raw), "meeting_url") {
		t.Fatalf("meeting_url key present in pending_payment JSON: %s", raw)
	}
}

func TestBookingDTO_MeetingLinkVisibleToParticipantWhenConfirmed(t *testing.T) {
	student, owner := uuid.New(), uuid.New()
	b := confirmedBooking(student, owner)

	for _, viewer := range []uuid.UUID{student, owner} {
		if got := toBookingDTO(b, viewer).MeetingURL; got != testMeetingURL {
			t.Errorf("viewer %s: meeting_url = %q, want %q", viewer, got, testMeetingURL)
		}
	}

	// completed is also allowed.
	b.Status = StatusCompleted
	if got := toBookingDTO(b, student).MeetingURL; got != testMeetingURL {
		t.Errorf("completed: meeting_url = %q, want %q", got, testMeetingURL)
	}

	// per-booking override wins over the teacher default.
	b.Status = StatusConfirmed
	b.MeetingURLOverride = "https://zoom.example/xyz"
	if got := toBookingDTO(b, student).MeetingURL; got != "https://zoom.example/xyz" {
		t.Errorf("override not applied: %q", got)
	}
}

func TestBookingDTO_MeetingLinkHiddenFromNonParticipant(t *testing.T) {
	b := confirmedBooking(uuid.New(), uuid.New())
	if got := toBookingDTO(b, uuid.New()).MeetingURL; got != "" {
		t.Fatalf("meeting_url leaked to a non-participant: %q", got)
	}
	// cancelled is never revealed, even to a participant.
	student := uuid.New()
	b = confirmedBooking(student, uuid.New())
	b.Status = StatusCancelled
	if got := toBookingDTO(b, student).MeetingURL; got != "" {
		t.Fatalf("meeting_url revealed on a cancelled booking: %q", got)
	}
}

// --- SetMeetingLink ---

func TestService_SetMeetingLink(t *testing.T) {
	owner := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, uuid.New(), owner)

	b, err := newService(repo).SetMeetingLink(ctx(), owner, id, "  https://meet.example/x  ")
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if b.MeetingURLOverride != "https://meet.example/x" {
		t.Errorf("override = %q, want trimmed url", b.MeetingURLOverride)
	}

	// clear
	b, err = newService(repo).SetMeetingLink(ctx(), owner, id, "")
	if err != nil || b.MeetingURLOverride != "" {
		t.Errorf("clear failed: %v / %q", err, b.MeetingURLOverride)
	}

	// non-owner
	if _, err := newService(repo).SetMeetingLink(ctx(), uuid.New(), id, "https://x.example"); !errors.Is(err, ErrNotTeacherOwner) {
		t.Errorf("non-owner err = %v, want ErrNotTeacherOwner", err)
	}

	// bad url
	var ve ValidationError
	if _, err := newService(repo).SetMeetingLink(ctx(), owner, id, "not-a-url"); !errors.As(err, &ve) {
		t.Errorf("bad url err = %v, want ValidationError", err)
	}
}

// --- no-show ---

func pastConfirmed(repo *fakeRepo, owner uuid.UUID) uuid.UUID {
	id := uuid.New()
	repo.store[id] = Booking{
		ID:             id,
		Status:         StatusConfirmed,
		StartAt:        fixedNow.Add(-2 * time.Hour),
		EndAt:          fixedNow.Add(-time.Hour),
		Student:        StudentSummary{ID: uuid.New(), DisplayName: "Student"},
		TeacherOwnerID: owner,
	}
	return id
}

func TestService_NoShow_StudentCompletesAndCaptures(t *testing.T) {
	owner := uuid.New()
	repo := newFakeRepo()
	id := pastConfirmed(repo, owner)
	gw := newFakeGateway(repo)
	gw.status[id] = "authorized"

	b, snap, err := newServiceWithGateway(repo, gw).NoShow(ctx(), owner, id, NoShowStudent)
	if err != nil {
		t.Fatalf("no-show: %v", err)
	}
	if b.Status != StatusCompleted {
		t.Errorf("status = %s, want completed", b.Status)
	}
	if b.NoShowParty != NoShowStudent {
		t.Errorf("no_show_party = %q, want student", b.NoShowParty)
	}
	if len(gw.captured) != 1 || gw.captured[0] != id {
		t.Errorf("capture not called: %+v", gw.captured)
	}
	if snap == nil || snap.Status != "captured" {
		t.Errorf("snapshot = %+v", snap)
	}
}

func TestService_NoShow_TeacherCancelsAndRefunds(t *testing.T) {
	owner := uuid.New()
	repo := newFakeRepo()
	id := pastConfirmed(repo, owner)
	gw := newFakeGateway(repo)
	gw.status[id] = "authorized"

	b, snap, err := newServiceWithGateway(repo, gw).NoShow(ctx(), owner, id, NoShowTeacher)
	if err != nil {
		t.Fatalf("no-show: %v", err)
	}
	if b.Status != StatusCancelled {
		t.Errorf("status = %s, want cancelled", b.Status)
	}
	if b.NoShowParty != NoShowTeacher {
		t.Errorf("no_show_party = %q, want teacher", b.NoShowParty)
	}
	if len(gw.refunded) != 1 || gw.refunded[0] != id {
		t.Errorf("refund not issued: %+v", gw.refunded)
	}
	if snap != nil {
		t.Errorf("snapshot = %+v, want nil for a teacher no-show", snap)
	}
}

func TestService_NoShow_TooEarly(t *testing.T) {
	owner := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, uuid.New(), owner) // start_at = fixedNow + 48h
	gw := newFakeGateway(repo)
	if _, _, err := newServiceWithGateway(repo, gw).NoShow(ctx(), owner, id, NoShowStudent); !errors.Is(err, ErrLessonNotStarted) {
		t.Fatalf("err = %v, want ErrLessonNotStarted", err)
	}
}

func TestService_NoShow_Guards(t *testing.T) {
	owner := uuid.New()
	repo := newFakeRepo()
	id := pastConfirmed(repo, owner)
	gw := newFakeGateway(repo)
	svc := newServiceWithGateway(repo, gw)

	if _, _, err := svc.NoShow(ctx(), uuid.New(), id, NoShowStudent); !errors.Is(err, ErrNotTeacherOwner) {
		t.Errorf("non-owner err = %v, want ErrNotTeacherOwner", err)
	}
	var ve ValidationError
	if _, _, err := svc.NoShow(ctx(), owner, id, "nobody"); !errors.As(err, &ve) {
		t.Errorf("bad party err = %v, want ValidationError", err)
	}

	pp := seedBooking(repo, StatusPendingPayment, uuid.New(), owner)
	b := repo.store[pp]
	b.StartAt = fixedNow.Add(-time.Hour)
	repo.store[pp] = b
	if _, _, err := svc.NoShow(ctx(), owner, pp, NoShowStudent); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("wrong-state err = %v, want ErrInvalidTransition", err)
	}
}

// --- reminders + notifications wiring ---

func TestService_Pay_SchedulesRemindersAndNotifies(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusPendingPayment, student, uuid.New())
	gw := newFakeGateway(repo)
	gw.status[id] = "requires_payment"

	svc := newServiceWithGateway(repo, gw)
	sched := &fakeScheduler{}
	notif := newFakeNotifier()
	svc.reminders = sched
	svc.notifier = notif

	if _, _, err := svc.Pay(ctx(), student, id, "pm_ok"); err != nil {
		t.Fatalf("pay: %v", err)
	}
	if len(sched.scheduled) != 1 || sched.scheduled[0] != id {
		t.Errorf("reminders not scheduled on pay: %+v", sched.scheduled)
	}
	if len(notif.confirmed) != 1 || notif.confirmed[0] != id {
		t.Errorf("confirmation mail not sent on pay: %+v", notif.confirmed)
	}
}

func TestService_Cancel_CancelsRemindersAndNotifies(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, student, uuid.New())
	gw := newFakeGateway(repo)
	gw.status[id] = "authorized"

	svc := newServiceWithGateway(repo, gw)
	sched := &fakeScheduler{}
	notif := newFakeNotifier()
	svc.reminders = sched
	svc.notifier = notif

	if _, err := svc.Cancel(ctx(), student, id, "changed my mind"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if len(sched.cancelled) != 1 || sched.cancelled[0] != id {
		t.Errorf("reminders not cancelled: %+v", sched.cancelled)
	}
	if len(notif.cancelled) != 1 || !notif.refunded[id] {
		t.Errorf("cancellation mail wrong: cancelled=%+v refunded=%v", notif.cancelled, notif.refunded)
	}
}

func TestService_NoShow_TeacherCancelsReminders(t *testing.T) {
	owner := uuid.New()
	repo := newFakeRepo()
	id := pastConfirmed(repo, owner)
	gw := newFakeGateway(repo)
	gw.status[id] = "authorized"

	svc := newServiceWithGateway(repo, gw)
	sched := &fakeScheduler{}
	svc.reminders = sched

	if _, _, err := svc.NoShow(ctx(), owner, id, NoShowTeacher); err != nil {
		t.Fatalf("no-show: %v", err)
	}
	if len(sched.cancelled) != 1 || sched.cancelled[0] != id {
		t.Errorf("reminders not cancelled on teacher no-show: %+v", sched.cancelled)
	}
}

func TestService_NilReminderSchedulerAndNotifier_AreNoOps(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusPendingPayment, student, uuid.New())
	gw := newFakeGateway(repo)
	gw.status[id] = "requires_payment"

	svc := newServiceWithGateway(repo, gw) // reminders + notifier left nil

	if _, _, err := svc.Pay(ctx(), student, id, "pm_ok"); err != nil {
		t.Fatalf("pay with nil ports: %v", err)
	}
	if _, err := svc.Cancel(ctx(), student, id, ""); err != nil {
		t.Fatalf("cancel with nil ports: %v", err)
	}
}

// --- handler routes ---

func TestHandler_NoShow_TooEarly409(t *testing.T) {
	owner := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, uuid.New(), owner)
	gw := newFakeGateway(repo)
	tm := testTokenManager()

	w := do(newTestRouterGW(repo, tm, gw), http.MethodPost, "/v1/bookings/"+id.String()+"/no-show",
		`{"party":"student"}`, bearerFor(tm, owner))
	if w.Code != http.StatusConflict || errCode(t, w) != "too_early" {
		t.Fatalf("status = %d code = %s, want 409 too_early", w.Code, errCode(t, w))
	}
}

func TestHandler_MeetingLink_NonOwner403(t *testing.T) {
	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, uuid.New(), uuid.New())
	tm := testTokenManager()

	w := do(newTestRouter(repo, tm), http.MethodPut, "/v1/bookings/"+id.String()+"/meeting-link",
		`{"url":"https://meet.example/x"}`, bearerFor(tm, uuid.New()))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

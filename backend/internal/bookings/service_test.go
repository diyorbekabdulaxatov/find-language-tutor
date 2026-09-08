package bookings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func teacherCtx() TeacherContext {
	return TeacherContext{
		ID:                uuid.New(),
		Slug:              "nodira-karimova",
		DisplayName:       "Nodira Karimova",
		Timezone:          "Asia/Tashkent",
		AvatarURL:         "https://cdn.example/a.png",
		Currency:          "UZS",
		PricePerHourMinor: 9_000_000,
		TrialPriceMinor:   ptrInt64(2_000_000),
		OwnerID:           uuid.New(),
	}
}

// stitchedSpans make Mon 23:30 UTC a bookable 60-minute start given fixedNow.
func stitchedSpans() []AvailabilitySpan {
	return []AvailabilitySpan{span(1, 1380, 1440), span(2, 0, 120)}
}

func ctx() context.Context { return context.Background() }

// --- Slots ---

func TestService_Slots_WindowCap(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	svc := newService(repo)

	from := fixedNow
	to := fixedNow.AddDate(0, 0, 22)
	_, err := svc.Slots(ctx(), "nodira-karimova", &from, &to, 60)
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestService_Slots_BadDuration(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	_, err := newService(repo).Slots(ctx(), "nodira-karimova", nil, nil, 45)
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestService_Slots_TeacherNotFound(t *testing.T) {
	repo := newFakeRepo()
	repo.tcErr = ErrTeacherNotFound
	_, err := newService(repo).Slots(ctx(), "ghost", nil, nil, 0)
	if !errors.Is(err, ErrTeacherNotFound) {
		t.Fatalf("err = %v, want ErrTeacherNotFound", err)
	}
}

func TestService_Slots_HappyPath(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()
	svc := newService(repo)

	from := fixedNow
	to := fixedNow.AddDate(0, 0, 3)
	res, err := svc.Slots(ctx(), "nodira-karimova", &from, &to, 0) // duration defaults to 60
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.DurationMinutes != 60 {
		t.Errorf("duration = %d, want 60", res.DurationMinutes)
	}
	if res.Timezone != "Asia/Tashkent" || res.TeacherSlug != "nodira-karimova" {
		t.Errorf("context not echoed: %+v", res)
	}
	if len(res.Slots) == 0 {
		t.Fatal("expected slots")
	}
	// price = 9_000_000 * 60 / 60
	if res.Slots[0].Price.AmountMinor != 9_000_000 || res.Slots[0].Price.Currency != "UZS" {
		t.Errorf("price = %+v", res.Slots[0].Price)
	}
	for _, s := range res.Slots {
		if s.StartAt.Location() != time.UTC {
			t.Errorf("slot not UTC: %s", s.StartAt)
		}
	}
}

// --- Create ---

func TestService_Create_TeacherNotFound(t *testing.T) {
	repo := newFakeRepo()
	repo.tcErr = ErrTeacherNotFound
	_, err := newService(repo).Create(ctx(), uuid.New(), CreateInput{TeacherSlug: "ghost", StartAt: fixedNow.Add(time.Hour), DurationMinutes: 60})
	if !errors.Is(err, ErrTeacherNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestService_Create_CannotBookSelf(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	_, err := newService(repo).Create(ctx(), repo.tc.OwnerID, CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: fixedNow.Add(24 * time.Hour), DurationMinutes: 60,
	})
	if !errors.Is(err, ErrCannotBookSelf) {
		t.Fatalf("err = %v, want ErrCannotBookSelf", err)
	}
}

func TestService_Create_PastStart(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	_, err := newService(repo).Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: fixedNow.Add(-time.Hour), DurationMinutes: 60,
	})
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestService_Create_BadDuration(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	_, err := newService(repo).Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: fixedNow.Add(time.Hour), DurationMinutes: 45,
	})
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestService_Create_TrialWithoutTrialPrice(t *testing.T) {
	repo := newFakeRepo()
	tc := teacherCtx()
	tc.TrialPriceMinor = nil
	repo.tc = tc
	repo.spans = stitchedSpans()
	_, err := newService(repo).Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: utc(2026, 1, 5, 23, 30), IsTrial: true,
	})
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestService_Create_SlotUnavailable(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()
	_, err := newService(repo).Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: utc(2026, 1, 5, 23, 10), DurationMinutes: 60, // misaligned
	})
	if !errors.Is(err, ErrSlotUnavailable) {
		t.Fatalf("err = %v, want ErrSlotUnavailable", err)
	}
}

func TestService_Create_HappyPath(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()
	svc := newService(repo)

	b, err := svc.Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: utc(2026, 1, 5, 23, 30), DurationMinutes: 60,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Status != StatusPendingPayment {
		t.Errorf("status = %s, want pending_payment", b.Status)
	}
	if repo.created.PriceMinor != 9_000_000 || repo.created.Currency != "UZS" {
		t.Errorf("persisted price = %d %s", repo.created.PriceMinor, repo.created.Currency)
	}
	if repo.created.DurationMinutes != 60 {
		t.Errorf("duration = %d", repo.created.DurationMinutes)
	}
	if !repo.created.EndAt.Equal(utc(2026, 1, 6, 0, 30)) {
		t.Errorf("end_at = %s, want Tue 00:30", repo.created.EndAt)
	}
}

func TestService_Create_TrialForcesThirtyMinutes(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()
	svc := newService(repo)

	_, err := svc.Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: utc(2026, 1, 5, 23, 30), DurationMinutes: 120, IsTrial: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.created.DurationMinutes != 30 {
		t.Errorf("trial duration = %d, want 30", repo.created.DurationMinutes)
	}
	if repo.created.PriceMinor != 2_000_000 {
		t.Errorf("trial price = %d, want 2_000_000", repo.created.PriceMinor)
	}
	if !repo.created.IsTrial {
		t.Error("is_trial not persisted")
	}
}

func TestService_Create_RaceReturnsSlotTaken(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()
	repo.createErr = ErrSlotTaken
	_, err := newService(repo).Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: utc(2026, 1, 5, 23, 30), DurationMinutes: 60,
	})
	if !errors.Is(err, ErrSlotTaken) {
		t.Fatalf("err = %v, want ErrSlotTaken", err)
	}
}

// --- List ---

func TestService_List_RoleFilters(t *testing.T) {
	caller := uuid.New()
	ownedTeacher := uuid.New()

	t.Run("student", func(t *testing.T) {
		repo := newFakeRepo()
		_, _ = newService(repo).List(ctx(), caller, RoleStudent, nil)
		if repo.lastFilter.StudentFilter != caller || repo.lastFilter.TeacherFilter != uuid.Nil {
			t.Errorf("filter = %+v", repo.lastFilter)
		}
	})

	t.Run("teacher with profile", func(t *testing.T) {
		repo := newFakeRepo()
		repo.ownsProfile = true
		repo.ownedID = ownedTeacher
		_, _ = newService(repo).List(ctx(), caller, RoleTeacher, nil)
		if repo.lastFilter.TeacherFilter != ownedTeacher || repo.lastFilter.StudentFilter != uuid.Nil {
			t.Errorf("filter = %+v", repo.lastFilter)
		}
	})

	t.Run("teacher without profile returns empty", func(t *testing.T) {
		repo := newFakeRepo()
		repo.listResult = []Booking{{ID: uuid.New()}}
		got, err := newService(repo).List(ctx(), caller, RoleTeacher, nil)
		if err != nil || len(got) != 0 {
			t.Errorf("got %d bookings, err %v; want 0, nil", len(got), err)
		}
	})

	t.Run("default matches both dimensions", func(t *testing.T) {
		repo := newFakeRepo()
		repo.ownsProfile = true
		repo.ownedID = ownedTeacher
		_, _ = newService(repo).List(ctx(), caller, RoleAny, nil)
		if repo.lastFilter.StudentFilter != caller || repo.lastFilter.TeacherFilter != ownedTeacher {
			t.Errorf("filter = %+v", repo.lastFilter)
		}
	})
}

func TestService_List_BadStatusAndRole(t *testing.T) {
	repo := newFakeRepo()
	bad := Status("nope")
	if _, err := newService(repo).List(ctx(), uuid.New(), RoleAny, &bad); err == nil {
		t.Error("expected ValidationError for bad status")
	}
	if _, err := newService(repo).List(ctx(), uuid.New(), Role("manager"), nil); err == nil {
		t.Error("expected ValidationError for bad role")
	}
}

// --- Get / Confirm / Cancel ---

func seedBooking(repo *fakeRepo, status Status, studentID, ownerID uuid.UUID) uuid.UUID {
	id := uuid.New()
	repo.store[id] = Booking{
		ID:             id,
		Status:         status,
		StartAt:        fixedNow.Add(48 * time.Hour),
		EndAt:          fixedNow.Add(49 * time.Hour),
		Student:        StudentSummary{ID: studentID, DisplayName: "Student"},
		TeacherOwnerID: ownerID,
	}
	return id
}

func TestService_Get(t *testing.T) {
	student := uuid.New()
	owner := uuid.New()
	stranger := uuid.New()

	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, student, owner)
	svc := newService(repo)

	if _, err := svc.Get(ctx(), student, id); err != nil {
		t.Errorf("student should see booking: %v", err)
	}
	if _, err := svc.Get(ctx(), owner, id); err != nil {
		t.Errorf("teacher-owner should see booking: %v", err)
	}
	if _, err := svc.Get(ctx(), stranger, id); !errors.Is(err, ErrForbidden) {
		t.Errorf("stranger err = %v, want ErrForbidden", err)
	}
	if _, err := svc.Get(ctx(), student, uuid.New()); !errors.Is(err, ErrBookingNotFound) {
		t.Errorf("missing err = %v, want ErrBookingNotFound", err)
	}
}

// --- Pay ---

func TestService_Pay_AuthorizesAndConfirms(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusPendingPayment, student, uuid.New())
	gw := newFakeGateway(repo)
	gw.status[id] = "requires_payment"

	b, snap, err := newServiceWithGateway(repo, gw).Pay(ctx(), student, id, "pm_ok")
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if b.Status != StatusConfirmed {
		t.Errorf("status = %s, want confirmed", b.Status)
	}
	if snap == nil || snap.Status != "authorized" {
		t.Errorf("snapshot = %+v", snap)
	}
}

func TestService_Pay_StudentOnly(t *testing.T) {
	repo := newFakeRepo()
	id := seedBooking(repo, StatusPendingPayment, uuid.New(), uuid.New())
	gw := newFakeGateway(repo)
	if _, _, err := newServiceWithGateway(repo, gw).Pay(ctx(), uuid.New(), id, "pm_ok"); !errors.Is(err, ErrNotStudent) {
		t.Errorf("err = %v, want ErrNotStudent", err)
	}
}

func TestService_Pay_Declined(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusPendingPayment, student, uuid.New())
	gw := newFakeGateway(repo)
	gw.authErr = PaymentFailedError{Reason: "the card was declined"}

	_, _, err := newServiceWithGateway(repo, gw).Pay(ctx(), student, id, "pm_decline")
	var pf PaymentFailedError
	if !errors.As(err, &pf) {
		t.Fatalf("err = %v, want PaymentFailedError", err)
	}
	if repo.store[id].Status != StatusPendingPayment {
		t.Errorf("booking moved off pending_payment on decline: %s", repo.store[id].Status)
	}
}

func TestService_Pay_DoublePay(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, student, uuid.New())
	gw := newFakeGateway(repo)
	if _, _, err := newServiceWithGateway(repo, gw).Pay(ctx(), student, id, "pm_ok"); !errors.Is(err, ErrAlreadyPaid) {
		t.Errorf("err = %v, want ErrAlreadyPaid", err)
	}
}

// --- Complete ---

func TestService_Complete_TooEarly(t *testing.T) {
	owner := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, uuid.New(), owner) // end_at = fixedNow + 49h
	gw := newFakeGateway(repo)
	if _, _, err := newServiceWithGateway(repo, gw).Complete(ctx(), owner, id); !errors.Is(err, ErrTooEarly) {
		t.Errorf("err = %v, want ErrTooEarly", err)
	}
}

func TestService_Complete_TeacherOwnerOnly(t *testing.T) {
	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, uuid.New(), uuid.New())
	gw := newFakeGateway(repo)
	if _, _, err := newServiceWithGateway(repo, gw).Complete(ctx(), uuid.New(), id); !errors.Is(err, ErrNotTeacherOwner) {
		t.Errorf("err = %v, want ErrNotTeacherOwner", err)
	}
}

func TestService_Complete_CapturesAndWritesLedger(t *testing.T) {
	owner := uuid.New()
	repo := newFakeRepo()
	id := uuid.New()
	repo.store[id] = Booking{
		ID:             id,
		Status:         StatusConfirmed,
		StartAt:        fixedNow.Add(-2 * time.Hour),
		EndAt:          fixedNow.Add(-time.Hour), // already ended
		Student:        StudentSummary{ID: uuid.New(), DisplayName: "Student"},
		TeacherOwnerID: owner,
	}
	gw := newFakeGateway(repo)
	gw.status[id] = "authorized"

	b, snap, err := newServiceWithGateway(repo, gw).Complete(ctx(), owner, id)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if b.Status != StatusCompleted {
		t.Errorf("status = %s, want completed", b.Status)
	}
	if len(gw.captured) != 1 || gw.captured[0] != id {
		t.Errorf("capture not called: %+v", gw.captured)
	}
	if snap == nil || snap.Status != "captured" {
		t.Errorf("snapshot = %+v", snap)
	}
}

func TestService_Complete_CaptureFailureLeavesConfirmed(t *testing.T) {
	owner := uuid.New()
	repo := newFakeRepo()
	id := uuid.New()
	repo.store[id] = Booking{
		ID: id, Status: StatusConfirmed,
		StartAt: fixedNow.Add(-2 * time.Hour), EndAt: fixedNow.Add(-time.Hour),
		Student:        StudentSummary{ID: uuid.New()},
		TeacherOwnerID: owner,
	}
	gw := newFakeGateway(repo)
	gw.captureErr = ErrCaptureFailed

	if _, _, err := newServiceWithGateway(repo, gw).Complete(ctx(), owner, id); !errors.Is(err, ErrCaptureFailed) {
		t.Fatalf("err = %v, want ErrCaptureFailed", err)
	}
	if repo.store[id].Status != StatusConfirmed {
		t.Errorf("status = %s, want still confirmed", repo.store[id].Status)
	}
}

func TestService_Cancel(t *testing.T) {
	student := uuid.New()
	owner := uuid.New()

	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, student, owner)
	b, err := newService(repo).Cancel(ctx(), student, id, "  changed my mind  ")
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if b.Status != StatusCancelled || b.CancelledAt == nil {
		t.Errorf("not cancelled: %+v", b)
	}
	if b.CancellationReason != "changed my mind" {
		t.Errorf("reason = %q, want trimmed", b.CancellationReason)
	}

	// cancelling again -> already cancelled
	if _, err := newService(repo).Cancel(ctx(), student, id, ""); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("err = %v, want ErrInvalidTransition", err)
	}
}

func TestService_Cancel_Completed(t *testing.T) {
	repo := newFakeRepo()
	id := seedBooking(repo, StatusCompleted, uuid.New(), uuid.New())
	b := repo.store[id]
	b.Student.ID = uuid.New()
	// use a real participant
	student := b.Student.ID
	repo.store[id] = b
	if _, err := newService(repo).Cancel(ctx(), student, id, ""); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("err = %v, want ErrInvalidTransition", err)
	}
}

func TestService_Cancel_RefundsAuthorizedPayment(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, student, uuid.New())
	gw := newFakeGateway(repo)
	gw.status[id] = "authorized"

	b, err := newServiceWithGateway(repo, gw).Cancel(ctx(), student, id, "changed my mind")
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if b.Status != StatusCancelled {
		t.Errorf("status = %s, want cancelled", b.Status)
	}
	if len(gw.refunded) != 1 || gw.refunded[0] != id {
		t.Errorf("refund not issued: %+v", gw.refunded)
	}
}

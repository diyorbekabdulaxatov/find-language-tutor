package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
)

// fakeDisputeReader is an in-memory bookings.DisputeReader.
type fakeDisputeReader struct {
	byBooking map[uuid.UUID]*BookingDispute
	err       error
}

func newFakeDisputeReader() *fakeDisputeReader {
	return &fakeDisputeReader{byBooking: map[uuid.UUID]*BookingDispute{}}
}

func (f *fakeDisputeReader) OpenForBooking(_ context.Context, id uuid.UUID) (*BookingDispute, bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	d, ok := f.byBooking[id]
	return d, ok, nil
}

// newTestRouterDisputes mounts the booking routes with a wired DisputeReader.
func newTestRouterDisputes(repo Repository, tm *auth.TokenManager, gw PaymentGateway, dr DisputeReader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := &Service{repo: repo, now: func() time.Time { return fixedNow }, logger: discardLogger()}
	svc.payments = gw
	svc.disputes = dr
	h := NewHandler(svc, discardLogger())
	RegisterRoutes(r.Group("/v1/bookings"), h, auth.RequireAuth(tm))
	return r
}

// --- withDispute pure logic ---

func TestWithDispute_CanRaiseDisputeTransitions(t *testing.T) {
	student, owner, stranger := uuid.New(), uuid.New(), uuid.New()
	b := Booking{Status: StatusCompleted, Student: StudentSummary{ID: student}, TeacherOwnerID: owner}

	for _, viewer := range []uuid.UUID{student, owner} {
		if !withDispute(bookingDTO{}, b, viewer, nil).CanRaiseDispute {
			t.Errorf("participant %s should be able to raise a dispute", viewer)
		}
	}
	if withDispute(bookingDTO{}, b, stranger, nil).CanRaiseDispute {
		t.Error("a non-participant must never be offered the dispute action")
	}
	if withDispute(bookingDTO{}, b, student, &BookingDispute{Status: "open"}).CanRaiseDispute {
		t.Error("an already-open dispute must block a second one")
	}

	b.Status = StatusConfirmed
	if !withDispute(bookingDTO{}, b, student, nil).CanRaiseDispute {
		t.Error("a confirmed lesson is disputable")
	}
	for _, s := range []Status{StatusPendingPayment, StatusCancelled} {
		b.Status = s
		if withDispute(bookingDTO{}, b, student, nil).CanRaiseDispute {
			t.Errorf("%s must not be disputable", s)
		}
	}
}

func TestWithDispute_EmbedsOpenDispute(t *testing.T) {
	student := uuid.New()
	b := Booking{Status: StatusCompleted, Student: StudentSummary{ID: student}}
	d := &BookingDispute{ID: uuid.New(), Status: "open", Reason: "ended early", CreatedAt: fixedNow}

	dto := withDispute(bookingDTO{}, b, student, d)
	if dto.OpenDispute == nil || dto.OpenDispute.Reason != "ended early" ||
		dto.OpenDispute.ID != d.ID.String() || dto.OpenDispute.Status != "open" {
		t.Fatalf("open dispute not embedded: %+v", dto.OpenDispute)
	}
	if !dto.OpenDispute.CreatedAt.Equal(fixedNow) {
		t.Fatalf("created_at = %v", dto.OpenDispute.CreatedAt)
	}
}

// --- handler wiring ---

func TestHandler_Get_OpenDisputeVisibleToBothParticipants(t *testing.T) {
	student, owner := uuid.New(), uuid.New()
	repo := newFakeRepo()
	id := completedBooking(repo, student, owner)
	dr := newFakeDisputeReader()
	dr.byBooking[id] = &BookingDispute{ID: uuid.New(), Status: "open", Reason: "no show", CreatedAt: fixedNow}
	tm := testTokenManager()
	r := newTestRouterDisputes(repo, tm, newFakeGateway(repo), dr)

	for _, viewer := range []uuid.UUID{student, owner} {
		w := do(r, http.MethodGet, "/v1/bookings/"+id.String(), "", bearerFor(tm, viewer))
		if w.Code != http.StatusOK {
			t.Fatalf("viewer %s: status = %d (%s)", viewer, w.Code, w.Body.String())
		}
		var body bookingDTO
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body.OpenDispute == nil || body.OpenDispute.Reason != "no show" {
			t.Errorf("viewer %s: open_dispute missing: %+v", viewer, body.OpenDispute)
		}
		if body.CanRaiseDispute {
			t.Errorf("viewer %s: can_raise_dispute must be false while one is open", viewer)
		}
	}
}

func TestHandler_List_EmbedsDisputeFields(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	disputed, clean, pending := uuid.New(), uuid.New(), uuid.New()
	repo.listResult = []Booking{
		{ID: disputed, Status: StatusCompleted, StartAt: fixedNow, EndAt: fixedNow.Add(time.Hour),
			Student: StudentSummary{ID: student}},
		{ID: clean, Status: StatusConfirmed, StartAt: fixedNow, EndAt: fixedNow.Add(time.Hour),
			Student: StudentSummary{ID: student}},
		{ID: pending, Status: StatusPendingPayment, StartAt: fixedNow, EndAt: fixedNow.Add(time.Hour),
			Student: StudentSummary{ID: student}},
	}
	dr := newFakeDisputeReader()
	dr.byBooking[disputed] = &BookingDispute{ID: uuid.New(), Status: "open", Reason: "x", CreatedAt: fixedNow}
	tm := testTokenManager()
	r := newTestRouterDisputes(repo, tm, nil, dr)

	w := do(r, http.MethodGet, "/v1/bookings?role=student", "", bearerFor(tm, student))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body bookingListDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Bookings) != 3 {
		t.Fatalf("got %d bookings", len(body.Bookings))
	}
	if body.Bookings[0].OpenDispute == nil || body.Bookings[0].CanRaiseDispute {
		t.Errorf("disputed booking: %+v", body.Bookings[0])
	}
	if body.Bookings[1].OpenDispute != nil || !body.Bookings[1].CanRaiseDispute {
		t.Errorf("clean confirmed booking: %+v", body.Bookings[1])
	}
	if body.Bookings[2].CanRaiseDispute {
		t.Errorf("pending_payment booking must not be disputable")
	}
}

// A nil DisputeReader (cmd/api without the disputes module, unit tests) is a
// safe no-op, and a lookup error must not fail the booking read.
func TestHandler_Get_DisputeReaderNilAndErrorAreSafe(t *testing.T) {
	student, owner := uuid.New(), uuid.New()
	repo := newFakeRepo()
	id := completedBooking(repo, student, owner)
	tm := testTokenManager()

	for name, dr := range map[string]DisputeReader{
		"nil":   nil,
		"error": &fakeDisputeReader{err: context.DeadlineExceeded},
	} {
		r := newTestRouterDisputes(repo, tm, newFakeGateway(repo), dr)
		w := do(r, http.MethodGet, "/v1/bookings/"+id.String(), "", bearerFor(tm, student))
		if w.Code != http.StatusOK {
			t.Fatalf("%s reader: status = %d, want 200", name, w.Code)
		}
		var body bookingDTO
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body.OpenDispute != nil {
			t.Errorf("%s reader: open_dispute should be nil", name)
		}
		if !body.CanRaiseDispute {
			t.Errorf("%s reader: can_raise_dispute should still be computed from state", name)
		}
	}
}

// --- admin force-cancel / refund on the service ---

func storedConfirmedBooking(repo *fakeRepo, student, owner uuid.UUID) uuid.UUID {
	id := uuid.New()
	repo.store[id] = Booking{
		ID:             id,
		Status:         StatusConfirmed,
		StartAt:        fixedNow.Add(24 * time.Hour),
		EndAt:          fixedNow.Add(25 * time.Hour),
		Price:          Money{AmountMinor: 9_000_000, Currency: "UZS"},
		Student:        StudentSummary{ID: student, DisplayName: "Student"},
		TeacherOwnerID: owner,
	}
	return id
}

func TestAdminForceCancel_ReusesTheCancelPath(t *testing.T) {
	repo := newFakeRepo()
	student, owner := uuid.New(), uuid.New()
	id := storedConfirmedBooking(repo, student, owner)
	gw := newFakeGateway(repo)
	svc := newServiceWithGateway(repo, gw)
	sched := &fakeScheduler{}
	svc.SetReminderScheduler(sched)
	notifier := newFakeNotifier()
	svc.SetNotifier(notifier)

	if err := svc.AdminForceCancel(context.Background(), id, "  chargeback  ", true); err != nil {
		t.Fatalf("force cancel: %v", err)
	}

	b := repo.store[id]
	if b.Status != StatusCancelled {
		t.Fatalf("status = %q", b.Status)
	}
	if b.CancelledBy != CancelledByAdmin {
		t.Fatalf("cancelled_by = %q, want admin", b.CancelledBy)
	}
	if b.CancellationReason != "chargeback" {
		t.Fatalf("reason = %q (should be trimmed)", b.CancellationReason)
	}
	if b.CancelledAt == nil {
		t.Fatal("cancelled_at not set")
	}
	// The shared path refunds, drops the reminders and mails the participants.
	if len(gw.refunded) != 1 || gw.refunded[0] != id {
		t.Fatalf("refunds: %+v", gw.refunded)
	}
	if len(sched.cancelled) != 1 || sched.cancelled[0] != id {
		t.Fatalf("reminders not cancelled: %+v", sched.cancelled)
	}
	if len(notifier.cancelled) != 1 || !notifier.refunded[id] {
		t.Fatalf("cancellation mail: %+v refunded=%v", notifier.cancelled, notifier.refunded)
	}
}

func TestAdminForceCancel_WithoutRefund(t *testing.T) {
	repo := newFakeRepo()
	id := storedConfirmedBooking(repo, uuid.New(), uuid.New())
	gw := newFakeGateway(repo)
	svc := newServiceWithGateway(repo, gw)
	notifier := newFakeNotifier()
	svc.SetNotifier(notifier)

	if err := svc.AdminForceCancel(context.Background(), id, "settled offline", false); err != nil {
		t.Fatalf("force cancel: %v", err)
	}
	if len(gw.refunded) != 0 {
		t.Fatalf("refund:false still refunded: %+v", gw.refunded)
	}
	if notifier.refunded[id] {
		t.Fatal("the cancellation mail claimed a refund that never happened")
	}
	if repo.store[id].Status != StatusCancelled {
		t.Fatal("booking not cancelled")
	}
}

func TestAdminForceCancel_StateAndNotFound(t *testing.T) {
	repo := newFakeRepo()
	student, owner := uuid.New(), uuid.New()
	svc := newServiceWithGateway(repo, newFakeGateway(repo))

	completed := completedBooking(repo, student, owner)
	if err := svc.AdminForceCancel(context.Background(), completed, "x", true); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("completed booking: %v, want ErrInvalidTransition", err)
	}

	cancelled := storedConfirmedBooking(repo, student, owner)
	if err := svc.AdminForceCancel(context.Background(), cancelled, "first", false); err != nil {
		t.Fatalf("first cancel: %v", err)
	}
	if err := svc.AdminForceCancel(context.Background(), cancelled, "again", false); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("second cancel: %v, want ErrInvalidTransition", err)
	}

	if err := svc.AdminForceCancel(context.Background(), uuid.New(), "x", true); !errors.Is(err, ErrBookingNotFound) {
		t.Fatalf("unknown booking: %v, want ErrBookingNotFound", err)
	}
}

// A pending_payment booking can be force-cancelled too (no money moved yet).
func TestAdminForceCancel_PendingPayment(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.store[id] = Booking{ID: id, Status: StatusPendingPayment, StartAt: fixedNow.Add(time.Hour),
		EndAt: fixedNow.Add(2 * time.Hour), Student: StudentSummary{ID: uuid.New()}}
	svc := newServiceWithGateway(repo, newFakeGateway(repo))

	if err := svc.AdminForceCancel(context.Background(), id, "spam", true); err != nil {
		t.Fatalf("force cancel: %v", err)
	}
	if repo.store[id].Status != StatusCancelled || repo.store[id].CancelledBy != CancelledByAdmin {
		t.Fatalf("booking: %+v", repo.store[id])
	}
}

func TestAdminRefund_RefundsWithoutTouchingTheBooking(t *testing.T) {
	repo := newFakeRepo()
	student, owner := uuid.New(), uuid.New()
	id := completedBooking(repo, student, owner)
	gw := newFakeGateway(repo)
	svc := newServiceWithGateway(repo, gw)

	if err := svc.AdminRefund(context.Background(), id); err != nil {
		t.Fatalf("admin refund: %v", err)
	}
	if len(gw.refunded) != 1 || gw.refunded[0] != id {
		t.Fatalf("refunds: %+v", gw.refunded)
	}
	if repo.store[id].Status != StatusCompleted {
		t.Fatalf("the booking lifecycle moved: %q", repo.store[id].Status)
	}

	if err := svc.AdminRefund(context.Background(), uuid.New()); !errors.Is(err, ErrBookingNotFound) {
		t.Fatalf("unknown booking: %v, want ErrBookingNotFound", err)
	}
}

// Without a payment gateway AdminRefund is a logged no-op rather than an error,
// so a dispute resolution never fails on a deployment without payments wired.
func TestAdminRefund_NoGatewayIsANoOp(t *testing.T) {
	repo := newFakeRepo()
	id := completedBooking(repo, uuid.New(), uuid.New())
	svc := newService(repo)

	if err := svc.AdminRefund(context.Background(), id); err != nil {
		t.Fatalf("admin refund without a gateway: %v", err)
	}
}

// The participant cancel path records who cancelled, so `admin` stays a
// meaningful signal.
func TestCancel_RecordsTheActingParticipant(t *testing.T) {
	repo := newFakeRepo()
	student, owner := uuid.New(), uuid.New()
	svc := newServiceWithGateway(repo, newFakeGateway(repo))

	byStudent := storedConfirmedBooking(repo, student, owner)
	if _, err := svc.Cancel(context.Background(), student, byStudent, "clash"); err != nil {
		t.Fatalf("student cancel: %v", err)
	}
	if got := repo.store[byStudent].CancelledBy; got != CancelledByStudent {
		t.Fatalf("cancelled_by = %q, want student", got)
	}

	byTeacher := storedConfirmedBooking(repo, student, owner)
	if _, err := svc.Cancel(context.Background(), owner, byTeacher, "ill"); err != nil {
		t.Fatalf("teacher cancel: %v", err)
	}
	if got := repo.store[byTeacher].CancelledBy; got != CancelledByTeacher {
		t.Fatalf("cancelled_by = %q, want teacher", got)
	}
}

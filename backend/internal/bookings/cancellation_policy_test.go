package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The cancellation policy: free until 24h before the start, then a student
// cancellation forfeits the fee (captured, paid to the teacher); a teacher
// cancellation always refunds; an unpaid booking is simply dropped.

// paidBookingStartingIn stores a confirmed (authorized) booking that starts
// `in` from the frozen clock.
func paidBookingStartingIn(repo *fakeRepo, student, owner uuid.UUID, in time.Duration) uuid.UUID {
	id := uuid.New()
	repo.store[id] = Booking{
		ID:             id,
		Status:         StatusConfirmed,
		StartAt:        fixedNow.Add(in),
		EndAt:          fixedNow.Add(in + time.Hour),
		Price:          Money{AmountMinor: 9_000_000, Currency: "UZS"},
		Student:        StudentSummary{ID: student, DisplayName: "Student"},
		TeacherOwnerID: owner,
	}
	return id
}

func TestCancel_StudentBeforeTheDeadlineIsRefunded(t *testing.T) {
	repo := newFakeRepo()
	gw := newFakeGateway(repo)
	notifier := newFakeNotifier()
	svc := newServiceWithGateway(repo, gw)
	svc.SetNotifier(notifier)
	student, owner := uuid.New(), uuid.New()

	id := paidBookingStartingIn(repo, student, owner, 25*time.Hour)
	b, err := svc.Cancel(context.Background(), student, id, "clash", false)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if b.CancellationOutcome != OutcomeRefunded {
		t.Fatalf("outcome = %q, want refunded", b.CancellationOutcome)
	}
	if len(gw.refunded) != 1 || len(gw.captured) != 0 {
		t.Fatalf("gateway: refunded=%v captured=%v, want one refund and no capture", gw.refunded, gw.captured)
	}
	if notifier.outcomes[id] != OutcomeRefunded {
		t.Fatalf("notifier told %q, want refunded", notifier.outcomes[id])
	}
}

func TestCancel_StudentInsideTheWindowMustAcknowledgeTheForfeit(t *testing.T) {
	repo := newFakeRepo()
	gw := newFakeGateway(repo)
	svc := newServiceWithGateway(repo, gw)
	student, owner := uuid.New(), uuid.New()

	id := paidBookingStartingIn(repo, student, owner, 23*time.Hour)
	_, err := svc.Cancel(context.Background(), student, id, "", false)
	var late LateCancellationError
	if !errors.As(err, &late) {
		t.Fatalf("err = %v, want LateCancellationError", err)
	}
	if !late.Policy.Late || !late.Policy.FreeCancelUntil.Equal(fixedNow.Add(-time.Hour)) {
		t.Fatalf("policy = %+v", late.Policy)
	}
	// Nothing happened: still confirmed, no money moved.
	if repo.store[id].Status != StatusConfirmed || len(gw.captured)+len(gw.refunded) != 0 {
		t.Fatalf("refused cancel must be a no-op: status=%s captured=%v refunded=%v", repo.store[id].Status, gw.captured, gw.refunded)
	}
}

func TestCancel_StudentInsideTheWindowForfeitsToTheTeacher(t *testing.T) {
	repo := newFakeRepo()
	gw := newFakeGateway(repo)
	notifier := newFakeNotifier()
	svc := newServiceWithGateway(repo, gw)
	svc.SetNotifier(notifier)
	student, owner := uuid.New(), uuid.New()

	id := paidBookingStartingIn(repo, student, owner, 23*time.Hour)
	b, err := svc.Cancel(context.Background(), student, id, "overslept", true)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if b.Status != StatusCancelled || b.CancellationOutcome != OutcomeForfeited || b.CancelledBy != CancelledByStudent {
		t.Fatalf("booking = status %s outcome %q by %q", b.Status, b.CancellationOutcome, b.CancelledBy)
	}
	// The fee is captured — the payout ledger row comes from the capture
	// webhook, exactly as for a completed lesson — and nothing is refunded.
	if len(gw.captured) != 1 || len(gw.refunded) != 0 {
		t.Fatalf("gateway: captured=%v refunded=%v", gw.captured, gw.refunded)
	}
	if notifier.outcomes[id] != OutcomeForfeited {
		t.Fatalf("notifier told %q, want forfeited", notifier.outcomes[id])
	}
}

func TestCancel_ForfeitCaptureFailureLeavesTheBookingConfirmed(t *testing.T) {
	repo := newFakeRepo()
	gw := newFakeGateway(repo)
	gw.captureErr = ErrCaptureFailed
	svc := newServiceWithGateway(repo, gw)
	student, owner := uuid.New(), uuid.New()

	id := paidBookingStartingIn(repo, student, owner, time.Hour)
	if _, err := svc.Cancel(context.Background(), student, id, "", true); !errors.Is(err, ErrCaptureFailed) {
		t.Fatalf("err = %v, want ErrCaptureFailed", err)
	}
	if repo.store[id].Status != StatusConfirmed {
		t.Fatalf("status = %s, want confirmed (retryable)", repo.store[id].Status)
	}
}

func TestCancel_TeacherInsideTheWindowStillRefunds(t *testing.T) {
	repo := newFakeRepo()
	gw := newFakeGateway(repo)
	svc := newServiceWithGateway(repo, gw)
	student, owner := uuid.New(), uuid.New()

	id := paidBookingStartingIn(repo, student, owner, time.Hour)
	b, err := svc.Cancel(context.Background(), owner, id, "ill", false)
	if err != nil {
		t.Fatalf("teacher cancel: %v", err)
	}
	if b.CancellationOutcome != OutcomeRefunded || len(gw.refunded) != 1 || len(gw.captured) != 0 {
		t.Fatalf("outcome=%q refunded=%v captured=%v", b.CancellationOutcome, gw.refunded, gw.captured)
	}
}

func TestCancel_UnpaidBookingIsDroppedWhateverTheTime(t *testing.T) {
	repo := newFakeRepo()
	gw := newFakeGateway(repo)
	svc := newServiceWithGateway(repo, gw)
	student, owner := uuid.New(), uuid.New()

	id := seedBooking(repo, StatusPendingPayment, student, owner)
	repo.store[id] = func(b Booking) Booking {
		b.StartAt = fixedNow.Add(time.Hour)
		b.EndAt = fixedNow.Add(2 * time.Hour)
		return b
	}(repo.store[id])
	b, err := svc.Cancel(context.Background(), student, id, "", false)
	if err != nil {
		t.Fatalf("cancel unpaid: %v", err)
	}
	if b.CancellationOutcome != OutcomeUnpaid || len(gw.captured) != 0 {
		t.Fatalf("outcome=%q captured=%v", b.CancellationOutcome, gw.captured)
	}
}

func TestCancel_WindowIsConfigurable(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithGateway(repo, newFakeGateway(repo))
	svc.SetFreeCancelWindow(2 * time.Hour)
	student, owner := uuid.New(), uuid.New()

	id := paidBookingStartingIn(repo, student, owner, 3*time.Hour)
	if _, err := svc.Cancel(context.Background(), student, id, "", false); err != nil {
		t.Fatalf("3h before with a 2h window should be free: %v", err)
	}
	id = paidBookingStartingIn(repo, student, owner, 90*time.Minute)
	var late LateCancellationError
	if _, err := svc.Cancel(context.Background(), student, id, "", false); !errors.As(err, &late) {
		t.Fatalf("90m before with a 2h window: err = %v, want late", err)
	}
}

// The wire contract: the policy rides on every cancellable booking, the
// forfeit needs acknowledge_forfeit, and the refusal is 409 late_cancellation.
func TestHandler_CancellationPolicyOnTheWire(t *testing.T) {
	repo := newFakeRepo()
	tm := testTokenManager()
	student, owner := uuid.New(), uuid.New()
	id := paidBookingStartingIn(repo, student, owner, 23*time.Hour)
	r := newTestRouterGW(repo, tm, newFakeGateway(repo))

	w := do(r, http.MethodGet, "/v1/bookings/"+id.String(), "", bearerFor(tm, student))
	if w.Code != http.StatusOK {
		t.Fatalf("get: %d %s", w.Code, w.Body.String())
	}
	var dto bookingDTO
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto.CancellationPolicy == nil || !dto.CancellationPolicy.Late || !dto.CancellationPolicy.FreeCancelUntil.Equal(fixedNow.Add(-time.Hour)) {
		t.Fatalf("policy on GET = %+v", dto.CancellationPolicy)
	}

	w = do(r, http.MethodPost, "/v1/bookings/"+id.String()+"/cancel", `{"reason":"x"}`, bearerFor(tm, student))
	if w.Code != http.StatusConflict || errCode(t, w) != "late_cancellation" {
		t.Fatalf("late cancel without ack: %d %s", w.Code, w.Body.String())
	}

	w = do(r, http.MethodPost, "/v1/bookings/"+id.String()+"/cancel", `{"reason":"x","acknowledge_forfeit":true}`, bearerFor(tm, student))
	if w.Code != http.StatusOK {
		t.Fatalf("late cancel with ack: %d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto.CancellationOutcome == nil || *dto.CancellationOutcome != "forfeited" || dto.CancellationPolicy != nil {
		t.Fatalf("cancelled dto: outcome=%v policy=%v", dto.CancellationOutcome, dto.CancellationPolicy)
	}
}

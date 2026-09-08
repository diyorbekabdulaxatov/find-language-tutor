package payments

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestService_Authorize_OK(t *testing.T) {
	s, repo := newTestService()
	bid := uuid.New()
	repo.seedPayment(bid, StatusRequiresPayment, 9_000_000, "UZS")

	p, err := s.Authorize(ctx(), bid, "pm_ok")
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if p.Status != StatusAuthorized {
		t.Errorf("status = %s, want authorized", p.Status)
	}
	if p.ProviderRef == "" {
		t.Error("provider_ref not set")
	}
	if !repo.bookingConfirmed[bid] {
		t.Error("booking not confirmed by the authorize webhook")
	}
}

func TestService_Authorize_Declined(t *testing.T) {
	s, repo := newTestService()
	bid := uuid.New()
	repo.seedPayment(bid, StatusRequiresPayment, 9_000_000, "UZS")

	_, err := s.Authorize(ctx(), bid, "pm_decline")
	var d PaymentDeclined
	if !errors.As(err, &d) {
		t.Fatalf("err = %v, want PaymentDeclined", err)
	}
	got, _ := repo.PaymentByBooking(ctx(), bid)
	if got.Status != StatusFailed {
		t.Errorf("status = %s, want failed", got.Status)
	}
	if repo.bookingConfirmed[bid] {
		t.Error("booking confirmed despite decline")
	}
}

func TestService_Authorize_DoublePay(t *testing.T) {
	s, repo := newTestService()
	bid := uuid.New()
	repo.seedPayment(bid, StatusAuthorized, 9_000_000, "UZS")

	if _, err := s.Authorize(ctx(), bid, "pm_ok"); !errors.Is(err, ErrAlreadyPaid) {
		t.Fatalf("err = %v, want ErrAlreadyPaid", err)
	}
}

func TestService_Capture_WritesLedger(t *testing.T) {
	s, repo := newTestService()
	bid := uuid.New()
	repo.seedPayment(bid, StatusRequiresPayment, 9_000_000, "UZS")

	if _, err := s.Authorize(ctx(), bid, "pm_ok"); err != nil {
		t.Fatalf("authorize: %v", err)
	}
	p, err := s.Capture(ctx(), bid)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if p.Status != StatusCaptured {
		t.Errorf("status = %s, want captured", p.Status)
	}
	if repo.ledger[bid] != LedgerAvailable {
		t.Errorf("ledger[%s] = %q, want available", bid, repo.ledger[bid])
	}
}

func TestService_Capture_TransientFailure(t *testing.T) {
	s, repo := newTestService()
	bid := uuid.New()
	repo.seedPayment(bid, StatusRequiresPayment, 9_000_000, "UZS")
	if _, err := s.Authorize(ctx(), bid, "pm_capture_fail"); err != nil {
		t.Fatalf("authorize: %v", err)
	}

	if _, err := s.Capture(ctx(), bid); !errors.Is(err, ErrCaptureFailed) {
		t.Fatalf("first capture err = %v, want ErrCaptureFailed", err)
	}
	// retry succeeds
	if _, err := s.Capture(ctx(), bid); err != nil {
		t.Fatalf("retry capture: %v", err)
	}
	if repo.ledger[bid] != LedgerAvailable {
		t.Errorf("ledger not written after retry: %q", repo.ledger[bid])
	}
}

func TestService_Refund_ReversesLedger(t *testing.T) {
	s, repo := newTestService()
	bid := uuid.New()
	repo.seedPayment(bid, StatusRequiresPayment, 9_000_000, "UZS")
	if _, err := s.Authorize(ctx(), bid, "pm_ok"); err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if _, err := s.Capture(ctx(), bid); err != nil {
		t.Fatalf("capture: %v", err)
	}

	p, err := s.Refund(ctx(), bid)
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	if p.Status != StatusRefunded {
		t.Errorf("status = %s, want refunded", p.Status)
	}
	if repo.ledger[bid] != LedgerReversed {
		t.Errorf("ledger[%s] = %q, want reversed", bid, repo.ledger[bid])
	}
}

func TestService_Refund_VoidsUnauthorizedIntent(t *testing.T) {
	s, repo := newTestService()
	bid := uuid.New()
	repo.seedPayment(bid, StatusRequiresPayment, 9_000_000, "UZS")

	p, err := s.Refund(ctx(), bid)
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	if p.Status != StatusFailed {
		t.Errorf("status = %s, want failed (voided)", p.Status)
	}
}

func TestService_HandleWebhook_Idempotent(t *testing.T) {
	s, repo := newTestService()
	bid := uuid.New()
	pid := repo.seedPayment(bid, StatusRequiresPayment, 9_000_000, "UZS")

	evt := Event{ID: "evt_fixed_1", Type: EventAuthorized, PaymentID: pid, ProviderRef: "ref_1"}

	applied, err := s.HandleWebhook(ctx(), evt)
	if err != nil || !applied {
		t.Fatalf("first: applied=%v err=%v, want true nil", applied, err)
	}
	applied, err = s.HandleWebhook(ctx(), evt)
	if err != nil || applied {
		t.Fatalf("replay: applied=%v err=%v, want false nil", applied, err)
	}
	if len(repo.applied) != 1 {
		t.Errorf("effect ran %d times, want exactly 1: %v", len(repo.applied), repo.applied)
	}
}

func TestService_Earnings_Math(t *testing.T) {
	s, repo := newTestService()
	owner := uuid.New()
	tid := uuid.New()
	repo.teacherByOwner[owner] = tid
	repo.earnings[tid] = []EarningLine{
		{BookingID: uuid.New(), AmountMinor: 5_000_000, Currency: "UZS", State: LedgerHeld},
		{BookingID: uuid.New(), AmountMinor: 7_000_000, Currency: "UZS", State: LedgerAvailable},
		{BookingID: uuid.New(), AmountMinor: 9_000_000, Currency: "UZS", State: LedgerAvailable},
		{BookingID: uuid.New(), AmountMinor: 3_000_000, Currency: "UZS", State: LedgerReversed},
	}

	e, err := s.Earnings(ctx(), owner)
	if err != nil {
		t.Fatalf("earnings: %v", err)
	}
	if e.HeldMinor != 5_000_000 {
		t.Errorf("held = %d, want 5_000_000", e.HeldMinor)
	}
	if e.AvailableMinor != 16_000_000 {
		t.Errorf("available = %d, want 16_000_000", e.AvailableMinor)
	}
	if e.TotalEarnedMinor != 21_000_000 {
		t.Errorf("total = %d, want 21_000_000 (reversed excluded)", e.TotalEarnedMinor)
	}
	if e.Currency != "UZS" || len(e.Lines) != 4 {
		t.Errorf("currency=%q lines=%d", e.Currency, len(e.Lines))
	}
}

func TestService_Earnings_NoProfile(t *testing.T) {
	s, _ := newTestService()
	if _, err := s.Earnings(ctx(), uuid.New()); !errors.Is(err, ErrNoTeacherProfile) {
		t.Fatalf("err = %v, want ErrNoTeacherProfile", err)
	}
}

func TestService_Earnings_EmptyUsesDefaultCurrency(t *testing.T) {
	s, repo := newTestService()
	owner := uuid.New()
	repo.teacherByOwner[owner] = uuid.New()

	e, err := s.Earnings(ctx(), owner)
	if err != nil {
		t.Fatalf("earnings: %v", err)
	}
	if e.Currency != defaultCurrency || e.TotalEarnedMinor != 0 {
		t.Errorf("got %+v", e)
	}
}

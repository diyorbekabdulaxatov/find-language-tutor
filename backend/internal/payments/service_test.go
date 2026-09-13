package payments

import (
	"errors"
	"testing"
	"time"

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
	if repo.ledger[bid] != LedgerHeld {
		t.Errorf("ledger[%s] = %q, want held (the clearing window just opened)", bid, repo.ledger[bid])
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
	if repo.ledger[bid] != LedgerHeld {
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

// TestService_Earnings_Math covers the clearing-window split: a `held` row is
// only counted as held while its available_at deadline is in the future, and it
// reads as available once the deadline has passed. `paid` lands in its own
// bucket but still counts as earned; `reversed` counts nowhere.
func TestService_Earnings_Math(t *testing.T) {
	s, repo := newTestService()
	owner := uuid.New()
	tid := uuid.New()
	repo.teacherByOwner[owner] = tid

	future := time.Now().UTC().Add(72 * time.Hour) // still inside the window
	past := time.Now().UTC().Add(-24 * time.Hour)  // cleared
	repo.earnings[tid] = []EarningLine{
		{BookingID: uuidPtr(uuid.New()), AmountMinor: 5_000_000, Currency: "UZS", State: LedgerHeld, AvailableAt: future},
		{BookingID: uuidPtr(uuid.New()), AmountMinor: 7_000_000, Currency: "UZS", State: LedgerHeld, AvailableAt: past},
		// A pre-000011 row: stored `available`, read exactly like a cleared one.
		{BookingID: uuidPtr(uuid.New()), AmountMinor: 9_000_000, Currency: "UZS", State: LedgerAvailable, AvailableAt: past},
		{BookingID: uuidPtr(uuid.New()), AmountMinor: 4_000_000, Currency: "UZS", State: LedgerPaid, AvailableAt: past},
		{BookingID: uuidPtr(uuid.New()), AmountMinor: 3_000_000, Currency: "UZS", State: LedgerReversed, AvailableAt: past},
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
	if e.PaidMinor != 4_000_000 {
		t.Errorf("paid = %d, want 4_000_000", e.PaidMinor)
	}
	if e.TotalEarnedMinor != 25_000_000 {
		t.Errorf("total = %d, want 25_000_000 (reversed excluded)", e.TotalEarnedMinor)
	}
	if e.Currency != "UZS" || len(e.Lines) != 5 {
		t.Errorf("currency=%q lines=%d", e.Currency, len(e.Lines))
	}
	// The lines carry the EFFECTIVE state, not the stored one.
	if e.Lines[1].State != LedgerAvailable {
		t.Errorf("cleared held line state = %q, want available", e.Lines[1].State)
	}
	if e.Lines[0].State != LedgerHeld {
		t.Errorf("uncleared line state = %q, want held", e.Lines[0].State)
	}
}

// TestService_Earnings_MixesBookingAndCourseLines covers phase C3: a
// teacher's earnings summary aggregates lesson-booking rows and course-sale
// rows identically — a course line is just another EarningLine, discriminated
// by CourseEnrollmentID/CourseTitle being set instead of BookingID.
func TestService_Earnings_MixesBookingAndCourseLines(t *testing.T) {
	s, repo := newTestService()
	owner := uuid.New()
	tid := uuid.New()
	repo.teacherByOwner[owner] = tid

	past := time.Now().UTC().Add(-24 * time.Hour) // cleared
	courseTitle := "Uzbek for Beginners"
	enrollmentID := uuid.New()
	repo.earnings[tid] = []EarningLine{
		{BookingID: uuidPtr(uuid.New()), StudentDisplayName: "Aziz", AmountMinor: 5_000_000, Currency: "UZS", State: LedgerHeld, AvailableAt: past},
		{CourseEnrollmentID: uuidPtr(enrollmentID), CourseTitle: &courseTitle, StudentDisplayName: "Bek", AmountMinor: 7_000_000, Currency: "UZS", State: LedgerPaid, AvailableAt: past},
	}

	e, err := s.Earnings(ctx(), owner)
	if err != nil {
		t.Fatalf("earnings: %v", err)
	}
	if len(e.Lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(e.Lines))
	}
	if e.TotalEarnedMinor != 12_000_000 {
		t.Errorf("total = %d, want 12_000_000 (booking + course)", e.TotalEarnedMinor)
	}
	if e.AvailableMinor != 5_000_000 || e.PaidMinor != 7_000_000 {
		t.Errorf("available=%d paid=%d, want 5_000_000 / 7_000_000", e.AvailableMinor, e.PaidMinor)
	}

	// The booking line carries a BookingID and no course fields; the course
	// line is the other way around.
	var sawBooking, sawCourse bool
	for _, l := range e.Lines {
		switch {
		case l.BookingID != nil:
			sawBooking = true
			if l.CourseEnrollmentID != nil || l.CourseTitle != nil {
				t.Errorf("a booking line must not carry course fields: %+v", l)
			}
		case l.CourseEnrollmentID != nil:
			sawCourse = true
			if l.CourseEnrollmentID.String() != enrollmentID.String() {
				t.Errorf("course_enrollment_id = %s, want %s", l.CourseEnrollmentID, enrollmentID)
			}
			if l.CourseTitle == nil || *l.CourseTitle != courseTitle {
				t.Errorf("course_title = %v, want %q", l.CourseTitle, courseTitle)
			}
		default:
			t.Errorf("line has neither BookingID nor CourseEnrollmentID: %+v", l)
		}
	}
	if !sawBooking || !sawCourse {
		t.Errorf("expected one booking line and one course line, sawBooking=%v sawCourse=%v", sawBooking, sawCourse)
	}
}

// TestService_Capture_HoldsUntilClearingWindow is the end-to-end version: a
// freshly captured lesson is held money, not available money.
func TestService_Capture_HoldsUntilClearingWindow(t *testing.T) {
	s, repo := newTestService()
	owner, tid, bid := uuid.New(), uuid.New(), uuid.New()
	repo.teacherByOwner[owner] = tid
	repo.seedPayment(bid, StatusRequiresPayment, 9_000_000, "UZS")
	if _, err := s.Authorize(ctx(), bid, "pm_ok"); err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if _, err := s.Capture(ctx(), bid); err != nil {
		t.Fatalf("capture: %v", err)
	}
	// The repository would stamp available_at = now + PAYOUTS_CLEARING_DAYS.
	repo.earnings[tid] = []EarningLine{{
		BookingID: uuidPtr(bid), AmountMinor: 9_000_000, Currency: "UZS",
		State: repo.ledger[bid], AvailableAt: time.Now().UTC().Add(7 * 24 * time.Hour),
	}}

	e, err := s.Earnings(ctx(), owner)
	if err != nil {
		t.Fatalf("earnings: %v", err)
	}
	if e.HeldMinor != 9_000_000 || e.AvailableMinor != 0 {
		t.Errorf("held=%d available=%d, want 9_000_000 / 0", e.HeldMinor, e.AvailableMinor)
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

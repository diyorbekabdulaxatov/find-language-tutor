package payments

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeCourseRepo is an in-memory CourseRepository. ApplyCourseEvent mirrors
// the transactional recipe in course_repository_postgres.go: event-id dedupe
// first, then the type-specific effect — no booking-confirm, no payout-ledger
// write (this module doesn't own either for course purchases).
type fakeCourseRepo struct {
	byID    map[uuid.UUID]*CoursePayment
	byPair  map[string]uuid.UUID // "courseID|studentID" -> payment id
	events  map[string]bool
	applied []string

	// phase C3: payout_ledger rows InsertCourseLedgerHeld would have written,
	// keyed by course_enrollment_id — ON CONFLICT DO NOTHING is emulated by
	// only ever writing the first call for a given enrollment id.
	ledger          map[uuid.UUID]int64 // enrollmentID -> amount_minor credited
	ledgerCalls     int                 // total calls, including no-op duplicates
	insertLedgerErr error
}

func newFakeCourseRepo() *fakeCourseRepo {
	return &fakeCourseRepo{
		byID:   map[uuid.UUID]*CoursePayment{},
		byPair: map[string]uuid.UUID{},
		events: map[string]bool{},
		ledger: map[uuid.UUID]int64{},
	}
}

// InsertCourseLedgerHeld emulates the real query's
// ON CONFLICT (course_enrollment_id) DO NOTHING: a second call for the same
// enrollmentID never overwrites the first credited amount.
func (r *fakeCourseRepo) InsertCourseLedgerHeld(_ context.Context, enrollmentID uuid.UUID, teacherShareMinor int64, _ string) error {
	r.ledgerCalls++
	if r.insertLedgerErr != nil {
		return r.insertLedgerErr
	}
	if _, exists := r.ledger[enrollmentID]; exists {
		return nil
	}
	r.ledger[enrollmentID] = teacherShareMinor
	return nil
}

func pairKey(courseID, studentID uuid.UUID) string {
	return courseID.String() + "|" + studentID.String()
}

func (r *fakeCourseRepo) EnsureCoursePayment(_ context.Context, courseID, studentID uuid.UUID, provider string, amount Money) (CoursePayment, error) {
	key := pairKey(courseID, studentID)
	if id, ok := r.byPair[key]; ok {
		return *r.byID[id], nil
	}
	id := uuid.New()
	r.byID[id] = &CoursePayment{
		ID: id, CourseID: courseID, StudentID: studentID, Provider: provider,
		Status: StatusRequiresPayment, Amount: amount,
	}
	r.byPair[key] = id
	return *r.byID[id], nil
}

func (r *fakeCourseRepo) CoursePaymentByCourse(_ context.Context, courseID, studentID uuid.UUID) (CoursePayment, error) {
	id, ok := r.byPair[pairKey(courseID, studentID)]
	if !ok {
		return CoursePayment{}, ErrPaymentNotFound
	}
	return *r.byID[id], nil
}

func (r *fakeCourseRepo) ApplyCourseEvent(_ context.Context, e Event) (bool, error) {
	if r.events[e.ID] {
		return false, nil
	}
	r.events[e.ID] = true

	p, ok := r.byID[e.PaymentID]
	if !ok {
		return false, ErrPaymentNotFound
	}
	now := time.Now().UTC()
	switch e.Type {
	case EventAuthorized:
		p.Status = StatusAuthorized
		p.ProviderRef = e.ProviderRef
		p.AuthorizedAt = &now
	case EventCaptured:
		p.Status = StatusCaptured
		p.CapturedAt = &now
	case EventRefunded:
		p.Status = StatusRefunded
		p.RefundedAt = &now
	case EventFailed:
		p.Status = StatusFailed
		p.LastError = e.Message
	default:
		return false, ErrPaymentNotFound
	}
	r.applied = append(r.applied, e.ID)
	return true, nil
}

// newTestCourseService wires the real deterministic FakeProvider with the
// service's own HandleCourseWebhook as its sink, so the idempotent webhook
// path runs on every provider operation — mirroring newTestService for the
// booking flow.
func newTestCourseService() (*CourseService, *fakeCourseRepo) {
	repo := newFakeCourseRepo()
	s := NewCourseService(repo, "fake", discardLogger(), 70)
	s.SetProvider(NewFakeProvider(s.EmitCourse))
	return s, repo
}

func TestCourseService_Purchase_HappyPath(t *testing.T) {
	s, repo := newTestCourseService()
	courseID, studentID := uuid.New(), uuid.New()

	p, err := s.Purchase(ctx(), courseID, studentID, 50_000, "UZS", "pm_ok")
	if err != nil {
		t.Fatalf("purchase: %v", err)
	}
	if p.Status != StatusCaptured {
		t.Errorf("status = %s, want captured", p.Status)
	}
	if p.ProviderRef == "" {
		t.Error("provider_ref not set")
	}
	got, _ := repo.CoursePaymentByCourse(ctx(), courseID, studentID)
	if got.Status != StatusCaptured {
		t.Errorf("stored status = %s, want captured", got.Status)
	}
}

func TestCourseService_Purchase_Declined(t *testing.T) {
	s, repo := newTestCourseService()
	courseID, studentID := uuid.New(), uuid.New()

	_, err := s.Purchase(ctx(), courseID, studentID, 50_000, "UZS", "pm_decline")
	var d PaymentDeclined
	if !errors.As(err, &d) {
		t.Fatalf("err = %v, want PaymentDeclined", err)
	}
	got, _ := repo.CoursePaymentByCourse(ctx(), courseID, studentID)
	if got.Status != StatusFailed {
		t.Errorf("status = %s, want failed", got.Status)
	}
}

func TestCourseService_Purchase_CaptureFailThenRetry(t *testing.T) {
	s, repo := newTestCourseService()
	courseID, studentID := uuid.New(), uuid.New()

	_, err := s.Purchase(ctx(), courseID, studentID, 50_000, "UZS", "pm_capture_fail")
	if !errors.Is(err, ErrCaptureFailed) {
		t.Fatalf("first purchase err = %v, want ErrCaptureFailed", err)
	}
	got, _ := repo.CoursePaymentByCourse(ctx(), courseID, studentID)
	if got.Status != StatusAuthorized {
		t.Errorf("status after capture failure = %s, want authorized (retryable)", got.Status)
	}

	// Retry: same courseID/studentID/token — Purchase should skip straight to
	// capture (no re-authorize) and succeed.
	p, err := s.Purchase(ctx(), courseID, studentID, 50_000, "UZS", "pm_capture_fail")
	if err != nil {
		t.Fatalf("retry purchase: %v", err)
	}
	if p.Status != StatusCaptured {
		t.Errorf("status after retry = %s, want captured", p.Status)
	}
}

func TestCourseService_Purchase_AlreadyCaptured_Idempotent(t *testing.T) {
	s, _ := newTestCourseService()
	courseID, studentID := uuid.New(), uuid.New()

	first, err := s.Purchase(ctx(), courseID, studentID, 50_000, "UZS", "pm_ok")
	if err != nil {
		t.Fatalf("first purchase: %v", err)
	}
	second, err := s.Purchase(ctx(), courseID, studentID, 50_000, "UZS", "pm_ok")
	if err != nil {
		t.Fatalf("second purchase: %v", err)
	}
	if second.ID != first.ID || second.Status != StatusCaptured {
		t.Errorf("second purchase should be a no-op returning the same captured payment: %+v", second)
	}
}

func TestCourseService_ApplyCourseEvent_DedupesReplay(t *testing.T) {
	s, repo := newTestCourseService()
	courseID, studentID := uuid.New(), uuid.New()
	if _, err := s.Purchase(ctx(), courseID, studentID, 50_000, "UZS", "pm_ok"); err != nil {
		t.Fatalf("purchase: %v", err)
	}
	p, _ := repo.CoursePaymentByCourse(ctx(), courseID, studentID)

	evt := Event{ID: eventID(p.ProviderRef, EventCaptured), Type: EventCaptured, PaymentID: p.ID}
	applied, err := s.HandleCourseWebhook(ctx(), evt)
	if err != nil {
		t.Fatalf("replay webhook: %v", err)
	}
	if applied {
		t.Error("replaying an already-applied event should report applied=false")
	}
}

// --- phase C3: CreditCourseSale ---

func TestCourseService_CreditCourseSale_ComputesTeacherShare(t *testing.T) {
	s, repo := newTestCourseService() // 70% teacher share
	enrollmentID := uuid.New()

	if err := s.CreditCourseSale(ctx(), enrollmentID, 100_000, "UZS"); err != nil {
		t.Fatalf("credit course sale: %v", err)
	}
	got, ok := repo.ledger[enrollmentID]
	if !ok {
		t.Fatal("expected a ledger row to be written")
	}
	if got != 70_000 {
		t.Errorf("teacher share = %d, want 70_000 (70%% of 100_000)", got)
	}
	if repo.ledgerCalls != 1 {
		t.Errorf("ledger calls = %d, want 1", repo.ledgerCalls)
	}
}

func TestCourseService_CreditCourseSale_RoundsHalfUp(t *testing.T) {
	repo := newFakeCourseRepo()
	s := NewCourseService(repo, "fake", discardLogger(), 33) // odd percent forces rounding
	enrollmentID := uuid.New()

	// 100_001 * 33 / 100 = 33000.33 -> rounds to 33000 (round-half-up on the
	// scaled integer, not float division).
	if err := s.CreditCourseSale(ctx(), enrollmentID, 100_001, "UZS"); err != nil {
		t.Fatalf("credit course sale: %v", err)
	}
	if got := repo.ledger[enrollmentID]; got != 33000 {
		t.Errorf("teacher share = %d, want 33000", got)
	}
}

func TestCourseService_CreditCourseSale_FreeCourseIsNoOp(t *testing.T) {
	s, repo := newTestCourseService()
	enrollmentID := uuid.New()

	if err := s.CreditCourseSale(ctx(), enrollmentID, 0, "UZS"); err != nil {
		t.Fatalf("credit course sale: %v", err)
	}
	if len(repo.ledger) != 0 || repo.ledgerCalls != 0 {
		t.Errorf("a free (0-priced) course sale must not write a ledger row: ledger=%v calls=%d", repo.ledger, repo.ledgerCalls)
	}
}

func TestCourseService_CreditCourseSale_DuplicateCallDoesNotDoubleCredit(t *testing.T) {
	s, repo := newTestCourseService()
	enrollmentID := uuid.New()

	if err := s.CreditCourseSale(ctx(), enrollmentID, 100_000, "UZS"); err != nil {
		t.Fatalf("first credit: %v", err)
	}
	// A retried capture (e.g. after a transient error) calls this again for
	// the same enrollment — the fake's ON CONFLICT DO NOTHING emulation must
	// keep the original amount, never add to it.
	if err := s.CreditCourseSale(ctx(), enrollmentID, 100_000, "UZS"); err != nil {
		t.Fatalf("second credit: %v", err)
	}
	if got := repo.ledger[enrollmentID]; got != 70_000 {
		t.Errorf("teacher share after duplicate call = %d, want 70_000 (unchanged, not doubled)", got)
	}
	if repo.ledgerCalls != 2 {
		t.Errorf("ledger calls = %d, want 2 (both attempted; only the first took effect)", repo.ledgerCalls)
	}
}

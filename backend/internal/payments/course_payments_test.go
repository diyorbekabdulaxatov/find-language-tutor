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
}

func newFakeCourseRepo() *fakeCourseRepo {
	return &fakeCourseRepo{
		byID:   map[uuid.UUID]*CoursePayment{},
		byPair: map[string]uuid.UUID{},
		events: map[string]bool{},
	}
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
	s := NewCourseService(repo, "fake", discardLogger())
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

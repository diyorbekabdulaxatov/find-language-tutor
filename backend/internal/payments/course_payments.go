// Phase C2: one-time course purchases. A sibling of the booking payment flow
// in payments.go / service.go: its own tables (course_payments,
// course_payment_events — see migration 000017), its own repository and
// service, but the SAME provider-agnostic machinery — Provider, Declined,
// PaymentDeclined, ErrCaptureFailed, Event, EventType, eventID — none of
// which is actually booking-specific despite living in payments.go. This file
// never touches the existing payments / payment_events tables, Repository, or
// Service.
package payments

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// CoursePayment is one course-purchase payment attempt-chain: one per
// (course, student) — course_payments.UNIQUE(course_id, student_id) plays the
// role payments.booking_id UNIQUE does for the booking flow. Same status
// lifecycle as Payment.
type CoursePayment struct {
	ID           uuid.UUID
	CourseID     uuid.UUID
	StudentID    uuid.UUID
	Provider     string
	ProviderRef  string
	Status       Status
	Amount       Money
	LastError    string
	AuthorizedAt *time.Time
	CapturedAt   *time.Time
	RefundedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CourseRepository is the persistence port for course purchases. Postgres
// impl in course_repository_postgres.go; fake in tests.
type CourseRepository interface {
	// EnsureCoursePayment creates the requires_payment intent for a
	// (course, student) pair, or returns the existing one. Idempotent —
	// mirrors Repository.EnsurePayment.
	EnsureCoursePayment(ctx context.Context, courseID, studentID uuid.UUID, provider string, amount Money) (CoursePayment, error)

	// CoursePaymentByCourse loads one payment by (course, student), or
	// ErrPaymentNotFound.
	CoursePaymentByCourse(ctx context.Context, courseID, studentID uuid.UUID) (CoursePayment, error)

	// ApplyCourseEvent processes one webhook event exactly once,
	// transactionally — the same idempotency idiom as Repository.ApplyEvent
	// (insert into course_payment_events first; a duplicate event_id means
	// already processed, applied=false/err=nil). No payout-ledger write here:
	// see InsertCourseLedgerHeld / CreditCourseSale below for why the
	// revenue-share credit happens on a different call path than the
	// booking flow's ApplyEvent-on-capture.
	ApplyCourseEvent(ctx context.Context, e Event) (applied bool, err error)

	// InsertCourseLedgerHeld writes one payout_ledger row (phase C3) for a
	// captured course sale: teacher_id is derived by joining enrollmentID to
	// its course, and the row opens the same PAYOUTS_CLEARING_DAYS window a
	// lesson's earning does. teacherShareMinor is already the teacher's
	// computed cut, not the full price — see CreditCourseSale.
	// ON CONFLICT (course_enrollment_id) DO NOTHING makes a repeat call for
	// the same enrollment a no-op.
	InsertCourseLedgerHeld(ctx context.Context, enrollmentID uuid.UUID, teacherShareMinor int64, currency string) error
}

// CourseService drives a course purchase. Unlike the booking flow's separate
// pay / complete steps, a course purchase is a single one-time charge with no
// later "lesson completed" event to capture against — so Purchase authorizes
// and captures in one call.
type CourseService struct {
	repo         CourseRepository
	provider     Provider
	providerName string
	logger       *slog.Logger

	// teacherSharePercent is the teacher's cut of a course sale
	// (COURSES_TEACHER_SHARE_PERCENT, default 70) — see CreditCourseSale.
	teacherSharePercent int
}

// NewCourseService builds the service. The provider is attached separately
// with SetProvider, same two-step construction as NewService (the in-process
// FakeProvider needs the service's own webhook handler as its event sink).
// teacherSharePercent must be in [0, 100]; an out-of-range value is clamped so
// a misconfigured env var can never pay out more than 100% or a negative
// amount.
func NewCourseService(repo CourseRepository, providerName string, logger *slog.Logger, teacherSharePercent int) *CourseService {
	if teacherSharePercent < 0 {
		teacherSharePercent = 0
	}
	if teacherSharePercent > 100 {
		teacherSharePercent = 100
	}
	return &CourseService{repo: repo, providerName: providerName, logger: logger, teacherSharePercent: teacherSharePercent}
}

// SetProvider attaches the payment provider. Must be called once before any
// Purchase. cmd/api wires a SECOND, independent FakeProvider instance here —
// not the one the booking flow uses — since course purchases and booking
// payments are unrelated event streams.
func (s *CourseService) SetProvider(p Provider) { s.provider = p }

// EmitCourse is this service's FakeProvider event sink, routing a
// provider-fired event through the same idempotent webhook path a real
// provider's HTTP callback would hit. Signature matches func(context.Context, Event) error.
func (s *CourseService) EmitCourse(ctx context.Context, e Event) error {
	_, err := s.HandleCourseWebhook(ctx, e)
	return err
}

// HandleCourseWebhook applies one provider event exactly once. A well-formed
// event that was already processed returns applied=false, err=nil.
func (s *CourseService) HandleCourseWebhook(ctx context.Context, e Event) (applied bool, err error) {
	if e.ID == "" {
		return false, errors.New("webhook event has no id")
	}
	if !e.Type.valid() {
		return false, errors.New("webhook event has an unknown type")
	}
	if e.PaymentID == uuid.Nil {
		return false, errors.New("webhook event has no payment id")
	}
	return s.repo.ApplyCourseEvent(ctx, e)
}

// Purchase authorizes and captures amountMinor for (courseID, studentID).
//
//   - requires_payment / failed: the normal or retry-after-decline path —
//     Authorize, then fall through to Capture.
//   - authorized: a previous attempt authorized but capture failed — skip
//     straight to Capture (the retry path).
//   - captured: already done; return as-is. The caller (courses.Service)
//     creates the enrollment idempotently regardless of whether this call
//     actually did any work.
func (s *CourseService) Purchase(ctx context.Context, courseID, studentID uuid.UUID, amountMinor int64, currency, methodToken string) (CoursePayment, error) {
	p, err := s.repo.EnsureCoursePayment(ctx, courseID, studentID, s.providerName, Money{AmountMinor: amountMinor, Currency: currency})
	if err != nil {
		return CoursePayment{}, err
	}

	switch p.Status {
	case StatusCaptured:
		return p, nil
	case StatusAuthorized:
		// Skip straight to the capture retry below.
	case StatusRequiresPayment, StatusFailed:
		if _, err := s.provider.Authorize(ctx, AuthorizeInput{
			PaymentID:   p.ID,
			AmountMinor: p.Amount.AmountMinor,
			Currency:    p.Amount.Currency,
			MethodToken: methodToken,
		}); err != nil {
			var d Declined
			if errors.As(err, &d) {
				return CoursePayment{}, PaymentDeclined{Reason: d.Reason}
			}
			return CoursePayment{}, err
		}
		// The provider fired payment.authorized through EmitCourse before
		// returning, so the payment is already `authorized` — reload it for
		// its provider_ref.
		p, err = s.repo.CoursePaymentByCourse(ctx, courseID, studentID)
		if err != nil {
			return CoursePayment{}, err
		}
	default:
		return CoursePayment{}, ErrNotPayable
	}

	if _, err := s.provider.Capture(ctx, p.ProviderRef); err != nil {
		s.logger.Warn("course payment capture failed",
			slog.String("course_id", courseID.String()),
			slog.String("student_id", studentID.String()),
			slog.String("provider_ref", p.ProviderRef),
			slog.Any("error", err))
		return CoursePayment{}, ErrCaptureFailed
	}

	return s.repo.CoursePaymentByCourse(ctx, courseID, studentID)
}

// CreditCourseSale credits the course's teacher their revenue-share of a
// captured course purchase into the shared payout_ledger (phase C3),
// satisfying courses.PaymentGateway.CreditCourseSale.
//
// Placement note: the booking flow writes its payout_ledger row
// (InsertLedgerHeld) inside ApplyEvent's EventCaptured branch, because a
// booking row already exists before its payment is even created. A course
// purchase is the other way around — courses.Service creates the
// course_enrollment only AFTER this package's Purchase (above) has already
// authorized and captured the charge — so there is no course_enrollment_id
// for ApplyCourseEvent's EventCaptured branch to attach a ledger row to.
// courses.Service.Purchase therefore calls this method itself, through the
// courses.PaymentGateway port, right after EnsureEnrollment succeeds.
//
// priceAmountMinor is the FULL price the student paid, not the teacher's cut
// — the revenue-share arithmetic happens here, in Go (never in SQL), using
// integer round-half-up so money is never a float. priceAmountMinor <= 0 is a
// no-op (nothing to pay a teacher for a free course — Purchase never actually
// reaches here for one, but this stays defensive). Idempotent: repository's
// InsertCourseLedgerHeld is ON CONFLICT (course_enrollment_id) DO NOTHING, so
// a retried/duplicate call for the same enrollment never double-credits.
func (s *CourseService) CreditCourseSale(ctx context.Context, enrollmentID uuid.UUID, priceAmountMinor int64, currency string) error {
	if priceAmountMinor <= 0 {
		return nil
	}
	teacherShare := teacherShareMinor(priceAmountMinor, s.teacherSharePercent)
	if err := s.repo.InsertCourseLedgerHeld(ctx, enrollmentID, teacherShare, currency); err != nil {
		return fmt.Errorf("credit course sale: %w", err)
	}
	return nil
}

// teacherShareMinor computes the teacher's cut of a course sale using
// round-half-up integer arithmetic — money is integer minor units everywhere
// in this codebase, never a float.
func teacherShareMinor(priceAmountMinor int64, percent int) int64 {
	return (priceAmountMinor*int64(percent) + 50) / 100
}

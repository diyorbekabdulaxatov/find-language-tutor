package payments

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/courses"
)

// CourseGateway adapts *CourseService to courses.PaymentGateway — the
// course-purchase sibling of Gateway (bookings.PaymentGateway). courses
// defines the port, this adapter implements it, cmd/api injects it with
// courses.Service.SetPaymentGateway. courses never imports this package.
type CourseGateway struct{ svc *CourseService }

// NewCourseGateway wraps the course payment service as a courses.PaymentGateway.
func NewCourseGateway(svc *CourseService) *CourseGateway { return &CourseGateway{svc: svc} }

var _ courses.PaymentGateway = (*CourseGateway)(nil)

func (g *CourseGateway) Purchase(ctx context.Context, courseID, studentID uuid.UUID, amountMinor int64, currency, methodToken string) (courses.PurchaseSnapshot, error) {
	p, err := g.svc.Purchase(ctx, courseID, studentID, amountMinor, currency, methodToken)
	if err != nil {
		return courses.PurchaseSnapshot{}, translateCourse(err)
	}
	return courses.PurchaseSnapshot{
		Status:      string(p.Status),
		AmountMinor: p.Amount.AmountMinor,
		Currency:    p.Amount.Currency,
	}, nil
}

// translateCourse maps payments-domain errors onto the sentinels the courses
// handler knows how to render, so courses stays free of any payments import.
func translateCourse(err error) error {
	var declined PaymentDeclined
	switch {
	case errors.As(err, &declined):
		return courses.PaymentFailedError{Reason: declined.Reason}
	case errors.Is(err, ErrCaptureFailed):
		return courses.ErrCaptureFailed
	default:
		return err
	}
}

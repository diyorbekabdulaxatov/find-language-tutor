package payments

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
)

// Gateway adapts *Service to bookings.PaymentGateway. It is the ONE place the
// payments and bookings modules meet in Go: bookings defines the port, this
// adapter implements it, and cmd/api injects it with
// bookings.Service.SetPaymentGateway. bookings never imports this package.
type Gateway struct{ svc *Service }

// NewGateway wraps the payments service as a bookings.PaymentGateway.
func NewGateway(svc *Service) *Gateway { return &Gateway{svc: svc} }

var _ bookings.PaymentGateway = (*Gateway)(nil)

func snap(p Payment) bookings.PaymentSnapshot {
	return bookings.PaymentSnapshot{
		Status:      string(p.Status),
		AmountMinor: p.Amount.AmountMinor,
		Currency:    p.Amount.Currency,
	}
}

func (g *Gateway) InitiatePayment(ctx context.Context, bookingID uuid.UUID, amountMinor int64, currency string) error {
	return g.svc.InitiatePayment(ctx, bookingID, amountMinor, currency)
}

func (g *Gateway) Authorize(ctx context.Context, bookingID uuid.UUID, methodToken string) (bookings.PaymentSnapshot, error) {
	p, err := g.svc.Authorize(ctx, bookingID, methodToken)
	if err != nil {
		return bookings.PaymentSnapshot{}, translate(err)
	}
	return snap(p), nil
}

func (g *Gateway) Capture(ctx context.Context, bookingID uuid.UUID) (bookings.PaymentSnapshot, error) {
	p, err := g.svc.Capture(ctx, bookingID)
	if err != nil {
		return bookings.PaymentSnapshot{}, translate(err)
	}
	return snap(p), nil
}

func (g *Gateway) Refund(ctx context.Context, bookingID uuid.UUID) (bookings.PaymentSnapshot, error) {
	p, err := g.svc.Refund(ctx, bookingID)
	if err != nil {
		return bookings.PaymentSnapshot{}, translate(err)
	}
	return snap(p), nil
}

func (g *Gateway) SnapshotForBooking(ctx context.Context, bookingID uuid.UUID) (bookings.PaymentSnapshot, bool, error) {
	status, amountMinor, currency, found, err := g.svc.SnapshotForBooking(ctx, bookingID)
	if err != nil || !found {
		return bookings.PaymentSnapshot{}, false, err
	}
	return bookings.PaymentSnapshot{Status: status, AmountMinor: amountMinor, Currency: currency}, true, nil
}

// translate maps payments-domain errors onto the sentinels the bookings handler
// knows how to render, so bookings stays free of any payments import.
func translate(err error) error {
	var declined PaymentDeclined
	switch {
	case errors.As(err, &declined):
		return bookings.PaymentFailedError{Reason: declined.Reason}
	case errors.Is(err, ErrAlreadyPaid):
		return bookings.ErrAlreadyPaid
	case errors.Is(err, ErrCaptureFailed):
		return bookings.ErrCaptureFailed
	case errors.Is(err, ErrNotPayable):
		return bookings.ErrInvalidTransition
	case errors.Is(err, ErrPaymentNotFound):
		return bookings.ErrPaymentRequired
	default:
		return err
	}
}

package payments

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
)

// Service holds the payment rules. Handlers and the bookings gateway call it;
// it never sees an *gin.Context.
type Service struct {
	repo         Repository
	provider     Provider
	providerName string
	logger       *slog.Logger
}

// NewService builds the service. The provider is attached separately with
// SetProvider because the in-process FakeProvider needs the service's
// HandleWebhook as its event sink (a construction cycle otherwise).
func NewService(repo Repository, providerName string, logger *slog.Logger) *Service {
	return &Service{repo: repo, providerName: providerName, logger: logger}
}

// SetProvider attaches the payment provider. Must be called once before any
// pay / capture / refund.
func (s *Service) SetProvider(p Provider) { s.provider = p }

// Emit is the FakeProvider's event sink: it routes a provider-fired event
// through the same idempotent HandleWebhook path a real provider's HTTP webhook
// would hit. Signature matches func(context.Context, Event) error.
func (s *Service) Emit(ctx context.Context, e Event) error {
	_, err := s.HandleWebhook(ctx, e)
	return err
}

// InitiatePayment creates the requires_payment intent for a freshly created
// booking. Implements bookings.PaymentGateway. Idempotent.
func (s *Service) InitiatePayment(ctx context.Context, bookingID uuid.UUID, amountMinor int64, currency string) error {
	_, err := s.repo.EnsurePayment(ctx, bookingID, s.providerName, Money{AmountMinor: amountMinor, Currency: currency})
	return err
}

// Authorize places a hold for the booking's price. On a provider decline it
// records payment.failed and returns PaymentDeclined (the bookings handler
// renders 402 payment_failed). On success the provider fires payment.authorized
// through Emit, which flips the payment to authorized and the booking to
// confirmed in one transaction.
func (s *Service) Authorize(ctx context.Context, bookingID uuid.UUID, methodToken string) (Payment, error) {
	p, err := s.repo.PaymentByBooking(ctx, bookingID)
	if err != nil {
		return Payment{}, err
	}
	switch p.Status {
	case StatusAuthorized, StatusCaptured:
		return Payment{}, ErrAlreadyPaid
	case StatusRefunded:
		return Payment{}, ErrNotPayable
	case StatusRequiresPayment, StatusFailed:
		// requires_payment is the normal path; failed is a retry after a decline.
	default:
		return Payment{}, ErrNotPayable
	}

	// The provider fires payment.authorized (or payment.failed) through Emit
	// before returning, so the payment + booking state are already updated.
	if _, err := s.provider.Authorize(ctx, AuthorizeInput{
		PaymentID:   p.ID,
		AmountMinor: p.Amount.AmountMinor,
		Currency:    p.Amount.Currency,
		MethodToken: methodToken,
	}); err != nil {
		var d Declined
		if errors.As(err, &d) {
			return Payment{}, PaymentDeclined{Reason: d.Reason}
		}
		return Payment{}, err
	}

	return s.repo.PaymentByBooking(ctx, bookingID)
}

// Capture settles the authorized hold for a booking (called when a lesson is
// completed). On success the provider fires payment.captured through Emit,
// which marks the payment captured and writes the payout-ledger row. A provider
// failure returns ErrCaptureFailed and leaves the payment authorized.
func (s *Service) Capture(ctx context.Context, bookingID uuid.UUID) (Payment, error) {
	p, err := s.repo.PaymentByBooking(ctx, bookingID)
	if err != nil {
		return Payment{}, err
	}
	if p.Status == StatusCaptured {
		return p, nil
	}
	if p.Status != StatusAuthorized {
		return Payment{}, ErrNotPayable
	}

	if _, err := s.provider.Capture(ctx, p.ProviderRef); err != nil {
		s.logger.Warn("payment capture failed",
			slog.String("booking_id", bookingID.String()),
			slog.String("provider_ref", p.ProviderRef),
			slog.Any("error", err))
		return Payment{}, ErrCaptureFailed
	}

	return s.repo.PaymentByBooking(ctx, bookingID)
}

// Refund releases the hold or refunds the capture for a booking (called when a
// confirmed/paid booking is cancelled). If nothing was ever authorized the
// intent is simply voided (payment.failed). On a real refund the provider fires
// payment.refunded through Emit, which marks the payment refunded and reverses
// the payout-ledger row if one exists.
func (s *Service) Refund(ctx context.Context, bookingID uuid.UUID) (Payment, error) {
	p, err := s.repo.PaymentByBooking(ctx, bookingID)
	if err != nil {
		if errors.Is(err, ErrPaymentNotFound) {
			return Payment{}, nil // nothing to undo
		}
		return Payment{}, err
	}

	switch p.Status {
	case StatusRefunded:
		return p, nil
	case StatusRequiresPayment, StatusFailed:
		// No hold to release — void the intent so it can't be paid later.
		evt := Event{
			ID:        eventID(p.ID.String()+"_void", EventFailed),
			Type:      EventFailed,
			PaymentID: p.ID,
			Message:   "voided: booking cancelled",
		}
		if _, err := s.HandleWebhook(ctx, evt); err != nil {
			return Payment{}, err
		}
		return s.repo.PaymentByBooking(ctx, bookingID)
	case StatusAuthorized, StatusCaptured:
		if _, err := s.provider.Refund(ctx, p.ProviderRef, p.Amount.AmountMinor); err != nil {
			return Payment{}, err
		}
		return s.repo.PaymentByBooking(ctx, bookingID)
	default:
		return Payment{}, ErrNotPayable
	}
}

// SnapshotForBooking returns the payment status + amount for embedding in a
// BookingDTO. found is false when the booking has no payment intent.
func (s *Service) SnapshotForBooking(ctx context.Context, bookingID uuid.UUID) (status string, amountMinor int64, currency string, found bool, err error) {
	p, err := s.repo.PaymentByBooking(ctx, bookingID)
	if err != nil {
		if errors.Is(err, ErrPaymentNotFound) {
			return "", 0, "", false, nil
		}
		return "", 0, "", false, err
	}
	return string(p.Status), p.Amount.AmountMinor, p.Amount.Currency, true, nil
}

// HandleWebhook applies one provider event exactly once. A well-formed event
// that was already processed returns applied=false, err=nil (the HTTP handler
// still answers 200). Idempotency is enforced by the payment_events primary
// key, not a check-then-insert, so concurrent duplicate deliveries are safe.
func (s *Service) HandleWebhook(ctx context.Context, e Event) (applied bool, err error) {
	if e.ID == "" {
		return false, errors.New("webhook event has no id")
	}
	if !e.Type.valid() {
		return false, errors.New("webhook event has an unknown type")
	}
	if e.PaymentID == uuid.Nil {
		return false, errors.New("webhook event has no payment id")
	}
	return s.repo.ApplyEvent(ctx, e)
}

// Earnings returns the teacher-owner's earnings summary.
func (s *Service) Earnings(ctx context.Context, ownerID uuid.UUID) (Earnings, error) {
	tid, ok, err := s.repo.TeacherIDByOwner(ctx, ownerID)
	if err != nil {
		return Earnings{}, err
	}
	if !ok {
		return Earnings{}, ErrNoTeacherProfile
	}
	lines, err := s.repo.EarningLines(ctx, tid)
	if err != nil {
		return Earnings{}, err
	}
	return summarize(lines, defaultCurrency), nil
}

// defaultCurrency is used for an earnings summary with no lessons yet. The
// platform is UZS-only for the MVP.
const defaultCurrency = "UZS"

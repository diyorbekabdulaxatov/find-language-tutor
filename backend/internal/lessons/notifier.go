package lessons

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/email"
)

// Notifier implements bookings.Notifier: it renders the lifecycle templates and
// sends them via the email backend. Sends are best-effort — a failure is logged,
// never returned, so it can't fail the HTTP request that triggered it.
type Notifier struct {
	mailer email.Emailer
	logger *slog.Logger
}

var _ bookings.Notifier = (*Notifier)(nil)

// NewNotifier wraps an email backend.
func NewNotifier(mailer email.Emailer, logger *slog.Logger) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{mailer: mailer, logger: logger}
}

func (n *Notifier) BookingConfirmed(ctx context.Context, b bookings.Booking) {
	n.send(ctx, "booking_confirmed", b.ID, ConfirmedMessages(b))
}

func (n *Notifier) BookingCancelled(ctx context.Context, b bookings.Booking, cancelledBy uuid.UUID, refunded bool) {
	n.send(ctx, "booking_cancelled", b.ID, CancelledMessages(b, cancelledBy, refunded))
}

func (n *Notifier) send(ctx context.Context, kind string, bookingID uuid.UUID, msgs []email.Message) {
	for _, m := range msgs {
		if err := n.mailer.Send(ctx, m); err != nil {
			n.logger.ErrorContext(ctx, "send lifecycle email",
				slog.String("kind", kind),
				slog.String("booking_id", bookingID.String()),
				slog.String("to", m.To),
				slog.Any("error", err))
		}
	}
}

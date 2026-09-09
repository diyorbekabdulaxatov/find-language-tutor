package disputes

import (
	"context"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
)

// BookingGateway adapts *Service to bookings.DisputeReader. It is the one place
// the disputes and bookings modules meet in Go: bookings defines the port, this
// adapter implements it, and cmd/api injects it with
// bookings.Service.SetDisputeReader. bookings never imports this package.
type BookingGateway struct{ svc *Service }

// NewBookingGateway wraps the disputes service as a bookings.DisputeReader.
func NewBookingGateway(svc *Service) *BookingGateway { return &BookingGateway{svc: svc} }

var _ bookings.DisputeReader = (*BookingGateway)(nil)

func (g *BookingGateway) OpenForBooking(ctx context.Context, bookingID uuid.UUID) (*bookings.BookingDispute, bool, error) {
	d, found, err := g.svc.OpenDisputeForBooking(ctx, bookingID)
	if err != nil || !found {
		return nil, false, err
	}
	return &bookings.BookingDispute{
		ID:        d.ID,
		Status:    string(d.Status),
		Reason:    d.Reason,
		CreatedAt: d.CreatedAt,
	}, true, nil
}

package reviews

import (
	"context"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
)

// BookingGateway adapts *Service to bookings.ReviewReader. It is the one place
// the reviews and bookings modules meet in Go: bookings defines the port, this
// adapter implements it, and cmd/api injects it with
// bookings.Service.SetReviewReader. bookings never imports this package.
type BookingGateway struct{ svc *Service }

// NewBookingGateway wraps the reviews service as a bookings.ReviewReader.
func NewBookingGateway(svc *Service) *BookingGateway { return &BookingGateway{svc: svc} }

var _ bookings.ReviewReader = (*BookingGateway)(nil)

func (g *BookingGateway) ForBooking(ctx context.Context, bookingID uuid.UUID) (*bookings.BookingReview, bool, error) {
	r, found, err := g.svc.ReviewForBooking(ctx, bookingID)
	if err != nil || !found {
		return nil, false, err
	}
	return &bookings.BookingReview{
		Rating:    r.Rating,
		Comment:   r.Comment,
		CreatedAt: r.CreatedAt,
	}, true, nil
}

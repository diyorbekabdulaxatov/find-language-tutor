package resources

import (
	"context"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
)

// BookingGateway adapts *Service to bookings.ResourceReader. It is the one
// place resources and bookings meet by an ordinary Go import: bookings
// defines the port, this adapter implements it, and cmd/api injects it with
// bookings.Service.SetResourceReader. bookings never imports this package (see
// resources.BookingReader / bookings.ResourceBookingGateway for the reverse
// direction, which avoids the cycle by structural typing instead).
type BookingGateway struct{ svc *Service }

// NewBookingGateway wraps the resources service as a bookings.ResourceReader.
func NewBookingGateway(svc *Service) *BookingGateway { return &BookingGateway{svc: svc} }

var _ bookings.ResourceReader = (*BookingGateway)(nil)

func (g *BookingGateway) ForBooking(ctx context.Context, bookingID, viewerID uuid.UUID) ([]bookings.BookingResource, error) {
	rows, err := g.svc.SummaryForBooking(ctx, bookingID, viewerID)
	if err != nil {
		return nil, err
	}
	out := make([]bookings.BookingResource, len(rows))
	for i, r := range rows {
		out[i] = bookings.BookingResource{
			ID:               r.ID,
			ResourceID:       r.ResourceID,
			Kind:             r.Kind,
			Position:         r.Position,
			DueAt:            r.DueAt,
			Type:             string(r.Type),
			Title:            r.Title,
			ResourceStatus:   string(r.ResourceStatus),
			SubmissionStatus: r.SubmissionStatus,
		}
	}
	return out, nil
}

package bookings

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ResourceBookingGateway adapts *Service to resources.BookingReader by
// structural typing ONLY — this file deliberately does not import
// internal/resources (see the long comment on resources.BookingReader for
// why: internal/resources already imports internal/bookings the other
// direction, to implement bookings.ResourceReader, so an import here would
// cycle). Its Booking method's signature — built only from uuid.UUID / bool /
// error — happens to match resources.BookingReader exactly, which is all Go's
// interface satisfaction needs. cmd/api wires it with
// resourceService.SetBookingReader(bookings.NewResourceBookingGateway(bookingService)).
type ResourceBookingGateway struct{ svc *Service }

// NewResourceBookingGateway wraps the bookings service as a resources.BookingReader.
func NewResourceBookingGateway(svc *Service) *ResourceBookingGateway {
	return &ResourceBookingGateway{svc: svc}
}

// Booking resolves a booking's two participants. found is false when the
// booking id is unknown.
func (g *ResourceBookingGateway) Booking(ctx context.Context, bookingID uuid.UUID) (teacherOwnerID, studentID uuid.UUID, found bool, err error) {
	b, err := g.svc.repo.GetBooking(ctx, bookingID)
	if err != nil {
		if errors.Is(err, ErrBookingNotFound) {
			return uuid.Nil, uuid.Nil, false, nil
		}
		return uuid.Nil, uuid.Nil, false, err
	}
	return b.TeacherOwnerID, b.Student.ID, true, nil
}

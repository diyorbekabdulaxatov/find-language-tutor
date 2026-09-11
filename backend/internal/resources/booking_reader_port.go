package resources

import (
	"context"

	"github.com/google/uuid"
)

// BookingReader is the slice of the bookings module the attach / detach /
// submission flow needs: does a booking exist, and who are its two
// participants (the account owning the teacher profile, and the student)?
//
// Deliberate deviation from the usual port shape (see reviews.BookingGateway /
// disputes.BookingGateway for the pattern this otherwise mirrors): those
// adapters return a small struct owned by the *consumer* package. Doing the
// same here — resources defining a BookingRef struct and bookings importing
// resources to build one — would import-cycle, because internal/bookings
// already needs to import internal/resources the other way (to implement
// bookings.ResourceReader for the booking DTO's embedded resource summary).
// Declaring this method with only uuid.UUID / bool / error lets *bookings.Service
// satisfy BookingReader by structural typing alone, with zero import of this
// package — so the dependency graph stays one-way (resources -> bookings) and
// cmd/api wires the two together untyped-adapter-free:
// resourceService.SetBookingReader(bookings.NewResourceBookingGateway(bookingService)).
//
// A nil reader makes every attach / detach / submission call fail closed
// (found=false, i.e. "booking not found"), never panic.
type BookingReader interface {
	// Booking resolves a booking's two participants. found is false when the
	// booking id is unknown.
	Booking(ctx context.Context, bookingID uuid.UUID) (teacherOwnerID, studentID uuid.UUID, found bool, err error)
}

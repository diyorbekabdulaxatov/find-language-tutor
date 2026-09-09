package bookings

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DisputeReader is the slice of the disputes module the booking flow needs to
// annotate a BookingDTO with `can_raise_dispute` / `open_dispute` without a
// second API call. It mirrors ReviewReader exactly: internal/disputes provides
// the adapter, this package never imports it, and cmd/api injects the concrete
// type with Service.SetDisputeReader. A nil reader is a safe no-op (guarded at
// every use).
type DisputeReader interface {
	// OpenForBooking returns the booking's OPEN dispute. found is false when the
	// booking has none (never disputed, or every dispute is closed).
	OpenForBooking(ctx context.Context, bookingID uuid.UUID) (dispute *BookingDispute, found bool, err error)
}

// BookingDispute is the read-model of a booking's open dispute embedded in
// booking responses (visible to both participants).
type BookingDispute struct {
	ID        uuid.UUID
	Status    string
	Reason    string
	CreatedAt time.Time
}

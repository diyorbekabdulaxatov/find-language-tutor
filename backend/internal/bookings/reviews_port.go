package bookings

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ReviewReader is the slice of the reviews module the booking flow needs to
// annotate a BookingDTO with `can_review` / `review` without a second API call.
// internal/reviews provides an adapter that satisfies it; this package never
// imports internal/reviews, so the dependency direction is one-way. cmd/api /
// internal/httpapi construct the adapter and inject it with
// Service.SetReviewReader. A nil reader is a safe no-op (guarded at every use).
type ReviewReader interface {
	// ForBooking returns the review attached to a booking. found is false when
	// the booking has not been reviewed.
	ForBooking(ctx context.Context, bookingID uuid.UUID) (review *BookingReview, found bool, err error)
}

// BookingReview is the read-model of a booking's review embedded in booking
// responses (visible to both participants).
type BookingReview struct {
	Rating    int
	Comment   string
	CreatedAt time.Time
}

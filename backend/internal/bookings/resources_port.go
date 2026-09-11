package bookings

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ResourceReader is the slice of the resources module the booking flow needs
// to annotate a BookingDTO with a `resources` summary (materials / homework
// attached to the lesson) without a second API call. internal/resources
// provides an adapter (resources.NewBookingGateway) that satisfies it; this
// package never imports internal/resources, so the dependency direction is
// one-way. cmd/api constructs the adapter and injects it with
// Service.SetResourceReader. A nil reader is a safe no-op (guarded at every
// use) — the full content + quiz player payload lives at
// GET /v1/bookings/{id}/resources, owned entirely by internal/resources.
type ResourceReader interface {
	// ForBooking returns the booking's attached-resource summaries. viewerID is
	// the caller viewing the booking — SubmissionStatus on each item is only
	// ever populated for the student viewer.
	ForBooking(ctx context.Context, bookingID, viewerID uuid.UUID) ([]BookingResource, error)
}

// BookingResource is the read-model of one resource attached to a booking,
// embedded in booking responses. It is a summary only — no quiz content.
type BookingResource struct {
	ID         uuid.UUID
	ResourceID uuid.UUID
	Kind       string
	Position   int
	DueAt      *time.Time

	Type           string
	Title          string
	ResourceStatus string

	// SubmissionStatus is the viewer's own submission status ("in_progress" /
	// "submitted" / "graded"); "" when the viewer is not the student, or has
	// not started this homework yet.
	SubmissionStatus string
}

package resources

import (
	"context"

	"github.com/google/uuid"
)

// CourseProgress is the optional, best-effort notification back to the
// courses module: a course-embedded resource's submission reaching
// `submitted` or `graded` marks that curriculum item complete. Mirrors
// Mailer's shape — nil is a safe no-op, and a failure here is logged and
// never fails the submit request (see Service.notifyCourseProgress).
type CourseProgress interface {
	ItemCompleted(ctx context.Context, enrollmentID, resourceID uuid.UUID) error
}

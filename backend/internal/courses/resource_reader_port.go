package courses

import (
	"context"

	"github.com/google/uuid"
)

// ResourceReader is the slice of the resources module courses needs to
// validate a `resource`-kind item: does resourceID belong to the calling
// teacher, and is it published (not draft, not archived)? internal/resources
// provides the adapter (resources.NewCourseGateway); this package never
// imports internal/resources, so the dependency graph stays one-way
// (resources -> courses).
//
// Unlike resources.BookingReader / bookings.ResourceReader (see the long
// comment there), no structural-typing trick is needed here: internal/courses
// is a one-way consumer of internal/resources in this phase (nothing in
// internal/resources needs anything from internal/courses), so a plain
// interface + an ordinary adapter that imports this package is enough — no
// cycle to avoid.
//
// A nil reader (SetResourceReader never called) makes AddItem fail closed for
// every `resource`-kind item.
type ResourceReader interface {
	// ResourceOwnedAndPublished reports whether resourceID belongs to
	// teacherID and is published and not archived.
	ResourceOwnedAndPublished(ctx context.Context, resourceID, teacherID uuid.UUID) (ok bool, err error)
}

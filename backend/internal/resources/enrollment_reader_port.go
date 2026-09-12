package resources

import (
	"context"

	"github.com/google/uuid"
)

// EnrollmentReader is the slice of the courses module the course-context
// submission flow needs: does an enrollment exist, who are its participants,
// and does its course actually embed a given resource?
//
// Deliberate deviation from the usual port shape (see resources.BookingReader's
// long comment for the pattern this mirrors): declaring this method set with
// only uuid.UUID / bool / error / primitives lets *courses.Service satisfy
// EnrollmentReader by structural typing alone, with zero import of this
// package — so the dependency graph stays one-way (resources -> courses) even
// though internal/courses already imports internal/resources the other way
// (resources.NewCourseGateway / resources.NewFileGateway implement courses'
// ResourceReader / FileReader ports). cmd/api wires the two together
// untyped-adapter-free: resourceService.SetEnrollmentReader(courseService).
//
// A nil reader makes every course-context submission call fail closed
// (found=false / not-granted), never panic.
type EnrollmentReader interface {
	// Enrollment resolves an enrollment's two participants for
	// authorization: the course's teacher's owning account (Grade's
	// authorization) and the enrolled student. found is false when the
	// enrollment id is unknown.
	Enrollment(ctx context.Context, enrollmentID uuid.UUID) (courseTeacherOwnerID, studentID uuid.UUID, found bool, err error)

	// EnrollmentGrantsResource reports whether the given enrollment's course
	// actually embeds resourceID as a curriculum item — the course-context
	// equivalent of GetBookingResourceByPair's "is this actually attached"
	// check.
	EnrollmentGrantsResource(ctx context.Context, enrollmentID, resourceID uuid.UUID) (bool, error)

	// StudentResourceAccess reports whether studentID has some active
	// enrollment granting access to resourceID, independent of which
	// specific enrollment — backs FileAssetAccessible's course-based
	// widening (a course-embedded resource's material/audio file).
	StudentResourceAccess(ctx context.Context, resourceID, studentID uuid.UUID) (bool, error)
}

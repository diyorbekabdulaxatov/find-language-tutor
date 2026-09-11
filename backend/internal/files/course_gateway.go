package files

import (
	"context"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/courses"
)

// CourseGateway adapts *Service to courses.FileReader: a course's video item
// / cover image must reference a file the calling account itself uploaded.
// courses defines the port, this adapter implements it, cmd/api injects it
// with courses.Service.SetFileReader. files never imports internal/courses.
type CourseGateway struct{ svc *Service }

// NewCourseGateway wraps the files service as a courses.FileReader.
func NewCourseGateway(svc *Service) *CourseGateway { return &CourseGateway{svc: svc} }

var _ courses.FileReader = (*CourseGateway)(nil)

func (g *CourseGateway) FileOwnedBy(ctx context.Context, fileAssetID, callerID uuid.UUID) (bool, string, error) {
	return g.svc.FileOwnedBy(ctx, fileAssetID, callerID)
}

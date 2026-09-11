package resources

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/courses"
)

// CourseGateway adapts *Service to courses.ResourceReader: a course's
// `resource`-kind item must reference one of the calling teacher's own
// published (not draft, not archived) resources. courses defines the port,
// this adapter implements it, cmd/api injects it with
// courses.Service.SetResourceReader. resources never imports internal/courses
// back — see the long comment on courses.ResourceReader for why no
// structural-typing trick is needed here (courses is a one-way consumer of
// resources in this phase).
type CourseGateway struct{ svc *Service }

// NewCourseGateway wraps the resources service as a courses.ResourceReader.
func NewCourseGateway(svc *Service) *CourseGateway { return &CourseGateway{svc: svc} }

var _ courses.ResourceReader = (*CourseGateway)(nil)

func (g *CourseGateway) ResourceOwnedAndPublished(ctx context.Context, resourceID, teacherID uuid.UUID) (bool, error) {
	r, err := g.svc.repo.ByID(ctx, resourceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return r.TeacherID == teacherID && r.Status == StatusPublished && r.ArchivedAt == nil, nil
}

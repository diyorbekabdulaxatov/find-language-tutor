package resources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

// PublicResource returns resourceID's student-safe content (correct answers
// stripped via Content.Public(), same stripping the booking-attachment view
// applies) marshalled to opaque JSON, for embedding in the course player.
// Phase C2's courses.Service has already authorized the caller (enrolled
// student or owning teacher) before calling through; this method does not
// re-check status/ownership beyond "does the resource exist".
func (g *CourseGateway) PublicResource(ctx context.Context, resourceID uuid.UUID) (courses.CourseResourceView, error) {
	r, err := g.svc.repo.ByID(ctx, resourceID)
	if err != nil {
		return courses.CourseResourceView{}, err
	}
	blob, err := json.Marshal(r.Content.Public())
	if err != nil {
		return courses.CourseResourceView{}, fmt.Errorf("marshal public resource content: %w", err)
	}
	return courses.CourseResourceView{
		ID:           r.ID,
		Type:         string(r.Type),
		Title:        r.Title,
		Instructions: r.Instructions,
		Content:      blob,
	}, nil
}

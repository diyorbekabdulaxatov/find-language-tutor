package courses

import (
	"context"

	"github.com/google/uuid"
)

// --- phase C3: admin moderation ---

const (
	adminDefaultPageSize = 20
	adminMaxPageSize     = 100
)

// AdminModerate returns a page of the operator moderation queue. Operator-only
// (the handler enforces `courses.moderate`). page defaults to 1, pageSize to
// 20 (capped at 100); an unrecognised Status/Suspended filter is a 400.
func (s *Service) AdminModerate(ctx context.Context, q AdminCourseQuery) (AdminCoursePage, error) {
	switch q.Status {
	case "", string(StatusDraft), string(StatusPublished), "archived":
	default:
		return AdminCoursePage{}, invalid("`status` must be one of draft, published, archived.")
	}
	switch q.Suspended {
	case "", "true", "false":
	default:
		return AdminCoursePage{}, invalid("`suspended` must be true or false.")
	}

	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = adminDefaultPageSize
	}
	if q.PageSize > adminMaxPageSize {
		q.PageSize = adminMaxPageSize
	}

	items, total, err := s.repo.AdminList(ctx, q, q.PageSize, (q.Page-1)*q.PageSize)
	if err != nil {
		return AdminCoursePage{}, err
	}
	if items == nil {
		items = []AdminCourse{}
	}
	return AdminCoursePage{Courses: items, Total: total}, nil
}

// AdminSuspend pulls a course from the storefront (catalog, catalog detail,
// cover image, new purchases) without touching the teacher's own
// draft/published/archived state and without revoking an already-enrolled
// student's access. Idempotent.
func (s *Service) AdminSuspend(ctx context.Context, id uuid.UUID) (AdminCourse, error) {
	return s.repo.SetSuspended(ctx, id, true)
}

// AdminUnsuspend restores a course to the storefront. Idempotent.
func (s *Service) AdminUnsuspend(ctx context.Context, id uuid.UUID) (AdminCourse, error) {
	return s.repo.SetSuspended(ctx, id, false)
}

package courses

import "time"

// Wire DTOs for the phase-C3 admin moderation queue. snake_case, in sync with
// openapi.yaml's AdminCourse / AdminCourseList schemas.

type adminTeacherRefDTO struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
}

// adminCourseDTO is one moderation-queue row: enough to moderate from a list
// without a second call, mirroring AdminReview's level of detail.
type adminCourseDTO struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	Teacher     adminTeacherRefDTO `json:"teacher"`
	Price       moneyDTO           `json:"price"`
	Status      string             `json:"status"`
	Archived    bool               `json:"archived"`
	Suspended   bool               `json:"suspended"`
	SuspendedAt *time.Time         `json:"suspended_at"`
	CreatedAt   time.Time          `json:"created_at"`
}

func toAdminCourseDTO(a AdminCourse) adminCourseDTO {
	return adminCourseDTO{
		ID:    a.Course.ID.String(),
		Title: a.Course.Title,
		Teacher: adminTeacherRefDTO{
			Slug:        a.Teacher.Slug,
			DisplayName: a.Teacher.DisplayName,
		},
		Price:       moneyDTO{AmountMinor: a.Course.PriceAmountMinor, Currency: a.Course.PriceCurrency},
		Status:      string(a.Course.Status),
		Archived:    a.Course.ArchivedAt != nil,
		Suspended:   a.Course.SuspendedAt != nil,
		SuspendedAt: a.Course.SuspendedAt,
		CreatedAt:   a.Course.CreatedAt.UTC(),
	}
}

type adminCourseListDTO struct {
	Courses []adminCourseDTO `json:"courses"`
	Total   int              `json:"total"`
}

func toAdminCourseListDTO(p AdminCoursePage) adminCourseListDTO {
	items := make([]adminCourseDTO, len(p.Courses))
	for i, c := range p.Courses {
		items[i] = toAdminCourseDTO(c)
	}
	return adminCourseListDTO{Courses: items, Total: p.Total}
}

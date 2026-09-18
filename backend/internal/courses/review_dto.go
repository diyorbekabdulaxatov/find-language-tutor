package courses

import "time"

// Wire DTOs, phase D2: course ratings and reviews. snake_case, in sync with
// openapi.yaml.

type createCourseReviewRequest struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

type courseReviewDTO struct {
	ID                 string    `json:"id"`
	Rating             int       `json:"rating"`
	Comment            string    `json:"comment"`
	StudentDisplayName string    `json:"student_display_name"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func toCourseReviewDTO(r Review) courseReviewDTO {
	return courseReviewDTO{
		ID: r.ID.String(), Rating: r.Rating, Comment: r.Comment,
		StudentDisplayName: r.StudentName,
		CreatedAt:          r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
}

type courseReviewListDTO struct {
	Reviews []courseReviewDTO `json:"reviews"`
	Total   int               `json:"total"`
	// Breakdown is the star histogram over every visible review, index 0 = 1★
	// … index 4 = 5★ — not just this page, so the bars don't move as the
	// reader pages through.
	Breakdown []int `json:"breakdown"`
}

func toCourseReviewListDTO(p ReviewPage) courseReviewListDTO {
	items := make([]courseReviewDTO, len(p.Reviews))
	for i, r := range p.Reviews {
		items[i] = toCourseReviewDTO(r)
	}
	return courseReviewListDTO{
		Reviews: items, Total: p.Total, Breakdown: p.Breakdown[:],
	}
}

// --- moderation ---

type adminCourseReviewDTO struct {
	ID          string    `json:"id"`
	CourseID    string    `json:"course_id"`
	CourseTitle string    `json:"course_title"`
	StudentName string    `json:"student_display_name"`
	Rating      int       `json:"rating"`
	Comment     string    `json:"comment"`
	Hidden      bool      `json:"hidden"`
	CreatedAt   time.Time `json:"created_at"`
}

func toAdminCourseReviewDTO(r AdminReview) adminCourseReviewDTO {
	return adminCourseReviewDTO{
		ID: r.ID.String(), CourseID: r.CourseID.String(), CourseTitle: r.CourseTitle,
		StudentName: r.StudentName, Rating: r.Rating, Comment: r.Comment,
		Hidden: r.Hidden, CreatedAt: r.CreatedAt.UTC(),
	}
}

type adminCourseReviewListDTO struct {
	Reviews []adminCourseReviewDTO `json:"reviews"`
	Total   int                    `json:"total"`
}

func toAdminCourseReviewListDTO(p AdminReviewPage) adminCourseReviewListDTO {
	items := make([]adminCourseReviewDTO, len(p.Reviews))
	for i, r := range p.Reviews {
		items[i] = toAdminCourseReviewDTO(r)
	}
	return adminCourseReviewListDTO{Reviews: items, Total: p.Total}
}

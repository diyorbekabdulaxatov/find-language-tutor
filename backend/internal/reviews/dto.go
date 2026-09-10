package reviews

import "time"

// Wire DTOs. Source of truth for the JSON shape; snake_case, RFC3339 UTC times,
// in sync with openapi.yaml.

// --- request bodies ---

type createReviewRequest struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

// --- POST /v1/bookings/{id}/review response ---

type reviewStudentDTO struct {
	DisplayName string `json:"display_name"`
}

type createdReviewDTO struct {
	ID          string           `json:"id"`
	Rating      int              `json:"rating"`
	Comment     string           `json:"comment"`
	CreatedAt   time.Time        `json:"created_at"`
	Student     reviewStudentDTO `json:"student"`
	TeacherSlug string           `json:"teacher_slug"`
}

func toCreatedReviewDTO(r Review) createdReviewDTO {
	return createdReviewDTO{
		ID:          r.ID.String(),
		Rating:      r.Rating,
		Comment:     r.Comment,
		CreatedAt:   r.CreatedAt.UTC(),
		Student:     reviewStudentDTO{DisplayName: r.StudentName},
		TeacherSlug: r.TeacherSlug,
	}
}

// --- GET /v1/teachers/{slug}/reviews response ---

type reviewListItemDTO struct {
	ID                 string    `json:"id"`
	Rating             int       `json:"rating"`
	Comment            string    `json:"comment"`
	CreatedAt          time.Time `json:"created_at"`
	StudentDisplayName string    `json:"student_display_name"`
}

type reviewListDTO struct {
	Reviews []reviewListItemDTO `json:"reviews"`
	Total   int                 `json:"total"`
}

func toReviewListDTO(p Page) reviewListDTO {
	items := make([]reviewListItemDTO, len(p.Reviews))
	for i, r := range p.Reviews {
		items[i] = reviewListItemDTO{
			ID:                 r.ID.String(),
			Rating:             r.Rating,
			Comment:            r.Comment,
			CreatedAt:          r.CreatedAt.UTC(),
			StudentDisplayName: r.StudentName,
		}
	}
	return reviewListDTO{Reviews: items, Total: p.Total}
}

// --- phase F: GET /v1/admin/reviews + moderation actions ---

type adminReviewTeacherDTO struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
}

type adminReviewDTO struct {
	ID          string                `json:"id"`
	Rating      int                   `json:"rating"`
	Comment     string                `json:"comment"`
	Hidden      bool                  `json:"hidden"`
	CreatedAt   time.Time             `json:"created_at"`
	Teacher     adminReviewTeacherDTO `json:"teacher"`
	StudentName string                `json:"student_display_name"`
	BookingID   *string               `json:"booking_id"`
	// Sample is true for a seeded, booking-less review (part of the historical
	// display set, never folded into the live rating aggregate).
	Sample bool `json:"sample"`
}

func toAdminReviewDTO(r AdminReview) adminReviewDTO {
	out := adminReviewDTO{
		ID:          r.ID.String(),
		Rating:      r.Rating,
		Comment:     r.Comment,
		Hidden:      r.Hidden,
		CreatedAt:   r.CreatedAt.UTC(),
		Teacher:     adminReviewTeacherDTO{Slug: r.TeacherSlug, DisplayName: r.TeacherName},
		StudentName: r.StudentName,
		Sample:      r.BookingID == nil,
	}
	if r.BookingID != nil {
		s := r.BookingID.String()
		out.BookingID = &s
	}
	return out
}

type adminReviewListDTO struct {
	Reviews []adminReviewDTO `json:"reviews"`
	Total   int              `json:"total"`
}

func toAdminReviewListDTO(p AdminPage) adminReviewListDTO {
	items := make([]adminReviewDTO, len(p.Reviews))
	for i, r := range p.Reviews {
		items[i] = toAdminReviewDTO(r)
	}
	return adminReviewListDTO{Reviews: items, Total: p.Total}
}

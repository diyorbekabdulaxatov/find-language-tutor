package resources

import (
	"time"

	"github.com/google/uuid"
)

// Wire DTOs. snake_case, in sync with openapi.yaml. `content` is the domain
// Content marshalled by its own json tags.

type createRequest struct {
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	Instructions string  `json:"instructions"`
	Publish      bool    `json:"publish"`
	Content      Content `json:"content"`
}

type updateRequest struct {
	Title        string  `json:"title"`
	Instructions string  `json:"instructions"`
	Content      Content `json:"content"`
}

type resourceDTO struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	Instructions string    `json:"instructions"`
	Status       string    `json:"status"`
	Archived     bool      `json:"archived"`
	Content      Content   `json:"content"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toResourceDTO(r Resource) resourceDTO {
	return resourceDTO{
		ID:           r.ID.String(),
		Type:         string(r.Type),
		Title:        r.Title,
		Instructions: r.Instructions,
		Status:       string(r.Status),
		Archived:     r.ArchivedAt != nil,
		Content:      r.Content,
		CreatedAt:    r.CreatedAt.UTC(),
		UpdatedAt:    r.UpdatedAt.UTC(),
	}
}

type resourceListDTO struct {
	Resources []resourceDTO `json:"resources"`
	Total     int           `json:"total"`
}

func toResourceListDTO(p Page) resourceListDTO {
	items := make([]resourceDTO, len(p.Resources))
	for i, r := range p.Resources {
		items[i] = toResourceDTO(r)
	}
	return resourceListDTO{Resources: items, Total: p.Total}
}

// --- phase A2: booking attachment ---

type attachResourceRequest struct {
	ResourceID string     `json:"resource_id"`
	Kind       string     `json:"kind"`
	DueAt      *time.Time `json:"due_at"`
}

type submissionSummaryDTO struct {
	ID           string     `json:"id"`
	Status       string     `json:"status"`
	AutoScore    *int       `json:"auto_score"`
	AutoMax      *int       `json:"auto_max"`
	TeacherScore *int       `json:"teacher_score"`
	SubmittedAt  *time.Time `json:"submitted_at"`
	GradedAt     *time.Time `json:"graded_at"`
}

func toSubmissionSummaryDTO(s SubmissionSummary) *submissionSummaryDTO {
	return &submissionSummaryDTO{
		ID: s.ID.String(), Status: s.Status, AutoScore: s.AutoScore, AutoMax: s.AutoMax,
		TeacherScore: s.TeacherScore, SubmittedAt: s.SubmittedAt, GradedAt: s.GradedAt,
	}
}

// attachedResourceDTO is the full booking-attachment view: id/kind/position
// plus the resource's display content (student's Correct fields already
// stripped by the service when the caller is the student) and, for the
// student only, their own submission summary.
type attachedResourceDTO struct {
	ID             string                `json:"id"`
	ResourceID     string                `json:"resource_id"`
	Kind           string                `json:"kind"`
	Position       int                   `json:"position"`
	DueAt          *time.Time            `json:"due_at"`
	Type           string                `json:"type"`
	Title          string                `json:"title"`
	Instructions   string                `json:"instructions"`
	ResourceStatus string                `json:"resource_status"`
	Content        Content               `json:"content"`
	Submission     *submissionSummaryDTO `json:"submission"`
}

func toAttachedResourceDTO(a AttachedResource) attachedResourceDTO {
	dto := attachedResourceDTO{
		ID:             a.ID.String(),
		ResourceID:     a.ResourceID.String(),
		Kind:           a.Kind,
		Position:       a.Position,
		DueAt:          a.DueAt,
		Type:           string(a.Type),
		Title:          a.Title,
		Instructions:   a.Instructions,
		ResourceStatus: string(a.ResourceStatus),
		Content:        a.Content,
	}
	if a.Submission != nil {
		dto.Submission = toSubmissionSummaryDTO(*a.Submission)
	}
	return dto
}

type attachedResourceListDTO struct {
	Attachments []attachedResourceDTO `json:"attachments"`
}

func toAttachedResourceListDTO(items []AttachedResource) attachedResourceListDTO {
	out := make([]attachedResourceDTO, len(items))
	for i, a := range items {
		out[i] = toAttachedResourceDTO(a)
	}
	return attachedResourceListDTO{Attachments: out}
}

// --- phase A3: submissions ---

type startSubmissionRequest struct {
	ResourceID string `json:"resource_id"`
	BookingID  string `json:"booking_id"`
}

type saveAnswersRequest struct {
	Answers map[string][]string `json:"answers"`
}

type gradeSubmissionRequest struct {
	Score    *int   `json:"score"`
	Feedback string `json:"feedback"`
}

type submissionDTO struct {
	ID              string              `json:"id"`
	ResourceID      string              `json:"resource_id"`
	BookingID       string              `json:"booking_id,omitempty"`
	Status          string              `json:"status"`
	Answers         map[string][]string `json:"answers"`
	AutoScore       *int                `json:"auto_score"`
	AutoMax         *int                `json:"auto_max"`
	TeacherScore    *int                `json:"teacher_score"`
	TeacherFeedback string              `json:"teacher_feedback"`
	SubmittedAt     *time.Time          `json:"submitted_at"`
	GradedAt        *time.Time          `json:"graded_at"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

func toSubmissionDTO(s Submission) submissionDTO {
	answers := s.Answers
	if answers == nil {
		answers = map[string][]string{}
	}
	dto := submissionDTO{
		ID:              s.ID.String(),
		ResourceID:      s.ResourceID.String(),
		Status:          s.Status,
		Answers:         answers,
		AutoScore:       s.AutoScore,
		AutoMax:         s.AutoMax,
		TeacherScore:    s.TeacherScore,
		TeacherFeedback: s.TeacherFeedback,
		SubmittedAt:     s.SubmittedAt,
		GradedAt:        s.GradedAt,
		CreatedAt:       s.CreatedAt.UTC(),
		UpdatedAt:       s.UpdatedAt.UTC(),
	}
	if s.BookingID != uuid.Nil {
		dto.BookingID = s.BookingID.String()
	}
	return dto
}

type submissionListDTO struct {
	Submissions []submissionDTO `json:"submissions"`
	Total       int             `json:"total"`
}

func toSubmissionListDTO(p SubmissionPage) submissionListDTO {
	items := make([]submissionDTO, len(p.Submissions))
	for i, s := range p.Submissions {
		items[i] = toSubmissionDTO(s)
	}
	return submissionListDTO{Submissions: items, Total: p.Total}
}

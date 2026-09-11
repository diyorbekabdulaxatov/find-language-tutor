// Package resources is a teacher's reusable library of learning content:
// materials, articles, and graded tasks (quiz / listening / reading / writing).
// A resource is authored once and later attached to a scheduled lesson as
// homework (phase A2) or dropped into a course curriculum (later). This phase
// (A1) is the library + authoring only.
//
// The type-specific payload lives in the `content` JSONB column, shaped by
// `type`:
//
//	material  {file_asset_id? | url?, description?}
//	article   {body}                                  -- markdown
//	quiz      {questions:[...]}
//	listening {audio_asset_id, questions:[...]}
//	reading   {passage, questions:[...]}
//	writing   {prompt, min_words?, rubric?}
//
// A question: {id, prompt, kind: single|multi|text, choices:[{id,text}],
// correct:[choiceId] (or accepted strings for `text`), points}. The `correct`
// answers are stored here; phase A2 strips them from the student-facing view.
//
// The Service validates the content on write. Postgres only guarantees valid
// JSON.
package resources

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Type is the resource kind (matches the resources.type CHECK).
type Type string

const (
	TypeMaterial  Type = "material"
	TypeArticle   Type = "article"
	TypeQuiz      Type = "quiz"
	TypeListening Type = "listening"
	TypeReading   Type = "reading"
	TypeWriting   Type = "writing"
)

func (t Type) valid() bool {
	switch t {
	case TypeMaterial, TypeArticle, TypeQuiz, TypeListening, TypeReading, TypeWriting:
		return true
	}
	return false
}

// quizLike reports whether the type carries a `questions` list.
func (t Type) quizLike() bool {
	return t == TypeQuiz || t == TypeListening || t == TypeReading
}

// Status is the library visibility.
type Status string

const (
	StatusDraft     Status = "draft"     // only the author sees it
	StatusPublished Status = "published" // assignable to lessons / courses
)

// Resource is the domain aggregate.
type Resource struct {
	ID           uuid.UUID
	TeacherID    uuid.UUID
	Type         Type
	Title        string
	Instructions string
	Content      Content
	Status       Status
	ArchivedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Content is the union of every type's payload; only the fields relevant to
// Resource.Type are populated. It round-trips through the `content` JSONB column.
type Content struct {
	// material
	FileAssetID *uuid.UUID `json:"file_asset_id,omitempty"`
	URL         string     `json:"url,omitempty"`
	Description string     `json:"description,omitempty"`

	// article
	Body string `json:"body,omitempty"`

	// quiz / listening / reading
	Passage      string     `json:"passage,omitempty"`
	AudioAssetID *uuid.UUID `json:"audio_asset_id,omitempty"`
	Questions    []Question `json:"questions,omitempty"`

	// writing
	Prompt   string `json:"prompt,omitempty"`
	MinWords int    `json:"min_words,omitempty"`
	Rubric   string `json:"rubric,omitempty"`
}

// Question is one item in a quiz-like resource.
type Question struct {
	ID      string   `json:"id"`
	Prompt  string   `json:"prompt"`
	Kind    string   `json:"kind"` // "single" | "multi" | "text"
	Choices []Choice `json:"choices,omitempty"`
	// Correct holds choice ids (single/multi) or accepted answer strings (text).
	Correct []string `json:"correct"`
	Points  int      `json:"points"`
}

type Choice struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// ListQuery is the validated input to the library list.
type ListQuery struct {
	Type            Type
	Status          Status
	IncludeArchived bool
	Page            int
	PageSize        int
}

// Page is a page of the library plus the total match count.
type Page struct {
	Resources []Resource
	Total     int
}

// submittable reports whether a resource type carries a submission flow
// (quiz / listening / reading auto-grade; writing is teacher-graded).
// material / article are display-only and cannot be assigned as homework.
func (t Type) submittable() bool {
	switch t {
	case TypeQuiz, TypeListening, TypeReading, TypeWriting:
		return true
	}
	return false
}

// --- phase A2: attaching a resource to a booking ---

// Attachment kinds (booking_resources.kind).
const (
	KindMaterial = "material"
	KindHomework = "homework"
)

func validKind(k string) bool { return k == KindMaterial || k == KindHomework }

// BookingResource is one resource attached to a booking, joined with the
// resource's own display fields. SubmissionStatus is populated only when the
// caller resolving it is the booking's student and a submission exists for
// this attachment; "" otherwise (never meaningful for the teacher view).
type BookingResource struct {
	ID         uuid.UUID
	BookingID  uuid.UUID
	ResourceID uuid.UUID
	Kind       string
	Position   int
	AssignedBy uuid.UUID
	DueAt      *time.Time
	CreatedAt  time.Time

	Type           Type
	Title          string
	Instructions   string
	ResourceStatus Status
	Content        Content

	SubmissionStatus string
}

// AttachedResource is a BookingResource plus (for the viewing student only)
// their own submission summary. Returned by GET /v1/bookings/{id}/resources.
type AttachedResource struct {
	BookingResource
	Submission *SubmissionSummary // nil for the teacher, or a student who hasn't started
}

// SubmissionSummary is the light view of a submission embedded next to an
// attached resource.
type SubmissionSummary struct {
	ID           uuid.UUID
	Status       string
	AutoScore    *int
	AutoMax      *int
	TeacherScore *int
	SubmittedAt  *time.Time
	GradedAt     *time.Time
}

// --- phase A3: submissions ---

// Submission statuses.
const (
	SubmissionInProgress = "in_progress"
	SubmissionSubmitted  = "submitted"
	SubmissionGraded     = "graded"
)

// Submission is one student's work against a submittable resource.
// BookingID is uuid.Nil unless Context is "lesson" (the only context phase A3
// writes).
type Submission struct {
	ID         uuid.UUID
	ResourceID uuid.UUID
	StudentID  uuid.UUID
	Context    string
	BookingID  uuid.UUID

	Status  string
	Answers map[string][]string

	AutoScore *int
	AutoMax   *int

	TeacherScore    *int
	TeacherFeedback string
	GradedBy        uuid.UUID
	GradedAt        *time.Time

	SubmittedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SubmitParams is what Submit writes: the auto-graded types go straight to
// `graded` with both scores set; `writing` goes to `submitted` with neither.
type SubmitParams struct {
	Status      string
	AutoScore   *int
	AutoMax     *int
	SubmittedAt time.Time
}

// SubmissionQuery is the validated input to the teacher grading inbox.
type SubmissionQuery struct {
	Status   string // "" defaults to `submitted`; "all" lifts the filter
	Page     int
	PageSize int
}

// SubmissionPage is a page of the grading inbox plus the total match count.
type SubmissionPage struct {
	Submissions []Submission
	Total       int
}

// Domain errors.
var (
	ErrNotFound   = errors.New("resources: resource not found")
	ErrForbidden  = errors.New("resources: not the owner")
	ErrNoTeacher  = errors.New("resources: caller has no teacher profile")
	ErrInUse      = errors.New("resources: resource is assigned and cannot be deleted")
	ErrTypeLocked = errors.New("resources: a resource's type cannot be changed")

	// ErrBookingNotFound — no booking has the requested id (via BookingReader,
	// or a nil BookingReader failing closed).
	ErrBookingNotFound = errors.New("resources: booking not found")

	// ErrAlreadyAttached — this resource is already attached to this booking
	// (the table's UNIQUE(booking_id, resource_id) rejected the insert).
	// Rendered 409.
	ErrAlreadyAttached = errors.New("resources: resource already attached to this booking")

	// ErrAttachmentNotFound — no attachment has the requested id on this booking.
	ErrAttachmentNotFound = errors.New("resources: attachment not found")

	// ErrHomeworkNotAssigned — the resource is not attached to this booking as
	// homework, so a submission cannot start.
	ErrHomeworkNotAssigned = errors.New("resources: this resource is not assigned as homework on this booking")

	// ErrSubmissionNotFound — no submission has the requested id.
	ErrSubmissionNotFound = errors.New("resources: submission not found")

	// ErrInvalidSubmissionState — the submission is not in a state that allows
	// this action (e.g. saving/submitting a non-in_progress submission, or
	// grading a non-submitted one). Rendered 409.
	ErrInvalidSubmissionState = errors.New("resources: submission is not in a state that allows this")
)

// ValidationError is a client-fixable authoring problem, rendered 400.
type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

func invalid(format string, a ...any) error { return ValidationError{fmt.Sprintf(format, a...)} }

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

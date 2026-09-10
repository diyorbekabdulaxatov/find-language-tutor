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

// Domain errors.
var (
	ErrNotFound   = errors.New("resources: resource not found")
	ErrForbidden  = errors.New("resources: not the owner")
	ErrNoTeacher  = errors.New("resources: caller has no teacher profile")
	ErrInUse      = errors.New("resources: resource is assigned and cannot be deleted")
	ErrTypeLocked = errors.New("resources: a resource's type cannot be changed")
)

// ValidationError is a client-fixable authoring problem, rendered 400.
type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

func invalid(format string, a ...any) error { return ValidationError{fmt.Sprintf(format, a...)} }

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

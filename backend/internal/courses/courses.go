// Package courses is a teacher's authoring workspace for self-paced video
// courses. A course groups ordered sections, each holding ordered items that
// are either an uploaded video or a reference to one of the teacher's own
// published resources (internal/resources' library — reused, not duplicated:
// a quiz or reading passage already built for a lesson can be dropped
// straight into a course).
//
// This phase (C1) is authoring only, the same scope discipline
// internal/resources' phase A1 used: no purchase, no catalog/landing page, no
// enrollment, no reviews, no payouts, no student-facing surface. Course
// visibility/access rules for a real (non-owner) viewer are not part of this
// phase.
package courses

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Status is the course's publication state.
type Status string

const (
	StatusDraft     Status = "draft"     // only the author sees it
	StatusPublished Status = "published" // (visibility rules for others: future work)
)

// ItemKind is a curriculum item's payload kind.
type ItemKind string

const (
	ItemKindVideo    ItemKind = "video"
	ItemKindResource ItemKind = "resource"
)

func (k ItemKind) valid() bool {
	return k == ItemKindVideo || k == ItemKindResource
}

// Course is the domain aggregate's root row. Its curriculum (sections and
// their items) is fetched and returned separately — see CourseDetail.
type Course struct {
	ID               uuid.UUID
	TeacherID        uuid.UUID
	Title            string
	Subtitle         string
	Description      string
	CoverAssetID     *uuid.UUID
	PriceAmountMinor int64
	PriceCurrency    string
	Status           Status
	// EverPublished is a one-way latch: true from the first time the course is
	// published, and never cleared by a later unpublish. Delete checks this,
	// not Status, so a course that went live once can never be deleted —
	// archive it instead.
	EverPublished bool
	ArchivedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Section is one top-level curriculum entry, ordered within its course by Position.
type Section struct {
	ID        uuid.UUID
	CourseID  uuid.UUID
	Title     string
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Item is one curriculum leaf, ordered within its section by Position. Exactly
// one of VideoAssetID / ResourceID is set, matching Kind (also enforced by a
// DB CHECK).
type Item struct {
	ID           uuid.UUID
	SectionID    uuid.UUID
	Kind         ItemKind
	Title        string // optional display-title override; "" falls back in the UI
	VideoAssetID *uuid.UUID
	ResourceID   *uuid.UUID
	Position     int
	CreatedAt    time.Time
}

// SectionDetail is one section with its items loaded, in position order.
type SectionDetail struct {
	Section Section
	Items   []Item
}

// CourseDetail is a course with its full curriculum tree loaded. Every
// course-mutating endpoint (create/update/publish/archive, and every
// section/item mutation) returns one, so a client can re-render its whole
// authoring view from a single response.
type CourseDetail struct {
	Course   Course
	Sections []SectionDetail
}

// ListQuery is the validated input to the library list.
type ListQuery struct {
	Status          Status
	IncludeArchived bool
	Page            int
	PageSize        int
}

// Page is a page of the library plus the total match count. Library rows are
// Course only (no curriculum) — cheap to list; GET the course by id for its
// full curriculum tree.
type Page struct {
	Courses []Course
	Total   int
}

// Domain errors.
var (
	ErrNotFound  = errors.New("courses: course not found")
	ErrForbidden = errors.New("courses: not the owner")
	ErrNoTeacher = errors.New("courses: caller has no teacher profile")
	// ErrInUse — the course has been published at least once; delete is
	// blocked forever after that. Archive it instead.
	ErrInUse           = errors.New("courses: course has been published and cannot be deleted")
	ErrSectionNotFound = errors.New("courses: section not found")
	ErrItemNotFound    = errors.New("courses: item not found")
)

// ValidationError is a client-fixable authoring problem, rendered 400.
type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

func invalid(format string, a ...any) error { return ValidationError{fmt.Sprintf(format, a...)} }

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

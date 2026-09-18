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
	"time"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/i18n"
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
	// SuspendedAt (phase C3) is an operator-only takedown, independent of
	// Status/ArchivedAt: non-nil pulls the course from the storefront (catalog,
	// catalog detail, cover image, new purchases) without touching the
	// teacher's own draft/published/archived state and without revoking
	// already-enrolled students' access. Set/cleared only via the
	// courses.moderate admin endpoints.
	SuspendedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
	// IsPreview (phase D1) marks a free sample lecture: anyone may stream it
	// from the course landing page without enrolling. Only a video item may
	// carry it — a resource item's payload has its own answer-stripping path
	// that has no business running for an anonymous viewer — and both this
	// service and a DB CHECK enforce that.
	IsPreview bool
	// DurationSeconds (phase D1) is the video's length, reported by the
	// client from the browser's own <video> metadata. Display metadata only:
	// it never gates access, pricing or payouts, so it is validated for
	// plausibility rather than trusted. 0 means "unknown" and every surface
	// omits the figure rather than rendering "0m".
	DurationSeconds int
	CreatedAt       time.Time
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
	// ErrTeacherNotApproved — the owning teacher profile is not (or no longer)
	// approved, so the course can't go on the storefront.
	ErrTeacherNotApproved = errors.New("courses: teacher profile is not approved")
	// ErrEmailNotVerified — the owning account has not confirmed its email;
	// publishing is refused (403 email_not_verified).
	ErrEmailNotVerified = errors.New("courses: email not verified")
	// ErrInUse — the course has been published at least once; delete is
	// blocked forever after that. Archive it instead.
	ErrInUse           = errors.New("courses: course has been published and cannot be deleted")
	ErrSectionNotFound = errors.New("courses: section not found")
	ErrItemNotFound    = errors.New("courses: item not found")
)

// ValidationError is a client-fixable authoring problem, rendered 400.
type ValidationError struct{ i18n.Msg }

func (e ValidationError) Error() string { return e.Msg.String() }

func invalid(format string, a ...any) error { return ValidationError{i18n.Message(format, a...)} }

const (
	defaultPageSize = 20
	maxPageSize     = 100

	// MaxItemDurationSeconds is the plausibility ceiling on a reported video
	// length (24h). A real lecture is minutes long; this only exists so a
	// broken or hostile client can't store an absurd number that would
	// render as "8760 hours" on the course card.
	MaxItemDurationSeconds = 24 * 60 * 60
)

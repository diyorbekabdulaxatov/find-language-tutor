// Phase C2 domain types: the public catalog, purchase/enrollment, the
// enrolled-student player, and progress tracking. Builds on phase C1's
// authoring types in courses.go.
package courses

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Domain errors (phase C2).
var (
	// ErrCannotBuyOwnCourse — the caller's own teacher profile owns the
	// course. Rendered 403.
	ErrCannotBuyOwnCourse = errors.New("courses: you can't buy your own course")

	// ErrPurchaseUnavailable — a priced course's Purchase was called with no
	// PaymentGateway wired (SetPaymentGateway never called). A free course
	// still enrolls fine without one. Rendered 503.
	ErrPurchaseUnavailable = errors.New("courses: course purchases aren't available right now")
)

// EnrollmentSource records how a student came to own a course.
type EnrollmentSource string

const (
	EnrollmentPurchase EnrollmentSource = "purchase"
	EnrollmentFree     EnrollmentSource = "free"
)

// Enrollment is one student's access grant to one course. AmountPaidMinor /
// Currency snapshot the price actually paid at purchase time (0 / the
// course's currency for a free enrollment), so a later price change never
// rewrites history.
type Enrollment struct {
	ID              uuid.UUID
	CourseID        uuid.UUID
	StudentID       uuid.UUID
	Source          EnrollmentSource
	AmountPaidMinor int64
	Currency        string
	CreatedAt       time.Time
}

// EnrollmentSummary is one "my learning" row: the enrollment, its course
// summary, and a computed progress percent (CompletedItems / TotalItems).
type EnrollmentSummary struct {
	Enrollment     Enrollment
	Course         Course
	TotalItems     int
	CompletedItems int
}

// ItemStatus is a curriculum item's per-enrollment progress state.
type ItemStatus string

const (
	ItemInProgress ItemStatus = "in_progress"
	ItemCompleted  ItemStatus = "completed"
)

// ItemProgress is one enrollment's progress on one curriculum item.
type ItemProgress struct {
	ID                   uuid.UUID
	EnrollmentID         uuid.UUID
	ItemID               uuid.UUID
	Status               ItemStatus
	VideoPositionSeconds int
	CompletedAt          *time.Time
	UpdatedAt            time.Time
}

// TeacherSummary is the light teacher identity embedded in catalog rows.
type TeacherSummary struct {
	ID          uuid.UUID
	DisplayName string
	Slug        string
}

// CatalogQuery is the validated input to the public catalog list.
type CatalogQuery struct {
	Q             string
	MaxPriceMinor *int64
	Sort          string // "newest" (default) | "price_asc" | "price_desc"
	Page          int
	PageSize      int
}

// CatalogEntry is one public catalog row: course summary + teacher summary +
// curriculum size, with no curriculum detail.
type CatalogEntry struct {
	Course       Course
	Teacher      TeacherSummary
	SectionCount int
	ItemCount    int
}

// CatalogPage is a page of the public catalog plus the total match count.
type CatalogPage struct {
	Entries []CatalogEntry
	Total   int
}

// ItemOutline is a curriculum item's public, pre-purchase outline: enough to
// show the curriculum's shape (title, kind, position) without leaking a video
// asset id or resource content.
type ItemOutline struct {
	ID       uuid.UUID
	Kind     ItemKind
	Title    string
	Position int
}

// SectionOutline is a curriculum section's public outline.
type SectionOutline struct {
	ID       uuid.UUID
	Title    string
	Position int
	Items    []ItemOutline
}

// CatalogDetail is the public course landing page: course + teacher summary,
// the curriculum outline only (no video URLs, no resource content), and the
// viewer's relationship to the course.
type CatalogDetail struct {
	Course     Course
	Teacher    TeacherSummary
	Outline    []SectionOutline
	IsEnrolled bool
	IsOwner    bool
}

// CourseResourceView is a published resource's student-safe view (correct
// answers already stripped) embedded in the course player. courses has no
// dependency on resources.Content's concrete type — Content is opaque JSON,
// produced by resources.NewCourseGateway calling the resource's own
// content-stripping method and marshalling the result.
type CourseResourceView struct {
	ID           uuid.UUID
	Type         string
	Title        string
	Instructions string
	Content      json.RawMessage
}

// LearnItem is one curriculum item in the enrolled-student player: a video
// item carries VideoAssetID (the frontend fetches bytes via GET
// /v1/files/{id}); a resource item carries Resource. Progress is always
// populated — zeroed / not-started when there is no progress row yet (always
// the case for the owner-preview view, which never has an enrollment).
type LearnItem struct {
	Item     Item
	Resource *CourseResourceView // set only for ItemKindResource
	Progress ItemProgress
}

// LearnSection is one curriculum section in the enrolled-student player.
type LearnSection struct {
	Section Section
	Items   []LearnItem
}

// LearnDetail is the full curriculum tree for the enrolled-student (or
// owner-preview) player.
type LearnDetail struct {
	Course   Course
	Sections []LearnSection
}

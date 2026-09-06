// Package teachers is the teacher-profiles domain module: search, filtering, and
// full profile reads. It owns its domain types, a service with the business
// rules, a repository port, and gin handlers.
package teachers

import "github.com/google/uuid"

type Kind string

const (
	KindProfessional Kind = "professional"
	KindCommunity    Kind = "community"
)

func (k Kind) valid() bool { return k == KindProfessional || k == KindCommunity }

type Level string // "native", "c2" … "a1"

type Currency string

const (
	CurrencyUZS Currency = "UZS"
	CurrencyUSD Currency = "USD"
)

// Money is an amount in minor units (tiyin for UZS, cents for USD), matching how
// Stripe represents amounts and avoiding floating-point on money.
type Money struct {
	AmountMinor int64
	Currency    Currency
}

type Language struct {
	Code  string // ISO 639-1
	Name  string
	Level Level
}

type Experience struct {
	Title  string
	Org    string
	Period string
}

// Teacher is the full domain aggregate. The list view uses a subset of these
// fields (see the summary DTO in handler.go); the profile view uses all of them.
type Teacher struct {
	ID          uuid.UUID
	Slug        string
	DisplayName string
	Headline    string
	Kind        Kind

	CountryCode string
	CountryName string
	City        string
	Timezone    string

	PricePerHour Money
	TrialPrice   *Money // nil when no trial lesson is offered

	Rating            float64
	ReviewCount       int
	LessonsCompleted  int
	StudentCount      int
	ResponseTimeHours int
	AcceptingStudents bool

	AvatarURL         string
	VideoThumbnailURL string
	IntroVideoURL     string
	About             string
	TeachingStyle     string

	Teaches    []Language
	AlsoSpeaks []Language
	Focus      []string // free-text focus tags, e.g. "IELTS", "Kids & teens"
	Experience []Experience
}

type Sort string

const (
	SortRecommended Sort = "recommended"
	SortPriceAsc    Sort = "price_asc"
	SortPriceDesc   Sort = "price_desc"
	SortRatingDesc  Sort = "rating_desc"
)

func (s Sort) valid() bool {
	switch s {
	case SortRecommended, SortPriceAsc, SortPriceDesc, SortRatingDesc:
		return true
	default:
		return false
	}
}

// ListParams are the validated inputs to a teacher search.
type ListParams struct {
	Language      string // taught-language code, e.g. "en"
	Kind          Kind   // "" means any
	MaxPriceMinor *int64 // nil means no ceiling
	Q             string // substring across name / headline / focus
	Sort          Sort
	Page          int // 1-based
	PageSize      int
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// normalized returns a copy with defaults applied and values clamped to safe
// ranges. Invalid enum values fall back to their defaults rather than erroring —
// the handler is responsible for rejecting genuinely malformed requests.
func (p ListParams) normalized() ListParams {
	out := p
	if !out.Sort.valid() {
		out.Sort = SortRecommended
	}
	if out.Kind != "" && !out.Kind.valid() {
		out.Kind = ""
	}
	if out.Page < 1 {
		out.Page = 1
	}
	if out.PageSize <= 0 {
		out.PageSize = defaultPageSize
	}
	if out.PageSize > maxPageSize {
		out.PageSize = maxPageSize
	}
	if out.MaxPriceMinor != nil && *out.MaxPriceMinor < 0 {
		out.MaxPriceMinor = nil
	}
	return out
}

func (p ListParams) offset() int32 { return int32((p.Page - 1) * p.PageSize) }

// LanguageFacet is one row of the taught-language filter.
type LanguageFacet struct {
	Code  string
	Name  string
	Count int
}

type Facets struct {
	Languages []LanguageFacet
}

// ListResult is a page of search results plus the catalog-wide facets the UI
// needs to render its filters.
type ListResult struct {
	Teachers []Teacher
	Total    int
	Facets   Facets
}

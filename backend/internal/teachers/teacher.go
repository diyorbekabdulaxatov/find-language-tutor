// Package teachers is the teacher-profiles domain module: search, filtering, and
// full profile reads. It owns its domain types, a service with the business
// rules, a repository port, and gin handlers.
package teachers

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Kind string

const (
	KindProfessional Kind = "professional"
	KindCommunity    Kind = "community"
)

func (k Kind) valid() bool { return k == KindProfessional || k == KindCommunity }

type Level string // "native", "c2" … "a1"

const (
	LevelNative Level = "native"
	LevelC2     Level = "c2"
	LevelC1     Level = "c1"
	LevelB2     Level = "b2"
	LevelB1     Level = "b1"
	LevelA2     Level = "a2"
	LevelA1     Level = "a1"
)

func (l Level) valid() bool {
	switch l {
	case LevelNative, LevelC2, LevelC1, LevelB2, LevelB1, LevelA2, LevelA1:
		return true
	default:
		return false
	}
}

// LanguageRole is a row's role in the teacher_languages table.
type LanguageRole string

const (
	RoleTeaches    LanguageRole = "teaches"
	RoleAlsoSpeaks LanguageRole = "also_speaks"
)

func (r LanguageRole) valid() bool { return r == RoleTeaches || r == RoleAlsoSpeaks }

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

// --- profile writes (POST /v1/teachers, PATCH /v1/teachers/{slug}) ---

// Ref is the slice of a teacher the write endpoints need for their ownership
// check: the id and the owning account (uuid.Nil when the profile is unclaimed).
type Ref struct {
	ID      uuid.UUID
	Slug    string
	OwnerID uuid.UUID
}

// LanguageEntry is one row of the languages collection in a write request. Role
// says whether the teacher teaches the language or merely also speaks it.
type LanguageEntry struct {
	Role  LanguageRole
	Code  string
	Name  string
	Level Level
}

// ProfileInput is the full set of caller-editable profile fields. Create takes
// it directly; Update builds it by merging a ProfilePatch onto the current
// profile so the same validation runs for both.
type ProfileInput struct {
	DisplayName       string
	Headline          string
	Kind              Kind
	CountryCode       string
	CountryName       string
	City              string
	Timezone          string
	PricePerHourMinor int64
	TrialPriceMinor   *int64 // nil = no trial lesson offered
	Currency          Currency
	About             string
	TeachingStyle     string
	AvatarURL         string
	IntroVideoURL     string
	VideoThumbnailURL string
	Languages         []LanguageEntry
	Focus             []string
	Experience        []Experience
}

// ProfilePatch is a partial edit: a nil pointer / slice means "field absent,
// leave unchanged". A non-nil slice (even empty) replaces that whole child
// collection.
type ProfilePatch struct {
	DisplayName       *string
	Headline          *string
	Kind              *string
	CountryCode       *string
	CountryName       *string
	City              *string
	Timezone          *string
	PricePerHourMinor *int64
	TrialPriceMinor   *int64
	Currency          *string
	About             *string
	TeachingStyle     *string
	AvatarURL         *string
	IntroVideoURL     *string
	VideoThumbnailURL *string
	Languages         *[]LanguageEntry
	Focus             *[]string
	Experience        *[]Experience
}

// ProfileUpdate is what the repository needs to persist an edit: the fully
// merged scalar fields, plus a flag per child collection saying whether to
// replace it.
type ProfileUpdate struct {
	Fields            ProfileInput
	ReplaceLanguages  bool
	ReplaceFocus      bool
	ReplaceExperience bool
}

// mergePatch applies a ProfilePatch onto the current profile, returning the full
// merged ProfileInput. It never touches server-controlled fields.
func mergePatch(cur *Teacher, p ProfilePatch) ProfileInput {
	in := ProfileInput{
		DisplayName:       cur.DisplayName,
		Headline:          cur.Headline,
		Kind:              cur.Kind,
		CountryCode:       cur.CountryCode,
		CountryName:       cur.CountryName,
		City:              cur.City,
		Timezone:          cur.Timezone,
		PricePerHourMinor: cur.PricePerHour.AmountMinor,
		Currency:          cur.PricePerHour.Currency,
		About:             cur.About,
		TeachingStyle:     cur.TeachingStyle,
		AvatarURL:         cur.AvatarURL,
		IntroVideoURL:     cur.IntroVideoURL,
		VideoThumbnailURL: cur.VideoThumbnailURL,
	}
	if cur.TrialPrice != nil {
		v := cur.TrialPrice.AmountMinor
		in.TrialPriceMinor = &v
	}
	// Current child collections carry through unless the patch replaces them.
	for _, l := range cur.Teaches {
		in.Languages = append(in.Languages, LanguageEntry{Role: RoleTeaches, Code: l.Code, Name: l.Name, Level: l.Level})
	}
	for _, l := range cur.AlsoSpeaks {
		in.Languages = append(in.Languages, LanguageEntry{Role: RoleAlsoSpeaks, Code: l.Code, Name: l.Name, Level: l.Level})
	}
	in.Focus = append(in.Focus, cur.Focus...)
	in.Experience = append(in.Experience, cur.Experience...)

	if p.DisplayName != nil {
		in.DisplayName = *p.DisplayName
	}
	if p.Headline != nil {
		in.Headline = *p.Headline
	}
	if p.Kind != nil {
		in.Kind = Kind(*p.Kind)
	}
	if p.CountryCode != nil {
		in.CountryCode = *p.CountryCode
	}
	if p.CountryName != nil {
		in.CountryName = *p.CountryName
	}
	if p.City != nil {
		in.City = *p.City
	}
	if p.Timezone != nil {
		in.Timezone = *p.Timezone
	}
	if p.PricePerHourMinor != nil {
		in.PricePerHourMinor = *p.PricePerHourMinor
	}
	if p.TrialPriceMinor != nil {
		v := *p.TrialPriceMinor
		in.TrialPriceMinor = &v
	}
	if p.Currency != nil {
		in.Currency = Currency(*p.Currency)
	}
	if p.About != nil {
		in.About = *p.About
	}
	if p.TeachingStyle != nil {
		in.TeachingStyle = *p.TeachingStyle
	}
	if p.AvatarURL != nil {
		in.AvatarURL = *p.AvatarURL
	}
	if p.IntroVideoURL != nil {
		in.IntroVideoURL = *p.IntroVideoURL
	}
	if p.VideoThumbnailURL != nil {
		in.VideoThumbnailURL = *p.VideoThumbnailURL
	}
	if p.Languages != nil {
		in.Languages = *p.Languages
	}
	if p.Focus != nil {
		in.Focus = *p.Focus
	}
	if p.Experience != nil {
		in.Experience = *p.Experience
	}
	return in
}

// ValidationError is a client-fixable problem with a write request. The handler
// renders it as a 400; anything else from the service is a 500.
type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return ValidationError{msg: fmt.Sprintf(format, args...)}
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }

// validateProfile checks a fully-populated ProfileInput. Shared by create and
// update so both endpoints enforce the same rules.
func validateProfile(in ProfileInput) error {
	required := []struct{ name, val string }{
		{"display_name", in.DisplayName},
		{"headline", in.Headline},
		{"country_code", in.CountryCode},
		{"country_name", in.CountryName},
		{"city", in.City},
		{"timezone", in.Timezone},
	}
	for _, f := range required {
		if blank(f.val) {
			return invalid("%s is required.", f.name)
		}
	}

	if !in.Kind.valid() {
		return invalid("kind must be %q or %q.", KindProfessional, KindCommunity)
	}
	if in.Currency != CurrencyUZS {
		return invalid("currency must be %q.", CurrencyUZS)
	}
	if in.PricePerHourMinor < 0 {
		return invalid("price_per_hour_minor must be >= 0.")
	}
	if in.TrialPriceMinor != nil && *in.TrialPriceMinor < 0 {
		return invalid("trial_price_minor must be >= 0.")
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return invalid("timezone %q is not a valid IANA timezone.", in.Timezone)
	}

	seen := make(map[string]bool, len(in.Languages))
	for i, l := range in.Languages {
		if !l.Role.valid() {
			return invalid("languages[%d]: role must be %q or %q.", i, RoleTeaches, RoleAlsoSpeaks)
		}
		if blank(l.Code) || blank(l.Name) {
			return invalid("languages[%d]: code and name are required.", i)
		}
		if !l.Level.valid() {
			return invalid("languages[%d]: level %q is not valid.", i, l.Level)
		}
		key := string(l.Role) + "\x00" + l.Code
		if seen[key] {
			return invalid("languages[%d]: duplicate %s entry for %q.", i, l.Role, l.Code)
		}
		seen[key] = true
	}

	for i, tag := range in.Focus {
		if blank(tag) {
			return invalid("focus[%d]: tag must not be empty.", i)
		}
	}
	for i, e := range in.Experience {
		if blank(e.Title) || blank(e.Org) || blank(e.Period) {
			return invalid("experience[%d]: title, org and period are required.", i)
		}
	}
	return nil
}

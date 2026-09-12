package courses

import (
	"encoding/json"
	"time"
)

// Wire DTOs, phase C2: catalog, purchase/enrollment, the player, progress.
// snake_case, in sync with openapi.yaml.

// --- catalog ---

type courseTeacherSummaryDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Slug        string `json:"slug"`
}

func toCourseTeacherSummaryDTO(t TeacherSummary) courseTeacherSummaryDTO {
	return courseTeacherSummaryDTO{ID: t.ID.String(), DisplayName: t.DisplayName, Slug: t.Slug}
}

type catalogEntryDTO struct {
	ID           string                  `json:"id"`
	Title        string                  `json:"title"`
	Subtitle     string                  `json:"subtitle"`
	CoverAssetID *string                 `json:"cover_asset_id"`
	Price        moneyDTO                `json:"price"`
	Teacher      courseTeacherSummaryDTO `json:"teacher"`
	SectionCount int                     `json:"section_count"`
	ItemCount    int                     `json:"item_count"`
}

func toCatalogEntryDTO(e CatalogEntry) catalogEntryDTO {
	dto := catalogEntryDTO{
		ID: e.Course.ID.String(), Title: e.Course.Title, Subtitle: e.Course.Subtitle,
		Price:        moneyDTO{AmountMinor: e.Course.PriceAmountMinor, Currency: e.Course.PriceCurrency},
		Teacher:      toCourseTeacherSummaryDTO(e.Teacher),
		SectionCount: e.SectionCount, ItemCount: e.ItemCount,
	}
	if e.Course.CoverAssetID != nil {
		v := e.Course.CoverAssetID.String()
		dto.CoverAssetID = &v
	}
	return dto
}

type catalogListDTO struct {
	Courses []catalogEntryDTO `json:"courses"`
	Total   int               `json:"total"`
}

func toCatalogListDTO(p CatalogPage) catalogListDTO {
	items := make([]catalogEntryDTO, len(p.Entries))
	for i, e := range p.Entries {
		items[i] = toCatalogEntryDTO(e)
	}
	return catalogListDTO{Courses: items, Total: p.Total}
}

type catalogItemOutlineDTO struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	Position int    `json:"position"`
}

type catalogSectionOutlineDTO struct {
	ID       string                  `json:"id"`
	Title    string                  `json:"title"`
	Position int                     `json:"position"`
	Items    []catalogItemOutlineDTO `json:"items"`
}

func toCatalogSectionOutlineDTO(sec SectionOutline) catalogSectionOutlineDTO {
	items := make([]catalogItemOutlineDTO, len(sec.Items))
	for i, it := range sec.Items {
		items[i] = catalogItemOutlineDTO{ID: it.ID.String(), Kind: string(it.Kind), Title: it.Title, Position: it.Position}
	}
	return catalogSectionOutlineDTO{ID: sec.ID.String(), Title: sec.Title, Position: sec.Position, Items: items}
}

type catalogDetailDTO struct {
	ID           string                     `json:"id"`
	Title        string                     `json:"title"`
	Subtitle     string                     `json:"subtitle"`
	Description  string                     `json:"description"`
	CoverAssetID *string                    `json:"cover_asset_id"`
	Price        moneyDTO                   `json:"price"`
	Teacher      courseTeacherSummaryDTO    `json:"teacher"`
	Sections     []catalogSectionOutlineDTO `json:"sections"`
	IsEnrolled   bool                       `json:"is_enrolled"`
	IsOwner      bool                       `json:"is_owner"`
}

func toCatalogDetailDTO(d CatalogDetail) catalogDetailDTO {
	sections := make([]catalogSectionOutlineDTO, len(d.Outline))
	for i, sec := range d.Outline {
		sections[i] = toCatalogSectionOutlineDTO(sec)
	}
	dto := catalogDetailDTO{
		ID: d.Course.ID.String(), Title: d.Course.Title, Subtitle: d.Course.Subtitle, Description: d.Course.Description,
		Price:      moneyDTO{AmountMinor: d.Course.PriceAmountMinor, Currency: d.Course.PriceCurrency},
		Teacher:    toCourseTeacherSummaryDTO(d.Teacher),
		Sections:   sections,
		IsEnrolled: d.IsEnrolled, IsOwner: d.IsOwner,
	}
	if d.Course.CoverAssetID != nil {
		v := d.Course.CoverAssetID.String()
		dto.CoverAssetID = &v
	}
	return dto
}

// --- purchase / enrollment ---

type purchaseCourseRequest struct {
	MethodToken string `json:"method_token"`
}

type courseEnrollmentDTO struct {
	ID              string    `json:"id"`
	Course          courseDTO `json:"course"`
	Source          string    `json:"source"`
	AmountPaid      moneyDTO  `json:"amount_paid"`
	ProgressPercent int       `json:"progress_percent"`
	CreatedAt       time.Time `json:"created_at"`
}

func toCourseEnrollmentDTO(s EnrollmentSummary) courseEnrollmentDTO {
	return courseEnrollmentDTO{
		ID:     s.Enrollment.ID.String(),
		Course: toCourseDTO(s.Course),
		Source: string(s.Enrollment.Source),
		AmountPaid: moneyDTO{
			AmountMinor: s.Enrollment.AmountPaidMinor,
			Currency:    s.Enrollment.Currency,
		},
		ProgressPercent: progressPercent(s.TotalItems, s.CompletedItems),
		CreatedAt:       s.Enrollment.CreatedAt.UTC(),
	}
}

type courseEnrollmentListDTO struct {
	Enrollments []courseEnrollmentDTO `json:"enrollments"`
}

func toCourseEnrollmentListDTO(rows []EnrollmentSummary) courseEnrollmentListDTO {
	items := make([]courseEnrollmentDTO, len(rows))
	for i, r := range rows {
		items[i] = toCourseEnrollmentDTO(r)
	}
	return courseEnrollmentListDTO{Enrollments: items}
}

func progressPercent(total, completed int) int {
	if total <= 0 {
		return 0
	}
	return completed * 100 / total
}

// --- the player ---

type itemProgressDTO struct {
	Status               string     `json:"status"`
	VideoPositionSeconds int        `json:"video_position_seconds"`
	CompletedAt          *time.Time `json:"completed_at"`
}

func toItemProgressDTO(p ItemProgress) itemProgressDTO {
	return itemProgressDTO{
		Status: string(p.Status), VideoPositionSeconds: p.VideoPositionSeconds, CompletedAt: p.CompletedAt,
	}
}

type courseResourceViewDTO struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Title        string          `json:"title"`
	Instructions string          `json:"instructions"`
	Content      json.RawMessage `json:"content"`
}

func toCourseResourceViewDTO(v CourseResourceView) courseResourceViewDTO {
	return courseResourceViewDTO{ID: v.ID.String(), Type: v.Type, Title: v.Title, Instructions: v.Instructions, Content: v.Content}
}

type learnItemDTO struct {
	ID           string                 `json:"id"`
	Kind         string                 `json:"kind"`
	Title        string                 `json:"title"`
	Position     int                    `json:"position"`
	VideoAssetID *string                `json:"video_asset_id"`
	Resource     *courseResourceViewDTO `json:"resource"`
	Progress     itemProgressDTO        `json:"progress"`
}

func toLearnItemDTO(li LearnItem) learnItemDTO {
	dto := learnItemDTO{
		ID: li.Item.ID.String(), Kind: string(li.Item.Kind), Title: li.Item.Title, Position: li.Item.Position,
		Progress: toItemProgressDTO(li.Progress),
	}
	if li.Item.VideoAssetID != nil {
		v := li.Item.VideoAssetID.String()
		dto.VideoAssetID = &v
	}
	if li.Resource != nil {
		v := toCourseResourceViewDTO(*li.Resource)
		dto.Resource = &v
	}
	return dto
}

type learnSectionDTO struct {
	ID    string         `json:"id"`
	Title string         `json:"title"`
	Items []learnItemDTO `json:"items"`
}

func toLearnSectionDTO(sec LearnSection) learnSectionDTO {
	items := make([]learnItemDTO, len(sec.Items))
	for i, it := range sec.Items {
		items[i] = toLearnItemDTO(it)
	}
	return learnSectionDTO{ID: sec.Section.ID.String(), Title: sec.Section.Title, Items: items}
}

type learnDetailDTO struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Sections []learnSectionDTO `json:"sections"`
}

func toLearnDetailDTO(d LearnDetail) learnDetailDTO {
	sections := make([]learnSectionDTO, len(d.Sections))
	for i, sec := range d.Sections {
		sections[i] = toLearnSectionDTO(sec)
	}
	return learnDetailDTO{ID: d.Course.ID.String(), Title: d.Course.Title, Sections: sections}
}

type recordProgressRequest struct {
	PositionSeconds *int  `json:"position_seconds"`
	Completed       *bool `json:"completed"`
}

package courses

import (
	"time"

	"github.com/google/uuid"
)

// Wire DTOs. snake_case, in sync with openapi.yaml.

type moneyDTO struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

type createCourseRequest struct {
	Title            string `json:"title"`
	Subtitle         string `json:"subtitle"`
	Description      string `json:"description"`
	PriceAmountMinor int64  `json:"price_amount_minor"`
	PriceCurrency    string `json:"price_currency"`
}

type updateCourseRequest struct {
	Title            string  `json:"title"`
	Subtitle         string  `json:"subtitle"`
	Description      string  `json:"description"`
	CoverAssetID     *string `json:"cover_asset_id"`
	PriceAmountMinor int64   `json:"price_amount_minor"`
	PriceCurrency    string  `json:"price_currency"`
}

type itemDTO struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	VideoAssetID *string   `json:"video_asset_id"`
	ResourceID   *string   `json:"resource_id"`
	Position     int       `json:"position"`
	CreatedAt    time.Time `json:"created_at"`
}

func toItemDTO(it Item) itemDTO {
	dto := itemDTO{
		ID: it.ID.String(), Kind: string(it.Kind), Title: it.Title,
		Position: it.Position, CreatedAt: it.CreatedAt.UTC(),
	}
	if it.VideoAssetID != nil {
		v := it.VideoAssetID.String()
		dto.VideoAssetID = &v
	}
	if it.ResourceID != nil {
		v := it.ResourceID.String()
		dto.ResourceID = &v
	}
	return dto
}

type sectionDTO struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Position  int       `json:"position"`
	Items     []itemDTO `json:"items"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toSectionDTO(sd SectionDetail) sectionDTO {
	items := make([]itemDTO, len(sd.Items))
	for i, it := range sd.Items {
		items[i] = toItemDTO(it)
	}
	return sectionDTO{
		ID: sd.Section.ID.String(), Title: sd.Section.Title, Position: sd.Section.Position,
		Items: items, CreatedAt: sd.Section.CreatedAt.UTC(), UpdatedAt: sd.Section.UpdatedAt.UTC(),
	}
}

// courseDTO is the lightweight library-list row: course fields only, no curriculum.
type courseDTO struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Subtitle     string    `json:"subtitle"`
	Description  string    `json:"description"`
	CoverAssetID *string   `json:"cover_asset_id"`
	Price        moneyDTO  `json:"price"`
	Status       string    `json:"status"`
	Archived     bool      `json:"archived"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toCourseDTO(c Course) courseDTO {
	dto := courseDTO{
		ID: c.ID.String(), Title: c.Title, Subtitle: c.Subtitle, Description: c.Description,
		Price:     moneyDTO{AmountMinor: c.PriceAmountMinor, Currency: c.PriceCurrency},
		Status:    string(c.Status),
		Archived:  c.ArchivedAt != nil,
		CreatedAt: c.CreatedAt.UTC(), UpdatedAt: c.UpdatedAt.UTC(),
	}
	if c.CoverAssetID != nil {
		v := c.CoverAssetID.String()
		dto.CoverAssetID = &v
	}
	return dto
}

type courseListDTO struct {
	Courses []courseDTO `json:"courses"`
	Total   int         `json:"total"`
}

func toCourseListDTO(p Page) courseListDTO {
	items := make([]courseDTO, len(p.Courses))
	for i, c := range p.Courses {
		items[i] = toCourseDTO(c)
	}
	return courseListDTO{Courses: items, Total: p.Total}
}

// courseDetailDTO is the full response shape for every course- and
// curriculum-mutating endpoint: the course plus its section/item tree, so a
// client can re-render its whole authoring view from one response.
type courseDetailDTO struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Subtitle     string       `json:"subtitle"`
	Description  string       `json:"description"`
	CoverAssetID *string      `json:"cover_asset_id"`
	Price        moneyDTO     `json:"price"`
	Status       string       `json:"status"`
	Archived     bool         `json:"archived"`
	Sections     []sectionDTO `json:"sections"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

func toCourseDetailDTO(d CourseDetail) courseDetailDTO {
	sections := make([]sectionDTO, len(d.Sections))
	for i, sd := range d.Sections {
		sections[i] = toSectionDTO(sd)
	}
	dto := courseDetailDTO{
		ID: d.Course.ID.String(), Title: d.Course.Title, Subtitle: d.Course.Subtitle, Description: d.Course.Description,
		Price:     moneyDTO{AmountMinor: d.Course.PriceAmountMinor, Currency: d.Course.PriceCurrency},
		Status:    string(d.Course.Status),
		Archived:  d.Course.ArchivedAt != nil,
		Sections:  sections,
		CreatedAt: d.Course.CreatedAt.UTC(), UpdatedAt: d.Course.UpdatedAt.UTC(),
	}
	if d.Course.CoverAssetID != nil {
		v := d.Course.CoverAssetID.String()
		dto.CoverAssetID = &v
	}
	return dto
}

// --- sections ---

type createSectionRequest struct {
	Title string `json:"title"`
}

type updateSectionRequest struct {
	Title string `json:"title"`
}

type reorderSectionsRequest struct {
	SectionIDs []string `json:"section_ids"`
}

// --- items ---

type createItemRequest struct {
	Kind         string  `json:"kind"`
	Title        string  `json:"title"`
	VideoAssetID *string `json:"video_asset_id"`
	ResourceID   *string `json:"resource_id"`
}

type updateItemRequest struct {
	Title string `json:"title"`
}

type reorderItemsRequest struct {
	ItemIDs []string `json:"item_ids"`
}

// parseOptionalUUID parses s as a uuid.UUID when non-empty; nil in, nil out.
func parseOptionalUUID(s *string) (*uuid.UUID, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// parseUUIDs parses every string in ss as a uuid.UUID, failing on the first bad one.
func parseUUIDs(ss []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, len(ss))
	for i, s := range ss {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, err
		}
		out[i] = id
	}
	return out, nil
}

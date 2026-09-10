package resources

import "time"

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

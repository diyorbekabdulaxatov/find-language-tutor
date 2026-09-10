package payouts

import (
	"time"
)

// Wire DTOs. Source of truth for the JSON shape; snake_case, RFC3339 UTC times,
// money in integer minor units, in sync with openapi.yaml.

type teacherRefDTO struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
}

type userRefDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type owedRowDTO struct {
	Teacher           teacherRefDTO `json:"teacher"`
	AvailableMinor    int64         `json:"available_minor"`
	Currency          string        `json:"currency"`
	OldestAvailableAt time.Time     `json:"oldest_available_at"`
}

type totalsDTO struct {
	AvailableTotalMinor int64  `json:"available_total_minor"`
	HeldTotalMinor      int64  `json:"held_total_minor"`
	PaidTotalMinor      int64  `json:"paid_total_minor"`
	Currency            string `json:"currency"`
}

type batchDTO struct {
	ID           string     `json:"id"`
	CreatedBy    userRefDTO `json:"created_by"`
	Status       string     `json:"status"`
	TotalMinor   int64      `json:"total_minor"`
	Currency     string     `json:"currency"`
	TeacherCount int        `json:"teacher_count"`
	LineCount    int        `json:"line_count"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}

type batchLineDTO struct {
	Teacher     teacherRefDTO `json:"teacher"`
	AmountMinor int64         `json:"amount_minor"`
	Currency    string        `json:"currency"`
	LessonCount int           `json:"lesson_count"`
}

// batchDetailDTO is one batch with its per-teacher lines inlined, the same
// shape-plus-detail pattern the admin booking detail uses.
type batchDetailDTO struct {
	batchDTO
	Lines []batchLineDTO `json:"lines"`
}

type dashboardDTO struct {
	Owed         []owedRowDTO `json:"owed"`
	Totals       totalsDTO    `json:"totals"`
	Batches      []batchDTO   `json:"batches"`
	BatchesTotal int          `json:"batches_total"`
}

// --- mapping ---

func toBatchDTO(b Batch) batchDTO {
	out := batchDTO{
		ID:           b.ID.String(),
		CreatedBy:    userRefDTO{ID: b.CreatedBy.ID.String(), DisplayName: b.CreatedBy.DisplayName},
		Status:       string(b.Status),
		TotalMinor:   b.TotalMinor,
		Currency:     b.Currency,
		TeacherCount: b.TeacherCount,
		LineCount:    b.LineCount,
		CreatedAt:    b.CreatedAt.UTC(),
	}
	if b.CompletedAt != nil {
		t := b.CompletedAt.UTC()
		out.CompletedAt = &t
	}
	return out
}

func toBatchDetailDTO(d BatchDetail) batchDetailDTO {
	lines := make([]batchLineDTO, len(d.Lines))
	for i, l := range d.Lines {
		lines[i] = batchLineDTO{
			Teacher:     teacherRefDTO{Slug: l.Teacher.Slug, DisplayName: l.Teacher.DisplayName},
			AmountMinor: l.AmountMinor,
			Currency:    l.Currency,
			LessonCount: l.LessonCount,
		}
	}
	return batchDetailDTO{batchDTO: toBatchDTO(d.Batch), Lines: lines}
}

func toDashboardDTO(d Dashboard) dashboardDTO {
	owed := make([]owedRowDTO, len(d.Owed))
	for i, o := range d.Owed {
		owed[i] = owedRowDTO{
			Teacher:           teacherRefDTO{Slug: o.Teacher.Slug, DisplayName: o.Teacher.DisplayName},
			AvailableMinor:    o.AvailableMinor,
			Currency:          o.Currency,
			OldestAvailableAt: o.OldestAvailableAt.UTC(),
		}
	}
	batches := make([]batchDTO, len(d.Batches))
	for i, b := range d.Batches {
		batches[i] = toBatchDTO(b)
	}
	return dashboardDTO{
		Owed: owed,
		Totals: totalsDTO{
			AvailableTotalMinor: d.Totals.AvailableTotalMinor,
			HeldTotalMinor:      d.Totals.HeldTotalMinor,
			PaidTotalMinor:      d.Totals.PaidTotalMinor,
			Currency:            d.Totals.Currency,
		},
		Batches:      batches,
		BatchesTotal: d.BatchesTotal,
	}
}

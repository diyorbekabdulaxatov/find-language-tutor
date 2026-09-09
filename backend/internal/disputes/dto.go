package disputes

import (
	"time"

	"github.com/google/uuid"
)

// Wire DTOs. Source of truth for the JSON shape; snake_case, RFC3339 UTC times,
// money in integer minor units, in sync with openapi.yaml.

// --- request bodies ---

type raiseDisputeRequest struct {
	Reason string `json:"reason"`
}

type resolveDisputeRequest struct {
	Outcome    string `json:"outcome"`
	Resolution string `json:"resolution"`
	Refund     bool   `json:"refund"`
}

// --- responses ---

type userRefDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type disputeDTO struct {
	ID         string      `json:"id"`
	BookingID  string      `json:"booking_id"`
	Status     string      `json:"status"`
	Reason     string      `json:"reason"`
	Resolution string      `json:"resolution"`
	RaisedBy   userRefDTO  `json:"raised_by"`
	ResolvedBy *userRefDTO `json:"resolved_by"`
	CreatedAt  time.Time   `json:"created_at"`
	ResolvedAt *time.Time  `json:"resolved_at"`
}

type disputeListDTO struct {
	Disputes []disputeDTO `json:"disputes"`
}

type moneyDTO struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

type teacherRefDTO struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
}

type studentRefDTO struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type disputeBookingDTO struct {
	ID      string        `json:"id"`
	Status  string        `json:"status"`
	StartAt time.Time     `json:"start_at"`
	Price   moneyDTO      `json:"price"`
	Teacher teacherRefDTO `json:"teacher"`
	Student studentRefDTO `json:"student"`
}

// queueItemDTO is one row of GET /v1/admin/disputes: the dispute fields inlined
// plus the booking it is about.
type queueItemDTO struct {
	disputeDTO
	Booking disputeBookingDTO `json:"booking"`
}

type queuePageDTO struct {
	Disputes []queueItemDTO `json:"disputes"`
	Total    int            `json:"total"`
}

// --- mapping ---

func toDisputeDTO(d Dispute) disputeDTO {
	out := disputeDTO{
		ID:         d.ID.String(),
		BookingID:  d.BookingID.String(),
		Status:     string(d.Status),
		Reason:     d.Reason,
		Resolution: d.Resolution,
		RaisedBy:   userRefDTO{ID: d.RaisedBy.ID.String(), DisplayName: d.RaisedBy.DisplayName},
		CreatedAt:  d.CreatedAt.UTC(),
	}
	if d.ResolvedBy != nil && d.ResolvedBy.ID != uuid.Nil {
		out.ResolvedBy = &userRefDTO{ID: d.ResolvedBy.ID.String(), DisplayName: d.ResolvedBy.DisplayName}
	}
	if d.ResolvedAt != nil {
		t := d.ResolvedAt.UTC()
		out.ResolvedAt = &t
	}
	return out
}

func toDisputeListDTO(ds []Dispute) disputeListDTO {
	out := make([]disputeDTO, len(ds))
	for i, d := range ds {
		out[i] = toDisputeDTO(d)
	}
	return disputeListDTO{Disputes: out}
}

func toQueuePageDTO(p Page) queuePageDTO {
	rows := make([]queueItemDTO, len(p.Disputes))
	for i, it := range p.Disputes {
		rows[i] = queueItemDTO{
			disputeDTO: toDisputeDTO(it.Dispute),
			Booking: disputeBookingDTO{
				ID:      it.Booking.ID.String(),
				Status:  it.Booking.Status,
				StartAt: it.Booking.StartAt.UTC(),
				Price:   moneyDTO{AmountMinor: it.Booking.Price.AmountMinor, Currency: it.Booking.Price.Currency},
				Teacher: teacherRefDTO{Slug: it.Booking.Teacher.Slug, DisplayName: it.Booking.Teacher.DisplayName},
				Student: studentRefDTO{
					ID:          it.Booking.Student.ID.String(),
					Email:       it.Booking.Student.Email,
					DisplayName: it.Booking.Student.DisplayName,
				},
			},
		}
	}
	return queuePageDTO{Disputes: rows, Total: p.Total}
}

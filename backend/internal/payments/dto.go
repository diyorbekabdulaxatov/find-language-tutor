package payments

import "time"

// Wire DTOs. Source of truth for the JSON shape; must stay in sync with
// openapi.yaml (snake_case, money as {amount_minor, currency}, RFC3339 UTC).

// webhookRequest is the body a payment provider POSTs to /v1/payments/webhook.
// The in-process fake builds the same shape when it calls the sink.
type webhookRequest struct {
	EventID     string `json:"event_id"`
	Type        string `json:"type"`
	PaymentID   string `json:"payment_id"`
	ProviderRef string `json:"provider_ref"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	Message     string `json:"message"`
}

type webhookResponse struct {
	Received bool `json:"received"`
	Applied  bool `json:"applied"`
}

type earningLineDTO struct {
	BookingID          string    `json:"booking_id"`
	StudentDisplayName string    `json:"student_display_name"`
	StartAt            time.Time `json:"start_at"`
	AmountMinor        int64     `json:"amount_minor"`
	State              string    `json:"state"`
}

type earningsDTO struct {
	TotalEarnedMinor int64            `json:"total_earned_minor"`
	HeldMinor        int64            `json:"held_minor"`
	AvailableMinor   int64            `json:"available_minor"`
	Currency         string           `json:"currency"`
	Lessons          []earningLineDTO `json:"lessons"`
}

func toEarningsDTO(e Earnings) earningsDTO {
	lessons := make([]earningLineDTO, len(e.Lines))
	for i, l := range e.Lines {
		lessons[i] = earningLineDTO{
			BookingID:          l.BookingID.String(),
			StudentDisplayName: l.StudentDisplayName,
			StartAt:            l.StartAt.UTC(),
			AmountMinor:        l.AmountMinor,
			State:              string(l.State),
		}
	}
	return earningsDTO{
		TotalEarnedMinor: e.TotalEarnedMinor,
		HeldMinor:        e.HeldMinor,
		AvailableMinor:   e.AvailableMinor,
		Currency:         e.Currency,
		Lessons:          lessons,
	}
}

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

// earningLineDTO is one earning row: a lesson booking, or (phase C3) a course
// sale. booking_id is now nullable (nil for a course row) since it can no
// longer be assumed present on every row — an unavoidable widening of an
// existing field, called out in the openapi schema; course_enrollment_id /
// course_title are new, additive, and nullable for a course row, nil for a
// lesson row.
type earningLineDTO struct {
	BookingID          *string `json:"booking_id"`
	CourseEnrollmentID *string `json:"course_enrollment_id"`
	CourseTitle        *string `json:"course_title"`
	// StudentDisplayName is the counterparty's name either way: the student
	// on a lesson booking, or the buyer of a course.
	StudentDisplayName string    `json:"student_display_name"`
	StartAt            time.Time `json:"start_at"`
	AmountMinor        int64     `json:"amount_minor"`
	State              string    `json:"state"`
	// AvailableAt is when the clearing window closes on a `held` earning — the
	// date a teacher is waiting for. Already in the past for every other state.
	AvailableAt time.Time `json:"available_at"`
}

type earningsDTO struct {
	TotalEarnedMinor int64            `json:"total_earned_minor"`
	HeldMinor        int64            `json:"held_minor"`
	AvailableMinor   int64            `json:"available_minor"`
	PaidMinor        int64            `json:"paid_minor"`
	Currency         string           `json:"currency"`
	Lessons          []earningLineDTO `json:"lessons"`
}

func toEarningsDTO(e Earnings) earningsDTO {
	lessons := make([]earningLineDTO, len(e.Lines))
	for i, l := range e.Lines {
		lessons[i] = earningLineDTO{
			StudentDisplayName: l.StudentDisplayName,
			StartAt:            l.StartAt.UTC(),
			AmountMinor:        l.AmountMinor,
			State:              string(l.State),
			AvailableAt:        l.AvailableAt.UTC(),
		}
		if l.BookingID != nil {
			id := l.BookingID.String()
			lessons[i].BookingID = &id
		}
		if l.CourseEnrollmentID != nil {
			id := l.CourseEnrollmentID.String()
			lessons[i].CourseEnrollmentID = &id
		}
		lessons[i].CourseTitle = l.CourseTitle
	}
	return earningsDTO{
		TotalEarnedMinor: e.TotalEarnedMinor,
		HeldMinor:        e.HeldMinor,
		AvailableMinor:   e.AvailableMinor,
		PaidMinor:        e.PaidMinor,
		Currency:         e.Currency,
		Lessons:          lessons,
	}
}

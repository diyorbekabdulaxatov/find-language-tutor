package bookings

import (
	"time"

	"github.com/google/uuid"
)

// Wire DTOs. Source of truth for the JSON shape; must stay in sync with
// openapi.yaml (snake_case, money as {amount_minor, currency}, times as RFC3339
// UTC strings).

type moneyDTO struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

type teacherSummaryDTO struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Timezone    string `json:"timezone"`
	AvatarURL   string `json:"avatar_url"`
}

type studentSummaryDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type bookingPaymentDTO struct {
	Status      string `json:"status"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

type bookingDTO struct {
	ID                 string     `json:"id"`
	Status             string     `json:"status"`
	StartAt            time.Time  `json:"start_at"`
	EndAt              time.Time  `json:"end_at"`
	DurationMinutes    int        `json:"duration_minutes"`
	IsTrial            bool       `json:"is_trial"`
	Price              moneyDTO   `json:"price"`
	CreatedAt          time.Time  `json:"created_at"`
	CancelledAt        *time.Time `json:"cancelled_at"`
	CancellationReason string     `json:"cancellation_reason,omitempty"`
	// MeetingURL is the effective video link — present ONLY when the caller is a
	// participant AND the booking is confirmed or completed. Empty/omitted for
	// everyone else (it must not leak to a pending_payment booking or a
	// non-participant).
	MeetingURL  string `json:"meeting_url,omitempty"`
	NoShowParty string `json:"no_show_party"`

	Teacher teacherSummaryDTO  `json:"teacher"`
	Student studentSummaryDTO  `json:"student"`
	Payment *bookingPaymentDTO `json:"payment"`
}

type bookingListDTO struct {
	Bookings []bookingDTO `json:"bookings"`
}

type slotDTO struct {
	StartAt time.Time `json:"start_at"`
	EndAt   time.Time `json:"end_at"`
	Price   moneyDTO  `json:"price"`
}

type slotsResponseDTO struct {
	TeacherSlug     string    `json:"teacher_slug"`
	Timezone        string    `json:"timezone"`
	From            time.Time `json:"from"`
	To              time.Time `json:"to"`
	DurationMinutes int       `json:"duration_minutes"`
	Slots           []slotDTO `json:"slots"`
}

// --- request bodies ---

type createBookingRequest struct {
	TeacherSlug     string    `json:"teacher_slug"`
	StartAt         time.Time `json:"start_at"`
	DurationMinutes int       `json:"duration_minutes"`
	IsTrial         bool      `json:"is_trial"`
}

type cancelBookingRequest struct {
	Reason string `json:"reason"`
}

type payBookingRequest struct {
	MethodToken string `json:"method_token"`
}

type meetingLinkRequest struct {
	URL string `json:"url"`
}

type noShowRequest struct {
	Party string `json:"party"`
}

// --- mapping ---

func toMoneyDTO(m Money) moneyDTO {
	return moneyDTO{AmountMinor: m.AmountMinor, Currency: m.Currency}
}

// meetingURLFor enforces the visibility rule: the effective link is revealed
// only to a participant of a confirmed / completed booking.
func meetingURLFor(b Booking, viewerID uuid.UUID) string {
	if !participant(b, viewerID) {
		return ""
	}
	if b.Status != StatusConfirmed && b.Status != StatusCompleted {
		return ""
	}
	return b.EffectiveMeetingURL()
}

func toBookingDTO(b Booking, viewerID uuid.UUID) bookingDTO {
	return bookingDTO{
		ID:                 b.ID.String(),
		Status:             string(b.Status),
		StartAt:            b.StartAt.UTC(),
		EndAt:              b.EndAt.UTC(),
		DurationMinutes:    b.DurationMinutes,
		IsTrial:            b.IsTrial,
		Price:              toMoneyDTO(b.Price),
		CreatedAt:          b.CreatedAt.UTC(),
		CancelledAt:        utcPtr(b.CancelledAt),
		CancellationReason: b.CancellationReason,
		MeetingURL:         meetingURLFor(b, viewerID),
		NoShowParty:        b.NoShowParty,
		Teacher: teacherSummaryDTO{
			Slug:        b.Teacher.Slug,
			DisplayName: b.Teacher.DisplayName,
			Timezone:    b.Teacher.Timezone,
			AvatarURL:   b.Teacher.AvatarURL,
		},
		Student: studentSummaryDTO{
			ID:          b.Student.ID.String(),
			DisplayName: b.Student.DisplayName,
		},
	}
}

func toBookingDTOWithPayment(b Booking, snap *PaymentSnapshot, viewerID uuid.UUID) bookingDTO {
	dto := toBookingDTO(b, viewerID)
	if snap != nil {
		dto.Payment = &bookingPaymentDTO{
			Status:      snap.Status,
			AmountMinor: snap.AmountMinor,
			Currency:    snap.Currency,
		}
	}
	return dto
}

func toBookingListDTO(bs []Booking, viewerID uuid.UUID) bookingListDTO {
	out := make([]bookingDTO, len(bs))
	for i, b := range bs {
		out[i] = toBookingDTO(b, viewerID)
	}
	return bookingListDTO{Bookings: out}
}

func toSlotsResponseDTO(r SlotResult) slotsResponseDTO {
	slots := make([]slotDTO, len(r.Slots))
	for i, s := range r.Slots {
		slots[i] = slotDTO{StartAt: s.StartAt.UTC(), EndAt: s.EndAt.UTC(), Price: toMoneyDTO(s.Price)}
	}
	return slotsResponseDTO{
		TeacherSlug:     r.TeacherSlug,
		Timezone:        r.Timezone,
		From:            r.From.UTC(),
		To:              r.To.UTC(),
		DurationMinutes: r.DurationMinutes,
		Slots:           slots,
	}
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

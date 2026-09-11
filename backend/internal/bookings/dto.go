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
	// CancelledBy is "" until the booking is cancelled, then student / teacher /
	// admin (an operator force-cancel).
	CancelledBy string `json:"cancelled_by"`
	// MeetingURL is the effective video link — present ONLY when the caller is a
	// participant AND the booking is confirmed or completed. Empty/omitted for
	// everyone else (it must not leak to a pending_payment booking or a
	// non-participant).
	MeetingURL  string `json:"meeting_url,omitempty"`
	NoShowParty string `json:"no_show_party"`

	// CanReview is true only when the viewer is the student, the booking is
	// `completed`, and no review exists yet — the frontend shows the post-lesson
	// prompt off this without a second call.
	CanReview bool `json:"can_review"`
	// Review is the caller's review of this booking if one exists (visible to
	// both participants); null otherwise.
	Review *bookingReviewDTO `json:"review"`

	// CanRaiseDispute is true only when the viewer is a participant, the booking
	// is `confirmed` or `completed`, and no dispute is currently open — the
	// frontend shows the "report a problem" action off this without a second
	// call.
	CanRaiseDispute bool `json:"can_raise_dispute"`
	// OpenDispute is the booking's currently open dispute (visible to both
	// participants); null when there is none.
	OpenDispute *bookingDisputeDTO `json:"open_dispute"`

	Teacher teacherSummaryDTO  `json:"teacher"`
	Student studentSummaryDTO  `json:"student"`
	Payment *bookingPaymentDTO `json:"payment"`

	// Resources is the lesson's attached materials / homework, summary only
	// (no quiz content — that's GET /v1/bookings/{id}/resources). Always an
	// array, never null.
	Resources []bookingResourceDTO `json:"resources"`
}

type bookingResourceDTO struct {
	ID               string     `json:"id"`
	ResourceID       string     `json:"resource_id"`
	Kind             string     `json:"kind"`
	Position         int        `json:"position"`
	DueAt            *time.Time `json:"due_at"`
	Type             string     `json:"type"`
	Title            string     `json:"title"`
	ResourceStatus   string     `json:"resource_status"`
	SubmissionStatus string     `json:"submission_status"`
}

type bookingReviewDTO struct {
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

type bookingDisputeDTO struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
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
		Resources: []bookingResourceDTO{},
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
		CancelledBy:        b.CancelledBy,
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

// withReview annotates a booking DTO with `can_review` / `review`. review is the
// caller's review of this booking (nil when none). can_review is true only for
// the student of a completed booking that has not been reviewed yet.
func withReview(dto bookingDTO, b Booking, viewerID uuid.UUID, review *BookingReview) bookingDTO {
	if review != nil {
		dto.Review = &bookingReviewDTO{
			Rating:    review.Rating,
			Comment:   review.Comment,
			CreatedAt: review.CreatedAt.UTC(),
		}
	}
	dto.CanReview = b.Student.ID == viewerID && b.Status == StatusCompleted && review == nil
	return dto
}

// withDispute annotates a booking DTO with `can_raise_dispute` / `open_dispute`.
// open is the booking's currently open dispute (nil when there is none). A
// dispute can be raised by either participant, only on a confirmed or completed
// lesson, and only while no other dispute is open.
func withDispute(dto bookingDTO, b Booking, viewerID uuid.UUID, open *BookingDispute) bookingDTO {
	if open != nil {
		dto.OpenDispute = &bookingDisputeDTO{
			ID:        open.ID.String(),
			Status:    open.Status,
			Reason:    open.Reason,
			CreatedAt: open.CreatedAt.UTC(),
		}
	}
	disputable := b.Status == StatusConfirmed || b.Status == StatusCompleted
	dto.CanRaiseDispute = participant(b, viewerID) && disputable && open == nil
	return dto
}

// withResources embeds a booking's attached-resource summaries. Always sets a
// non-null (possibly empty) array.
func withResources(dto bookingDTO, resources []BookingResource) bookingDTO {
	out := make([]bookingResourceDTO, len(resources))
	for i, r := range resources {
		out[i] = bookingResourceDTO{
			ID:               r.ID.String(),
			ResourceID:       r.ResourceID.String(),
			Kind:             r.Kind,
			Position:         r.Position,
			DueAt:            utcPtr(r.DueAt),
			Type:             r.Type,
			Title:            r.Title,
			ResourceStatus:   r.ResourceStatus,
			SubmissionStatus: r.SubmissionStatus,
		}
	}
	dto.Resources = out
	return dto
}

func toBookingListDTO(bs []Booking, viewerID uuid.UUID, reviews []*BookingReview, disputes []*BookingDispute, resourcesAll [][]BookingResource) bookingListDTO {
	out := make([]bookingDTO, len(bs))
	for i, b := range bs {
		dto := toBookingDTO(b, viewerID)
		var r *BookingReview
		if i < len(reviews) {
			r = reviews[i]
		}
		var d *BookingDispute
		if i < len(disputes) {
			d = disputes[i]
		}
		var res []BookingResource
		if i < len(resourcesAll) {
			res = resourcesAll[i]
		}
		dto = withDispute(withReview(dto, b, viewerID, r), b, viewerID, d)
		out[i] = withResources(dto, res)
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

package bookings

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Domain errors. The handler maps each to an HTTP status; anything else is 500.
var (
	// ErrTeacherNotFound — no teacher has the requested slug.
	ErrTeacherNotFound = errors.New("teacher not found")

	// ErrBookingNotFound — no booking has the requested id.
	ErrBookingNotFound = errors.New("booking not found")

	// ErrForbidden — the caller is neither the student nor the teacher-owner.
	ErrForbidden = errors.New("not a participant in this booking")

	// ErrCannotBookSelf — a teacher-owner tried to book their own profile.
	ErrCannotBookSelf = errors.New("cannot book your own teacher profile")

	// ErrSlotUnavailable — start_at is not a currently bookable slot (outside the
	// weekly availability, misaligned, in the past, or already taken by a
	// booking we can see). Rendered as 409.
	ErrSlotUnavailable = errors.New("requested slot is not available")

	// ErrSlotTaken — the DB double-booking EXCLUDE constraint rejected the insert
	// (lost a race). Rendered as 409 slot_taken.
	ErrSlotTaken = errors.New("slot was just taken")

	// ErrInvalidTransition — the booking is not in a state this action allows.
	// Rendered as 409.
	ErrInvalidTransition = errors.New("booking is not in a state that allows this")

	// ErrLessonNotStarted — a no-show was reported before start_at. Rendered as
	// 409 too_early (a lesson can't be missed until it has started).
	ErrLessonNotStarted = errors.New("the lesson has not started yet")
)

// Role filters GET /v1/bookings.
type Role string

const (
	RoleAny     Role = ""
	RoleStudent Role = "student"
	RoleTeacher Role = "teacher"
)

// CreateInput is the validated POST /v1/bookings request.
type CreateInput struct {
	TeacherSlug     string
	StartAt         time.Time
	DurationMinutes int
	IsTrial         bool
}

// ListFilter is passed to the repository. StudentFilter / TeacherFilter are
// matched with OR; uuid.Nil means "do not match this dimension".
type ListFilter struct {
	StudentFilter uuid.UUID
	TeacherFilter uuid.UUID
	Status        *Status
}

// CreateBookingParams is the repository's insert payload.
type CreateBookingParams struct {
	TeacherID       uuid.UUID
	StudentID       uuid.UUID
	StartAt         time.Time
	EndAt           time.Time
	DurationMinutes int
	PriceMinor      int64
	Currency        string
	IsTrial         bool
}

// Repository is the persistence port. The concrete implementation
// (repositoryPostgres) lives alongside; tests use a fake.
type Repository interface {
	// TeacherContextBySlug resolves a slug, or ErrTeacherNotFound.
	TeacherContextBySlug(ctx context.Context, slug string) (TeacherContext, error)

	// TeacherIDOwnedBy returns the teacher profile owned by ownerID. ok is false
	// when the account owns no profile.
	TeacherIDOwnedBy(ctx context.Context, ownerID uuid.UUID) (id uuid.UUID, ok bool, err error)

	// WeeklyAvailability returns every recurring weekly span for a teacher.
	WeeklyAvailability(ctx context.Context, teacherID uuid.UUID) ([]AvailabilitySpan, error)

	// BookedIntervals returns non-cancelled bookings for a teacher overlapping
	// [from, to).
	BookedIntervals(ctx context.Context, teacherID uuid.UUID, from, to time.Time) ([]Interval, error)

	// CreateBooking inserts a pending_payment booking and returns it hydrated.
	// It maps the double-booking EXCLUDE violation to ErrSlotTaken.
	CreateBooking(ctx context.Context, p CreateBookingParams) (Booking, error)

	// GetBooking returns one hydrated booking, or ErrBookingNotFound.
	GetBooking(ctx context.Context, id uuid.UUID) (Booking, error)

	// ListBookings returns hydrated bookings matching the filter, newest first.
	ListBookings(ctx context.Context, f ListFilter) ([]Booking, error)

	// SetStatus writes a new status and returns the hydrated booking.
	SetStatus(ctx context.Context, id uuid.UUID, status Status) (Booking, error)

	// Cancel marks a booking cancelled (status, cancelled_at, reason) and
	// returns the hydrated booking.
	Cancel(ctx context.Context, id uuid.UUID, reason string) (Booking, error)

	// SetMeetingLinkOverride writes the per-booking meeting link ("" clears it)
	// and returns the hydrated booking.
	SetMeetingLinkOverride(ctx context.Context, id uuid.UUID, url string) (Booking, error)

	// SetNoShowParty records who missed the lesson ("student" / "teacher"). The
	// status transition is applied separately by the caller.
	SetNoShowParty(ctx context.Context, id uuid.UUID, party string) error
}

// Service holds the booking business rules. Handlers call it; it never sees a
// *gin.Context.
type Service struct {
	repo      Repository
	now       func() time.Time
	payments  PaymentGateway    // nil until SetPaymentGateway; guarded at every use
	reminders ReminderScheduler // nil until SetReminderScheduler; guarded
	notifier  Notifier          // nil until SetNotifier; guarded
	reviews   ReviewReader      // nil until SetReviewReader; guarded
	logger    *slog.Logger
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now, logger: slog.Default()}
}

// SetPaymentGateway wires the payments adapter in. Called once at startup
// (cmd/api / internal/httpapi). Without it, pay / complete return
// ErrPaymentRequired and cancel skips the refund.
func (s *Service) SetPaymentGateway(gw PaymentGateway) { s.payments = gw }

// SetReminderScheduler wires the asynq-backed lesson-reminder scheduler in.
// Optional: a nil scheduler makes every schedule/cancel call a no-op.
func (s *Service) SetReminderScheduler(r ReminderScheduler) { s.reminders = r }

// SetNotifier wires the transactional-email notifier in. Optional: a nil
// notifier makes every send a no-op.
func (s *Service) SetNotifier(n Notifier) { s.notifier = n }

// SetReviewReader wires the reviews module's read port in. Optional: a nil
// reader leaves `review` nil and computes `can_review` from booking state alone.
func (s *Service) SetReviewReader(r ReviewReader) { s.reviews = r }

// reviewFor best-effort loads a booking's review; a lookup error is logged and
// treated as "no review" so it never fails a booking read.
func (s *Service) reviewFor(ctx context.Context, bookingID uuid.UUID) *BookingReview {
	if s.reviews == nil {
		return nil
	}
	r, found, err := s.reviews.ForBooking(ctx, bookingID)
	if err != nil {
		s.log().Error("load booking review", slog.String("booking_id", bookingID.String()), slog.Any("error", err))
		return nil
	}
	if !found {
		return nil
	}
	return r
}

// ReviewFor exposes reviewFor to the handler so it can annotate booking DTOs
// with `can_review` / `review`.
func (s *Service) ReviewFor(ctx context.Context, bookingID uuid.UUID) *BookingReview {
	return s.reviewFor(ctx, bookingID)
}

// --- guarded port calls (all safe with a nil port) ---

func (s *Service) scheduleReminders(ctx context.Context, b Booking) {
	if s.reminders == nil {
		return
	}
	if err := s.reminders.Schedule(ctx, b.ID, b.StartAt); err != nil {
		s.log().Error("schedule reminders", slog.String("booking_id", b.ID.String()), slog.Any("error", err))
	}
}

func (s *Service) cancelReminders(ctx context.Context, bookingID uuid.UUID) {
	if s.reminders == nil {
		return
	}
	if err := s.reminders.Cancel(ctx, bookingID); err != nil {
		s.log().Error("cancel reminders", slog.String("booking_id", bookingID.String()), slog.Any("error", err))
	}
}

func (s *Service) notifyConfirmed(ctx context.Context, b Booking) {
	if s.notifier == nil {
		return
	}
	s.notifier.BookingConfirmed(ctx, b)
}

func (s *Service) notifyCancelled(ctx context.Context, b Booking, cancelledBy uuid.UUID, refunded bool) {
	if s.notifier == nil {
		return
	}
	s.notifier.BookingCancelled(ctx, b, cancelledBy, refunded)
}

func (s *Service) log() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

// Slots lists the concrete bookable start times for a teacher over [from, to].
// Public — no auth. from defaults to now, to defaults to from+14d, and the
// window is capped at 21 days (ValidationError beyond that). duration defaults
// to 60 and must be one of 30/60/90/120.
func (s *Service) Slots(ctx context.Context, slug string, from, to *time.Time, durationMinutes int) (SlotResult, error) {
	now := s.now().UTC()

	start := now
	if from != nil {
		start = from.UTC()
	}
	if start.Before(now) {
		start = now
	}

	end := start.AddDate(0, 0, defaultWindowDays)
	if to != nil {
		end = to.UTC()
	}
	if !end.After(start) {
		return SlotResult{}, invalid("`to` must be after `from`.")
	}
	if end.Sub(start) > time.Duration(maxWindowDays)*24*time.Hour {
		return SlotResult{}, invalid("the window between `from` and `to` may not exceed %d days.", maxWindowDays)
	}

	if durationMinutes == 0 {
		durationMinutes = DefaultDurationMinutes
	}
	if !allowedDurations[durationMinutes] {
		return SlotResult{}, invalid("`duration` must be one of 30, 60, 90, 120.")
	}

	tc, err := s.repo.TeacherContextBySlug(ctx, slug)
	if err != nil {
		return SlotResult{}, err
	}

	spans, err := s.repo.WeeklyAvailability(ctx, tc.ID)
	if err != nil {
		return SlotResult{}, err
	}
	booked, err := s.repo.BookedIntervals(ctx, tc.ID, start, end)
	if err != nil {
		return SlotResult{}, err
	}

	price := hourlyPrice(tc.PricePerHourMinor, durationMinutes, tc.Currency)
	q := SlotQuery{From: start, To: end, DurationMinutes: durationMinutes}
	slots := generateSlots(spans, booked, q, func() Money { return price }, now)

	return SlotResult{
		TeacherSlug:     tc.Slug,
		Timezone:        tc.Timezone,
		From:            start,
		To:              end,
		DurationMinutes: durationMinutes,
		Slots:           slots,
	}, nil
}

// Create books a lesson for studentID. It re-derives the bookable slot set
// server-side and never trusts the client's price or alignment.
func (s *Service) Create(ctx context.Context, studentID uuid.UUID, in CreateInput) (Booking, error) {
	now := s.now().UTC()

	tc, err := s.repo.TeacherContextBySlug(ctx, in.TeacherSlug)
	if err != nil {
		return Booking{}, err
	}
	if tc.OwnerID != uuid.Nil && tc.OwnerID == studentID {
		return Booking{}, ErrCannotBookSelf
	}

	// is_trial forces a fixed 30-minute length; otherwise the requested duration
	// must be one of the allowed values.
	duration := in.DurationMinutes
	if in.IsTrial {
		duration = trialDurationMinutes
	} else if !allowedDurations[duration] {
		return Booking{}, invalid("`duration_minutes` must be one of 30, 60, 90, 120.")
	}

	start := in.StartAt.UTC()
	if !start.After(now) {
		return Booking{}, invalid("`start_at` must be in the future.")
	}
	end := start.Add(time.Duration(duration) * time.Minute)

	// Pricing.
	var price Money
	if in.IsTrial {
		if tc.TrialPriceMinor == nil {
			return Booking{}, invalid("this teacher does not offer a trial lesson.")
		}
		price = Money{AmountMinor: *tc.TrialPriceMinor, Currency: tc.Currency}
	} else {
		price = hourlyPrice(tc.PricePerHourMinor, duration, tc.Currency)
	}

	// Re-check availability server-side.
	spans, err := s.repo.WeeklyAvailability(ctx, tc.ID)
	if err != nil {
		return Booking{}, err
	}
	booked, err := s.repo.BookedIntervals(ctx, tc.ID, start, end)
	if err != nil {
		return Booking{}, err
	}
	if !isBookableStart(spans, booked, start, duration, now) {
		return Booking{}, ErrSlotUnavailable
	}

	b, err := s.repo.CreateBooking(ctx, CreateBookingParams{
		TeacherID:       tc.ID,
		StudentID:       studentID,
		StartAt:         start,
		EndAt:           end,
		DurationMinutes: duration,
		PriceMinor:      price.AmountMinor,
		Currency:        price.Currency,
		IsTrial:         in.IsTrial,
	})
	if err != nil {
		return Booking{}, err
	}

	// Open the payment intent for the new booking. The student pays it via
	// POST /v1/bookings/{id}/pay. A rare failure here leaves a pending_payment
	// booking with no intent; the pay call re-tries the intent creation.
	if s.payments != nil {
		if perr := s.payments.InitiatePayment(ctx, b.ID, b.Price.AmountMinor, b.Price.Currency); perr != nil {
			s.log().Error("initiate payment", slog.String("booking_id", b.ID.String()), slog.Any("error", perr))
		}
	}
	return b, nil
}

// List returns the bookings the caller participates in, filtered by role and
// optional status.
func (s *Service) List(ctx context.Context, callerID uuid.UUID, role Role, status *Status) ([]Booking, error) {
	if status != nil && !status.valid() {
		return nil, invalid("`status` must be one of pending_payment, confirmed, completed, cancelled.")
	}

	var f ListFilter
	f.Status = status

	switch role {
	case RoleStudent:
		f.StudentFilter = callerID
	case RoleTeacher:
		id, ok, err := s.repo.TeacherIDOwnedBy(ctx, callerID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Booking{}, nil // caller owns no teacher profile
		}
		f.TeacherFilter = id
	case RoleAny:
		f.StudentFilter = callerID
		id, ok, err := s.repo.TeacherIDOwnedBy(ctx, callerID)
		if err != nil {
			return nil, err
		}
		if ok {
			f.TeacherFilter = id
		}
	default:
		return nil, invalid("`role` must be omitted, `student`, or `teacher`.")
	}

	return s.repo.ListBookings(ctx, f)
}

// Get returns one booking. The caller must be the student or the teacher-owner.
func (s *Service) Get(ctx context.Context, callerID, bookingID uuid.UUID) (Booking, error) {
	b, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if !participant(b, callerID) {
		return Booking{}, ErrForbidden
	}
	return b, nil
}

// GetWithPayment is Get plus the embedded payment snapshot (nil when the
// booking has no intent). Used by GET /v1/bookings/{id}.
func (s *Service) GetWithPayment(ctx context.Context, callerID, bookingID uuid.UUID) (Booking, *PaymentSnapshot, error) {
	b, err := s.Get(ctx, callerID, bookingID)
	if err != nil {
		return Booking{}, nil, err
	}
	return b, s.snapshot(ctx, bookingID), nil
}

// snapshot best-effort loads the payment snapshot; a lookup error is logged and
// treated as "no payment" so it never fails a booking read.
func (s *Service) snapshot(ctx context.Context, bookingID uuid.UUID) *PaymentSnapshot {
	if s.payments == nil {
		return nil
	}
	snap, found, err := s.payments.SnapshotForBooking(ctx, bookingID)
	if err != nil {
		s.log().Error("load payment snapshot", slog.String("booking_id", bookingID.String()), slog.Any("error", err))
		return nil
	}
	if !found {
		return nil
	}
	return &snap
}

// Pay authorizes payment for a booking and, on success, moves it
// pending_payment -> confirmed (the payment webhook performs the transition).
// Student-only. Returns ErrAlreadyPaid (409) if already confirmed/paid,
// PaymentFailedError (402) on a provider decline.
func (s *Service) Pay(ctx context.Context, callerID, bookingID uuid.UUID, methodToken string) (Booking, *PaymentSnapshot, error) {
	if s.payments == nil {
		return Booking{}, nil, ErrPaymentRequired
	}
	b, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		return Booking{}, nil, err
	}
	if b.Student.ID != callerID {
		return Booking{}, nil, ErrNotStudent
	}
	switch b.Status {
	case StatusConfirmed, StatusCompleted:
		return Booking{}, nil, ErrAlreadyPaid
	case StatusPendingPayment:
		// the payable state
	default: // cancelled
		return Booking{}, nil, ErrInvalidTransition
	}

	snap, err := s.payments.Authorize(ctx, bookingID, strings.TrimSpace(methodToken))
	if err != nil {
		return Booking{}, nil, err
	}

	b, err = s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		return Booking{}, nil, err
	}

	// NOTE: the MVP's fake payment provider is synchronous — Authorize has
	// already run the payment.authorized webhook, so the booking is confirmed
	// here and scheduling reminders + sending the confirmation mail from this
	// request path is correct. A real async provider (Payme / Click / Uzum)
	// would instead trigger this from HandleWebhook when the authorized event
	// lands.
	if b.Status == StatusConfirmed {
		s.scheduleReminders(ctx, b)
		s.notifyConfirmed(ctx, b)
	}
	return b, &snap, nil
}

// Complete moves confirmed -> completed and captures the payment, then writes
// the teacher's payout-ledger entry (done inside the capture webhook).
// Teacher-owner only. Allowed only once the lesson's end_at is in the past
// (ErrTooEarly / 409 otherwise).
//
// Ordering note: we capture BEFORE flipping the status, so a capture failure
// leaves the booking confirmed and retryable rather than completed-but-unpaid.
// (Phase 5 layers reminders / meeting links / emails on top of this.)
func (s *Service) Complete(ctx context.Context, callerID, bookingID uuid.UUID) (Booking, *PaymentSnapshot, error) {
	if s.payments == nil {
		return Booking{}, nil, ErrPaymentRequired
	}
	b, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		return Booking{}, nil, err
	}
	if b.TeacherOwnerID == uuid.Nil || b.TeacherOwnerID != callerID {
		return Booking{}, nil, ErrNotTeacherOwner
	}
	if b.Status != StatusConfirmed {
		return Booking{}, nil, ErrInvalidTransition
	}
	if !s.now().UTC().After(b.EndAt) {
		return Booking{}, nil, ErrTooEarly
	}

	snap, err := s.payments.Capture(ctx, bookingID)
	if err != nil {
		return Booking{}, nil, err
	}

	b, err = s.repo.SetStatus(ctx, bookingID, StatusCompleted)
	if err != nil {
		return Booking{}, nil, err
	}
	return b, &snap, nil
}

// Cancel moves pending_payment | confirmed -> cancelled. Participant-only. When
// a payment gateway is wired, the booking's intent is also refunded/voided (a
// full refund releases the hold; if a payout-ledger row exists it is reversed).
//
// TODO(phase-5): cancellation window / penalties. For the MVP a participant may
// cancel at any time; we only record who (implicitly, via the caller) and when.
func (s *Service) Cancel(ctx context.Context, callerID, bookingID uuid.UUID, reason string) (Booking, error) {
	b, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if !participant(b, callerID) {
		return Booking{}, ErrForbidden
	}
	if b.Status != StatusPendingPayment && b.Status != StatusConfirmed {
		return Booking{}, ErrInvalidTransition
	}

	cancelled, err := s.repo.Cancel(ctx, bookingID, strings.TrimSpace(reason))
	if err != nil {
		return Booking{}, err
	}

	refunded := false
	if s.payments != nil {
		refunded = true
		if _, rerr := s.payments.Refund(ctx, bookingID); rerr != nil {
			// The booking is already cancelled; a stuck refund must not fail the
			// request. TODO(payments): enqueue a refund retry.
			s.log().Error("refund on cancel", slog.String("booking_id", bookingID.String()), slog.Any("error", rerr))
		}
	}

	// Rescheduling cancels old jobs; a cancelled booking must drop its
	// reminders, and the other party is told.
	s.cancelReminders(ctx, bookingID)
	s.notifyCancelled(ctx, cancelled, callerID, refunded)
	return cancelled, nil
}

// SetMeetingLink sets (or clears, with "") the per-booking meeting-link
// override. Teacher-owner only. A non-empty URL must parse as an http(s) URL.
func (s *Service) SetMeetingLink(ctx context.Context, callerID, bookingID uuid.UUID, link string) (Booking, error) {
	b, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if b.TeacherOwnerID == uuid.Nil || b.TeacherOwnerID != callerID {
		return Booking{}, ErrNotTeacherOwner
	}
	link = strings.TrimSpace(link)
	if link != "" && !validMeetingURL(link) {
		return Booking{}, invalid("`url` must be an http(s) URL, or empty to clear the link.")
	}
	return s.repo.SetMeetingLinkOverride(ctx, bookingID, link)
}

// NoShow records that a participant missed a confirmed lesson. Teacher-owner
// only for the MVP. Allowed only from `confirmed` and only once start_at is in
// the past (ErrLessonNotStarted / 409 too_early otherwise).
//
//	party == "student" — the teacher showed up, the student did not: treat like
//	                     complete (booking -> completed, capture, payout ledger).
//	party == "teacher" — the teacher self-reports they could not make it:
//	                     booking -> cancelled, refund the student.
func (s *Service) NoShow(ctx context.Context, callerID, bookingID uuid.UUID, party string) (Booking, *PaymentSnapshot, error) {
	if party != NoShowStudent && party != NoShowTeacher {
		return Booking{}, nil, invalid("`party` must be `student` or `teacher`.")
	}

	b, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		return Booking{}, nil, err
	}
	if b.TeacherOwnerID == uuid.Nil || b.TeacherOwnerID != callerID {
		return Booking{}, nil, ErrNotTeacherOwner
	}
	if b.Status != StatusConfirmed {
		return Booking{}, nil, ErrInvalidTransition
	}
	if !s.now().UTC().After(b.StartAt) {
		return Booking{}, nil, ErrLessonNotStarted
	}

	if party == NoShowStudent {
		// Same money path as Complete: capture BEFORE flipping status so a
		// capture failure leaves the booking confirmed and retryable.
		if s.payments == nil {
			return Booking{}, nil, ErrPaymentRequired
		}
		snap, err := s.payments.Capture(ctx, bookingID)
		if err != nil {
			return Booking{}, nil, err
		}
		if err := s.repo.SetNoShowParty(ctx, bookingID, NoShowStudent); err != nil {
			return Booking{}, nil, err
		}
		b, err = s.repo.SetStatus(ctx, bookingID, StatusCompleted)
		if err != nil {
			return Booking{}, nil, err
		}
		s.cancelReminders(ctx, bookingID)
		return b, &snap, nil
	}

	// party == "teacher": cancel + refund, same path as Cancel.
	if _, err := s.repo.Cancel(ctx, bookingID, "teacher no-show"); err != nil {
		return Booking{}, nil, err
	}
	if err := s.repo.SetNoShowParty(ctx, bookingID, NoShowTeacher); err != nil {
		return Booking{}, nil, err
	}
	refunded := false
	if s.payments != nil {
		refunded = true
		if _, rerr := s.payments.Refund(ctx, bookingID); rerr != nil {
			s.log().Error("refund on teacher no-show", slog.String("booking_id", bookingID.String()), slog.Any("error", rerr))
		}
	}
	s.cancelReminders(ctx, bookingID)

	b, err = s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		return Booking{}, nil, err
	}
	s.notifyCancelled(ctx, b, callerID, refunded)
	return b, nil, nil
}

// validMeetingURL reports whether s is a syntactically valid absolute http(s)
// URL with a host.
func validMeetingURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func participant(b Booking, callerID uuid.UUID) bool {
	return b.Student.ID == callerID || (b.TeacherOwnerID != uuid.Nil && b.TeacherOwnerID == callerID)
}

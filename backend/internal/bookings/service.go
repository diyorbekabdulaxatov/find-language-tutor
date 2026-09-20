package bookings

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/i18n"
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

	// ErrTrialAlreadyBooked means the student already holds a non-cancelled
	// trial with this teacher (one per student per teacher, enforced by the
	// bookings_one_trial_per_student_idx unique index).
	ErrTrialAlreadyBooked = errors.New("trial lesson already booked with this teacher")

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

	// LessonTypeID is the offering the student picked. uuid.Nil keeps the
	// pre-L1 path: priced from the teacher's hourly rate.
	LessonTypeID uuid.UUID
}

// TrialEligibility is the answer to "may this student still book a trial with
// this teacher?". BookingID is set only with TrialReasonAlreadyBooked.
type TrialEligibility struct {
	Eligible  bool
	Reason    TrialIneligibleReason
	BookingID uuid.UUID
}

// TrialIneligibleReason is the wire-level reason a trial cannot be booked.
type TrialIneligibleReason string

const (
	TrialReasonAlreadyBooked TrialIneligibleReason = "already_booked"
	TrialReasonOwnProfile    TrialIneligibleReason = "own_profile"
)

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
	LessonTypeID    uuid.NullUUID
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
	// It maps the double-booking EXCLUDE violation to ErrSlotTaken and the
	// one-trial-per-student unique violation to ErrTrialAlreadyBooked.
	CreateBooking(ctx context.Context, p CreateBookingParams) (Booking, error)

	// StudentTrialBooking returns the non-cancelled trial the student already
	// holds with the teacher. ok is false when there is none.
	StudentTrialBooking(ctx context.Context, teacherID, studentID uuid.UUID) (id uuid.UUID, ok bool, err error)

	// GetBooking returns one hydrated booking, or ErrBookingNotFound.
	GetBooking(ctx context.Context, id uuid.UUID) (Booking, error)

	// ListBookings returns hydrated bookings matching the filter, newest first.
	ListBookings(ctx context.Context, f ListFilter) ([]Booking, error)

	// SetStatus writes a new status and returns the hydrated booking.
	SetStatus(ctx context.Context, id uuid.UUID, status Status) (Booking, error)

	// Cancel marks a booking cancelled (status, cancelled_at, reason, who
	// cancelled it — one of CancelledByStudent / Teacher / Admin — and what
	// happened to the money) and returns the hydrated booking.
	Cancel(ctx context.Context, id uuid.UUID, reason, by string, outcome CancellationOutcome) (Booking, error)

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
	disputes  DisputeReader     // nil until SetDisputeReader; guarded
	resources ResourceReader    // nil until SetResourceReader; guarded

	// lessonTypes prices a booking against one of the teacher's offerings
	// (italki-style). nil keeps the pre-L1 hourly path — see LessonTypeReader.
	lessonTypes LessonTypeReader

	// freeCancelWindow is how long before start_at a student may still cancel
	// for a full refund; zero means DefaultFreeCancelWindow.
	freeCancelWindow time.Duration

	logger *slog.Logger
}

// SetFreeCancelWindow overrides DefaultFreeCancelWindow (BOOKINGS_FREE_CANCEL_HOURS).
func (s *Service) SetFreeCancelWindow(d time.Duration) { s.freeCancelWindow = d }

func (s *Service) freeWindow() time.Duration {
	if s.freeCancelWindow > 0 {
		return s.freeCancelWindow
	}
	return DefaultFreeCancelWindow
}

// CancellationPolicy is what the student is told before cancelling: the
// deadline for a free cancellation and whether it has already passed.
type CancellationPolicy struct {
	FreeCancelUntil time.Time
	// Late is true once the deadline has passed — a student cancellation from
	// here on forfeits the fee.
	Late bool
}

// Policy computes the cancellation policy for a booking as of now.
func (s *Service) Policy(b Booking) CancellationPolicy {
	until := b.StartAt.Add(-s.freeWindow())
	return CancellationPolicy{FreeCancelUntil: until, Late: s.now().UTC().After(until)}
}

// LateCancellationError is returned when a student cancels inside the free
// window without acknowledging the forfeit. 409 late_cancellation.
type LateCancellationError struct {
	i18n.Msg
	Policy CancellationPolicy
}

func (e LateCancellationError) Error() string { return e.Msg.String() }

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

// SetDisputeReader wires the disputes module's read port in. Optional: a nil
// reader leaves `open_dispute` nil and computes `can_raise_dispute` from
// booking state alone.
func (s *Service) SetDisputeReader(r DisputeReader) { s.disputes = r }

// OpenDisputeFor best-effort loads a booking's open dispute for the handler to
// annotate booking DTOs with `can_raise_dispute` / `open_dispute`. A lookup
// error is logged and treated as "no dispute" so it never fails a booking read.
func (s *Service) OpenDisputeFor(ctx context.Context, bookingID uuid.UUID) *BookingDispute {
	if s.disputes == nil {
		return nil
	}
	d, found, err := s.disputes.OpenForBooking(ctx, bookingID)
	if err != nil {
		s.log().Error("load booking dispute", slog.String("booking_id", bookingID.String()), slog.Any("error", err))
		return nil
	}
	if !found {
		return nil
	}
	return d
}

// SetResourceReader wires the resources module's read port in. Optional: a
// nil reader leaves `resources` empty on every BookingDTO.
func (s *Service) SetResourceReader(r ResourceReader) { s.resources = r }

// resourcesFor best-effort loads a booking's attached-resource summaries; a
// lookup error is logged and treated as "no resources" so it never fails a
// booking read.
func (s *Service) resourcesFor(ctx context.Context, bookingID, viewerID uuid.UUID) []BookingResource {
	if s.resources == nil {
		return nil
	}
	rs, err := s.resources.ForBooking(ctx, bookingID, viewerID)
	if err != nil {
		s.log().Error("load booking resources", slog.String("booking_id", bookingID.String()), slog.Any("error", err))
		return nil
	}
	return rs
}

// ResourcesFor exposes resourcesFor to the handler so it can annotate booking
// DTOs with `resources`.
func (s *Service) ResourcesFor(ctx context.Context, bookingID, viewerID uuid.UUID) []BookingResource {
	return s.resourcesFor(ctx, bookingID, viewerID)
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

func (s *Service) notifyCancelled(ctx context.Context, b Booking, cancelledBy uuid.UUID, outcome CancellationOutcome) {
	if s.notifier == nil {
		return
	}
	s.notifier.BookingCancelled(ctx, b, cancelledBy, outcome)
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
func (s *Service) Slots(ctx context.Context, slug string, from, to *time.Time, durationMinutes int, lessonTypeID uuid.UUID) (SlotResult, error) {
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
	// With an offering the length is whatever it is priced at; without one the
	// legacy list applies.
	if lessonTypeID == uuid.Nil && !allowedDurations[durationMinutes] {
		return SlotResult{}, invalid("`duration` must be one of 30, 60, 90, 120.")
	}

	tc, err := s.repo.TeacherContextBySlug(ctx, slug)
	if err != nil {
		return SlotResult{}, err
	}

	// Every slot in the window costs the same, so the offering is priced once.
	var offering *Offering
	if lessonTypeID != uuid.Nil {
		o, err := s.offering(ctx, tc.ID, lessonTypeID, durationMinutes)
		if err != nil {
			return SlotResult{}, err
		}
		offering = &o
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
	if offering != nil {
		price = offering.Price
	}
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

	// An offering (lesson type) sets both the length and the price; without one
	// the legacy path applies — is_trial forces 30 minutes, everything else
	// must be one of the allowed lengths.
	var offering *Offering
	if in.LessonTypeID != uuid.Nil {
		o, err := s.offering(ctx, tc.ID, in.LessonTypeID, in.DurationMinutes)
		if err != nil {
			return Booking{}, err
		}
		offering = &o
	}

	duration := in.DurationMinutes
	switch {
	case offering != nil:
		// validated against the offering's own price list
	case in.IsTrial:
		duration = trialDurationMinutes
	case !allowedDurations[duration]:
		return Booking{}, invalid("`duration_minutes` must be one of 30, 60, 90, 120.")
	}

	start := in.StartAt.UTC()
	if !start.After(now) {
		return Booking{}, invalid("`start_at` must be in the future.")
	}
	end := start.Add(time.Duration(duration) * time.Minute)

	// Pricing. The offering's own price wins; the client's is never trusted.
	var price Money
	isTrial := in.IsTrial
	switch {
	case offering != nil:
		price = offering.Price
		isTrial = offering.IsTrial
	case in.IsTrial:
		if tc.TrialPriceMinor == nil {
			return Booking{}, invalid("this teacher does not offer a trial lesson.")
		}
		price = Money{AmountMinor: *tc.TrialPriceMinor, Currency: tc.Currency}
	default:
		price = hourlyPrice(tc.PricePerHourMinor, duration, tc.Currency)
	}

	// One trial per student per teacher. This is the friendly early answer;
	// the unique index behind CreateBooking is what settles a race.
	if isTrial {
		if _, used, err := s.repo.StudentTrialBooking(ctx, tc.ID, studentID); err != nil {
			return Booking{}, err
		} else if used {
			return Booking{}, ErrTrialAlreadyBooked
		}
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
		IsTrial:         isTrial,
		LessonTypeID:    lessonTypeRef(offering),
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

// TrialEligibility says whether studentID may still book a trial with the
// teacher. Ineligible reasons: the student owns the profile, or already holds
// a non-cancelled trial with this teacher (the booking id is returned so the
// UI can link to it). Whether the teacher offers a trial at all is not this
// call's concern — the offering list answers that.
func (s *Service) TrialEligibility(ctx context.Context, studentID uuid.UUID, teacherSlug string) (TrialEligibility, error) {
	tc, err := s.repo.TeacherContextBySlug(ctx, teacherSlug)
	if err != nil {
		return TrialEligibility{}, err
	}
	if tc.OwnerID != uuid.Nil && tc.OwnerID == studentID {
		return TrialEligibility{Reason: TrialReasonOwnProfile}, nil
	}
	id, used, err := s.repo.StudentTrialBooking(ctx, tc.ID, studentID)
	if err != nil {
		return TrialEligibility{}, err
	}
	if used {
		return TrialEligibility{Reason: TrialReasonAlreadyBooked, BookingID: id}, nil
	}
	return TrialEligibility{Eligible: true}, nil
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

// Cancel moves pending_payment | confirmed -> cancelled. Participant-only.
//
// The policy (italki's): a teacher may cancel at any time and the student is
// always refunded in full. A student may cancel for a full refund until
// freeWindow before the start; after that the fee is forfeited — captured and
// paid to the teacher exactly as a completed lesson — and the student must
// say so (acknowledgeForfeit) or get LateCancellationError back, so no client
// can charge them by accident. An unpaid (pending_payment) booking is simply
// dropped whenever it is cancelled: there is nothing to forfeit.
func (s *Service) Cancel(ctx context.Context, callerID, bookingID uuid.UUID, reason string, acknowledgeForfeit bool) (Booking, error) {
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
	by := cancellerFor(b, callerID)

	if by == CancelledByStudent && b.Status == StatusConfirmed {
		if p := s.Policy(b); p.Late {
			if !acknowledgeForfeit {
				return Booking{}, LateCancellationError{
					Msg:    i18n.Message("Free cancellation ended %d hours before the lesson. Cancelling now means the full fee is charged.", int(s.freeWindow().Hours())),
					Policy: p,
				}
			}
			return s.forfeit(ctx, b, reason, callerID)
		}
	}
	return s.doCancel(ctx, bookingID, reason, by, callerID, true)
}

// forfeit is the late-student-cancellation money path: capture the hold FIRST
// (same as Complete — a capture failure leaves the booking confirmed and the
// request retryable), then cancel without a refund. The capture webhook writes
// the teacher's payout-ledger row, so the teacher is paid as for a lesson.
func (s *Service) forfeit(ctx context.Context, b Booking, reason string, actorID uuid.UUID) (Booking, error) {
	if s.payments != nil {
		if _, err := s.payments.Capture(ctx, b.ID); err != nil {
			return Booking{}, err
		}
	}
	cancelled, err := s.repo.Cancel(ctx, b.ID, strings.TrimSpace(reason), CancelledByStudent, OutcomeForfeited)
	if err != nil {
		return Booking{}, err
	}
	s.cancelReminders(ctx, b.ID)
	s.notifyCancelled(ctx, cancelled, actorID, OutcomeForfeited)
	return cancelled, nil
}

// AdminForceCancel is the operator override behind
// POST /v1/admin/bookings/{id}/force-cancel (permission bookings.force_cancel).
// It is the same cancel path as Cancel — the refund, the reminder teardown and
// the "your lesson was cancelled" mail all happen exactly as they do for a
// participant cancellation — minus the participant check, and recording
// cancelled_by = 'admin' so the reason a lesson vanished stays auditable.
//
// refund=false skips the refund call entirely (the operator settled the money
// some other way); with refund=true an authorized intent is released and a
// captured one refunded, exactly as on a participant cancel.
//
// Authorization is the caller's (the RBAC guard on the route); the state rule is
// enforced here: only pending_payment / confirmed can be cancelled, anything
// else is ErrInvalidTransition (409 invalid_state).
func (s *Service) AdminForceCancel(ctx context.Context, bookingID uuid.UUID, reason string, refund bool) error {
	b, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		return err
	}
	if b.Status != StatusPendingPayment && b.Status != StatusConfirmed {
		return ErrInvalidTransition
	}
	_, err = s.doCancel(ctx, bookingID, reason, CancelledByAdmin, uuid.Nil, refund)
	return err
}

// AdminRefund refunds a booking's payment WITHOUT touching the booking's
// lifecycle. It backs the "refund the student" outcome of resolving a dispute:
// the lesson stays completed / confirmed, only the money moves back. A booking
// whose intent was never authorized is a no-op void inside the payments module.
// With no payment gateway wired it is a logged no-op.
func (s *Service) AdminRefund(ctx context.Context, bookingID uuid.UUID) error {
	if _, err := s.repo.GetBooking(ctx, bookingID); err != nil {
		return err
	}
	if s.payments == nil {
		s.log().Warn("admin refund skipped: no payment gateway", slog.String("booking_id", bookingID.String()))
		return nil
	}
	if _, err := s.payments.Refund(ctx, bookingID); err != nil {
		return err
	}
	return nil
}

// doCancel is THE cancellation path: write the cancellation (status,
// cancelled_at, reason, cancelled_by), refund when asked, drop any pending
// reminders, and mail the other party. Participant cancel, teacher no-show and
// the admin force-cancel all funnel through it, so none of them can drift.
//
// actorID is the account that triggered it (uuid.Nil for an admin override, so
// the notifier mails both participants rather than "the other party").
func (s *Service) doCancel(ctx context.Context, bookingID uuid.UUID, reason, by string, actorID uuid.UUID, refund bool) (Booking, error) {
	// The outcome is decided before the write so the row and the email agree.
	// A confirmed booking holds an authorized payment; pending_payment never
	// had one. refund=false is the operator's "settled elsewhere" — unrecorded.
	var outcome CancellationOutcome
	if refund {
		outcome = OutcomeUnpaid
		if b, err := s.repo.GetBooking(ctx, bookingID); err == nil && b.Status == StatusConfirmed {
			outcome = OutcomeRefunded
		}
	}

	cancelled, err := s.repo.Cancel(ctx, bookingID, strings.TrimSpace(reason), by, outcome)
	if err != nil {
		return Booking{}, err
	}

	if refund && s.payments != nil {
		if _, rerr := s.payments.Refund(ctx, bookingID); rerr != nil {
			// The booking is already cancelled; a stuck refund must not fail the
			// request. TODO(payments): enqueue a refund retry.
			s.log().Error("refund on cancel", slog.String("booking_id", bookingID.String()), slog.Any("error", rerr))
		}
	}

	// Rescheduling cancels old jobs; a cancelled booking must drop its
	// reminders, and the other party is told.
	s.cancelReminders(ctx, bookingID)
	s.notifyCancelled(ctx, cancelled, actorID, outcome)
	return cancelled, nil
}

// cancellerFor maps the acting participant onto the cancelled_by value.
func cancellerFor(b Booking, callerID uuid.UUID) string {
	if b.Student.ID == callerID {
		return CancelledByStudent
	}
	return CancelledByTeacher
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

	// party == "teacher": cancel + refund through the shared cancel path. The
	// no-show flag is written FIRST so the booking doCancel hydrates (and mails)
	// already carries it.
	if err := s.repo.SetNoShowParty(ctx, bookingID, NoShowTeacher); err != nil {
		return Booking{}, nil, err
	}
	b, err = s.doCancel(ctx, bookingID, "teacher no-show", CancelledByTeacher, callerID, true)
	if err != nil {
		return Booking{}, nil, err
	}
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

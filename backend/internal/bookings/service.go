package bookings

import (
	"context"
	"errors"
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
}

// Service holds the booking business rules. Handlers call it; it never sees a
// *gin.Context.
type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
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

	return s.repo.CreateBooking(ctx, CreateBookingParams{
		TeacherID:       tc.ID,
		StudentID:       studentID,
		StartAt:         start,
		EndAt:           end,
		DurationMinutes: duration,
		PriceMinor:      price.AmountMinor,
		Currency:        price.Currency,
		IsTrial:         in.IsTrial,
	})
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

// Confirm moves pending_payment -> confirmed. Participant-only. (Phase 4 will
// move this behind payment success.)
func (s *Service) Confirm(ctx context.Context, callerID, bookingID uuid.UUID) (Booking, error) {
	b, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if !participant(b, callerID) {
		return Booking{}, ErrForbidden
	}
	if b.Status != StatusPendingPayment {
		return Booking{}, ErrInvalidTransition
	}
	return s.repo.SetStatus(ctx, bookingID, StatusConfirmed)
}

// Cancel moves pending_payment | confirmed -> cancelled. Participant-only.
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
	return s.repo.Cancel(ctx, bookingID, strings.TrimSpace(reason))
}

func participant(b Booking, callerID uuid.UUID) bool {
	return b.Student.ID == callerID || (b.TeacherOwnerID != uuid.Nil && b.TeacherOwnerID == callerID)
}

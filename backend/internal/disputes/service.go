package disputes

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"
)

// Repository is the persistence port. The concrete implementation
// (repositoryPostgres) lives alongside; tests use a fake.
type Repository interface {
	// BookingForDispute returns the participant + status slice of a booking, or
	// ErrBookingNotFound.
	BookingForDispute(ctx context.Context, bookingID uuid.UUID) (BookingRef, error)

	// Create inserts an open dispute. It maps the
	// disputes_one_open_per_booking violation (SQLSTATE 23505) to
	// ErrDisputeExists — race-safe, never a check-then-insert.
	Create(ctx context.Context, bookingID, raisedBy uuid.UUID, reason string) (Dispute, error)

	// ListForBooking returns a booking's whole dispute thread, newest first.
	ListForBooking(ctx context.Context, bookingID uuid.UUID) ([]Dispute, error)

	// OpenForBooking returns the booking's open dispute; ok is false when there
	// is none.
	OpenForBooking(ctx context.Context, bookingID uuid.UUID) (Dispute, bool, error)

	// ListQueue returns a page of the operator queue (status "" = every status)
	// and the total match count.
	ListQueue(ctx context.Context, status string, limit, offset int) (items []QueueItem, total int, err error)

	// Resolve closes an OPEN dispute. It returns ErrDisputeNotFound for an
	// unknown id and ErrAlreadyResolved when the dispute is no longer open (the
	// UPDATE is guarded on status = 'open', so a lost race is reported rather
	// than clobbering the other operator's resolution).
	Resolve(ctx context.Context, disputeID uuid.UUID, status Status, resolution string, resolvedBy uuid.UUID) (Dispute, error)
}

// Service holds the dispute business rules. Handlers call it; it never sees a
// *gin.Context.
type Service struct {
	repo     Repository
	refunder Refunder // nil until SetRefunder; guarded at every use
	logger   *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{repo: repo, logger: logger}
}

// SetRefunder wires the booking-refund port in. Called once at startup
// (cmd/api). Optional: without it, resolving a dispute with `refund: true` logs
// and moves on.
func (s *Service) SetRefunder(r Refunder) { s.refunder = r }

// Raise records a participant's dispute against a booking. The booking must be
// `confirmed` or `completed` (ErrNotAllowed otherwise) and may have at most one
// open dispute at a time (ErrDisputeExists, enforced by the DB's partial unique
// index).
func (s *Service) Raise(ctx context.Context, callerID, bookingID uuid.UUID, reason string) (Dispute, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Dispute{}, invalid("`reason` is required.")
	}
	if len([]rune(reason)) > MaxReasonLen {
		return Dispute{}, invalid("`reason` must be at most %d characters.", MaxReasonLen)
	}

	b, err := s.repo.BookingForDispute(ctx, bookingID)
	if err != nil {
		return Dispute{}, err
	}
	if !participant(b, callerID) {
		return Dispute{}, ErrForbidden
	}
	if b.Status != bookingConfirmed && b.Status != bookingCompleted {
		return Dispute{}, ErrNotAllowed
	}

	return s.repo.Create(ctx, bookingID, callerID, reason)
}

// ListForBooking returns the booking's dispute thread, newest first.
// Participant-only.
func (s *Service) ListForBooking(ctx context.Context, callerID, bookingID uuid.UUID) ([]Dispute, error) {
	b, err := s.repo.BookingForDispute(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if !participant(b, callerID) {
		return nil, ErrForbidden
	}
	ds, err := s.repo.ListForBooking(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if ds == nil {
		ds = []Dispute{}
	}
	return ds, nil
}

// ListQueue returns one page of the operator queue. status defaults to `open`;
// the literal "all" lifts the filter so an operator can review closed disputes
// too. page defaults to 1, pageSize to 20 (capped at 100).
func (s *Service) ListQueue(ctx context.Context, status string, page, pageSize int) (Page, error) {
	filter := status
	switch {
	case filter == "":
		filter = string(StatusOpen)
	case filter == "all":
		filter = "" // no status predicate
	case !Status(filter).valid():
		return Page{}, invalid("`status` must be one of open, resolved, rejected, all.")
	}

	limit, offset := normalizePage(page, pageSize)
	items, total, err := s.repo.ListQueue(ctx, filter, limit, offset)
	if err != nil {
		return Page{}, err
	}
	if items == nil {
		items = []QueueItem{}
	}
	return Page{Disputes: items, Total: total}, nil
}

// Resolve closes a dispute as `resolved` or `rejected` with a required
// resolution note. When the outcome is `resolved` and refund is true, the
// booking's payment is refunded through the Refunder port — the booking itself
// is deliberately left alone (an operator who also wants the lesson cancelled
// uses force-cancel).
//
// A refund failure is logged, not returned: the operator's decision is already
// committed and must not be lost to a payment-provider hiccup. Same rule as the
// refund on cancel. TODO(payments): enqueue a refund retry.
func (s *Service) Resolve(ctx context.Context, adminID, disputeID uuid.UUID, outcome, resolution string, refund bool) (Dispute, error) {
	st := Status(strings.TrimSpace(outcome))
	if st != StatusResolved && st != StatusRejected {
		return Dispute{}, invalid("`outcome` must be `resolved` or `rejected`.")
	}
	resolution = strings.TrimSpace(resolution)
	if resolution == "" {
		return Dispute{}, invalid("`resolution` is required.")
	}
	if len([]rune(resolution)) > MaxResolutionLen {
		return Dispute{}, invalid("`resolution` must be at most %d characters.", MaxResolutionLen)
	}

	d, err := s.repo.Resolve(ctx, disputeID, st, resolution, adminID)
	if err != nil {
		return Dispute{}, err
	}

	if st == StatusResolved && refund {
		s.refund(ctx, d.BookingID)
	}
	return d, nil
}

// OpenDisputeForBooking returns the booking's open dispute (ok=false when there
// is none). Backs the bookings.DisputeReader port.
func (s *Service) OpenDisputeForBooking(ctx context.Context, bookingID uuid.UUID) (Dispute, bool, error) {
	return s.repo.OpenForBooking(ctx, bookingID)
}

// refund is the guarded Refunder call: nil port or a provider error is logged
// and swallowed.
func (s *Service) refund(ctx context.Context, bookingID uuid.UUID) {
	if s.refunder == nil {
		s.log().Warn("dispute refund skipped: no refunder wired", slog.String("booking_id", bookingID.String()))
		return
	}
	if err := s.refunder.AdminRefund(ctx, bookingID); err != nil {
		s.log().Error("refund on dispute resolution", slog.String("booking_id", bookingID.String()), slog.Any("error", err))
	}
}

func (s *Service) log() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

func participant(b BookingRef, callerID uuid.UUID) bool {
	return b.StudentID == callerID || (b.TeacherOwnerID != uuid.Nil && b.TeacherOwnerID == callerID)
}

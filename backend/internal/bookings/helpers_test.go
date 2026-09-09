package bookings

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testTokenManager() *auth.TokenManager {
	return auth.NewTokenManager("bookings-test-secret", time.Minute)
}

func bearerFor(tm *auth.TokenManager, id uuid.UUID) string {
	tok, err := tm.IssueAccess(auth.User{ID: id, Email: "demo@example.com", DisplayName: "Demo"}, time.Now())
	if err != nil {
		panic(err)
	}
	return "Bearer " + tok
}

// fixedNow is the reference instant for the deterministic tests. 2026-01-05 is a
// Monday (00:00 UTC).
var fixedNow = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

func ptrInt64(v int64) *int64 { return &v }

// fakeRepo is an in-memory Repository for service + handler tests.
type fakeRepo struct {
	tc    TeacherContext
	tcErr error

	ownedID     uuid.UUID
	ownsProfile bool
	ownerErr    error

	spans  []AvailabilitySpan
	booked []Interval

	store     map[uuid.UUID]Booking
	createErr error
	created   *CreateBookingParams

	listResult []Booking
	lastFilter ListFilter
	statusErr  error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{store: map[uuid.UUID]Booking{}} }

func (f *fakeRepo) TeacherContextBySlug(context.Context, string) (TeacherContext, error) {
	return f.tc, f.tcErr
}

func (f *fakeRepo) TeacherIDOwnedBy(context.Context, uuid.UUID) (uuid.UUID, bool, error) {
	return f.ownedID, f.ownsProfile, f.ownerErr
}

func (f *fakeRepo) WeeklyAvailability(context.Context, uuid.UUID) ([]AvailabilitySpan, error) {
	return f.spans, nil
}

func (f *fakeRepo) BookedIntervals(context.Context, uuid.UUID, time.Time, time.Time) ([]Interval, error) {
	return f.booked, nil
}

func (f *fakeRepo) CreateBooking(_ context.Context, p CreateBookingParams) (Booking, error) {
	f.created = &p
	if f.createErr != nil {
		return Booking{}, f.createErr
	}
	id := uuid.New()
	b := Booking{
		ID:              id,
		Status:          StatusPendingPayment,
		StartAt:         p.StartAt,
		EndAt:           p.EndAt,
		DurationMinutes: p.DurationMinutes,
		IsTrial:         p.IsTrial,
		Price:           Money{AmountMinor: p.PriceMinor, Currency: p.Currency},
		CreatedAt:       fixedNow,
		Teacher: TeacherSummary{
			Slug: f.tc.Slug, DisplayName: f.tc.DisplayName,
			Timezone: f.tc.Timezone, AvatarURL: f.tc.AvatarURL,
		},
		Student:           StudentSummary{ID: p.StudentID, DisplayName: "Student"},
		TeacherOwnerID:    f.tc.OwnerID,
		TeacherMeetingURL: f.tc.MeetingURL,
	}
	f.store[id] = b
	return b, nil
}

func (f *fakeRepo) GetBooking(_ context.Context, id uuid.UUID) (Booking, error) {
	b, ok := f.store[id]
	if !ok {
		return Booking{}, ErrBookingNotFound
	}
	return b, nil
}

func (f *fakeRepo) ListBookings(_ context.Context, filter ListFilter) ([]Booking, error) {
	f.lastFilter = filter
	return f.listResult, nil
}

func (f *fakeRepo) SetStatus(_ context.Context, id uuid.UUID, status Status) (Booking, error) {
	if f.statusErr != nil {
		return Booking{}, f.statusErr
	}
	b := f.store[id]
	b.Status = status
	f.store[id] = b
	return b, nil
}

func (f *fakeRepo) Cancel(_ context.Context, id uuid.UUID, reason, by string) (Booking, error) {
	if f.statusErr != nil {
		return Booking{}, f.statusErr
	}
	b := f.store[id]
	b.Status = StatusCancelled
	t := fixedNow
	b.CancelledAt = &t
	b.CancellationReason = reason
	b.CancelledBy = by
	f.store[id] = b
	return b, nil
}

func (f *fakeRepo) SetMeetingLinkOverride(_ context.Context, id uuid.UUID, url string) (Booking, error) {
	b := f.store[id]
	b.MeetingURLOverride = url
	f.store[id] = b
	return b, nil
}

func (f *fakeRepo) SetNoShowParty(_ context.Context, id uuid.UUID, party string) error {
	b := f.store[id]
	b.NoShowParty = party
	f.store[id] = b
	return nil
}

// fakeScheduler records Schedule / Cancel calls for assertions.
type fakeScheduler struct {
	scheduled []uuid.UUID
	cancelled []uuid.UUID
	err       error
}

func (s *fakeScheduler) Schedule(_ context.Context, id uuid.UUID, _ time.Time) error {
	s.scheduled = append(s.scheduled, id)
	return s.err
}

func (s *fakeScheduler) Cancel(_ context.Context, id uuid.UUID) error {
	s.cancelled = append(s.cancelled, id)
	return s.err
}

// fakeNotifier records the lifecycle emails the service asked for.
type fakeNotifier struct {
	confirmed []uuid.UUID
	cancelled []uuid.UUID
	refunded  map[uuid.UUID]bool
}

func newFakeNotifier() *fakeNotifier {
	return &fakeNotifier{refunded: map[uuid.UUID]bool{}}
}

func (n *fakeNotifier) BookingConfirmed(_ context.Context, b Booking) {
	n.confirmed = append(n.confirmed, b.ID)
}

func (n *fakeNotifier) BookingCancelled(_ context.Context, b Booking, _ uuid.UUID, refunded bool) {
	n.cancelled = append(n.cancelled, b.ID)
	n.refunded[b.ID] = refunded
}

// newService builds a Service over the fake repo with a frozen clock and a
// discard logger. No payment gateway (pay / complete return ErrPaymentRequired).
func newService(repo Repository) *Service {
	return &Service{repo: repo, now: func() time.Time { return fixedNow }, logger: discardLogger()}
}

// newServiceWithGateway is newService plus a wired payment gateway.
func newServiceWithGateway(repo Repository, gw PaymentGateway) *Service {
	s := newService(repo)
	s.payments = gw
	return s
}

// fakeGateway is an in-memory bookings.PaymentGateway. Authorize emulates the
// payment webhook by flipping the booking to confirmed in the backing repo,
// exactly as the real payments module does inside HandleWebhook.
type fakeGateway struct {
	repo *fakeRepo

	authErr    error // Authorize returns this when set (e.g. PaymentFailedError)
	captureErr error // Capture returns this when set (e.g. ErrCaptureFailed)

	initiated map[uuid.UUID]Money
	captured  []uuid.UUID
	refunded  []uuid.UUID
	status    map[uuid.UUID]string // booking id -> payment status
}

func newFakeGateway(repo *fakeRepo) *fakeGateway {
	return &fakeGateway{
		repo:      repo,
		initiated: map[uuid.UUID]Money{},
		status:    map[uuid.UUID]string{},
	}
}

func (g *fakeGateway) InitiatePayment(_ context.Context, bookingID uuid.UUID, amountMinor int64, currency string) error {
	g.initiated[bookingID] = Money{AmountMinor: amountMinor, Currency: currency}
	g.status[bookingID] = "requires_payment"
	return nil
}

func (g *fakeGateway) Authorize(ctx context.Context, bookingID uuid.UUID, _ string) (PaymentSnapshot, error) {
	if g.authErr != nil {
		g.status[bookingID] = "failed"
		return PaymentSnapshot{}, g.authErr
	}
	g.status[bookingID] = "authorized"
	// Emulate the payment.authorized webhook: pending_payment -> confirmed.
	if _, err := g.repo.SetStatus(ctx, bookingID, StatusConfirmed); err != nil {
		return PaymentSnapshot{}, err
	}
	return g.snapshot(bookingID), nil
}

func (g *fakeGateway) Capture(_ context.Context, bookingID uuid.UUID) (PaymentSnapshot, error) {
	if g.captureErr != nil {
		return PaymentSnapshot{}, g.captureErr
	}
	g.status[bookingID] = "captured"
	g.captured = append(g.captured, bookingID)
	return g.snapshot(bookingID), nil
}

func (g *fakeGateway) Refund(_ context.Context, bookingID uuid.UUID) (PaymentSnapshot, error) {
	g.status[bookingID] = "refunded"
	g.refunded = append(g.refunded, bookingID)
	return g.snapshot(bookingID), nil
}

func (g *fakeGateway) SnapshotForBooking(_ context.Context, bookingID uuid.UUID) (PaymentSnapshot, bool, error) {
	st, ok := g.status[bookingID]
	if !ok {
		return PaymentSnapshot{}, false, nil
	}
	amt := g.initiated[bookingID]
	return PaymentSnapshot{Status: st, AmountMinor: amt.AmountMinor, Currency: amt.Currency}, true, nil
}

func (g *fakeGateway) snapshot(bookingID uuid.UUID) PaymentSnapshot {
	amt := g.initiated[bookingID]
	return PaymentSnapshot{Status: g.status[bookingID], AmountMinor: amt.AmountMinor, Currency: amt.Currency}
}

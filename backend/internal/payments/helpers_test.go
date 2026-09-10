package payments

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func ctx() context.Context { return context.Background() }

func testTokenManager() *auth.TokenManager {
	return auth.NewTokenManager("payments-test-secret", time.Minute)
}

func bearerFor(tm *auth.TokenManager, id uuid.UUID) string {
	tok, err := tm.IssueAccess(auth.User{ID: id, Email: "demo@example.com", DisplayName: "Demo"}, time.Now())
	if err != nil {
		panic(err)
	}
	return "Bearer " + tok
}

// fakeRepo is an in-memory Repository. ApplyEvent mirrors the transactional
// recipe in repository_postgres.go: event-id dedupe first, then the
// type-specific effect. `applied` records every event id that actually ran an
// effect, so a test can assert "same event_id twice = one effect".
type fakeRepo struct {
	byID      map[uuid.UUID]*Payment
	byBooking map[uuid.UUID]uuid.UUID

	events  map[string]bool
	applied []string

	bookingConfirmed map[uuid.UUID]bool
	ledger           map[uuid.UUID]LedgerState

	teacherByOwner map[uuid.UUID]uuid.UUID
	earnings       map[uuid.UUID][]EarningLine

	ensureErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		byID:             map[uuid.UUID]*Payment{},
		byBooking:        map[uuid.UUID]uuid.UUID{},
		events:           map[string]bool{},
		bookingConfirmed: map[uuid.UUID]bool{},
		ledger:           map[uuid.UUID]LedgerState{},
		teacherByOwner:   map[uuid.UUID]uuid.UUID{},
		earnings:         map[uuid.UUID][]EarningLine{},
	}
}

func (r *fakeRepo) seedPayment(bookingID uuid.UUID, status Status, amountMinor int64, currency string) uuid.UUID {
	id := uuid.New()
	r.byID[id] = &Payment{
		ID: id, BookingID: bookingID, Provider: "fake",
		Status: status, Amount: Money{AmountMinor: amountMinor, Currency: currency},
	}
	r.byBooking[bookingID] = id
	return id
}

func (r *fakeRepo) EnsurePayment(_ context.Context, bookingID uuid.UUID, provider string, amount Money) (Payment, error) {
	if r.ensureErr != nil {
		return Payment{}, r.ensureErr
	}
	if id, ok := r.byBooking[bookingID]; ok {
		return *r.byID[id], nil
	}
	id := r.seedPayment(bookingID, StatusRequiresPayment, amount.AmountMinor, amount.Currency)
	r.byID[id].Provider = provider
	return *r.byID[id], nil
}

func (r *fakeRepo) PaymentByBooking(_ context.Context, bookingID uuid.UUID) (Payment, error) {
	id, ok := r.byBooking[bookingID]
	if !ok {
		return Payment{}, ErrPaymentNotFound
	}
	return *r.byID[id], nil
}

func (r *fakeRepo) PaymentByID(_ context.Context, id uuid.UUID) (Payment, error) {
	p, ok := r.byID[id]
	if !ok {
		return Payment{}, ErrPaymentNotFound
	}
	return *p, nil
}

func (r *fakeRepo) TeacherIDByOwner(_ context.Context, ownerID uuid.UUID) (uuid.UUID, bool, error) {
	id, ok := r.teacherByOwner[ownerID]
	return id, ok, nil
}

func (r *fakeRepo) EarningLines(_ context.Context, teacherID uuid.UUID) ([]EarningLine, error) {
	return r.earnings[teacherID], nil
}

func (r *fakeRepo) ApplyEvent(_ context.Context, e Event) (bool, error) {
	if r.events[e.ID] {
		return false, nil // already processed — no-op
	}
	r.events[e.ID] = true

	p, ok := r.byID[e.PaymentID]
	if !ok {
		return false, ErrPaymentNotFound
	}

	now := time.Now().UTC()
	switch e.Type {
	case EventAuthorized:
		p.Status = StatusAuthorized
		p.ProviderRef = e.ProviderRef
		p.AuthorizedAt = &now
		r.bookingConfirmed[p.BookingID] = true
	case EventCaptured:
		p.Status = StatusCaptured
		p.CapturedAt = &now
		if _, exists := r.ledger[p.BookingID]; !exists {
			// The row opens the clearing window and stays held; nothing flips
			// it to available (that state is derived from available_at).
			r.ledger[p.BookingID] = LedgerHeld
		}
	case EventRefunded:
		p.Status = StatusRefunded
		p.RefundedAt = &now
		// A row already paid out by a batch cannot be un-paid, matching the
		// MarkLedgerReversed guard.
		if st, exists := r.ledger[p.BookingID]; exists && st != LedgerPaid {
			r.ledger[p.BookingID] = LedgerReversed
		}
	case EventFailed:
		p.Status = StatusFailed
		p.LastError = e.Message
	default:
		return false, ErrPaymentNotFound
	}

	r.applied = append(r.applied, e.ID)
	return true, nil
}

// newTestService wires the real deterministic FakeProvider with the service's
// own HandleWebhook as its sink, so the idempotent webhook path runs on every
// provider operation.
func newTestService() (*Service, *fakeRepo) {
	repo := newFakeRepo()
	s := NewService(repo, "fake", discardLogger())
	s.SetProvider(NewFakeProvider(s.Emit))
	return s, repo
}

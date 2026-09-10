package payments

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// FakeProvider is a deterministic, in-memory payments.Provider for the MVP
// (Stripe does not operate in Uzbekistan; a real Payme / Click / Uzum adapter
// drops in behind the same port later).
//
// The MethodToken passed to Authorize drives the outcome:
//
//	"pm_ok"           -> authorizes; capture and refund succeed
//	"pm_decline"      -> Authorize returns Declined (booking stays pending)
//	"pm_capture_fail" -> authorizes, but the FIRST Capture for that ref fails;
//	                     a retry succeeds
//
// Any other token behaves like "pm_ok".
//
// After each state change the fake emits the matching webhook event through the
// injected sink synchronously. The sink is the payments Service's HandleWebhook,
// so the idempotent webhook code path is exercised on every operation exactly
// as it would be for a real provider's HTTP callback.
type FakeProvider struct {
	sink func(context.Context, Event) error

	mu           sync.Mutex
	seq          int
	tokenByRef   map[string]string    // providerRef -> MethodToken it was created with
	paymentByRef map[string]uuid.UUID // providerRef -> payment id
	captureTried map[string]bool      // providerRef -> Capture attempted at least once
}

// NewFakeProvider builds the fake with the given event sink (typically
// (*Service).Emit).
func NewFakeProvider(sink func(context.Context, Event) error) *FakeProvider {
	return &FakeProvider{
		sink:         sink,
		tokenByRef:   map[string]string{},
		paymentByRef: map[string]uuid.UUID{},
		captureTried: map[string]bool{},
	}
}

func (f *FakeProvider) nextRef() string {
	f.seq++
	return fmt.Sprintf("fake_ref_%06d", f.seq)
}

// Authorize places a (fake) hold. "pm_decline" emits payment.failed and returns
// Declined; everything else emits payment.authorized.
func (f *FakeProvider) Authorize(ctx context.Context, in AuthorizeInput) (Intent, error) {
	if in.MethodToken == "pm_decline" {
		reason := "the card was declined"
		if err := f.sink(ctx, Event{
			ID:        eventID(in.PaymentID.String()+"_decline", EventFailed),
			Type:      EventFailed,
			PaymentID: in.PaymentID,
			Message:   reason,
		}); err != nil {
			return Intent{}, err
		}
		return Intent{}, Declined{Reason: reason}
	}

	f.mu.Lock()
	ref := f.nextRef()
	f.tokenByRef[ref] = in.MethodToken
	f.paymentByRef[ref] = in.PaymentID
	f.mu.Unlock()

	if err := f.sink(ctx, Event{
		ID:          eventID(ref, EventAuthorized),
		Type:        EventAuthorized,
		PaymentID:   in.PaymentID,
		ProviderRef: ref,
		AmountMinor: in.AmountMinor,
		Currency:    in.Currency,
	}); err != nil {
		return Intent{}, err
	}
	return Intent{ProviderRef: ref, Status: IntentAuthorized}, nil
}

// Capture settles the hold. For a ref created with "pm_capture_fail" the first
// attempt returns an error and emits nothing; a retry succeeds.
func (f *FakeProvider) Capture(ctx context.Context, providerRef string) (Intent, error) {
	f.mu.Lock()
	token, known := f.tokenByRef[providerRef]
	paymentID := f.paymentByRef[providerRef]
	firstTry := !f.captureTried[providerRef]
	f.captureTried[providerRef] = true
	f.mu.Unlock()

	if !known {
		return Intent{}, fmt.Errorf("fake: unknown provider ref %q", providerRef)
	}
	if token == "pm_capture_fail" && firstTry {
		return Intent{}, fmt.Errorf("fake: transient capture failure for %q", providerRef)
	}

	if err := f.sink(ctx, Event{
		ID:          eventID(providerRef, EventCaptured),
		Type:        EventCaptured,
		PaymentID:   paymentID,
		ProviderRef: providerRef,
	}); err != nil {
		return Intent{}, err
	}
	return Intent{ProviderRef: providerRef, Status: IntentCaptured}, nil
}

// Refund releases the hold / refunds the capture and emits payment.refunded.
func (f *FakeProvider) Refund(ctx context.Context, providerRef string, amountMinor int64) (Intent, error) {
	f.mu.Lock()
	_, known := f.tokenByRef[providerRef]
	paymentID := f.paymentByRef[providerRef]
	f.mu.Unlock()

	if !known {
		return Intent{}, fmt.Errorf("fake: unknown provider ref %q", providerRef)
	}

	if err := f.sink(ctx, Event{
		ID:          eventID(providerRef, EventRefunded),
		Type:        EventRefunded,
		PaymentID:   paymentID,
		ProviderRef: providerRef,
		AmountMinor: amountMinor,
	}); err != nil {
		return Intent{}, err
	}
	return Intent{ProviderRef: providerRef, Status: IntentRefunded}, nil
}

package payments

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// WebhookVerifier authenticates an incoming provider webhook before its body is
// applied. A verification failure is a 401 and the event is never processed.
//
// Every real provider (Payme, Click, Uzum, …) signs its callbacks differently;
// the verifier selected by PAYMENTS_PROVIDER owns that scheme. The MVP ships two:
//
//   - noopVerifier  — accepts everything. Selected when PAYMENTS_WEBHOOK_SECRET
//     is unset (local dev, and the in-process fake, which never makes an HTTP
//     call anyway — it routes events straight through Service.HandleWebhook).
//   - hmacVerifier  — HMAC-SHA256 over "<timestamp>.<body>", with the timestamp
//     bound into the MAC and required to be recent, so a captured callback can't
//     be replayed. This is the provider-neutral scheme a real adapter can keep
//     or replace.
type WebhookVerifier interface {
	// Verify checks the raw request body against its headers. nil accepts.
	Verify(h WebhookHeaders, body []byte) error
	// Name identifies the scheme, for logs.
	Name() string
}

// WebhookHeaders is the header subset a verifier may read — kept as a plain
// struct so a verifier never touches *http.Request / gin.
type WebhookHeaders struct {
	Signature string // X-Payment-Signature
	Timestamp string // X-Payment-Timestamp — unix seconds
}

// DefaultWebhookMaxSkew is how far the X-Payment-Timestamp may sit from now
// (in either direction) before the request is rejected as stale / replayed.
const DefaultWebhookMaxSkew = 5 * time.Minute

// NewWebhookVerifier picks the verifier for a provider. Today every provider
// shares the HMAC scheme (and an empty secret disables verification); the switch
// is where a real Payme / Click adapter would slot its own.
func NewWebhookVerifier(provider, secret string, maxSkew time.Duration) WebhookVerifier {
	if maxSkew <= 0 {
		maxSkew = DefaultWebhookMaxSkew
	}
	if secret == "" {
		return noopVerifier{}
	}
	switch provider {
	// case "click": return clickVerifier{...}
	// case "payme": return paymeVerifier{...}
	default:
		return &hmacVerifier{secret: []byte(secret), maxSkew: maxSkew, now: time.Now}
	}
}

// --- noop ---

type noopVerifier struct{}

func (noopVerifier) Verify(WebhookHeaders, []byte) error { return nil }
func (noopVerifier) Name() string                        { return "none" }

// --- hmac ---

type hmacVerifier struct {
	secret  []byte
	maxSkew time.Duration
	now     func() time.Time
}

func (v *hmacVerifier) Name() string { return "hmac-sha256" }

func (v *hmacVerifier) Verify(h WebhookHeaders, body []byte) error {
	if h.Signature == "" {
		return errors.New("missing X-Payment-Signature")
	}
	if h.Timestamp == "" {
		return errors.New("missing X-Payment-Timestamp")
	}

	secs, err := strconv.ParseInt(h.Timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("X-Payment-Timestamp is not a unix timestamp: %w", err)
	}
	skew := v.now().Sub(time.Unix(secs, 0))
	if skew < 0 {
		skew = -skew
	}
	if skew > v.maxSkew {
		return fmt.Errorf("timestamp is %s from now, outside the %s window", skew.Round(time.Second), v.maxSkew)
	}

	mac := hmac.New(sha256.New, v.secret)
	mac.Write([]byte(h.Timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(want), []byte(h.Signature)) != 1 {
		return errors.New("signature mismatch")
	}
	return nil
}

// SignWebhook produces the X-Payment-Signature value for a body at a given time.
// Used by tests and by any local tool that posts to the webhook by hand.
func SignWebhook(secret string, ts time.Time, body []byte) (signature, timestamp string) {
	timestamp = strconv.FormatInt(ts.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil)), timestamp
}

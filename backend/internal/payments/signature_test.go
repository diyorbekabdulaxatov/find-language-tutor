package payments

import (
	"strings"
	"testing"
	"time"
)

func TestNewWebhookVerifier_Selection(t *testing.T) {
	if got := NewWebhookVerifier("fake", "", 0).Name(); got != "none" {
		t.Errorf("empty secret → %q, want none", got)
	}
	if got := NewWebhookVerifier("fake", "s", 0).Name(); got != "hmac-sha256" {
		t.Errorf("secret set → %q, want hmac-sha256", got)
	}
	if _, ok := NewWebhookVerifier("fake", "s", 0).(*hmacVerifier); !ok {
		t.Fatal("want *hmacVerifier")
	}
}

func TestHMACVerifier_SignRoundTrip(t *testing.T) {
	const secret = "top-secret"
	now := time.Date(2026, 3, 4, 9, 30, 0, 0, time.UTC)
	v := &hmacVerifier{secret: []byte(secret), maxSkew: DefaultWebhookMaxSkew, now: func() time.Time { return now }}
	body := []byte(`{"event_id":"evt_42","type":"payment.captured"}`)

	sig, ts := SignWebhook(secret, now.Add(-time.Minute), body)
	if err := v.Verify(WebhookHeaders{Signature: sig, Timestamp: ts}, body); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestHMACVerifier_Rejects(t *testing.T) {
	const secret = "top-secret"
	now := time.Date(2026, 3, 4, 9, 30, 0, 0, time.UTC)
	v := &hmacVerifier{secret: []byte(secret), maxSkew: 5 * time.Minute, now: func() time.Time { return now }}
	body := []byte(`{"event_id":"evt_42"}`)
	goodSig, goodTs := SignWebhook(secret, now, body)

	cases := []struct {
		name string
		h    WebhookHeaders
		body []byte
		want string
	}{
		{"no signature", WebhookHeaders{Timestamp: goodTs}, body, "missing X-Payment-Signature"},
		{"no timestamp", WebhookHeaders{Signature: goodSig}, body, "missing X-Payment-Timestamp"},
		{"timestamp not a number", WebhookHeaders{Signature: goodSig, Timestamp: "yesterday"}, body, "not a unix timestamp"},
		{
			"stale timestamp",
			func() WebhookHeaders { s, ts := SignWebhook(secret, now.Add(-30*time.Minute), body); return WebhookHeaders{Signature: s, Timestamp: ts} }(),
			body,
			"outside the",
		},
		{
			"future timestamp",
			func() WebhookHeaders { s, ts := SignWebhook(secret, now.Add(30*time.Minute), body); return WebhookHeaders{Signature: s, Timestamp: ts} }(),
			body,
			"outside the",
		},
		{"wrong secret", func() WebhookHeaders { s, ts := SignWebhook("other", now, body); return WebhookHeaders{Signature: s, Timestamp: ts} }(), body, "signature mismatch"},
		{"tampered body", WebhookHeaders{Signature: goodSig, Timestamp: goodTs}, []byte(`{"event_id":"evt_99"}`), "signature mismatch"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Verify(tc.h, tc.body)
			if err == nil {
				t.Fatalf("want rejection")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestNoopVerifier_AcceptsEverything(t *testing.T) {
	if err := (noopVerifier{}).Verify(WebhookHeaders{}, []byte("whatever")); err != nil {
		t.Fatalf("noop rejected: %v", err)
	}
}

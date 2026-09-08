// Package email is the transactional-email backend for the findtutor platform:
// a small Emailer port with two implementations — a real one that POSTs to the
// Resend HTTP API, and a logging one used in development when no API key is
// configured — plus the booking-lifecycle templates (confirmed / cancelled /
// reminder).
//
// The package is provider-thin on purpose: standard net/http, no SDK. Callers
// build a Message (usually via the template helpers in this package) and hand
// it to Send.
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Message is one email to one recipient.
type Message struct {
	To       string
	ToName   string
	Subject  string
	HTMLBody string
	TextBody string
}

// Emailer sends transactional mail. Implementations must be safe for concurrent
// use.
type Emailer interface {
	Send(ctx context.Context, m Message) error
}

// Config is the slice of app configuration this package needs. It mirrors the
// fields on config.Config so importing config here is unnecessary.
type Config struct {
	ResendAPIKey string
	EmailFrom    string
}

// New picks an Emailer based on the config: the Resend backend when an API key
// is set, otherwise a logging backend (the dev default).
func New(cfg Config, logger *slog.Logger) Emailer {
	if logger == nil {
		logger = slog.Default()
	}
	from := strings.TrimSpace(cfg.EmailFrom)
	if from == "" {
		from = "findtutor <noreply@findtutor.local>"
	}
	if strings.TrimSpace(cfg.ResendAPIKey) == "" {
		logger.Info("email: RESEND_API_KEY not set, using logging emailer")
		return &logEmailer{logger: logger}
	}
	return &resendEmailer{
		apiKey: cfg.ResendAPIKey,
		from:   from,
		http:   &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
}

// logEmailer logs each message instead of sending it. Used when no Resend key
// is configured (development).
type logEmailer struct{ logger *slog.Logger }

func (e *logEmailer) Send(ctx context.Context, m Message) error {
	e.logger.InfoContext(ctx, "email (not sent: logging emailer)",
		slog.String("to", m.To),
		slog.String("subject", m.Subject),
		slog.String("body", firstLine(m.TextBody, m.HTMLBody)),
	)
	return nil
}

// resendEmailer POSTs to https://api.resend.com/emails.
type resendEmailer struct {
	apiKey string
	from   string
	http   *http.Client
	logger *slog.Logger
}

const resendEndpoint = "https://api.resend.com/emails"

func (e *resendEmailer) Send(ctx context.Context, m Message) error {
	to := m.To
	if m.ToName != "" {
		to = fmt.Sprintf("%s <%s>", m.ToName, m.To)
	}
	payload := map[string]any{
		"from":    e.from,
		"to":      []string{to},
		"subject": m.Subject,
	}
	if m.HTMLBody != "" {
		payload["html"] = m.HTMLBody
	}
	if m.TextBody != "" {
		payload["text"] = m.TextBody
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal email: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build email request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.http.Do(req)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("resend returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

func firstLine(candidates ...string) string {
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if i := strings.IndexByte(c, '\n'); i >= 0 {
			return strings.TrimSpace(c[:i])
		}
		return c
	}
	return ""
}

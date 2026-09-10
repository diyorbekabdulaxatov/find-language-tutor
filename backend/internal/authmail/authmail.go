// Package authmail adapts internal/email to the auth.Mailer port: it owns the
// public link URLs and the templates, so internal/auth never imports
// internal/email. cmd/api builds one and calls auth.Service.SetMailer.
package authmail

import (
	"context"
	"log/slog"
	"strings"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/email"
)

// Mailer sends the account-recovery emails via an email.Emailer, building the
// links against baseURL (the public frontend origin, e.g. http://localhost:3000).
type Mailer struct {
	emailer email.Emailer
	baseURL string
	logger  *slog.Logger
}

// New builds the adapter. baseURL is trimmed of a trailing slash.
func New(emailer email.Emailer, baseURL string, logger *slog.Logger) *Mailer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Mailer{
		emailer: emailer,
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		logger:  logger,
	}
}

var _ auth.Mailer = (*Mailer)(nil)

func (m *Mailer) SendPasswordReset(ctx context.Context, to, name, rawToken string) error {
	subject, html, text := email.PasswordResetContent(name, m.link("/reset-password/", rawToken))
	return m.emailer.Send(ctx, email.Message{
		To: to, ToName: name, Subject: subject, HTMLBody: html, TextBody: text,
	})
}

func (m *Mailer) SendEmailVerification(ctx context.Context, to, name, rawToken string) error {
	subject, html, text := email.EmailVerificationContent(name, m.link("/verify-email/", rawToken))
	return m.emailer.Send(ctx, email.Message{
		To: to, ToName: name, Subject: subject, HTMLBody: html, TextBody: text,
	})
}

func (m *Mailer) link(path, token string) string {
	return m.baseURL + path + token
}

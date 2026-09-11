// Package resourcesmail adapts internal/email to the resources.Mailer port:
// internal/resources never imports internal/email. cmd/api builds one and
// calls resources.Service.SetMailer. Mirrors internal/authmail's shape.
package resourcesmail

import (
	"context"
	"log/slog"
	"strings"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/email"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/resources"
)

// Mailer sends the submission-graded email via an email.Emailer. baseURL is
// accepted for parity with the other mail adapters (and future use once
// submissions have a direct frontend link); it is not used by today's
// template.
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

var _ resources.Mailer = (*Mailer)(nil)

func (m *Mailer) SubmissionGraded(ctx context.Context, to, studentName, resourceTitle string, score, max *int, feedback string) error {
	subject, html, text := email.SubmissionGradedContent(studentName, resourceTitle, score, max, feedback)
	return m.emailer.Send(ctx, email.Message{
		To: to, ToName: studentName, Subject: subject, HTMLBody: html, TextBody: text,
	})
}

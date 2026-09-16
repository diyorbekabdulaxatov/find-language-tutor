// Package moderationmail adapts internal/email to the two moderation ports:
// admin.Mailer (tell a teacher their profile was approved / rejected /
// suspended) and teachers.Notifier (tell the moderation inbox a profile is
// waiting). It owns the link URLs and the templates, so neither internal/admin
// nor internal/teachers imports internal/email. cmd/api builds one and wires
// it into both services.
package moderationmail

import (
	"context"
	"log/slog"
	"strings"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/admin"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/email"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/i18n"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/teachers"
)

// Mailer sends moderation emails via an email.Emailer. baseURL is the public
// frontend origin; inbox is the list of addresses that get queue alerts
// (empty = no alerts, only teacher-facing mail).
type Mailer struct {
	emailer email.Emailer
	baseURL string
	inbox   []string
	logger  *slog.Logger
}

// New builds the adapter. baseURL is trimmed of a trailing slash; blank inbox
// entries are dropped.
func New(emailer email.Emailer, baseURL string, inbox []string, logger *slog.Logger) *Mailer {
	if logger == nil {
		logger = slog.Default()
	}
	var to []string
	for _, a := range inbox {
		if a = strings.TrimSpace(a); a != "" {
			to = append(to, a)
		}
	}
	return &Mailer{
		emailer: emailer,
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		inbox:   to,
		logger:  logger,
	}
}

var (
	_ admin.Mailer      = (*Mailer)(nil)
	_ teachers.Notifier = (*Mailer)(nil)
)

// TeacherStatusChanged implements admin.Mailer. Unknown statuses (there are
// none today — approve/reject/suspend are the only transitions) send nothing.
func (m *Mailer) TeacherStatusChanged(ctx context.Context, locale, to, name string, status teachers.Status, note string) error {
	var subject, html, text string
	switch status {
	case teachers.StatusApproved:
		subject, html, text = email.TeacherApprovedContent(locale, name, m.baseURL+"/dashboard")
	case teachers.StatusRejected:
		subject, html, text = email.TeacherRejectedContent(locale, name, note, m.baseURL+"/dashboard")
	case teachers.StatusSuspended:
		subject, html, text = email.TeacherSuspendedContent(locale, name, note)
	default:
		return nil
	}
	return m.emailer.Send(ctx, email.Message{
		To: to, ToName: name, Subject: subject, HTMLBody: html, TextBody: text,
	})
}

// TeacherSubmitted implements teachers.Notifier: one alert per inbox address,
// detached from the request and best effort (failures are logged). The inbox
// is staff, so the alert is in the default locale rather than the teacher's.
func (m *Mailer) TeacherSubmitted(ctx context.Context, slug, displayName string, resubmitted bool) {
	if len(m.inbox) == 0 {
		return
	}
	subject, html, text := email.TeacherSubmittedContent(i18n.Default, displayName, m.baseURL+"/admin/teachers/"+slug, resubmitted)
	go func(ctx context.Context) {
		for _, to := range m.inbox {
			if err := m.emailer.Send(ctx, email.Message{To: to, Subject: subject, HTMLBody: html, TextBody: text}); err != nil {
				m.logger.Error("send moderation queue alert",
					slog.String("slug", slug), slog.String("to", to), slog.Any("error", err))
			}
		}
	}(context.WithoutCancel(ctx))
}

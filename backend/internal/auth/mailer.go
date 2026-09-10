package auth

import "context"

// Mailer sends the two account-recovery emails. It is an optional port on the
// Service (SetMailer): a nil Mailer is a guarded no-op, so cmd/api without email
// configured and the unit tests still work. The adapter (internal/authmail)
// owns link construction and the templates — this package never imports
// internal/email.
type Mailer interface {
	// SendPasswordReset mails a reset link carrying the raw token.
	SendPasswordReset(ctx context.Context, to, name, rawToken string) error
	// SendEmailVerification mails a verification link carrying the raw token.
	SendEmailVerification(ctx context.Context, to, name, rawToken string) error
}

// noopMailer is the default before SetMailer.
type noopMailer struct{}

func (noopMailer) SendPasswordReset(context.Context, string, string, string) error     { return nil }
func (noopMailer) SendEmailVerification(context.Context, string, string, string) error { return nil }

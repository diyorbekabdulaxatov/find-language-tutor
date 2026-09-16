package teachers

import (
	"context"

	"github.com/google/uuid"
)

// AccountReader answers "has this account confirmed its email?". Defined
// here (the consuming module) and implemented by auth.Service; cmd/api wires
// it with SetAccountReader. Nil fails closed — a profile can't be created
// until the port is wired, which is the safe default.
type AccountReader interface {
	EmailVerified(ctx context.Context, userID uuid.UUID) (bool, error)
}

// Notifier hears about profiles entering the moderation queue — on first
// submission and on every resubmission after a rejection. Implemented by
// internal/moderationmail (an email to the ops inbox); nil is a no-op.
type Notifier interface {
	TeacherSubmitted(ctx context.Context, slug, displayName string, resubmitted bool)
}

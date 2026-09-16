package courses

import (
	"context"

	"github.com/google/uuid"
)

// AccountReader is the slice of the auth module courses needs for the
// publish gate: has the owning account confirmed its email address? The
// auth service satisfies it directly (auth.Service.EmailVerified); this
// package never imports internal/auth.
//
// A nil reader (SetAccountReader never called) makes SetPublished(true) fail
// closed with ErrEmailNotVerified — an unwired gate must not open.
type AccountReader interface {
	EmailVerified(ctx context.Context, userID uuid.UUID) (bool, error)
}

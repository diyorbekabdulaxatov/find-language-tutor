// Package auth is the authentication domain module: email/password accounts,
// argon2id password hashing, stateless HS256 access tokens, and rotating opaque
// refresh tokens backed by a sessions table.
//
// Layout mirrors the other modules: domain types + errors here, a Service with
// the business rules, a Repository port (Postgres impl alongside, fake in
// tests), gin handlers, and a RegisterRoutes func. The service never sees an
// *gin.Context — the handler and the RequireAuth middleware translate HTTP
// identity into plain values before calling in.
package auth

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// User is the account aggregate as the rest of the app sees it — never the
// password hash.
type User struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Session is one issued refresh token. The raw token itself is never stored —
// only RefreshTokenHash (SHA-256). ReplacedBy points at the session that
// superseded this one during rotation.
type Session struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *uuid.UUID
	UserAgent  string
	CreatedAt  time.Time
}

func (s Session) revoked() bool              { return s.RevokedAt != nil }
func (s Session) expired(now time.Time) bool { return !s.ExpiresAt.After(now) }

// Errors returned by the service. Handlers map these to HTTP statuses; callers
// should test with errors.Is.
var (
	// ErrEmailTaken — Register with an address that already has an account.
	ErrEmailTaken = errors.New("auth: email already registered")
	// ErrInvalidCredentials — Login with an unknown email or wrong password.
	// Deliberately does not say which.
	ErrInvalidCredentials = errors.New("auth: invalid email or password")
	// ErrInvalidRefreshToken — the refresh cookie is missing, malformed,
	// unknown, expired, or already revoked.
	ErrInvalidRefreshToken = errors.New("auth: invalid refresh token")
	// ErrUserNotFound — CurrentUser for an id with no row (e.g. deleted account
	// still holding a valid-looking access token).
	ErrUserNotFound = errors.New("auth: user not found")
)

// ValidationError is a client-fixable problem with registration input (bad
// email, short password). The handler renders it as a 400.
type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

func invalid(msg string) error { return ValidationError{msg: msg} }

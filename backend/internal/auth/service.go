package auth

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

// minPasswordLen is the floor enforced at registration. No max here — argon2
// handles long inputs fine; the handler caps the request body size.
const minPasswordLen = 8

// NewUser / NewSession are the repository's create inputs.
type NewUser struct {
	Email        string
	PasswordHash string
	DisplayName  string
}

type NewSession struct {
	UserID           uuid.UUID
	RefreshTokenHash []byte
	ExpiresAt        time.Time
	UserAgent        string
}

// Repository is the persistence port. The Postgres implementation lives
// alongside; tests use an in-memory fake.
type Repository interface {
	// CreateUser inserts an account. Returns ErrEmailTaken if the email
	// (case-insensitive) already exists.
	CreateUser(ctx context.Context, in NewUser) (User, error)
	// UserWithHashByEmail returns the account and its stored password hash, or
	// ErrUserNotFound.
	UserWithHashByEmail(ctx context.Context, email string) (User, string, error)
	// UserByID returns the account, or ErrUserNotFound.
	UserByID(ctx context.Context, id uuid.UUID) (User, error)
	// UpdateUser edits the mutable account fields (currently just the display
	// name) and returns the updated account, or ErrUserNotFound.
	UpdateUser(ctx context.Context, id uuid.UUID, displayName string) (User, error)

	// CreateSession stores a new refresh-token session.
	CreateSession(ctx context.Context, in NewSession) (Session, error)
	// SessionByRefreshHash looks a session up by the SHA-256 of its raw token,
	// or ErrInvalidRefreshToken.
	SessionByRefreshHash(ctx context.Context, hash []byte) (Session, error)
	// RevokeSession marks one session revoked (no-op if already revoked),
	// optionally recording the session that replaced it.
	RevokeSession(ctx context.Context, id uuid.UUID, replacedBy *uuid.UUID) error
	// RevokeAllUserSessions revokes every still-active session for a user
	// (used on refresh-token reuse detection).
	RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error
}

// AuthResult is what a successful register / login / refresh produces. The
// handler puts AccessToken in the JSON body and RefreshToken in the cookie.
type AuthResult struct {
	User           User
	AccessToken    string
	AccessTTL      time.Duration
	RefreshToken   string // raw opaque token — cookie value, never persisted
	RefreshExpires time.Time
}

// Service holds the auth business logic. Handlers call it; it never sees an
// *gin.Context.
type Service struct {
	repo       Repository
	tokens     *TokenManager
	refreshTTL time.Duration
	now        func() time.Time
}

func NewService(repo Repository, tokens *TokenManager, refreshTTL time.Duration) *Service {
	return &Service{repo: repo, tokens: tokens, refreshTTL: refreshTTL, now: time.Now}
}

// Register creates an account and immediately logs it in. A ValidationError
// means the input was client-fixable; ErrEmailTaken means the address is in use.
func (s *Service) Register(ctx context.Context, email, password, displayName, userAgent string) (AuthResult, error) {
	email = normalizeEmail(email)
	displayName = strings.TrimSpace(displayName)

	if !validEmail(email) {
		return AuthResult{}, invalid("A valid email address is required.")
	}
	if len(password) < minPasswordLen {
		return AuthResult{}, invalid("Password must be at least 8 characters.")
	}
	if displayName == "" {
		return AuthResult{}, invalid("A display name is required.")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return AuthResult{}, err
	}

	user, err := s.repo.CreateUser(ctx, NewUser{Email: email, PasswordHash: hash, DisplayName: displayName})
	if err != nil {
		return AuthResult{}, err // ErrEmailTaken or a real failure
	}

	return s.startSessionResult(ctx, user, userAgent)
}

// Login verifies credentials and starts a session. Always returns
// ErrInvalidCredentials for any bad-credential case, without saying which field
// was wrong.
func (s *Service) Login(ctx context.Context, email, password, userAgent string) (AuthResult, error) {
	email = normalizeEmail(email)

	user, hash, err := s.repo.UserWithHashByEmail(ctx, email)
	if err != nil {
		// Spend roughly the same time as a real verify to blunt the timing
		// oracle that would otherwise reveal whether the email exists.
		_, _ = VerifyPassword(password, dummyHash)
		return AuthResult{}, ErrInvalidCredentials
	}

	ok, err := VerifyPassword(password, hash)
	if err != nil || !ok {
		return AuthResult{}, ErrInvalidCredentials
	}

	return s.startSessionResult(ctx, user, userAgent)
}

// Refresh rotates a refresh token: the presented session is revoked and a fresh
// access+refresh pair is issued. If the presented session was already revoked
// the token has been re-used — every session for that user is revoked and
// ErrInvalidRefreshToken is returned.
func (s *Service) Refresh(ctx context.Context, rawRefreshToken, userAgent string) (AuthResult, error) {
	if rawRefreshToken == "" {
		return AuthResult{}, ErrInvalidRefreshToken
	}

	sess, err := s.repo.SessionByRefreshHash(ctx, hashRefreshToken(rawRefreshToken))
	if err != nil {
		return AuthResult{}, ErrInvalidRefreshToken
	}

	if sess.revoked() {
		// Reuse of a rotated-away token: assume the chain is compromised.
		_ = s.repo.RevokeAllUserSessions(ctx, sess.UserID)
		return AuthResult{}, ErrInvalidRefreshToken
	}
	if sess.expired(s.now()) {
		_ = s.repo.RevokeSession(ctx, sess.ID, nil)
		return AuthResult{}, ErrInvalidRefreshToken
	}

	user, err := s.repo.UserByID(ctx, sess.UserID)
	if err != nil {
		return AuthResult{}, ErrInvalidRefreshToken
	}

	res, err := s.startSession(ctx, user, userAgent)
	if err != nil {
		return AuthResult{}, err
	}

	// Link the old session to the new one and revoke it.
	if err := s.repo.RevokeSession(ctx, sess.ID, &res.newSessionID); err != nil {
		return AuthResult{}, err
	}

	return res.AuthResult, nil
}

// Logout revokes the session behind the presented refresh token. It is
// idempotent: an unknown or empty token is not an error (the handler still
// clears the cookie and returns 204).
func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	if rawRefreshToken == "" {
		return nil
	}
	sess, err := s.repo.SessionByRefreshHash(ctx, hashRefreshToken(rawRefreshToken))
	if err != nil {
		return nil
	}
	return s.repo.RevokeSession(ctx, sess.ID, nil)
}

// CurrentUser loads the account for an authenticated request.
func (s *Service) CurrentUser(ctx context.Context, id uuid.UUID) (User, error) {
	return s.repo.UserByID(ctx, id)
}

// UpdateCurrentUser edits the caller's own account. displayName is nil when the
// client did not send the field (a no-op that just returns the current account);
// an empty/whitespace display name is a ValidationError. Email changes are out
// of scope and silently ignored by the handler.
func (s *Service) UpdateCurrentUser(ctx context.Context, id uuid.UUID, displayName *string) (User, error) {
	if displayName == nil {
		return s.repo.UserByID(ctx, id)
	}
	name := strings.TrimSpace(*displayName)
	if name == "" {
		return User{}, invalid("A display name is required.")
	}
	return s.repo.UpdateUser(ctx, id, name)
}

// sessionResult carries the new session id back to Refresh without exposing it
// on the public AuthResult.
type sessionResult struct {
	AuthResult
	newSessionID uuid.UUID
}

func (s *Service) startSession(ctx context.Context, user User, userAgent string) (sessionResult, error) {
	now := s.now()

	access, err := s.tokens.IssueAccess(user, now)
	if err != nil {
		return sessionResult{}, err
	}

	raw, hash, err := newRefreshToken()
	if err != nil {
		return sessionResult{}, err
	}

	expires := now.Add(s.refreshTTL)
	sess, err := s.repo.CreateSession(ctx, NewSession{
		UserID:           user.ID,
		RefreshTokenHash: hash,
		ExpiresAt:        expires,
		UserAgent:        userAgent,
	})
	if err != nil {
		return sessionResult{}, err
	}

	return sessionResult{
		AuthResult: AuthResult{
			User:           user,
			AccessToken:    access,
			AccessTTL:      s.tokens.AccessTTL(),
			RefreshToken:   raw,
			RefreshExpires: expires,
		},
		newSessionID: sess.ID,
	}, nil
}

// Register/Login want a plain AuthResult; unwrap.
func (s *Service) startSessionResult(ctx context.Context, user User, ua string) (AuthResult, error) {
	r, err := s.startSession(ctx, user, ua)
	return r.AuthResult, err
}

func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func validEmail(s string) bool {
	if len(s) < 3 || len(s) > 254 || !strings.Contains(s, "@") {
		return false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return false
	}
	// Reject display-name forms ("Foo <a@b>"); we want the bare address.
	if addr.Address != s {
		return false
	}
	at := strings.LastIndex(s, "@")
	return strings.Contains(s[at+1:], ".")
}

// dummyHash is a valid argon2id hash of a random throwaway password, used to
// equalise Login timing when the email is unknown.
var dummyHash = mustHash("timing-equaliser-not-a-real-password")

func mustHash(pw string) string {
	h, err := HashPassword(pw)
	if err != nil {
		panic(err)
	}
	return h
}

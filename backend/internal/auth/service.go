package auth

import (
	"context"
	"log/slog"
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

	// --- account recovery (migration 000013) ---

	// UserByEmail returns the account for a login-style email lookup, or
	// ErrUserNotFound. Like UserWithHashByEmail without the hash.
	UserByEmail(ctx context.Context, email string) (User, error)
	// SetUserPassword replaces the stored password hash.
	SetUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
	// MarkEmailVerified stamps email_verified_at (idempotent).
	MarkEmailVerified(ctx context.Context, userID uuid.UUID) error

	// CreateAuthToken stores a single-use link token (SHA-256 only) and
	// returns nothing — the raw value is the caller's to email.
	CreateAuthToken(ctx context.Context, userID uuid.UUID, purpose TokenPurpose, tokenHash []byte, expiresAt time.Time) error
	// LiveAuthToken resolves a raw token hash + purpose to the owning user,
	// only while the token is unconsumed and unexpired. ErrInvalidToken otherwise.
	LiveAuthToken(ctx context.Context, tokenHash []byte, purpose TokenPurpose) (userID uuid.UUID, err error)
	// AuthTokenUser resolves a hash + purpose to the owning user regardless of
	// consumed / expired state (ok=false when no such token ever existed).
	AuthTokenUser(ctx context.Context, tokenHash []byte, purpose TokenPurpose) (userID uuid.UUID, ok bool, err error)
	// ConsumeUserAuthTokens invalidates a user's outstanding tokens of a purpose.
	ConsumeUserAuthTokens(ctx context.Context, userID uuid.UUID, purpose TokenPurpose) error
	// LatestAuthTokenAt is the created_at of the newest still-live token of a
	// purpose for a user (zero time when there is none) — the resend cooldown.
	LatestAuthTokenAt(ctx context.Context, userID uuid.UUID, purpose TokenPurpose) (time.Time, error)

	// ResetPassword applies a validated reset in one transaction: set the new
	// password hash, consume the token, and revoke every session.
	ResetPassword(ctx context.Context, userID uuid.UUID, tokenHash []byte, passwordHash string) error
	// ConfirmEmail applies a validated verification in one transaction: stamp
	// email_verified_at and consume the token.
	ConfirmEmail(ctx context.Context, userID uuid.UUID, tokenHash []byte) error
}

// AuthResult is what a successful register / login / refresh produces. The
// handler puts AccessToken in the JSON body and RefreshToken in the cookie.
type AuthResult struct {
	User           User
	Permissions    []string // caller's effective permission keys, sorted
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
	perms      PermissionsPort // nil until SetPermissionsPort; nil is a safe no-op
	mailer     Mailer          // noopMailer until SetMailer
	logger     *slog.Logger
}

func NewService(repo Repository, tokens *TokenManager, refreshTTL time.Duration) *Service {
	return &Service{
		repo: repo, tokens: tokens, refreshTTL: refreshTTL,
		now: time.Now, mailer: noopMailer{}, logger: slog.Default(),
	}
}

// SetPermissionsPort wires the RBAC permission resolver in. Optional: without it
// every auth response carries an empty `permissions` array.
func (s *Service) SetPermissionsPort(p PermissionsPort) { s.perms = p }

// SetMailer wires the account-recovery email sender in. Optional: without it the
// forgot-password / verify-email flows still work but send nothing.
func (s *Service) SetMailer(m Mailer) {
	if m != nil {
		s.mailer = m
	}
}

// SetLogger overrides the default slog logger (recovery flows log send failures
// rather than surfacing them).
func (s *Service) SetLogger(l *slog.Logger) {
	if l != nil {
		s.logger = l
	}
}

// PermissionsFor returns the caller's effective permission keys for embedding in
// GET /v1/auth/me. Nil-safe; never errors out a request.
func (s *Service) PermissionsFor(ctx context.Context, id uuid.UUID) []string {
	return s.permissionsFor(ctx, id)
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

	// Mail the "confirm your address" link. Best-effort — a mail failure must
	// not fail the registration (the user can resend from the app).
	s.issueEmailVerification(ctx, user)

	return s.startSessionResult(ctx, user, userAgent)
}

// --- account recovery (migration 000013) ---

// RequestPasswordReset mails a reset link if the address has an account. It
// always returns nil: the caller (the handler) answers 202 either way, so an
// attacker can't probe which emails are registered.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	if !validEmail(email) {
		return nil
	}
	user, err := s.repo.UserByEmail(ctx, email)
	if err != nil {
		return nil // unknown address — say nothing
	}
	if s.onCooldown(ctx, user.ID, PurposePasswordReset) {
		return nil
	}

	raw, hash, err := newOpaqueToken()
	if err != nil {
		s.logger.Error("password reset: token", slog.Any("error", err))
		return nil
	}
	_ = s.repo.ConsumeUserAuthTokens(ctx, user.ID, PurposePasswordReset)
	if err := s.repo.CreateAuthToken(ctx, user.ID, PurposePasswordReset, hash, s.now().Add(passwordResetTTL)); err != nil {
		s.logger.Error("password reset: store token", slog.Any("error", err))
		return nil
	}
	if err := s.mailer.SendPasswordReset(ctx, user.Email, user.DisplayName, raw); err != nil {
		s.logger.Error("password reset: send mail", slog.String("user_id", user.ID.String()), slog.Any("error", err))
	}
	return nil
}

// ResetPassword redeems a reset token and sets a new password. The token is
// consumed and every session is revoked in one transaction. ErrInvalidToken for
// an unknown / used / expired token; ValidationError for a weak password.
func (s *Service) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if len(newPassword) < minPasswordLen {
		return invalid("Password must be at least 8 characters.")
	}
	hash := hashOpaqueToken(rawToken)
	userID, err := s.repo.LiveAuthToken(ctx, hash, PurposePasswordReset)
	if err != nil {
		return ErrInvalidToken
	}
	pwHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.repo.ResetPassword(ctx, userID, hash, pwHash)
}

// VerifyEmail redeems an email-verification token. A double-submit (a client
// retry, React StrictMode's double-invoked effect) that hits a just-consumed
// token still returns nil when the account is already verified — the outcome
// this exact token produced.
func (s *Service) VerifyEmail(ctx context.Context, rawToken string) error {
	hash := hashOpaqueToken(rawToken)
	userID, err := s.repo.LiveAuthToken(ctx, hash, PurposeEmailVerification)
	if err != nil {
		if uid, ok, e := s.repo.AuthTokenUser(ctx, hash, PurposeEmailVerification); e == nil && ok {
			if u, e := s.repo.UserByID(ctx, uid); e == nil && u.EmailVerified {
				return nil
			}
		}
		return ErrInvalidToken
	}
	return s.repo.ConfirmEmail(ctx, userID, hash)
}

// ResendEmailVerification re-issues the verification link for the caller's own
// account. ErrAlreadyVerified when there is nothing to confirm.
func (s *Service) ResendEmailVerification(ctx context.Context, userID uuid.UUID) error {
	user, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.EmailVerified {
		return ErrAlreadyVerified
	}
	if s.onCooldown(ctx, user.ID, PurposeEmailVerification) {
		return nil
	}
	s.issueEmailVerification(ctx, user)
	return nil
}

// issueEmailVerification creates + mails a verification token. Best-effort: all
// failures are logged, none propagate.
func (s *Service) issueEmailVerification(ctx context.Context, user User) {
	raw, hash, err := newOpaqueToken()
	if err != nil {
		s.logger.Error("email verification: token", slog.Any("error", err))
		return
	}
	_ = s.repo.ConsumeUserAuthTokens(ctx, user.ID, PurposeEmailVerification)
	if err := s.repo.CreateAuthToken(ctx, user.ID, PurposeEmailVerification, hash, s.now().Add(emailVerificationTTL)); err != nil {
		s.logger.Error("email verification: store token", slog.Any("error", err))
		return
	}
	if err := s.mailer.SendEmailVerification(ctx, user.Email, user.DisplayName, raw); err != nil {
		s.logger.Error("email verification: send mail", slog.String("user_id", user.ID.String()), slog.Any("error", err))
	}
}

// onCooldown reports whether a still-live token of this purpose was issued for
// the user within resendCooldown.
func (s *Service) onCooldown(ctx context.Context, userID uuid.UUID, purpose TokenPurpose) bool {
	last, err := s.repo.LatestAuthTokenAt(ctx, userID, purpose)
	if err != nil {
		return false // fail open — a DB hiccup shouldn't block recovery
	}
	return !last.IsZero() && s.now().Sub(last) < resendCooldown
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
			Permissions:    s.permissionsFor(ctx, user.ID),
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

package auth

import (
	"context"
	"encoding/hex"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeRepo is an in-memory Repository for service + handler tests.
type fakeRepo struct {
	mu           sync.Mutex
	usersByID    map[uuid.UUID]*storedUser
	usersByEmail map[string]*storedUser
	sessions     map[uuid.UUID]*Session
	byHash       map[string]uuid.UUID
	authTokens   map[string]*fakeAuthToken // hash hex -> token

	failCreateUser error
}

type fakeAuthToken struct {
	userID     uuid.UUID
	purpose    TokenPurpose
	expiresAt  time.Time
	consumedAt *time.Time
	createdAt  time.Time
}

type storedUser struct {
	user User
	hash string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		usersByID:    map[uuid.UUID]*storedUser{},
		usersByEmail: map[string]*storedUser{},
		sessions:     map[uuid.UUID]*Session{},
		byHash:       map[string]uuid.UUID{},
		authTokens:   map[string]*fakeAuthToken{},
	}
}

func (r *fakeRepo) CreateUser(_ context.Context, in NewUser) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failCreateUser != nil {
		return User{}, r.failCreateUser
	}
	if _, ok := r.usersByEmail[in.Email]; ok {
		return User{}, ErrEmailTaken
	}
	now := time.Now()
	u := User{ID: uuid.New(), Email: in.Email, DisplayName: in.DisplayName, CreatedAt: now, UpdatedAt: now}
	su := &storedUser{user: u, hash: in.PasswordHash}
	r.usersByEmail[in.Email] = su
	r.usersByID[u.ID] = su
	return u, nil
}

func (r *fakeRepo) UserWithHashByEmail(_ context.Context, email string) (User, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	su, ok := r.usersByEmail[email]
	if !ok {
		return User{}, "", ErrUserNotFound
	}
	return su.user, su.hash, nil
}

func (r *fakeRepo) UserByID(_ context.Context, id uuid.UUID) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	su, ok := r.usersByID[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return su.user, nil
}

func (r *fakeRepo) UpdateUser(_ context.Context, id uuid.UUID, displayName string) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	su, ok := r.usersByID[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	su.user.DisplayName = displayName
	su.user.UpdatedAt = time.Now()
	return su.user, nil
}

func (r *fakeRepo) CreateSession(_ context.Context, in NewSession) (Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := Session{
		ID:        uuid.New(),
		UserID:    in.UserID,
		ExpiresAt: in.ExpiresAt,
		UserAgent: in.UserAgent,
		CreatedAt: time.Now(),
	}
	r.sessions[s.ID] = &s
	r.byHash[hex.EncodeToString(in.RefreshTokenHash)] = s.ID
	return s, nil
}

func (r *fakeRepo) SessionByRefreshHash(_ context.Context, hash []byte) (Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byHash[hex.EncodeToString(hash)]
	if !ok {
		return Session{}, ErrInvalidRefreshToken
	}
	return *r.sessions[id], nil
}

func (r *fakeRepo) RevokeSession(_ context.Context, id uuid.UUID, replacedBy *uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok || s.RevokedAt != nil {
		return nil
	}
	now := time.Now()
	s.RevokedAt = &now
	s.ReplacedBy = replacedBy
	return nil
}

func (r *fakeRepo) RevokeAllUserSessions(_ context.Context, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, s := range r.sessions {
		if s.UserID == userID && s.RevokedAt == nil {
			s.RevokedAt = &now
		}
	}
	return nil
}

// --- account recovery ---

func (r *fakeRepo) UserByEmail(_ context.Context, email string) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	su, ok := r.usersByEmail[email]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return su.user, nil
}

func (r *fakeRepo) SetUserPassword(_ context.Context, userID uuid.UUID, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	su, ok := r.usersByID[userID]
	if !ok {
		return ErrUserNotFound
	}
	su.hash = hash
	su.user.UpdatedAt = time.Now()
	return nil
}

func (r *fakeRepo) MarkEmailVerified(_ context.Context, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	su, ok := r.usersByID[userID]
	if !ok {
		return ErrUserNotFound
	}
	su.user.EmailVerified = true
	return nil
}

func (r *fakeRepo) CreateAuthToken(_ context.Context, userID uuid.UUID, purpose TokenPurpose, tokenHash []byte, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.authTokens[hex.EncodeToString(tokenHash)] = &fakeAuthToken{
		userID: userID, purpose: purpose, expiresAt: expiresAt, createdAt: time.Now(),
	}
	return nil
}

func (r *fakeRepo) LiveAuthToken(_ context.Context, tokenHash []byte, purpose TokenPurpose) (uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.authTokens[hex.EncodeToString(tokenHash)]
	if !ok || t.purpose != purpose || t.consumedAt != nil || !t.expiresAt.After(time.Now()) {
		return uuid.Nil, ErrInvalidToken
	}
	return t.userID, nil
}

func (r *fakeRepo) ConsumeUserAuthTokens(_ context.Context, userID uuid.UUID, purpose TokenPurpose) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, t := range r.authTokens {
		if t.userID == userID && t.purpose == purpose && t.consumedAt == nil {
			t.consumedAt = &now
		}
	}
	return nil
}

func (r *fakeRepo) LatestAuthTokenAt(_ context.Context, userID uuid.UUID, purpose TokenPurpose) (time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var latest time.Time
	for _, t := range r.authTokens {
		if t.userID == userID && t.purpose == purpose && t.consumedAt == nil && t.createdAt.After(latest) {
			latest = t.createdAt
		}
	}
	return latest, nil
}

func (r *fakeRepo) ResetPassword(_ context.Context, userID uuid.UUID, tokenHash []byte, passwordHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	su, ok := r.usersByID[userID]
	if !ok {
		return ErrUserNotFound
	}
	su.hash = passwordHash
	su.user.UpdatedAt = time.Now()
	now := time.Now()
	if t, ok := r.authTokens[hex.EncodeToString(tokenHash)]; ok {
		t.consumedAt = &now
	}
	for _, s := range r.sessions {
		if s.UserID == userID && s.RevokedAt == nil {
			s.RevokedAt = &now
		}
	}
	return nil
}

func (r *fakeRepo) ConfirmEmail(_ context.Context, userID uuid.UUID, tokenHash []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	su, ok := r.usersByID[userID]
	if !ok {
		return ErrUserNotFound
	}
	su.user.EmailVerified = true
	now := time.Now()
	if t, ok := r.authTokens[hex.EncodeToString(tokenHash)]; ok {
		t.consumedAt = &now
	}
	return nil
}

func (r *fakeRepo) activeSessionCount(userID uuid.UUID) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, s := range r.sessions {
		if s.UserID == userID && s.RevokedAt == nil {
			n++
		}
	}
	return n
}

// newTestService wires a service with a fast token manager (short access TTL)
// and a 24h refresh TTL.
func newTestService(repo Repository) (*Service, *TokenManager) {
	tm := NewTokenManager("test-secret", 15*time.Minute)
	return NewService(repo, tm, 24*time.Hour), tm
}

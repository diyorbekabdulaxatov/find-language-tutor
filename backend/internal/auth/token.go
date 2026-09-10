package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// refreshTokenBytes is the entropy in an opaque refresh token before base64url
// encoding. 32 bytes = 256 bits.
const refreshTokenBytes = 32

// AccessClaims is the payload of an access-token JWT. sub / iat / exp come from
// jwt.RegisteredClaims; email and display_name ride along so the frontend can
// render the header without a /me round-trip.
type AccessClaims struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	jwt.RegisteredClaims
}

// UserID parses the subject claim as a uuid.
func (c AccessClaims) UserID() (uuid.UUID, error) {
	return uuid.Parse(c.Subject)
}

// TokenManager issues and verifies access tokens. It is safe for concurrent use.
type TokenManager struct {
	secret    []byte
	accessTTL time.Duration
}

// NewTokenManager builds a manager for HS256 access tokens.
func NewTokenManager(secret string, accessTTL time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), accessTTL: accessTTL}
}

// AccessTTL is the lifetime stamped on newly issued access tokens.
func (m *TokenManager) AccessTTL() time.Duration { return m.accessTTL }

// IssueAccess mints a signed access token for u, valid for AccessTTL from now.
func (m *TokenManager) IssueAccess(u User, now time.Time) (string, error) {
	claims := AccessClaims{
		Email:       u.Email,
		DisplayName: u.DisplayName,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// ParseAccess verifies signature + expiry and returns the claims. Any problem
// (bad signature, wrong alg, expired, malformed) comes back as an error.
func (m *TokenManager) ParseAccess(tokenString string) (*AccessClaims, error) {
	var claims AccessClaims
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	return &claims, nil
}

// newOpaqueToken returns a fresh random token (base64url, no padding) and its
// SHA-256 hash. Only the hash is ever persisted; the raw value lives in a cookie
// (refresh tokens) or an emailed link (recovery tokens).
func newOpaqueToken() (raw string, hash []byte, err error) {
	b := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("auth: read token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashOpaqueToken(raw), nil
}

// hashOpaqueToken is the one-way function from a raw token to its stored form.
func hashOpaqueToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func newRefreshToken() (raw string, hash []byte, err error) { return newOpaqueToken() }
func hashRefreshToken(raw string) []byte                    { return hashOpaqueToken(raw) }

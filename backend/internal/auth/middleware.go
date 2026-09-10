package auth

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

// gin context keys. Namespaced strings rather than a bare word so nothing else
// collides with them; handlers should not read these directly — use UserID /
// Claims.
const (
	ctxUserIDKey = "auth.user_id"
	ctxClaimsKey = "auth.claims"
)

// RequireAuth verifies the `Authorization: Bearer <jwt>` access token and, on
// success, stashes the user id and claims in the gin context. On any failure it
// writes a 401 envelope and aborts.
func RequireAuth(tm *TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := authenticate(c, tm); !ok {
			web.Unauthorized(c, "A valid access token is required.")
			return
		}
		c.Next()
	}
}

// OptionalAuth sets identity when a valid token is present and is otherwise a
// no-op (no 401). Useful for endpoints that personalise output but do not
// require a login.
func OptionalAuth(tm *TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _ = authenticate(c, tm)
		c.Next()
	}
}

// authenticate parses the bearer token and populates the context. Returns the
// claims and whether authentication succeeded.
func authenticate(c *gin.Context, tm *TokenManager) (*AccessClaims, bool) {
	raw := bearerToken(c.GetHeader("Authorization"))
	if raw == "" {
		return nil, false
	}
	claims, err := tm.ParseAccess(raw)
	if err != nil {
		return nil, false
	}
	uid, err := claims.UserID()
	if err != nil {
		return nil, false
	}
	c.Set(ctxUserIDKey, uid)
	c.Set(ctxClaimsKey, claims)
	return claims, true
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

// UserID returns the authenticated user's id from the gin context. The bool is
// false when the request was not authenticated (RequireAuth not run, or
// OptionalAuth found no token).
func UserID(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(ctxUserIDKey)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

// Claims returns the full verified access-token claims, if present.
func Claims(c *gin.Context) (*AccessClaims, bool) {
	v, ok := c.Get(ctxClaimsKey)
	if !ok {
		return nil, false
	}
	claims, ok := v.(*AccessClaims)
	return claims, ok
}

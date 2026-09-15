package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/ratelimit"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

// Rule is one limit: at most Limit hits per Window for whatever Key derives
// from the request. Key returning ok=false skips the rule for that request
// (e.g. an unauthenticated hit on a per-user rule).
type Rule struct {
	Name   string
	Limit  int
	Window time.Duration
	Key    func(c *gin.Context) (key string, ok bool)
}

// PerIP keys on the client address. Only meaningful once SetTrustedProxies is
// configured — otherwise X-Forwarded-For is spoofable and so is the limit.
func PerIP(c *gin.Context) (string, bool) {
	return "ip:" + c.ClientIP(), true
}

// PerUser keys on the authenticated user; must run after RequireAuth.
func PerUser(c *gin.Context) (string, bool) {
	uid, ok := auth.UserID(c)
	if !ok {
		return "", false
	}
	return "user:" + uid.String(), true
}

// maxPeekBytes bounds how much of a JSON body PerBodyField will buffer to
// find its key. Auth bodies are a few hundred bytes; anything past this is
// not a legitimate login/recovery request and simply isn't keyed by field.
const maxPeekBytes = 64 << 10

// PerBodyField keys on a top-level string field of the JSON body (e.g. the
// email on a login), normalised to lowercase. The body is buffered and put
// back so the handler still sees it. Missing/unparseable → rule skipped.
func PerBodyField(field string) func(*gin.Context) (string, bool) {
	return func(c *gin.Context) (string, bool) {
		if c.Request.Body == nil {
			return "", false
		}
		raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxPeekBytes+1))
		_ = c.Request.Body.Close()
		c.Request.Body = io.NopCloser(bytes.NewReader(raw))
		if err != nil || len(raw) > maxPeekBytes {
			return "", false
		}
		var m map[string]json.RawMessage
		if json.Unmarshal(raw, &m) != nil {
			return "", false
		}
		var v string
		if json.Unmarshal(m[field], &v) != nil {
			return "", false
		}
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			return "", false
		}
		return field + ":" + v, true
	}
}

// RateLimit enforces every rule; the first one exceeded renders a 429 with
// Retry-After. A limiter backend error fails open (logged) — an unreachable
// Redis must not take the login page down with it.
func RateLimit(l ratelimit.Limiter, logger *slog.Logger, rules ...Rule) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, r := range rules {
			key, ok := r.Key(c)
			if !ok {
				continue
			}
			allowed, retryAfter, err := l.Allow(c.Request.Context(), r.Name+":"+key, r.Limit, r.Window)
			if err != nil {
				logger.Warn("rate limiter unavailable, failing open",
					slog.String("rule", r.Name), slog.Any("error", err))
				continue
			}
			if !allowed {
				web.TooManyRequests(c, retryAfter)
				return
			}
		}
		c.Next()
	}
}

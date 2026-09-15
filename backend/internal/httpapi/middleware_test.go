package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/ratelimit"
)

func init() { gin.SetMode(gin.TestMode) }

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestRateLimit_BlocksOverLimitWithRetryAfter(t *testing.T) {
	r := gin.New()
	r.POST("/x", RateLimit(ratelimit.NewMemory(), discard(),
		Rule{Name: "t", Limit: 2, Window: time.Minute, Key: PerIP},
	), func(c *gin.Context) { c.Status(200) })

	for i := 1; i <= 2; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/x", nil))
		if w.Code != 200 {
			t.Fatalf("hit %d: %d", i, w.Code)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/x", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd hit: %d, want 429", w.Code)
	}
	if ra := w.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Errorf("Retry-After=%q, want a positive number of seconds", ra)
	}
	if !strings.Contains(w.Body.String(), `"rate_limited"`) {
		t.Errorf("body=%s, want rate_limited error code", w.Body.String())
	}
}

func TestRateLimit_PerBodyFieldKeysOnEmailAndPreservesBody(t *testing.T) {
	var seenBody string
	r := gin.New()
	r.POST("/login", RateLimit(ratelimit.NewMemory(), discard(),
		Rule{Name: "t", Limit: 1, Window: time.Minute, Key: PerBodyField("email")},
	), func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		seenBody = string(b)
		c.Status(200)
	})

	post := func(body string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w.Code
	}

	if code := post(`{"email":"A@Example.com","password":"x"}`); code != 200 {
		t.Fatalf("first: %d", code)
	}
	if seenBody != `{"email":"A@Example.com","password":"x"}` {
		t.Errorf("handler saw body %q — middleware consumed it", seenBody)
	}
	// Same account, different casing → same key → blocked.
	if code := post(`{"email":"a@example.com","password":"y"}`); code != 429 {
		t.Errorf("same email (case-folded): %d, want 429", code)
	}
	// Different account → independent.
	if code := post(`{"email":"b@example.com","password":"y"}`); code != 200 {
		t.Errorf("other email: %d, want 200", code)
	}
	// No email at all → rule skipped, not blocked.
	if code := post(`{"password":"y"}`); code != 200 {
		t.Errorf("no email: %d, want 200 (rule skipped)", code)
	}
}

// failingLimiter simulates Redis being down.
type failingLimiter struct{}

func (failingLimiter) Allow(_ context.Context, _ string, _ int, _ time.Duration) (bool, time.Duration, error) {
	return false, 0, io.ErrUnexpectedEOF
}

func TestRateLimit_FailsOpenWhenBackendErrors(t *testing.T) {
	r := gin.New()
	r.POST("/x", RateLimit(failingLimiter{}, discard(),
		Rule{Name: "t", Limit: 1, Window: time.Minute, Key: PerIP},
	), func(c *gin.Context) { c.Status(200) })
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/x", nil))
		if w.Code != 200 {
			t.Fatalf("hit %d with a broken limiter: %d, want 200 (fail open)", i, w.Code)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	for _, hsts := range []bool{false, true} {
		r := gin.New()
		r.Use(SecurityHeaders(hsts))
		r.GET("/", func(c *gin.Context) { c.Status(200) })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

		h := w.Header()
		if h.Get("X-Content-Type-Options") != "nosniff" ||
			h.Get("X-Frame-Options") != "DENY" ||
			!strings.Contains(h.Get("Content-Security-Policy"), "default-src 'none'") {
			t.Errorf("hsts=%v: missing hardening headers: %v", hsts, h)
		}
		if got := h.Get("Strict-Transport-Security") != ""; got != hsts {
			t.Errorf("hsts=%v: HSTS header present=%v", hsts, got)
		}
	}
}

func TestMaxBodyBytes_CapsAllButSkippedRoutes(t *testing.T) {
	r := gin.New()
	r.Use(MaxBodyBytes(16, "/big"))
	read := func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.Status(413)
			return
		}
		c.Status(200)
	}
	r.POST("/small", read)
	r.POST("/big", read)

	body := strings.Repeat("x", 64)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/small", strings.NewReader(body)))
	if w.Code != 413 {
		t.Errorf("/small with 64 bytes: %d, want capped", w.Code)
	}
	if !strings.Contains(w.Body.String(), "payload_too_large") {
		t.Errorf("declared Content-Length over the cap should be refused up front, got body %s", w.Body.String())
	}
	// Chunked (unknown length): the reader cap still bites, via the handler.
	w = httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/small", strings.NewReader(body))
	req.ContentLength = -1
	r.ServeHTTP(w, req)
	if w.Code != 413 {
		t.Errorf("/small chunked over cap: %d, want 413 from the handler's read error", w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/big", strings.NewReader(body)))
	if w.Code != 200 {
		t.Errorf("/big (skipped) with 64 bytes: %d, want 200", w.Code)
	}
}

func TestTrustedProxies_ForwardedForIgnoredByDefault(t *testing.T) {
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.GET("/", func(c *gin.Context) { c.String(200, c.ClientIP()) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.7:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.ServeHTTP(w, req)
	if w.Body.String() != "10.0.0.7" {
		t.Errorf("client ip=%q, want the peer address (forged XFF must be ignored)", w.Body.String())
	}
}

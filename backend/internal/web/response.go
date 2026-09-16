// Package web holds HTTP primitives shared by every module's handlers: the
// error envelope and small response helpers. It is a leaf package (only gin
// and i18n), so both internal/httpapi and the domain modules can import it
// without a cycle.
//
// Every message passed to these helpers is written in English at the call
// site and translated here into the request's locale (set on the gin context
// by the Locale middleware). Messages with arguments should go through the
// *f variants or a Localizer error so the format string — not the rendered
// text — is what gets looked up.
package web

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/i18n"
)

// LocaleKey is the gin context key the Locale middleware stores the
// negotiated locale under.
const LocaleKey = "locale"

// Locale returns the request's locale (default when the middleware didn't run).
func Locale(c *gin.Context) string {
	if loc := c.GetString(LocaleKey); i18n.Supported(loc) {
		return loc
	}
	return i18n.Default
}

// ErrorBody is the wire shape for every error response, matching openapi.yaml's
// Error schema: {"error": {"code": "...", "message": "..."}}.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeRaw(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorBody{Error: ErrorDetail{Code: code, Message: message}})
}

// WriteError renders a JSON error with the given HTTP status and aborts the
// chain. message is looked up in the locale's catalog as-is.
func WriteError(c *gin.Context, status int, code, message string) {
	writeRaw(c, status, code, i18n.T(Locale(c), message))
}

// WriteErrorf is WriteError for a message with arguments: the format string
// is translated first, then filled in.
func WriteErrorf(c *gin.Context, status int, code, format string, args ...any) {
	writeRaw(c, status, code, i18n.Tf(Locale(c), format, args...))
}

// WriteErrorFrom renders an error value: a Localizer (the module
// ValidationError types) says itself in the locale; anything else is looked
// up by its English text.
func WriteErrorFrom(c *gin.Context, status int, code string, err error) {
	var l i18n.Localizer
	if errors.As(err, &l) {
		writeRaw(c, status, code, l.Localize(Locale(c)))
		return
	}
	WriteError(c, status, code, err.Error())
}

func BadRequest(c *gin.Context, message string) {
	WriteError(c, http.StatusBadRequest, "bad_request", message)
}

func BadRequestf(c *gin.Context, format string, args ...any) {
	WriteErrorf(c, http.StatusBadRequest, "bad_request", format, args...)
}

// BadRequestErr renders a validation error from a service (400).
func BadRequestErr(c *gin.Context, err error) {
	WriteErrorFrom(c, http.StatusBadRequest, "bad_request", err)
}

func Unauthorized(c *gin.Context, message string) {
	WriteError(c, http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(c *gin.Context, message string) {
	WriteError(c, http.StatusForbidden, "forbidden", message)
}

func Forbiddenf(c *gin.Context, format string, args ...any) {
	WriteErrorf(c, http.StatusForbidden, "forbidden", format, args...)
}

func NotFound(c *gin.Context, message string) {
	WriteError(c, http.StatusNotFound, "not_found", message)
}

func NotFoundf(c *gin.Context, format string, args ...any) {
	WriteErrorf(c, http.StatusNotFound, "not_found", format, args...)
}

func PayloadTooLarge(c *gin.Context, message string) {
	WriteError(c, http.StatusRequestEntityTooLarge, "payload_too_large", message)
}

// TooManyRequests renders a 429 with a Retry-After header (whole seconds,
// rounded up so a client that honours it never retries into the same window).
func TooManyRequests(c *gin.Context, retryAfter time.Duration) {
	secs := int(math.Ceil(retryAfter.Seconds()))
	if secs < 1 {
		secs = 1
	}
	c.Header("Retry-After", strconv.Itoa(secs))
	WriteError(c, http.StatusTooManyRequests, "rate_limited", "Too many requests. Please slow down and try again shortly.")
}

func Internal(c *gin.Context) {
	WriteError(c, http.StatusInternalServerError, "internal", "Something went wrong on our end.")
}

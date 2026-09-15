// Package web holds HTTP primitives shared by every module's handlers: the
// error envelope and small response helpers. It is a leaf package (only gin),
// so both internal/httpapi and the domain modules can import it without a cycle.
package web

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrorBody is the wire shape for every error response, matching openapi.yaml's
// Error schema: {"error": {"code": "...", "message": "..."}}.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError renders a JSON error with the given HTTP status and aborts the chain.
func WriteError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorBody{Error: ErrorDetail{Code: code, Message: message}})
}

func BadRequest(c *gin.Context, message string) {
	WriteError(c, http.StatusBadRequest, "bad_request", message)
}

func Unauthorized(c *gin.Context, message string) {
	WriteError(c, http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(c *gin.Context, message string) {
	WriteError(c, http.StatusForbidden, "forbidden", message)
}

func NotFound(c *gin.Context, message string) {
	WriteError(c, http.StatusNotFound, "not_found", message)
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

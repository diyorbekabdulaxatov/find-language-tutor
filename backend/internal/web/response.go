// Package web holds HTTP primitives shared by every module's handlers: the
// error envelope and small response helpers. It is a leaf package (only gin),
// so both internal/httpapi and the domain modules can import it without a cycle.
package web

import (
	"net/http"

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

func Forbidden(c *gin.Context, message string) {
	WriteError(c, http.StatusForbidden, "forbidden", message)
}

func NotFound(c *gin.Context, message string) {
	WriteError(c, http.StatusNotFound, "not_found", message)
}

func Internal(c *gin.Context) {
	WriteError(c, http.StatusInternalServerError, "internal", "Something went wrong on our end.")
}

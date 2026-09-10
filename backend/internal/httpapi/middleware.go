package httpapi

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

const requestIDHeader = "X-Request-ID"

// RequestID assigns each request a correlation id (honouring an inbound
// X-Request-ID) and echoes it on the response.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}

// StructuredLogger logs one line per request via slog.
func StructuredLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		reqID, _ := c.Get("request_id")
		logger.Info("request",
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("query", c.Request.URL.RawQuery),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("took", time.Since(start)),
			slog.Any("request_id", reqID),
			slog.String("client_ip", c.ClientIP()),
		)
	}
}

// Recovery converts a panic into a 500 JSON error and logs the stack.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, err any) {
		reqID, _ := c.Get("request_id")
		logger.Error("panic recovered",
			slog.Any("error", err),
			slog.Any("request_id", reqID),
			slog.String("path", c.Request.URL.Path),
		)
		web.Internal(c)
	})
}

// CORS allows the browser frontend (and nothing else) to call the API.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowed[origin]; ok {
			// Credentialed CORS: the refresh token is an HttpOnly cookie the
			// browser only sends when the response echoes the specific origin
			// (never "*") together with Allow-Credentials.
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, "+requestIDHeader)
			c.Header("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// healthHandler reports liveness plus the status of each backing service.
// Returns 200 when everything is reachable, 503 otherwise.
func healthHandler(pool *pgxpool.Pool, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		checks := map[string]string{}
		healthy := true

		if err := pool.Ping(ctx); err != nil {
			checks["postgres"] = "down"
			healthy = false
		} else {
			checks["postgres"] = "ok"
		}

		if err := rdb.Ping(ctx).Err(); err != nil {
			checks["redis"] = "down"
			healthy = false
		} else {
			checks["redis"] = "ok"
		}

		status := "ok"
		code := http.StatusOK
		if !healthy {
			status = "degraded"
			code = http.StatusServiceUnavailable
		}

		c.JSON(code, gin.H{"status": status, "checks": checks})
	}
}

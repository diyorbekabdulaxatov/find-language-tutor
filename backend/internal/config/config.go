// Package config loads runtime configuration from environment variables.
//
// In development a .env file at the backend root is loaded first (see Load).
// In production the process environment is the only source.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Env is one of "development", "test", "production".
	Env string

	// HTTP
	Port           string
	AllowedOrigins []string // CORS allow-list for the browser frontend

	// Postgres — a pgx-compatible connection string.
	DatabaseURL string

	// Redis — used for the asynq job queue and for caching later.
	RedisURL string

	// Auth (placeholder — no verification wired yet).
	// When we add Clerk/Auth0 this becomes the issuer + JWKS URL.
	AuthIssuer   string
	AuthAudience string

	// ShutdownTimeout bounds graceful shutdown.
	ShutdownTimeout time.Duration
}

// Load reads configuration. It loads a .env file if one exists next to the
// binary's working directory, then reads the environment. Missing required
// values are a fatal error returned to the caller.
func Load() (*Config, error) {
	// Best-effort: absence of .env is fine (production), a malformed one is not.
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			return nil, fmt.Errorf("loading .env: %w", err)
		}
	}

	cfg := &Config{
		Env:             getenv("APP_ENV", "development"),
		Port:            getenv("PORT", "8080"),
		AllowedOrigins:  splitAndTrim(getenv("ALLOWED_ORIGINS", "http://localhost:3000")),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisURL:        getenv("REDIS_URL", "redis://localhost:6379/0"),
		AuthIssuer:      os.Getenv("AUTH_ISSUER"),
		AuthAudience:    os.Getenv("AUTH_AUDIENCE"),
		ShutdownTimeout: getenvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func splitAndTrim(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

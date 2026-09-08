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

	// Auth. Access tokens are stateless HS256 JWTs signed with JWTSecret;
	// refresh tokens are opaque and stored (hashed) as sessions rows.
	JWTSecret       string        // AUTH_JWT_SECRET — HS256 signing key
	AccessTokenTTL  time.Duration // AUTH_ACCESS_TTL  (default 15m)
	RefreshTokenTTL time.Duration // AUTH_REFRESH_TTL (default 720h / 30d)
	CookieDomain    string        // AUTH_COOKIE_DOMAIN — empty in dev
	CookieSecure    bool          // Secure flag on the refresh cookie

	// ShutdownTimeout bounds graceful shutdown.
	ShutdownTimeout time.Duration

	// Payments. PaymentsProvider selects the payments.Provider implementation
	// ("fake" is the only one for the MVP — Stripe does not operate in
	// Uzbekistan). PaymentsWebhookSecret, when set, is the HMAC-SHA256 key the
	// POST /v1/payments/webhook handler verifies against the X-Payment-Signature
	// header; empty disables verification (dev only).
	PaymentsProvider      string
	PaymentsWebhookSecret string

	// Email. ResendAPIKey selects the transactional-email backend: when empty
	// (the dev default) a logging emailer is used; when set, mail is POSTed to
	// the Resend API. EmailFrom is the From header.
	ResendAPIKey string
	EmailFrom    string
}

// devJWTSecret is used only when APP_ENV != production and AUTH_JWT_SECRET is
// unset, so `make run` works out of the box. Tokens signed with it are not
// secure — production must set AUTH_JWT_SECRET.
const devJWTSecret = "dev-only-insecure-secret-change-me"

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

	env := getenv("APP_ENV", "development")

	cfg := &Config{
		Env:             env,
		Port:            getenv("PORT", "8080"),
		AllowedOrigins:  splitAndTrim(getenv("ALLOWED_ORIGINS", "http://localhost:3000")),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisURL:        getenv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:       os.Getenv("AUTH_JWT_SECRET"),
		AccessTokenTTL:  getenvDuration("AUTH_ACCESS_TTL", 15*time.Minute),
		RefreshTokenTTL: getenvDuration("AUTH_REFRESH_TTL", 30*24*time.Hour),
		CookieDomain:    os.Getenv("AUTH_COOKIE_DOMAIN"),
		CookieSecure:    getenvBool("AUTH_COOKIE_SECURE", env == "production"),
		ShutdownTimeout: getenvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),

		PaymentsProvider:      getenv("PAYMENTS_PROVIDER", "fake"),
		PaymentsWebhookSecret: os.Getenv("PAYMENTS_WEBHOOK_SECRET"),

		ResendAPIKey: os.Getenv("RESEND_API_KEY"),
		EmailFrom:    getenv("EMAIL_FROM", "findtutor <noreply@findtutor.local>"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.JWTSecret == "" {
		if cfg.IsProduction() {
			return nil, fmt.Errorf("AUTH_JWT_SECRET is required in production")
		}
		cfg.JWTSecret = devJWTSecret
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

func getenvBool(key string, fallback bool) bool {
	switch strings.ToLower(os.Getenv(key)) {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
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

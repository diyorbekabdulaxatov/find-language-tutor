// Command api runs the HTTP server for the findtutor backend.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/availability"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/config"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/httpapi"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/teachers"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("fatal", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Root context cancelled on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("connected to postgres")

	redisOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return err
	}
	rdb := redis.NewClient(redisOpt)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Warn("redis not reachable at startup", slog.Any("error", err))
	} else {
		logger.Info("connected to redis")
	}

	tokenManager := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	authHandler := auth.NewHandler(
		auth.NewService(auth.NewPostgresRepository(pool), tokenManager, cfg.RefreshTokenTTL),
		tokenManager,
		auth.CookieConfig{
			Name:   "ftr_session",
			Path:   "/v1/auth",
			Domain: cfg.CookieDomain,
			Secure: cfg.CookieSecure,
			MaxAge: cfg.RefreshTokenTTL,
		},
		logger,
	)

	teacherHandler := teachers.NewHandler(
		teachers.NewService(teachers.NewPostgresRepository(pool)),
		logger,
	)

	availabilityHandler := availability.NewHandler(
		availability.NewService(availability.NewPostgresRepository(pool)),
		logger,
	)

	router := httpapi.NewRouter(httpapi.Deps{
		Config:              cfg,
		Logger:              logger,
		Pool:                pool,
		Redis:               rdb,
		AuthHandler:         authHandler,
		AuthMiddleware:      auth.RequireAuth(tokenManager),
		TeacherHandler:      teacherHandler,
		AvailabilityHandler: availabilityHandler,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Info("server stopped cleanly")
	return nil
}

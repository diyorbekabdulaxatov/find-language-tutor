// Command api runs the HTTP server for the FindTutor backend.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/admin"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/authmail"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/availability"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/config"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/disputes"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/email"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/files"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/httpapi"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/lessons"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/payments"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/payouts"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/resources"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/reviews"
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

	// RBAC: per-request permission resolution. Also feeds the caller's
	// permissions into the auth responses (nil-safe port).
	rbacService := rbac.NewService(rbac.NewPostgresRepository(pool))
	rbacGuard := rbac.NewGuard(rbacService)
	rbacHandler := rbac.NewHandler(rbacService, logger)

	authService := auth.NewService(auth.NewPostgresRepository(pool), tokenManager, cfg.RefreshTokenTTL)
	authService.SetPermissionsPort(rbac.NewAuthPermissions(rbacService))
	authHandler := auth.NewHandler(
		authService,
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

	teacherService := teachers.NewService(teachers.NewPostgresRepository(pool))
	teacherHandler := teachers.NewHandler(teacherService, logger)

	// Admin surface (phase A/B). Reads across tables directly; reuses the
	// teachers read model for the one full-profile endpoint. Authorization is
	// per-route via rbacGuard.
	adminService := admin.NewService(admin.NewPostgresRepository(pool), teacherService)
	adminHandler := admin.NewHandler(adminService, logger)

	availabilityHandler := availability.NewHandler(
		availability.NewService(availability.NewPostgresRepository(pool)),
		logger,
	)

	bookingService := bookings.NewService(bookings.NewPostgresRepository(pool))
	bookingHandler := bookings.NewHandler(bookingService, logger)

	// Phase 5: lesson reminders (asynq) + transactional email. Both are optional
	// ports on the booking service — a nil implementation is a safe no-op.
	if redisOpt, rerr := asynq.ParseRedisURI(cfg.RedisURL); rerr != nil {
		logger.Warn("reminders disabled: bad REDIS_URL", slog.Any("error", rerr))
	} else {
		reminderScheduler := lessons.NewScheduler(redisOpt)
		defer reminderScheduler.Close()
		bookingService.SetReminderScheduler(reminderScheduler)
	}
	mailer := email.New(email.Config{ResendAPIKey: cfg.ResendAPIKey, EmailFrom: cfg.EmailFrom}, logger)
	bookingService.SetNotifier(lessons.NewNotifier(mailer, logger))

	// Account-recovery email (verify address / reset password). Optional port —
	// without it the flows still work but send nothing.
	authService.SetMailer(authmail.New(mailer, cfg.AppBaseURL, logger))
	authService.SetLogger(logger)

	// Payments. The MVP uses a deterministic in-process fake (Stripe does not
	// operate in Uzbekistan); a real Payme / Click / Uzum adapter drops in
	// behind payments.Provider later. The fake's event sink is the payments
	// service's own HandleWebhook, so the idempotent webhook path runs on every
	// operation.
	paymentService := payments.NewService(
		payments.NewPostgresRepository(pool, cfg.PayoutsClearingDays),
		cfg.PaymentsProvider,
		logger,
	)
	paymentService.SetProvider(payments.NewFakeProvider(paymentService.Emit))
	bookingService.SetPaymentGateway(payments.NewGateway(paymentService))
	paymentHandler := payments.NewHandler(
		paymentService,
		payments.NewWebhookVerifier(cfg.PaymentsProvider, cfg.PaymentsWebhookSecret, cfg.PaymentsWebhookMaxSkew),
		logger,
	)

	// Phase 6: lesson reviews. The reviews module exposes a read port back to
	// bookings so a BookingDTO carries can_review / review without a second call.
	reviewService := reviews.NewService(reviews.NewPostgresRepository(pool), logger)
	reviewHandler := reviews.NewHandler(reviewService, logger)
	bookingService.SetReviewReader(reviews.NewBookingGateway(reviewService))

	// Phase D: lesson disputes + the bookings admin surface. The disputes module
	// owns both the participant routes (mounted on /v1/bookings) and the
	// operator queue (mounted on /v1/admin behind the RBAC guard); it exposes a
	// read port back to bookings so a BookingDTO carries can_raise_dispute /
	// open_dispute without a second call. The two write paths that reach into
	// bookings — the admin force-cancel and the refund on a resolved dispute —
	// go through ports defined by their consumers and satisfied by
	// *bookings.Service.
	disputeService := disputes.NewService(disputes.NewPostgresRepository(pool), logger)
	disputeService.SetRefunder(bookingService)
	disputeHandler := disputes.NewHandler(disputeService, logger)
	bookingService.SetDisputeReader(disputes.NewBookingGateway(disputeService))
	adminService.SetBookingModerator(bookingService)

	// Phase E: teacher payouts. Its own module mounted on /v1/admin (like
	// disputes), owning payout_batches and the settlement writes on
	// payout_ledger; internal/payments keeps the ledger writes on the money path
	// and GET /v1/payments/me. The two share the table, not Go types — neither
	// imports the other, so there is no port to wire here.
	payoutHandler := payouts.NewHandler(
		payouts.NewService(payouts.NewPostgresRepository(pool), logger),
		logger,
	)

	// Phase A1: the learning-resource library + its blob store. The store is
	// local disk in dev (FILES_DISK_PATH); an R2 backend drops in behind the
	// same port later. internal/resources references file_asset ids only.
	blob, err := files.NewBlob(cfg.FilesStore, cfg.FilesDiskPath)
	if err != nil {
		return fmt.Errorf("file store: %w", err)
	}
	fileHandler := files.NewHandler(
		files.NewService(files.NewPostgresRepository(pool), blob, logger),
		logger,
	)
	resourceHandler := resources.NewHandler(
		resources.NewService(resources.NewPostgresRepository(pool), logger),
		logger,
	)

	router := httpapi.NewRouter(httpapi.Deps{
		Config:              cfg,
		Logger:              logger,
		Pool:                pool,
		Redis:               rdb,
		AuthHandler:         authHandler,
		AuthMiddleware:      auth.RequireAuth(tokenManager),
		AdminHandler:        adminHandler,
		RBACHandler:         rbacHandler,
		RBACGuard:           rbacGuard,
		TeacherHandler:      teacherHandler,
		AvailabilityHandler: availabilityHandler,
		BookingHandler:      bookingHandler,
		PaymentHandler:      paymentHandler,
		ReviewHandler:       reviewHandler,
		DisputeHandler:      disputeHandler,
		PayoutHandler:       payoutHandler,
		FileHandler:         fileHandler,
		ResourceHandler:     resourceHandler,
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

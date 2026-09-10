// Command worker runs the asynq background job processor. It is a separate
// binary from the API so the two scale and deploy independently.
//
// Jobs:
//   - lesson:reminder — enqueued (24h + 1h before start) by the API when a
//     booking is paid; this binary loads the booking, and if it is still
//     confirmed, emails the student and teacher the meeting link.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/config"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/email"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/lessons"
)

// TaskLessonReminder mirrors lessons.TaskLessonReminder.
const TaskLessonReminder = lessons.TaskLessonReminder

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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("connected to postgres")

	mailer := email.New(email.Config{ResendAPIKey: cfg.ResendAPIKey, EmailFrom: cfg.EmailFrom}, logger)
	bookingRepo := bookings.NewPostgresRepository(pool)

	redisOpt, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		return err
	}

	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
		Logger: newAsynqLogger(logger),
	})

	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskLessonReminder, handleLessonReminder(logger, bookingRepo, mailer))

	if err := srv.Start(mux); err != nil {
		return err
	}
	logger.Info("worker started")

	<-ctx.Done()
	logger.Info("shutdown signal received")
	srv.Shutdown()
	logger.Info("worker stopped cleanly")
	return nil
}

// bookingLoader is the slice of bookings.Repository the reminder handler needs.
type bookingLoader interface {
	GetBooking(ctx context.Context, id uuid.UUID) (bookings.Booking, error)
}

// handleLessonReminder loads the booking and, only if it is still confirmed,
// emails both participants the reminder with the effective meeting link. A
// cancelled / completed booking is a no-op (log + nil, no retry). A send error
// is returned so asynq retries.
func handleLessonReminder(logger *slog.Logger, repo bookingLoader, mailer email.Emailer) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload lessons.ReminderPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			// Malformed payload will never succeed — do not retry.
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		id, err := uuid.Parse(payload.BookingID)
		if err != nil {
			return fmt.Errorf("%w: bad booking id %q", asynq.SkipRetry, payload.BookingID)
		}

		b, err := repo.GetBooking(ctx, id)
		if errors.Is(err, bookings.ErrBookingNotFound) {
			logger.InfoContext(ctx, "lesson reminder: booking gone, skipping", slog.String("booking_id", payload.BookingID))
			return nil
		}
		if err != nil {
			return err // transient — let asynq retry
		}

		if b.Status != bookings.StatusConfirmed {
			logger.InfoContext(ctx, "lesson reminder: booking no longer confirmed, skipping",
				slog.String("booking_id", payload.BookingID),
				slog.String("status", string(b.Status)))
			return nil
		}

		for _, m := range lessons.ReminderMessages(b, payload.Kind) {
			if err := mailer.Send(ctx, m); err != nil {
				return err // retry the whole task
			}
		}
		logger.InfoContext(ctx, "lesson reminder sent",
			slog.String("booking_id", payload.BookingID),
			slog.String("kind", payload.Kind))
		return nil
	}
}

// Command worker runs the asynq background job processor. It is a separate
// binary from the API so the two scale and deploy independently.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/config"
)

// Task type names. Real payloads/handlers land here as the booking and email
// features are built (lesson reminders, payout release, etc.).
const (
	TaskLessonReminder = "lesson:reminder"
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
	mux.HandleFunc(TaskLessonReminder, handleLessonReminder(logger))

	if err := srv.Start(mux); err != nil {
		return err
	}
	logger.Info("worker started")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	logger.Info("shutdown signal received")
	srv.Shutdown()
	logger.Info("worker stopped cleanly")
	return nil
}

// handleLessonReminder is a placeholder that just logs. It becomes "send the
// student and teacher an email with the video link" once Resend is wired.
func handleLessonReminder(logger *slog.Logger) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload struct {
			BookingID string `json:"booking_id"`
		}
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return err
		}
		logger.InfoContext(ctx, "lesson reminder (stub)", slog.String("booking_id", payload.BookingID))
		return nil
	}
}

package lessons

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
)

// Scheduler implements bookings.ReminderScheduler with asynq. It enqueues two
// tasks per booking with deterministic ids (reminder:24h:<id>, reminder:1h:<id>)
// so a re-schedule is idempotent and Cancel can delete by id.
type Scheduler struct {
	client    *asynq.Client
	inspector *asynq.Inspector
	now       func() time.Time
}

var _ bookings.ReminderScheduler = (*Scheduler)(nil)

// NewScheduler builds a Scheduler from an asynq redis connection option
// (asynq.ParseRedisURI(cfg.RedisURL)).
func NewScheduler(redis asynq.RedisConnOpt) *Scheduler {
	return &Scheduler{
		client:    asynq.NewClient(redis),
		inspector: asynq.NewInspector(redis),
		now:       time.Now,
	}
}

// Close releases the underlying redis connections.
func (s *Scheduler) Close() error {
	err := s.client.Close()
	if ierr := s.inspector.Close(); ierr != nil && err == nil {
		err = ierr
	}
	return err
}

// Schedule enqueues the 24h and 1h reminders. A run time already in the past is
// skipped. An id already enqueued (asynq.ErrTaskIDConflict / ErrDuplicateTask)
// is treated as success — the reminder is already there.
func (s *Scheduler) Schedule(ctx context.Context, bookingID uuid.UUID, startAt time.Time) error {
	now := s.now()
	for _, off := range reminderOffsets {
		runAt := startAt.Add(-off.before)
		if runAt.Before(now) {
			continue // too late to be useful
		}
		payload, err := json.Marshal(ReminderPayload{BookingID: bookingID.String(), Kind: off.kind})
		if err != nil {
			return fmt.Errorf("marshal reminder payload: %w", err)
		}
		task := asynq.NewTask(TaskLessonReminder, payload)
		_, err = s.client.EnqueueContext(ctx, task,
			asynq.TaskID(taskID(off.kind, bookingID.String())),
			asynq.ProcessAt(runAt),
			asynq.Queue(ReminderQueue),
			asynq.MaxRetry(5),
		)
		if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) && !errors.Is(err, asynq.ErrDuplicateTask) {
			return fmt.Errorf("enqueue %s reminder: %w", off.kind, err)
		}
	}
	return nil
}

// Cancel deletes both reminder tasks for a booking. A not-found task is fine
// (already fired, already cancelled, or never scheduled).
func (s *Scheduler) Cancel(_ context.Context, bookingID uuid.UUID) error {
	for _, off := range reminderOffsets {
		id := taskID(off.kind, bookingID.String())
		if err := s.inspector.DeleteTask(ReminderQueue, id); err != nil &&
			!errors.Is(err, asynq.ErrTaskNotFound) && !errors.Is(err, asynq.ErrQueueNotFound) {
			return fmt.Errorf("delete %s reminder: %w", off.kind, err)
		}
	}
	return nil
}

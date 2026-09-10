package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/email"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/lessons"
)

type fakeLoader struct {
	b   bookings.Booking
	err error
}

func (f fakeLoader) GetBooking(context.Context, uuid.UUID) (bookings.Booking, error) {
	return f.b, f.err
}

type fakeMailer struct {
	sent int
	err  error
}

func (m *fakeMailer) Send(context.Context, email.Message) error {
	m.sent++
	return m.err
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func reminderTask(t *testing.T, id uuid.UUID, kind string) *asynq.Task {
	t.Helper()
	p, _ := json.Marshal(lessons.ReminderPayload{BookingID: id.String(), Kind: kind})
	return asynq.NewTask(lessons.TaskLessonReminder, p)
}

func confirmedBooking(id uuid.UUID) bookings.Booking {
	return bookings.Booking{
		ID:              id,
		Status:          bookings.StatusConfirmed,
		StartAt:         time.Now().Add(time.Hour),
		DurationMinutes: 60,
		Student:         bookings.StudentSummary{DisplayName: "Aziz"},
		StudentEmail:    "aziz@example.com",
		Teacher:         bookings.TeacherSummary{DisplayName: "Nodira", Timezone: "Asia/Tashkent"},
		TeacherEmail:    "nodira@example.com",
	}
}

func TestHandleLessonReminder_SendsForConfirmed(t *testing.T) {
	id := uuid.New()
	mailer := &fakeMailer{}
	h := handleLessonReminder(discardLogger(), fakeLoader{b: confirmedBooking(id)}, mailer)

	if err := h(context.Background(), reminderTask(t, id, "1h")); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if mailer.sent != 2 {
		t.Errorf("sent = %d, want 2 (student + teacher)", mailer.sent)
	}
}

func TestHandleLessonReminder_SkipsCancelled(t *testing.T) {
	id := uuid.New()
	b := confirmedBooking(id)
	b.Status = bookings.StatusCancelled
	mailer := &fakeMailer{}
	h := handleLessonReminder(discardLogger(), fakeLoader{b: b}, mailer)

	if err := h(context.Background(), reminderTask(t, id, "24h")); err != nil {
		t.Fatalf("handler should not error on a cancelled booking: %v", err)
	}
	if mailer.sent != 0 {
		t.Errorf("sent = %d, want 0 for a cancelled booking", mailer.sent)
	}
}

func TestHandleLessonReminder_MissingBookingIsNoOp(t *testing.T) {
	mailer := &fakeMailer{}
	h := handleLessonReminder(discardLogger(), fakeLoader{err: bookings.ErrBookingNotFound}, mailer)
	if err := h(context.Background(), reminderTask(t, uuid.New(), "1h")); err != nil {
		t.Fatalf("missing booking should be a no-op, got %v", err)
	}
	if mailer.sent != 0 {
		t.Errorf("sent = %d, want 0", mailer.sent)
	}
}

func TestHandleLessonReminder_SendErrorRetries(t *testing.T) {
	id := uuid.New()
	mailer := &fakeMailer{err: io.ErrClosedPipe}
	h := handleLessonReminder(discardLogger(), fakeLoader{b: confirmedBooking(id)}, mailer)
	if err := h(context.Background(), reminderTask(t, id, "1h")); err == nil {
		t.Fatal("want the send error propagated so asynq retries")
	}
}

func TestHandleLessonReminder_BadPayloadSkipsRetry(t *testing.T) {
	mailer := &fakeMailer{}
	h := handleLessonReminder(discardLogger(), fakeLoader{}, mailer)
	task := asynq.NewTask(lessons.TaskLessonReminder, []byte("not json"))
	err := h(context.Background(), task)
	if err == nil {
		t.Fatal("want an error for a malformed payload")
	}
}

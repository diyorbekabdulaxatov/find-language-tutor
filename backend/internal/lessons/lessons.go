// Package lessons wires the Phase-5 booking side-effects — scheduled lesson
// reminders (asynq) and transactional email (internal/email) — to the ports the
// bookings module defines (bookings.ReminderScheduler, bookings.Notifier).
//
// bookings never imports this package; cmd/api injects Scheduler + Notifier with
// bookingService.SetReminderScheduler / SetNotifier, and cmd/worker reuses
// ReminderMessages + the task name / payload defined here.
package lessons

import (
	"time"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/email"
)

// TaskLessonReminder is the asynq task type for a lesson reminder. It matches
// the constant cmd/worker registers a handler for.
const TaskLessonReminder = "lesson:reminder"

// ReminderQueue is the asynq queue reminders are enqueued on and deleted from.
const ReminderQueue = "default"

// ReminderPayload is the JSON payload of a lesson:reminder task.
type ReminderPayload struct {
	BookingID string `json:"booking_id"`
	Kind      string `json:"kind"` // "24h" | "1h"
}

// reminderOffsets is the fixed schedule: 24h then 1h before start_at.
var reminderOffsets = []struct {
	kind   string
	before time.Duration
}{
	{"24h", 24 * time.Hour},
	{"1h", time.Hour},
}

// taskID is the deterministic asynq task id for one reminder of one booking, so
// re-scheduling is idempotent and Cancel can delete by id.
func taskID(kind, bookingID string) string {
	return "reminder:" + kind + ":" + bookingID
}

// lessonInfo maps a domain booking onto the template data. Times render in the
// teacher's timezone (the student has none yet); an unknown tz falls back to UTC
// inside the template.
func lessonInfo(b bookings.Booking) email.LessonInfo {
	return email.LessonInfo{
		TeacherName:     b.Teacher.DisplayName,
		StudentName:     b.Student.DisplayName,
		StartAt:         b.StartAt,
		Timezone:        b.Teacher.Timezone,
		DurationMinutes: b.DurationMinutes,
		PriceMinor:      b.Price.AmountMinor,
		Currency:        b.Price.Currency,
		MeetingURL:      b.EffectiveMeetingURL(),
	}
}

type recipient struct{ addr, name, locale string }

// recipients returns the two participants, skipping any with a blank address.
// Each carries their own locale, so a lesson between a Russian-speaking
// student and an Uzbek-speaking teacher produces two differently worded mails.
func recipients(b bookings.Booking) []recipient {
	var out []recipient
	if b.StudentEmail != "" {
		out = append(out, recipient{b.StudentEmail, b.Student.DisplayName, b.StudentLocale})
	}
	if b.TeacherEmail != "" {
		out = append(out, recipient{b.TeacherEmail, b.Teacher.DisplayName, b.TeacherLocale})
	}
	return out
}

// content renders one template in one locale.
type content func(loc string) (subject, html, text string)

// ReminderMessages renders the lesson_reminder email for both participants.
// Shared by the worker's task handler.
func ReminderMessages(b bookings.Booking, kind string) []email.Message {
	return fanOut(b, func(loc string) (string, string, string) {
		return email.LessonReminderContent(loc, lessonInfo(b), email.ReminderKind(kind))
	})
}

// ConfirmedMessages renders booking_confirmed for both participants.
func ConfirmedMessages(b bookings.Booking) []email.Message {
	return fanOut(b, func(loc string) (string, string, string) {
		return email.BookingConfirmedContent(loc, lessonInfo(b))
	})
}

// CancelledMessages renders booking_cancelled for the party who did NOT trigger
// the cancellation (the "other party").
func CancelledMessages(b bookings.Booking, cancelledBy uuid.UUID, refunded bool) []email.Message {
	var out []email.Message
	for _, r := range recipients(b) {
		if cancelledBy != uuid.Nil {
			if r.addr == b.StudentEmail && b.Student.ID == cancelledBy {
				continue
			}
			if r.addr == b.TeacherEmail && b.TeacherOwnerID == cancelledBy {
				continue
			}
		}
		subject, html, text := email.BookingCancelledContent(r.locale, lessonInfo(b), refunded)
		out = append(out, email.Message{To: r.addr, ToName: r.name, Subject: subject, HTMLBody: html, TextBody: text})
	}
	return out
}

func fanOut(b bookings.Booking, render content) []email.Message {
	var out []email.Message
	for _, r := range recipients(b) {
		subject, html, text := render(r.locale)
		out = append(out, email.Message{To: r.addr, ToName: r.name, Subject: subject, HTMLBody: html, TextBody: text})
	}
	return out
}

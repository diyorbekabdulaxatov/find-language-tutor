package lessons

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
)

func sampleBooking() bookings.Booking {
	return bookings.Booking{
		ID:                uuid.New(),
		Status:            bookings.StatusConfirmed,
		StartAt:           time.Date(2026, 2, 3, 9, 0, 0, 0, time.UTC),
		EndAt:             time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC),
		DurationMinutes:   60,
		Price:             bookings.Money{AmountMinor: 9_000_000, Currency: "UZS"},
		TeacherMeetingURL: "https://meet.example/room",
		Student:           bookings.StudentSummary{ID: uuid.New(), DisplayName: "Aziz"},
		StudentEmail:      "aziz@example.com",
		Teacher:           bookings.TeacherSummary{DisplayName: "Nodira", Timezone: "Asia/Tashkent"},
		TeacherEmail:      "nodira@example.com",
		TeacherOwnerID:    uuid.New(),
	}
}

func TestReminderMessages_FanOutToBothParties(t *testing.T) {
	b := sampleBooking()
	msgs := ReminderMessages(b, "1h")
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	got := map[string]bool{}
	for _, m := range msgs {
		got[m.To] = true
		if !strings.Contains(m.TextBody, "https://meet.example/room") {
			t.Errorf("message missing meeting link: %s", m.TextBody)
		}
		if !strings.Contains(m.TextBody, "in 1 hour") {
			t.Errorf("message missing reminder phrase: %s", m.TextBody)
		}
	}
	if !got["aziz@example.com"] || !got["nodira@example.com"] {
		t.Errorf("recipients = %v", got)
	}
}

func TestReminderMessages_UsesOverrideLink(t *testing.T) {
	b := sampleBooking()
	b.MeetingURLOverride = "https://zoom.example/xyz"
	for _, m := range ReminderMessages(b, "24h") {
		if !strings.Contains(m.TextBody, "https://zoom.example/xyz") {
			t.Errorf("override link not used: %s", m.TextBody)
		}
	}
}

func TestReminderMessages_SkipsBlankAddresses(t *testing.T) {
	b := sampleBooking()
	b.TeacherEmail = ""
	msgs := ReminderMessages(b, "1h")
	if len(msgs) != 1 || msgs[0].To != "aziz@example.com" {
		t.Fatalf("want only the student message, got %+v", msgs)
	}
}

func TestCancelledMessages_SkipsTheCanceller(t *testing.T) {
	b := sampleBooking()
	// student cancels -> only the teacher is told.
	msgs := CancelledMessages(b, b.Student.ID, true)
	if len(msgs) != 1 || msgs[0].To != "nodira@example.com" {
		t.Fatalf("student-cancel: want teacher-only, got %+v", msgs)
	}
	if !strings.Contains(msgs[0].TextBody, "refunded") {
		t.Errorf("refund note missing: %s", msgs[0].TextBody)
	}
	// teacher cancels -> only the student is told.
	msgs = CancelledMessages(b, b.TeacherOwnerID, false)
	if len(msgs) != 1 || msgs[0].To != "aziz@example.com" {
		t.Fatalf("teacher-cancel: want student-only, got %+v", msgs)
	}
}

func TestConfirmedMessages_BothParties(t *testing.T) {
	msgs := ConfirmedMessages(sampleBooking())
	if len(msgs) != 2 {
		t.Fatalf("want 2 messages, got %d", len(msgs))
	}
	for _, m := range msgs {
		if !strings.Contains(m.Subject, "confirmed") {
			t.Errorf("subject = %q", m.Subject)
		}
	}
}

func TestScheduler_TaskIDShape(t *testing.T) {
	id := uuid.New().String()
	if got := taskID("24h", id); got != "reminder:24h:"+id {
		t.Errorf("taskID = %q", got)
	}
	if got := taskID("1h", id); got != "reminder:1h:"+id {
		t.Errorf("taskID = %q", got)
	}
}

package email

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestNew_PicksLogEmailerWithoutKey(t *testing.T) {
	if _, ok := New(Config{}, nil).(*logEmailer); !ok {
		t.Fatalf("no API key: want *logEmailer")
	}
	if _, ok := New(Config{ResendAPIKey: "re_123"}, nil).(*resendEmailer); !ok {
		t.Fatalf("with API key: want *resendEmailer")
	}
}

func TestLogEmailer_Send(t *testing.T) {
	if err := (&logEmailer{logger: discard()}).Send(context.Background(), Message{
		To: "a@example.com", Subject: "hi", TextBody: "line one\nline two",
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func sampleInfo() LessonInfo {
	return LessonInfo{
		TeacherName:     "Nodira",
		StudentName:     "Aziz",
		StartAt:         time.Date(2026, 2, 3, 9, 0, 0, 0, time.UTC),
		Timezone:        "Asia/Tashkent",
		DurationMinutes: 60,
		PriceMinor:      9_000_000,
		Currency:        "UZS",
		MeetingURL:      "https://meet.example/room",
	}
}

func TestTemplates_IncludeMeetingLinkAndTime(t *testing.T) {
	li := sampleInfo()

	subj, html, text := BookingConfirmedContent(li)
	for _, want := range []string{"https://meet.example/room", "9000000 UZS", "confirmed"} {
		if !strings.Contains(text, want) {
			t.Errorf("confirmed text missing %q: %s", want, text)
		}
	}
	if !strings.Contains(html, "meet.example") {
		t.Errorf("confirmed html missing link: %s", html)
	}
	// rendered in the recipient timezone (+05 in Tashkent, so 14:00, not 09:00).
	if !strings.Contains(subj, "14:00") {
		t.Errorf("subject not in teacher tz: %s", subj)
	}

	_, _, rtext := LessonReminderContent(li, Reminder1h)
	if !strings.Contains(rtext, "in 1 hour") || !strings.Contains(rtext, "https://meet.example/room") {
		t.Errorf("reminder text wrong: %s", rtext)
	}
	_, _, r24 := LessonReminderContent(li, Reminder24h)
	if !strings.Contains(r24, "in 24 hours") {
		t.Errorf("24h reminder text wrong: %s", r24)
	}

	_, _, ctext := BookingCancelledContent(li, true)
	if !strings.Contains(ctext, "refunded") {
		t.Errorf("cancelled text missing refund note: %s", ctext)
	}
	_, _, cnote := BookingCancelledContent(li, false)
	if strings.Contains(cnote, "has been refunded") {
		t.Errorf("no-refund note should not claim a refund: %s", cnote)
	}
}

func TestLessonInfo_FallsBackToUTC(t *testing.T) {
	li := sampleInfo()
	li.Timezone = "Not/AZone"
	if got := li.when(); !strings.Contains(got, "UTC") {
		t.Errorf("bad tz should fall back to UTC, got %q", got)
	}
}

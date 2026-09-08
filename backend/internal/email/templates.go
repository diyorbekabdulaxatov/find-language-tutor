package email

import (
	"fmt"
	"strings"
	"time"
)

// ReminderKind selects the wording of the single lesson-reminder template.
type ReminderKind string

const (
	Reminder24h ReminderKind = "24h"
	Reminder1h  ReminderKind = "1h"
)

func (k ReminderKind) phrase() string {
	switch k {
	case Reminder1h:
		return "in 1 hour"
	default:
		return "in 24 hours"
	}
}

// LessonInfo is the data every booking template renders from. Times are UTC
// instants; Timezone is the IANA name they are displayed in (empty -> UTC).
//
// Timezone choice: a student account has no timezone of its own yet, so student
// mail is rendered in the teacher's timezone and the offset/name is shown so it
// is unambiguous. Teacher mail uses the same (the teacher's) timezone.
type LessonInfo struct {
	TeacherName     string
	StudentName     string
	StartAt         time.Time
	Timezone        string
	DurationMinutes int
	PriceMinor      int64
	Currency        string
	MeetingURL      string
}

func (li LessonInfo) when() string {
	loc, err := time.LoadLocation(li.Timezone)
	if err != nil || li.Timezone == "" {
		loc = time.UTC
	}
	return li.StartAt.In(loc).Format("Mon, 02 Jan 2006 15:04 MST")
}

func (li LessonInfo) priceLine() string {
	if li.Currency == "" {
		return fmt.Sprintf("%d (minor units)", li.PriceMinor)
	}
	return fmt.Sprintf("%d %s (minor units)", li.PriceMinor, li.Currency)
}

func meetingLine(url string) string {
	if url == "" {
		return "Meeting link: (the teacher has not set one yet — check back closer to the lesson)"
	}
	return "Meeting link: " + url
}

// wrap renders a plain-text body and a minimal HTML body from the same lines.
func wrap(subject string, lines []string) (html, text string) {
	text = strings.Join(lines, "\n")
	var b strings.Builder
	b.WriteString("<h2>")
	b.WriteString(htmlEscape(subject))
	b.WriteString("</h2>\n")
	for _, ln := range lines {
		if ln == "" {
			b.WriteString("<br/>\n")
			continue
		}
		b.WriteString("<p>")
		b.WriteString(htmlEscape(ln))
		b.WriteString("</p>\n")
	}
	return b.String(), text
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// BookingConfirmedContent is sent to BOTH participants right after payment.
func BookingConfirmedContent(li LessonInfo) (subject, html, text string) {
	subject = fmt.Sprintf("Lesson with %s confirmed — %s", li.TeacherName, li.when())
	lines := []string{
		fmt.Sprintf("Your lesson with %s is confirmed.", li.TeacherName),
		"",
		"When: " + li.when(),
		fmt.Sprintf("Length: %d minutes", li.DurationMinutes),
		"Price: " + li.priceLine(),
		meetingLine(li.MeetingURL),
		"",
		"See you then!",
	}
	html, text = wrap(subject, lines)
	return subject, html, text
}

// BookingCancelledContent is sent to the other party on a cancellation / teacher
// no-show.
func BookingCancelledContent(li LessonInfo, refunded bool) (subject, html, text string) {
	subject = fmt.Sprintf("Lesson with %s cancelled — %s", li.TeacherName, li.when())
	refundNote := "No payment had been taken, so there is nothing to refund."
	if refunded {
		refundNote = "Any payment you made for this lesson has been refunded in full."
	}
	lines := []string{
		fmt.Sprintf("Your lesson with %s scheduled for %s has been cancelled.", li.TeacherName, li.when()),
		"",
		refundNote,
		"",
		"You can book another time from the teacher's profile.",
	}
	html, text = wrap(subject, lines)
	return subject, html, text
}

// LessonReminderContent is the single reminder template; kind sets the wording.
func LessonReminderContent(li LessonInfo, kind ReminderKind) (subject, html, text string) {
	subject = fmt.Sprintf("Reminder: lesson with %s %s", li.TeacherName, kind.phrase())
	lines := []string{
		fmt.Sprintf("Your lesson with %s starts %s.", li.TeacherName, kind.phrase()),
		"",
		"When: " + li.when(),
		fmt.Sprintf("Length: %d minutes", li.DurationMinutes),
		meetingLine(li.MeetingURL),
	}
	html, text = wrap(subject, lines)
	return subject, html, text
}

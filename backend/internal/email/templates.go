package email

import (
	"strings"
	"time"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/i18n"
)

// Every template takes the recipient's locale (users.locale) first. The
// English text below is the catalog key — see internal/i18n.

// ReminderKind selects the wording of the single lesson-reminder template.
type ReminderKind string

const (
	Reminder24h ReminderKind = "24h"
	Reminder1h  ReminderKind = "1h"
)

func (k ReminderKind) phrase(loc string) string {
	switch k {
	case Reminder1h:
		return i18n.T(loc, "in 1 hour")
	default:
		return i18n.T(loc, "in 24 hours")
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

func (li LessonInfo) when(loc string) string {
	tz, err := time.LoadLocation(li.Timezone)
	if err != nil || li.Timezone == "" {
		tz = time.UTC
	}
	return i18n.FormatDateTime(loc, li.StartAt.In(tz))
}

func (li LessonInfo) priceLine(loc string) string {
	if li.Currency == "" {
		return i18n.Tf(loc, "%d (minor units)", li.PriceMinor)
	}
	return i18n.Tf(loc, "%d %s (minor units)", li.PriceMinor, li.Currency)
}

func meetingLine(loc, url string) string {
	if url == "" {
		return i18n.T(loc, "Meeting link: (the teacher has not set one yet — check back closer to the lesson)")
	}
	return i18n.Tf(loc, "Meeting link: %s", url)
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
func BookingConfirmedContent(loc string, li LessonInfo) (subject, html, text string) {
	subject = i18n.Tf(loc, "Lesson with %s confirmed — %s", li.TeacherName, li.when(loc))
	lines := []string{
		i18n.Tf(loc, "Your lesson with %s is confirmed.", li.TeacherName),
		"",
		i18n.Tf(loc, "When: %s", li.when(loc)),
		i18n.Tf(loc, "Length: %d minutes", li.DurationMinutes),
		i18n.Tf(loc, "Price: %s", li.priceLine(loc)),
		meetingLine(loc, li.MeetingURL),
		"",
		i18n.T(loc, "See you then!"),
	}
	html, text = wrap(subject, lines)
	return subject, html, text
}

// BookingRescheduledContent is sent to the teacher when the student moves a
// lesson. li carries the NEW time; previousStart the old one.
func BookingRescheduledContent(loc string, li LessonInfo, previousStart time.Time) (subject, html, text string) {
	was := li
	was.StartAt = previousStart
	subject = i18n.Tf(loc, "Lesson with %s moved — now %s", li.StudentName, li.when(loc))
	lines := []string{
		i18n.Tf(loc, "%s moved your lesson.", li.StudentName),
		"",
		i18n.Tf(loc, "Was: %s", was.when(loc)),
		i18n.Tf(loc, "Now: %s", li.when(loc)),
		i18n.Tf(loc, "Length: %d minutes", li.DurationMinutes),
		meetingLine(loc, li.MeetingURL),
		"",
		i18n.T(loc, "The old time is open again in your calendar. If the new time does not work for you, cancel the lesson and the student is refunded in full."),
	}
	html, text = wrap(subject, lines)
	return subject, html, text
}

// CancelOutcome mirrors bookings.CancellationOutcome for the cancellation
// email: what happened to the student's payment.
type CancelOutcome string

const (
	CancelRefunded  CancelOutcome = "refunded"
	CancelForfeited CancelOutcome = "forfeited"
	CancelUnpaid    CancelOutcome = "unpaid"
)

// BookingCancelledContent is sent to the other party on a cancellation /
// teacher no-show. toTeacher picks the recipient's side: the lesson is named
// after their counterpart and the money note is theirs.
func BookingCancelledContent(loc string, li LessonInfo, outcome CancelOutcome, toTeacher bool) (subject, html, text string) {
	other := li.TeacherName
	if toTeacher {
		other = li.StudentName
	}
	subject = i18n.Tf(loc, "Lesson with %s cancelled — %s", other, li.when(loc))

	var moneyNote, next string
	switch {
	case toTeacher && outcome == CancelForfeited:
		moneyNote = i18n.T(loc, "It was cancelled after the free-cancellation deadline, so you will be paid the full fee as for a completed lesson.")
	case toTeacher && outcome == CancelRefunded:
		moneyNote = i18n.T(loc, "The student has been refunded in full.")
	case toTeacher:
		moneyNote = i18n.T(loc, "No payment had been taken for this lesson.")
	case outcome == CancelForfeited:
		moneyNote = i18n.T(loc, "It was cancelled after the free-cancellation deadline, so the full fee has been charged.")
	case outcome == CancelRefunded:
		moneyNote = i18n.T(loc, "Any payment you made for this lesson has been refunded in full.")
	default:
		moneyNote = i18n.T(loc, "No payment had been taken, so there is nothing to refund.")
	}
	if toTeacher {
		next = i18n.T(loc, "The time is open again in your calendar.")
	} else {
		next = i18n.T(loc, "You can book another time from the teacher's profile.")
	}

	lines := []string{
		i18n.Tf(loc, "Your lesson with %s scheduled for %s has been cancelled.", other, li.when(loc)),
		"",
		moneyNote,
		"",
		next,
	}
	html, text = wrap(subject, lines)
	return subject, html, text
}

// --- account recovery ---

// PasswordResetContent is the "reset your password" email.
func PasswordResetContent(loc, name, resetURL string) (subject, html, text string) {
	subject = i18n.T(loc, "Reset your FindTutor password")
	lines := []string{
		greetingLine(loc, name),
		"",
		i18n.T(loc, "We got a request to reset your FindTutor password. Open this link to choose a new one:"),
		resetURL,
		"",
		i18n.T(loc, "The link expires in 1 hour and can be used once. If you didn't ask for this, you can ignore this email — your password won't change."),
	}
	html, text = wrap(subject, lines)
	return subject, html, text
}

// EmailVerificationContent is the "confirm your address" email.
func EmailVerificationContent(loc, name, verifyURL string) (subject, html, text string) {
	subject = i18n.T(loc, "Confirm your FindTutor email")
	lines := []string{
		greetingLine(loc, name),
		"",
		i18n.T(loc, "Confirm your email address to finish setting up your FindTutor account:"),
		verifyURL,
		"",
		i18n.T(loc, "The link expires in 24 hours."),
	}
	html, text = wrap(subject, lines)
	return subject, html, text
}

func greetingLine(loc, name string) string {
	if strings.TrimSpace(name) == "" {
		return i18n.T(loc, "Hi,")
	}
	return i18n.Tf(loc, "Hi %s,", name)
}

// SubmissionGradedContent is sent to a student once a teacher grades their
// writing submission (auto-graded quiz-like types never reach a teacher, so
// never send this for those). score / max are nil when the teacher left no
// numeric score (feedback-only grading).
func SubmissionGradedContent(loc, studentName, resourceTitle string, score, max *int, feedback string) (subject, html, text string) {
	subject = i18n.Tf(loc, "Your %q submission was graded", resourceTitle)
	lines := []string{
		greetingLine(loc, studentName),
		"",
		i18n.Tf(loc, "Your teacher graded your submission for %q.", resourceTitle),
	}
	switch {
	case score != nil && max != nil:
		lines = append(lines, i18n.Tf(loc, "Score: %d / %d", *score, *max))
	case score != nil:
		lines = append(lines, i18n.Tf(loc, "Score: %d", *score))
	}
	feedback = strings.TrimSpace(feedback)
	if feedback != "" {
		lines = append(lines, "", i18n.Tf(loc, "Feedback: %s", feedback))
	}
	html, text = wrap(subject, lines)
	return subject, html, text
}

// LessonReminderContent is the single reminder template; kind sets the wording.
func LessonReminderContent(loc string, li LessonInfo, kind ReminderKind) (subject, html, text string) {
	subject = i18n.Tf(loc, "Reminder: lesson with %s %s", li.TeacherName, kind.phrase(loc))
	lines := []string{
		i18n.Tf(loc, "Your lesson with %s starts %s.", li.TeacherName, kind.phrase(loc)),
		"",
		i18n.Tf(loc, "When: %s", li.when(loc)),
		i18n.Tf(loc, "Length: %d minutes", li.DurationMinutes),
		meetingLine(loc, li.MeetingURL),
	}
	html, text = wrap(subject, lines)
	return subject, html, text
}

// --- teacher moderation ---

// TeacherApprovedContent is sent to a teacher when a moderator approves their
// profile: they are now on the storefront and can publish.
func TeacherApprovedContent(loc, name, dashboardURL string) (subject, html, text string) {
	subject = i18n.T(loc, "Your FindTutor teacher profile is approved")
	lines := []string{
		greetingLine(loc, name),
		"",
		i18n.T(loc, "Good news — your teacher profile has been approved. Students can now find you in the catalog, book lessons and buy your courses."),
		"",
		i18n.T(loc, "Next steps: set your weekly availability and publish your first course from the dashboard:"),
		dashboardURL,
	}
	html, text = wrap(subject, lines)
	return subject, html, text
}

// TeacherRejectedContent is sent when a moderator rejects a pending profile.
// note is the moderator's reason (required by the admin API); the teacher
// answers it by editing the profile, which puts it back in the queue.
func TeacherRejectedContent(loc, name, note, dashboardURL string) (subject, html, text string) {
	subject = i18n.T(loc, "Your FindTutor teacher profile needs changes")
	lines := []string{
		greetingLine(loc, name),
		"",
		i18n.T(loc, "A moderator reviewed your teacher profile and could not approve it yet."),
		i18n.Tf(loc, "Reason: %s", strings.TrimSpace(note)),
		"",
		i18n.T(loc, "Update your profile to address the note — saving it sends it back for review automatically:"),
		dashboardURL,
	}
	html, text = wrap(subject, lines)
	return subject, html, text
}

// TeacherSuspendedContent is sent when a moderator suspends an approved
// teacher: the profile and every course are hidden until re-approved.
func TeacherSuspendedContent(loc, name, note string) (subject, html, text string) {
	subject = i18n.T(loc, "Your FindTutor teacher profile has been suspended")
	lines := []string{
		greetingLine(loc, name),
		"",
		i18n.T(loc, "A moderator has suspended your teacher profile. It is hidden from students, and your courses are no longer for sale. Existing bookings are not affected."),
		i18n.Tf(loc, "Reason: %s", strings.TrimSpace(note)),
		"",
		i18n.T(loc, "If you believe this is a mistake, reply to this email and our team will take another look."),
	}
	html, text = wrap(subject, lines)
	return subject, html, text
}

// TeacherSubmittedContent alerts the moderation inbox that a profile is
// waiting for review. resubmitted marks a rejected profile the teacher has
// edited, so the moderator knows to compare against their note.
func TeacherSubmittedContent(loc, teacherName, adminURL string, resubmitted bool) (subject, html, text string) {
	if resubmitted {
		subject = i18n.Tf(loc, "Teacher profile resubmitted: %s", teacherName)
	} else {
		subject = i18n.Tf(loc, "New teacher profile to review: %s", teacherName)
	}
	lines := []string{
		i18n.Tf(loc, "%s is waiting for moderation.", teacherName),
	}
	if resubmitted {
		lines = append(lines, i18n.T(loc, "This profile was rejected earlier and has been edited since."))
	}
	lines = append(lines, "", i18n.T(loc, "Review it here:"), adminURL)
	html, text = wrap(subject, lines)
	return subject, html, text
}

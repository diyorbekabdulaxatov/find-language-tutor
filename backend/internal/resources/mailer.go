package resources

import "context"

// Mailer sends the one submission-lifecycle email: telling a student their
// writing task was graded. It is an optional port on the Service (SetMailer):
// a nil Mailer is a guarded no-op, so cmd/api without email configured and the
// unit tests still work. The adapter (internal/resourcesmail) owns link
// construction and the template — this package never imports internal/email.
type Mailer interface {
	// SubmissionGraded notifies the student a teacher graded their submission.
	// score / max are nil when the teacher left no numeric score (feedback-only
	// grading).
	SubmissionGraded(ctx context.Context, to, studentName, resourceTitle string, score, max *int, feedback string) error
}

// noopMailer is the default before SetMailer.
type noopMailer struct{}

func (noopMailer) SubmissionGraded(context.Context, string, string, string, *int, *int, string) error {
	return nil
}

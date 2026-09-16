package admin

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/teachers"
)

type statusMail struct {
	locale, to, name, note string
	status                 teachers.Status
}

type fakeMailer struct {
	mu   sync.Mutex
	sent []statusMail
	fail error
}

func (f *fakeMailer) TeacherStatusChanged(_ context.Context, locale, to, name string, status teachers.Status, note string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, statusMail{locale, to, name, note, status})
	return f.fail
}

func TestModeration_EmailsTeacherOnEveryTransition(t *testing.T) {
	repo, profiles := modFixtures()
	m := repo.moderation["pending-t"]
	m.Owner.Locale = "ru"
	repo.moderation["pending-t"] = m

	svc := NewService(repo, profiles)
	mail := &fakeMailer{}
	svc.SetMailer(mail)
	ctx := context.Background()

	if _, err := svc.Reject(ctx, "pending-t", "add a photo"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if _, err := svc.Approve(ctx, "pending-t"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := svc.Suspend(ctx, "pending-t", "spam"); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	svc.WaitNotifications()

	want := []statusMail{
		{"ru", "p@x", "P", "add a photo", teachers.StatusRejected},
		{"ru", "p@x", "P", "", teachers.StatusApproved},
		{"ru", "p@x", "P", "spam", teachers.StatusSuspended},
	}
	if len(mail.sent) != len(want) {
		t.Fatalf("sent %d mails, want %d: %+v", len(mail.sent), len(want), mail.sent)
	}
	// Goroutines may land out of order; compare as a set.
	seen := map[statusMail]bool{}
	for _, s := range mail.sent {
		seen[s] = true
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("missing mail %+v in %+v", w, mail.sent)
		}
	}
}

func TestModeration_MailFailureDoesNotFailTransition(t *testing.T) {
	repo, profiles := modFixtures()
	svc := NewService(repo, profiles)
	svc.SetMailer(&fakeMailer{fail: errors.New("smtp down")})

	d, err := svc.Approve(context.Background(), "pending-t")
	svc.WaitNotifications()
	if err != nil || d.Moderation.Status != "approved" {
		t.Fatalf("approve with failing mailer: err=%v status=%q", err, d.Moderation.Status)
	}
}

func TestModeration_NoMailerNoOwnerIsSilent(t *testing.T) {
	repo, profiles := modFixtures()
	m := repo.moderation["pending-t"]
	m.Owner = Owner{} // orphaned profile (no users row)
	repo.moderation["pending-t"] = m

	svc := NewService(repo, profiles)
	mail := &fakeMailer{}
	svc.SetMailer(mail)
	if _, err := svc.Approve(context.Background(), "pending-t"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	svc.WaitNotifications()
	if len(mail.sent) != 0 {
		t.Errorf("mailed an owner-less profile: %+v", mail.sent)
	}
}

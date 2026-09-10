package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// fakeMailer records the raw tokens the service hands it.
type fakeMailer struct {
	mu           sync.Mutex
	resetTokens  []string
	verifyTokens []string
	resetCalls   int
	verifyCalls  int
	err          error
}

func (m *fakeMailer) SendPasswordReset(_ context.Context, _, _, raw string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetCalls++
	m.resetTokens = append(m.resetTokens, raw)
	return m.err
}

func (m *fakeMailer) SendEmailVerification(_ context.Context, _, _, raw string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifyCalls++
	m.verifyTokens = append(m.verifyTokens, raw)
	return m.err
}

func (m *fakeMailer) lastReset() string  { return m.resetTokens[len(m.resetTokens)-1] }
func (m *fakeMailer) lastVerify() string { return m.verifyTokens[len(m.verifyTokens)-1] }

func newRecoveryService(t *testing.T) (*Service, *fakeRepo, *fakeMailer) {
	t.Helper()
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	fm := &fakeMailer{}
	svc.SetMailer(fm)
	svc.SetLogger(discardLogger())
	return svc, repo, fm
}

func TestForgotPassword_UnknownEmail_Silent(t *testing.T) {
	svc, _, fm := newRecoveryService(t)
	if err := svc.RequestPasswordReset(context.Background(), "nobody@example.com"); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if fm.resetCalls != 0 {
		t.Errorf("mailed a reset for an unknown address")
	}
}

func TestPasswordResetFlow(t *testing.T) {
	svc, repo, fm := newRecoveryService(t)
	ctx := context.Background()

	reg, err := svc.Register(ctx, "ada@example.com", "originalpw123", "Ada", "agent")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if repo.activeSessionCount(reg.User.ID) != 1 {
		t.Fatalf("want 1 session after register")
	}

	if err := svc.RequestPasswordReset(ctx, "ADA@example.com"); err != nil {
		t.Fatalf("request reset: %v", err)
	}
	if fm.resetCalls != 1 {
		t.Fatalf("reset email count = %d, want 1", fm.resetCalls)
	}
	token := fm.lastReset()

	// weak new password is rejected before the token is spent
	var ve ValidationError
	if err := svc.ResetPassword(ctx, token, "short"); !errors.As(err, &ve) {
		t.Fatalf("weak password: got %v, want ValidationError", err)
	}

	if err := svc.ResetPassword(ctx, token, "brandnewpw456"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	// every session revoked
	if n := repo.activeSessionCount(reg.User.ID); n != 0 {
		t.Errorf("active sessions after reset = %d, want 0", n)
	}
	// old password no longer works, new one does
	if _, err := svc.Login(ctx, "ada@example.com", "originalpw123", "agent"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("old password still works")
	}
	if _, err := svc.Login(ctx, "ada@example.com", "brandnewpw456", "agent"); err != nil {
		t.Errorf("new password rejected: %v", err)
	}
	// token is single-use
	if err := svc.ResetPassword(ctx, token, "yetanotherpw789"); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("reused token: got %v, want ErrInvalidToken", err)
	}
}

func TestResetPassword_BadToken(t *testing.T) {
	svc, _, _ := newRecoveryService(t)
	if err := svc.ResetPassword(context.Background(), "not-a-real-token", "validpassword1"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("got %v, want ErrInvalidToken", err)
	}
}

func TestForgotPassword_Cooldown(t *testing.T) {
	svc, _, fm := newRecoveryService(t)
	ctx := context.Background()
	if _, err := svc.Register(ctx, "ada@example.com", "originalpw123", "Ada", "agent"); err != nil {
		t.Fatalf("register: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := svc.RequestPasswordReset(ctx, "ada@example.com"); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	if fm.resetCalls != 1 {
		t.Errorf("reset emails = %d, want 1 (cooldown)", fm.resetCalls)
	}
}

func TestEmailVerificationFlow(t *testing.T) {
	svc, repo, fm := newRecoveryService(t)
	ctx := context.Background()

	reg, err := svc.Register(ctx, "ada@example.com", "originalpw123", "Ada", "agent")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if reg.User.EmailVerified {
		t.Errorf("fresh account should be unverified")
	}
	if fm.verifyCalls != 1 {
		t.Fatalf("verification email count = %d, want 1", fm.verifyCalls)
	}

	if err := svc.VerifyEmail(ctx, fm.lastVerify()); err != nil {
		t.Fatalf("verify: %v", err)
	}
	u, _ := repo.UserByID(ctx, reg.User.ID)
	if !u.EmailVerified {
		t.Errorf("account not marked verified")
	}

	// already verified -> resend is a 409
	if err := svc.ResendEmailVerification(ctx, reg.User.ID); !errors.Is(err, ErrAlreadyVerified) {
		t.Errorf("resend when verified: got %v, want ErrAlreadyVerified", err)
	}
	// re-submitting the (now consumed) token is a no-op success, not an error —
	// the account is already verified, which is what this token achieved.
	if err := svc.VerifyEmail(ctx, fm.lastVerify()); err != nil {
		t.Errorf("re-verify with consumed token: got %v, want nil", err)
	}
	// a token that never existed still fails
	if err := svc.VerifyEmail(ctx, "totally-made-up-token"); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("unknown verify token: got %v, want ErrInvalidToken", err)
	}
}

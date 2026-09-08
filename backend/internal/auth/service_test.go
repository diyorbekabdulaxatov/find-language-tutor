package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestService_Register_ThenLogin(t *testing.T) {
	repo := newFakeRepo()
	svc, tm := newTestService(repo)
	ctx := context.Background()

	res, err := svc.Register(ctx, "Ada@Example.com", "hunter2hunter", "Ada Lovelace", "test-agent")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if res.User.Email != "ada@example.com" {
		t.Errorf("email not normalized: %q", res.User.Email)
	}
	if _, err := tm.ParseAccess(res.AccessToken); err != nil {
		t.Errorf("access token from register does not parse: %v", err)
	}
	if res.RefreshToken == "" {
		t.Error("no refresh token issued")
	}

	login, err := svc.Login(ctx, "ada@example.com", "hunter2hunter", "test-agent")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.User.ID != res.User.ID {
		t.Errorf("login returned a different user")
	}
}

func TestService_Register_DuplicateEmail(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()

	if _, err := svc.Register(ctx, "dup@example.com", "password123", "One", ""); err != nil {
		t.Fatalf("first register: %v", err)
	}
	_, err := svc.Register(ctx, "DUP@example.com", "password123", "Two", "")
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken", err)
	}
}

func TestService_Register_Validation(t *testing.T) {
	svc, _ := newTestService(newFakeRepo())
	ctx := context.Background()

	cases := map[string][3]string{
		"bad email":      {"not-an-email", "password123", "Name"},
		"short password": {"a@b.com", "short", "Name"},
		"empty display":  {"a@b.com", "password123", "   "},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := svc.Register(ctx, in[0], in[1], in[2], "")
			var ve ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
		})
	}
}

func TestService_Login_BadCredentials(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()
	_, _ = svc.Register(ctx, "real@example.com", "password123", "Real", "")

	if _, err := svc.Login(ctx, "real@example.com", "wrongpass", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong password: err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := svc.Login(ctx, "ghost@example.com", "whatever0", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown email: err = %v, want ErrInvalidCredentials", err)
	}
}

func TestService_Refresh_RotatesAndInvalidatesOldToken(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()

	reg, err := svc.Register(ctx, "rot@example.com", "password123", "Rot", "agent-1")
	if err != nil {
		t.Fatal(err)
	}

	next, err := svc.Refresh(ctx, reg.RefreshToken, "agent-1")
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if next.RefreshToken == reg.RefreshToken {
		t.Fatal("refresh did not rotate the token")
	}

	// The new token works.
	if _, err := svc.Refresh(ctx, next.RefreshToken, "agent-1"); err != nil {
		t.Fatalf("refresh with the new token failed: %v", err)
	}
}

func TestService_Refresh_ReuseKillsChain(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()

	reg, _ := svc.Register(ctx, "reuse@example.com", "password123", "Reuse", "")
	userID := reg.User.ID

	// Rotate once: reg.RefreshToken is now revoked (replaced).
	rotated, err := svc.Refresh(ctx, reg.RefreshToken, "")
	if err != nil {
		t.Fatal(err)
	}
	if repo.activeSessionCount(userID) != 1 {
		t.Fatalf("active sessions after one rotation = %d, want 1", repo.activeSessionCount(userID))
	}

	// Replay the old (revoked) token: reuse detected.
	if _, err := svc.Refresh(ctx, reg.RefreshToken, ""); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("replay: err = %v, want ErrInvalidRefreshToken", err)
	}

	// The whole chain — including the currently-valid rotated token — is dead.
	if n := repo.activeSessionCount(userID); n != 0 {
		t.Fatalf("active sessions after reuse = %d, want 0", n)
	}
	if _, err := svc.Refresh(ctx, rotated.RefreshToken, ""); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("rotated token still works after reuse: err = %v", err)
	}
}

func TestService_Refresh_Expired(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()

	reg, _ := svc.Register(ctx, "exp@example.com", "password123", "Exp", "")

	// Jump past the refresh TTL.
	svc.now = func() time.Time { return time.Now().Add(48 * time.Hour) }

	if _, err := svc.Refresh(ctx, reg.RefreshToken, ""); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("err = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestService_Refresh_MissingOrUnknown(t *testing.T) {
	svc, _ := newTestService(newFakeRepo())
	ctx := context.Background()

	if _, err := svc.Refresh(ctx, "", ""); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("empty: %v", err)
	}
	if _, err := svc.Refresh(ctx, "made-up-token", ""); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("unknown: %v", err)
	}
}

func TestService_Logout_RevokesSession(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()

	reg, _ := svc.Register(ctx, "out@example.com", "password123", "Out", "")
	if err := svc.Logout(ctx, reg.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if repo.activeSessionCount(reg.User.ID) != 0 {
		t.Error("session still active after logout")
	}
	// Refresh with the logged-out token fails.
	if _, err := svc.Refresh(ctx, reg.RefreshToken, ""); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("refresh after logout: %v", err)
	}
	// Logout is idempotent.
	if err := svc.Logout(ctx, reg.RefreshToken); err != nil {
		t.Errorf("second logout: %v", err)
	}
	if err := svc.Logout(ctx, ""); err != nil {
		t.Errorf("empty logout: %v", err)
	}
}

func TestService_CurrentUser(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()

	reg, _ := svc.Register(ctx, "me@example.com", "password123", "Me", "")
	u, err := svc.CurrentUser(ctx, reg.User.ID)
	if err != nil || u.Email != "me@example.com" {
		t.Fatalf("CurrentUser = (%+v, %v)", u, err)
	}
}

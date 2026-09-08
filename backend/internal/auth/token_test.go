package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func testUser() User {
	return User{ID: uuid.New(), Email: "ada@example.com", DisplayName: "Ada"}
}

func TestTokenManager_IssueAndParse(t *testing.T) {
	tm := NewTokenManager("shhh", 15*time.Minute)
	u := testUser()

	tok, err := tm.IssueAccess(u, time.Now())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	claims, err := tm.ParseAccess(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Subject != u.ID.String() {
		t.Errorf("sub = %q, want %q", claims.Subject, u.ID)
	}
	if claims.Email != u.Email || claims.DisplayName != u.DisplayName {
		t.Errorf("convenience claims wrong: %+v", claims)
	}
	got, err := claims.UserID()
	if err != nil || got != u.ID {
		t.Errorf("UserID() = (%v, %v)", got, err)
	}
}

func TestTokenManager_RejectsExpired(t *testing.T) {
	tm := NewTokenManager("shhh", 15*time.Minute)
	tok, _ := tm.IssueAccess(testUser(), time.Now().Add(-time.Hour))

	if _, err := tm.ParseAccess(tok); err == nil {
		t.Fatal("expired token parsed without error")
	}
}

func TestTokenManager_RejectsWrongSecret(t *testing.T) {
	tok, _ := NewTokenManager("right", time.Minute).IssueAccess(testUser(), time.Now())

	if _, err := NewTokenManager("wrong", time.Minute).ParseAccess(tok); err == nil {
		t.Fatal("token verified under the wrong secret")
	}
}

func TestTokenManager_RejectsMalformed(t *testing.T) {
	tm := NewTokenManager("shhh", time.Minute)
	for _, bad := range []string{"", "not-a-jwt", "a.b.c"} {
		if _, err := tm.ParseAccess(bad); err == nil {
			t.Errorf("ParseAccess(%q) succeeded", bad)
		}
	}
}

func TestRefreshToken_HashIsStableAndOpaque(t *testing.T) {
	raw, hash, err := newRefreshToken()
	if err != nil {
		t.Fatalf("newRefreshToken: %v", err)
	}
	if len(raw) < 40 {
		t.Errorf("raw token suspiciously short: %q", raw)
	}
	if len(hash) != 32 {
		t.Errorf("hash len = %d, want 32 (sha-256)", len(hash))
	}
	again := hashRefreshToken(raw)
	if string(again) != string(hash) {
		t.Error("hashRefreshToken not deterministic")
	}
}

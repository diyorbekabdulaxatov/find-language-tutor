package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestMemory_AllowsUpToLimitThenBlocks(t *testing.T) {
	m := NewMemory()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return base }
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		ok, _, err := m.Allow(ctx, "k", 3, time.Minute)
		if err != nil || !ok {
			t.Fatalf("hit %d: ok=%v err=%v, want allowed", i, ok, err)
		}
	}
	ok, retry, err := m.Allow(ctx, "k", 3, time.Minute)
	if err != nil || ok {
		t.Fatalf("4th hit: ok=%v err=%v, want blocked", ok, err)
	}
	if retry <= 0 || retry > time.Minute {
		t.Errorf("retryAfter=%v, want within (0, 1m]", retry)
	}
}

func TestMemory_KeysAreIndependent(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if ok, _, _ := m.Allow(ctx, "a", 2, time.Minute); !ok {
			t.Fatal("a should be allowed")
		}
	}
	if ok, _, _ := m.Allow(ctx, "a", 2, time.Minute); ok {
		t.Fatal("a should be blocked")
	}
	if ok, _, _ := m.Allow(ctx, "b", 2, time.Minute); !ok {
		t.Fatal("b is a different key and should be allowed")
	}
}

func TestMemory_WindowResets(t *testing.T) {
	m := NewMemory()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	now := base
	m.now = func() time.Time { return now }
	ctx := context.Background()

	m.Allow(ctx, "k", 1, time.Minute)
	if ok, _, _ := m.Allow(ctx, "k", 1, time.Minute); ok {
		t.Fatal("second hit in the same window should block")
	}
	now = base.Add(time.Minute)
	if ok, _, _ := m.Allow(ctx, "k", 1, time.Minute); !ok {
		t.Fatal("next window should allow again")
	}
}

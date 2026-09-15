// Package ratelimit is a small fixed-window request limiter with a Redis
// backend for production (shared across API instances) and an in-memory one
// for tests and Redis-less local runs.
//
// Fixed windows are deliberate: they are one INCR + EXPIRE per request, easy
// to reason about, and the burst-at-the-boundary weakness (up to 2x the limit
// across two adjacent windows) is fine for the abuse this guards against —
// credential stuffing, signup spam, recovery-email floods — none of which
// need a precise sliding window.
package ratelimit

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter answers "may this key make one more request in the current window?".
type Limiter interface {
	// Allow counts one hit against key and reports whether it is within
	// limit for the window. When it is not, retryAfter is how long until the
	// window resets. err is only returned when the backend itself failed.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration, err error)
}

// windowStart floors now to the window so every request in the same span
// shares one counter key.
func windowStart(now time.Time, window time.Duration) int64 {
	return now.UnixNano() / int64(window)
}

// ---------------------------------------------------------------- memory ---

type memEntry struct {
	window int64
	count  int
}

// Memory is a process-local Limiter. Not shared across instances — use it for
// tests and single-process dev runs only.
type Memory struct {
	mu      sync.Mutex
	entries map[string]memEntry
	now     func() time.Time
}

func NewMemory() *Memory {
	return &Memory{entries: map[string]memEntry{}, now: time.Now}
}

func (m *Memory) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	now := m.now()
	ws := windowStart(now, window)

	m.mu.Lock()
	defer m.mu.Unlock()

	e, ok := m.entries[key]
	if !ok || e.window != ws {
		e = memEntry{window: ws}
	}
	e.count++
	m.entries[key] = e

	// Opportunistic GC so a long-running dev process doesn't grow forever.
	if len(m.entries) > 10_000 {
		for k, v := range m.entries {
			if v.window != ws {
				delete(m.entries, k)
			}
		}
	}

	if e.count > limit {
		resetAt := time.Unix(0, (ws+1)*int64(window))
		return false, resetAt.Sub(now), nil
	}
	return true, 0, nil
}

// ----------------------------------------------------------------- redis ---

// Redis is a Limiter backed by a shared Redis, so the limit holds across every
// API instance. Keys are "rl:<key>:<window-start>" and expire on their own.
type Redis struct {
	rdb    *redis.Client
	prefix string
}

func NewRedis(rdb *redis.Client) *Redis {
	return &Redis{rdb: rdb, prefix: "rl:"}
}

// allowScript increments the window counter and sets its TTL the first time,
// atomically, so a crash between the two can't leave an immortal key.
var allowScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
local ttl = redis.call('PTTL', KEYS[1])
return {n, ttl}
`)

func (r *Redis) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	ws := windowStart(time.Now(), window)
	rkey := r.prefix + key + ":" + strconv.FormatInt(ws, 10)

	res, err := allowScript.Run(ctx, r.rdb, []string{rkey}, window.Milliseconds()).Slice()
	if err != nil {
		return false, 0, err
	}
	if len(res) != 2 {
		return false, 0, errors.New("ratelimit: unexpected script reply")
	}
	n, _ := res[0].(int64)
	ttlMs, _ := res[1].(int64)
	if ttlMs < 0 {
		ttlMs = window.Milliseconds()
	}
	if n > int64(limit) {
		return false, time.Duration(ttlMs) * time.Millisecond, nil
	}
	return true, 0, nil
}

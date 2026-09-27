package server

import (
	"strconv"
	"testing"
	"time"
)

func newTestLimiter(limit int, window time.Duration) (*failureLimiter, *time.Time) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newFailureLimiter(limit, window)
	l.now = func() time.Time { return now }
	return l, &now
}

func TestFailureLimiterBlocksAtLimit(t *testing.T) {
	t.Parallel()

	l, now := newTestLimiter(3, time.Minute)
	for i := range 3 {
		if d := l.retryAfter("a"); d != 0 {
			t.Fatalf("after %d failures: retryAfter = %s, want 0", i, d)
		}
		l.fail("a")
		*now = now.Add(10 * time.Second)
	}
	// Failures at 0s, 10s and 20s; now is 30s, so the oldest leaves at 60s.
	if d := l.retryAfter("a"); d != 30*time.Second {
		t.Errorf("retryAfter = %s, want 30s", d)
	}
	if d := l.retryAfter("b"); d != 0 {
		t.Errorf("other key: retryAfter = %s, want 0", d)
	}

	*now = now.Add(30 * time.Second)
	if d := l.retryAfter("a"); d != 0 {
		t.Errorf("after the oldest failure left the window: retryAfter = %s, want 0", d)
	}
}

func TestFailureLimiterReset(t *testing.T) {
	t.Parallel()

	l, _ := newTestLimiter(2, time.Minute)
	l.fail("a")
	l.fail("a")
	if l.retryAfter("a") == 0 {
		t.Fatal("not blocked at the limit")
	}
	l.reset("a")
	if d := l.retryAfter("a"); d != 0 {
		t.Errorf("after reset: retryAfter = %s, want 0", d)
	}
}

func TestFailureLimiterKeepsAtMostLimitFailures(t *testing.T) {
	t.Parallel()

	l, _ := newTestLimiter(2, time.Minute)
	for range 10 {
		l.fail("a")
	}
	if n := len(l.failures["a"]); n != 2 {
		t.Errorf("stored failures = %d, want 2", n)
	}
}

func TestFailureLimiterPrunesExpiredKeys(t *testing.T) {
	t.Parallel()

	l, now := newTestLimiter(1, time.Minute)
	for i := range failureLimiterPruneSize {
		l.fail(strconv.Itoa(i))
	}
	*now = now.Add(time.Minute)
	l.fail("new")
	if n := len(l.failures); n != 1 {
		t.Errorf("tracked keys = %d, want 1", n)
	}
}

package server

import (
	"sync"
	"time"
)

// Login throttling: after this many failed sign-ins within the window, from
// one client address or for one username, further attempts are refused
// until the oldest failure leaves the window. The per-username limit also
// stops guessing spread over many addresses, at the price that such an
// attack can lock the account out for the window.
const (
	loginFailuresPerIP   = 10
	loginFailuresPerUser = 10
	loginFailureWindow   = 15 * time.Minute
)

// failureLimiterPruneSize is the number of tracked keys above which recording
// a failure first sweeps out keys whose failures have all left the window.
const failureLimiterPruneSize = 4096

// failureLimiter counts failures per key in a sliding window and blocks a
// key once it has limit failures in it. It is safe for concurrent use.
type failureLimiter struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu sync.Mutex
	// failures holds each key's most recent failure times, oldest first,
	// at most limit of them.
	failures map[string][]time.Time
}

func newFailureLimiter(limit int, window time.Duration) *failureLimiter {
	return &failureLimiter{limit: limit, window: window, now: time.Now, failures: make(map[string][]time.Time)}
}

// retryAfter returns how long key stays blocked, or 0 if it isn't.
func (l *failureLimiter) retryAfter(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	times := l.recent(key, l.now())
	if len(times) < l.limit {
		return 0
	}
	return times[0].Add(l.window).Sub(l.now())
}

// fail records a failure for key.
func (l *failureLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.failures) >= failureLimiterPruneSize {
		for k := range l.failures {
			if len(l.recent(k, now)) == 0 {
				delete(l.failures, k)
			}
		}
	}
	times := append(l.recent(key, now), now)
	if len(times) > l.limit {
		times = times[len(times)-l.limit:]
	}
	l.failures[key] = times
}

// reset forgets key's failures.
func (l *failureLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

// recent returns key's failures still within the window at now. It must be
// called with mu held.
func (l *failureLimiter) recent(key string, now time.Time) []time.Time {
	times := l.failures[key]
	for len(times) > 0 && !times[0].Add(l.window).After(now) {
		times = times[1:]
	}
	return times
}

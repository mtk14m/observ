package auth

import (
	"sync"
	"time"
)

// limiter slows down password guessing: after max failures for a key
// (email and client address) within window, the key is blocked until the
// window ends.
type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	now    func() time.Time
	seen   map[string]*attempts
}

type attempts struct {
	failures int
	first    time.Time
}

func newLimiter(maxFailures int, window time.Duration, now func() time.Time) *limiter {
	return &limiter{max: maxFailures, window: window, now: now, seen: map[string]*attempts{}}
}

// blocked returns how long key must wait, or zero.
func (l *limiter) blocked(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.seen[key]
	if !ok {
		return 0
	}
	end := a.first.Add(l.window)
	if !l.now().Before(end) {
		delete(l.seen, key)
		return 0
	}
	if a.failures < l.max {
		return 0
	}
	return end.Sub(l.now())
}

func (l *limiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	a, ok := l.seen[key]
	if !ok || !now.Before(a.first.Add(l.window)) {
		a = &attempts{first: now}
		l.seen[key] = a
	}
	a.failures++
	// Keep memory bounded: drop expired entries now and then.
	if len(l.seen) > 10_000 {
		for k, v := range l.seen {
			if !now.Before(v.first.Add(l.window)) {
				delete(l.seen, k)
			}
		}
	}
}

func (l *limiter) succeed(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.seen, key)
}

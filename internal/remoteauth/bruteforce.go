// SPDX-License-Identifier: AGPL-3.0-or-later

package remoteauth

import (
	"sync"
	"time"
)

// Brute-force lockout schedule for password login attempts, separate from
// the general per-IP rate limiter in internal/webserver/server.go (that one
// caps raw request throughput; this one specifically punishes repeated
// *wrong-password* guesses against one identity, which a generic
// requests-per-second limiter doesn't distinguish from normal traffic).
//
// The first bruteForceFreeAttempts failures are unlocked (typos happen);
// every failure after that at least doubles the lockout, capped at
// bruteForceMaxLockout — a trivial "admin"+"1", "admin"+"2", ... script
// hits multi-minute waits within a dozen guesses.
const (
	bruteForceFreeAttempts = 2
	bruteForceBaseLockout  = 2 * time.Second
	bruteForceMaxLockout   = 5 * time.Minute
)

type attemptState struct {
	failures    int
	lockedUntil time.Time
}

// Limiter tracks login failures per key (caller decides what a key is —
// typically "remoteIP|username", so one attacker IP hammering many
// usernames, or many IPs hammering one username, are both tracked
// correctly rather than sharing a single global counter). Sized for a
// self-hosted single-user server's login traffic, not internet-scale
// multi-tenant load — there is no background eviction goroutine, since the
// realistic number of distinct (IP, username) pairs a personal server ever
// sees in practice is small; a long-running public-facing deployment that
// needed to bound this map's memory would need to add periodic sweeping,
// not assumed to be necessary here.
type Limiter struct {
	mu       sync.Mutex
	attempts map[string]*attemptState
	now      func() time.Time
}

// NewLimiter returns a ready-to-use Limiter.
func NewLimiter() *Limiter {
	return &Limiter{attempts: make(map[string]*attemptState), now: time.Now}
}

// Allowed reports whether key is currently allowed to attempt a login, and
// if not, how long until it may try again.
func (l *Limiter) Allowed(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.attempts[key]
	if !ok {
		return true, 0
	}
	now := l.now()
	if now.Before(st.lockedUntil) {
		return false, st.lockedUntil.Sub(now)
	}
	return true, 0
}

// RecordFailure registers a failed login attempt for key and returns the
// lockout duration now in effect (0 if still within the free-attempt
// budget).
func (l *Limiter) RecordFailure(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.attempts[key]
	if !ok {
		st = &attemptState{}
		l.attempts[key] = st
	}
	st.failures++

	if st.failures <= bruteForceFreeAttempts {
		return 0
	}
	shift := st.failures - bruteForceFreeAttempts - 1
	lockout := bruteForceBaseLockout
	for i := 0; i < shift; i++ {
		lockout *= 2
		if lockout >= bruteForceMaxLockout {
			lockout = bruteForceMaxLockout
			break
		}
	}
	st.lockedUntil = l.now().Add(lockout)
	return lockout
}

// RecordSuccess clears key's failure history after a successful login.
func (l *Limiter) RecordSuccess(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package remoteauth

import (
	"testing"
	"time"
)

func TestLimiter_FreeAttemptsAreNotLocked(t *testing.T) {
	l := NewLimiter()
	for i := 0; i < bruteForceFreeAttempts; i++ {
		if lockout := l.RecordFailure("1.2.3.4|admin"); lockout != 0 {
			t.Fatalf("attempt %d: expected no lockout within free budget, got %v", i+1, lockout)
		}
	}
	allowed, _ := l.Allowed("1.2.3.4|admin")
	if !allowed {
		t.Fatal("expected key to still be allowed after only free attempts")
	}
}

func TestLimiter_LocksOutAfterFreeAttemptsExhausted(t *testing.T) {
	l := NewLimiter()
	for i := 0; i < bruteForceFreeAttempts; i++ {
		l.RecordFailure("1.2.3.4|admin")
	}
	lockout := l.RecordFailure("1.2.3.4|admin")
	if lockout <= 0 {
		t.Fatal("expected a positive lockout once free attempts are exhausted")
	}
	allowed, retryAfter := l.Allowed("1.2.3.4|admin")
	if allowed {
		t.Fatal("expected key to be locked out")
	}
	if retryAfter <= 0 {
		t.Fatal("expected a positive retry-after duration")
	}
}

func TestLimiter_LockoutGrowsAndCaps(t *testing.T) {
	l := NewLimiter()
	var last time.Duration
	for i := 0; i < 20; i++ {
		lockout := l.RecordFailure("1.2.3.4|admin")
		if lockout > 0 && lockout < last {
			t.Fatalf("attempt %d: expected non-decreasing lockout, got %v after %v", i+1, lockout, last)
		}
		if lockout > bruteForceMaxLockout {
			t.Fatalf("attempt %d: lockout %v exceeded cap %v", i+1, lockout, bruteForceMaxLockout)
		}
		if lockout > 0 {
			last = lockout
		}
	}
	if last != bruteForceMaxLockout {
		t.Fatalf("expected lockout to reach the cap %v after 20 failures, got %v", bruteForceMaxLockout, last)
	}
}

func TestLimiter_UnlocksAfterLockoutExpires(t *testing.T) {
	l := NewLimiter()
	fakeNow := time.Now()
	l.now = func() time.Time { return fakeNow }

	for i := 0; i <= bruteForceFreeAttempts; i++ {
		l.RecordFailure("1.2.3.4|admin")
	}
	allowed, _ := l.Allowed("1.2.3.4|admin")
	if allowed {
		t.Fatal("expected key to be locked immediately after crossing the free-attempt budget")
	}

	fakeNow = fakeNow.Add(bruteForceMaxLockout + time.Second)
	allowed, _ = l.Allowed("1.2.3.4|admin")
	if !allowed {
		t.Fatal("expected key to be allowed again once the lockout window has passed")
	}
}

func TestLimiter_RecordSuccessClearsHistory(t *testing.T) {
	l := NewLimiter()
	for i := 0; i <= bruteForceFreeAttempts; i++ {
		l.RecordFailure("1.2.3.4|admin")
	}
	l.RecordSuccess("1.2.3.4|admin")
	allowed, _ := l.Allowed("1.2.3.4|admin")
	if !allowed {
		t.Fatal("expected a successful login to clear the lockout/failure history")
	}
	// A fresh failure right after should be treated as attempt #1 again,
	// i.e. still within the free budget.
	if lockout := l.RecordFailure("1.2.3.4|admin"); lockout != 0 {
		t.Fatalf("expected history reset to restart the free-attempt budget, got lockout %v", lockout)
	}
}

func TestLimiter_KeysAreIndependent(t *testing.T) {
	l := NewLimiter()
	for i := 0; i <= bruteForceFreeAttempts; i++ {
		l.RecordFailure("1.2.3.4|admin")
	}
	allowed, _ := l.Allowed("5.6.7.8|admin")
	if !allowed {
		t.Fatal("expected an unrelated key to be unaffected by another key's lockout")
	}
}

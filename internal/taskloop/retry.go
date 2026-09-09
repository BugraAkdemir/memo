package taskloop

import (
	"sync"
	"time"
)

// DefaultRetryInterval is how long a rate-limited task list waits before the
// loop tries to continue from the same item.
const DefaultRetryInterval = 10 * time.Minute

// RetryScheduler holds one-shot timers for task lists parked in
// "waiting-limit". When a timer fires it calls resume(listID); the host wires
// that to re-enter the engine loop. Re-arming an already-armed list restarts
// its timer.
type RetryScheduler struct {
	interval time.Duration
	resume   func(listID string)
	mu       sync.Mutex
	timers   map[string]*time.Timer
	stopped  bool // set once by CancelAll; Arm and fired timers become no-ops
}

func NewRetryScheduler(interval time.Duration, resume func(listID string)) *RetryScheduler {
	if interval <= 0 {
		interval = DefaultRetryInterval
	}
	return &RetryScheduler{
		interval: interval,
		resume:   resume,
		timers:   make(map[string]*time.Timer),
	}
}

// Arm (re)starts the wait timer for listID with the scheduler's default
// interval. On fire it removes its own entry and calls resume(listID).
func (s *RetryScheduler) Arm(listID string) {
	if s == nil {
		return
	}
	s.ArmWithDelay(listID, s.interval)
}

// ArmWithDelay is Arm with an explicit wait — used for the transient-error
// escalating backoff (5 min, then 10 min) which is shorter/different from the
// fixed rate-limit interval.
func (s *RetryScheduler) ArmWithDelay(listID string, d time.Duration) {
	if s == nil {
		return
	}
	if d <= 0 {
		d = s.interval
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return // engine is shutting down — don't schedule a resume into it
	}
	if old, ok := s.timers[listID]; ok {
		old.Stop()
	}
	s.timers[listID] = time.AfterFunc(d, func() {
		s.mu.Lock()
		delete(s.timers, listID)
		stopped := s.stopped
		s.mu.Unlock()
		// time.Timer.Stop() is a no-op once the AfterFunc is already queued,
		// so CancelAll can't unschedule a timer that fires in the shutdown
		// window — this check is what actually stops resume() then (BUG-SCAN6).
		if stopped || s.resume == nil {
			return
		}
		s.resume(listID)
	})
}

// Cancel stops and forgets listID's timer, if any.
func (s *RetryScheduler) Cancel(listID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[listID]; ok {
		t.Stop()
		delete(s.timers, listID)
	}
}

// CancelAll stops and forgets every pending timer and permanently disables
// the scheduler. Used on engine shutdown so a list parked in a rate-limit /
// transient backoff can't fire resume() into an engine that is being torn
// down — including a timer that already fired and whose AfterFunc is queued
// or running (Stop() is a no-op by then; the stopped flag covers that case).
func (s *RetryScheduler) CancelAll() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	for id, t := range s.timers {
		t.Stop()
		delete(s.timers, id)
	}
}

// Pending reports whether listID currently has a wait timer.
func (s *RetryScheduler) Pending(listID string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.timers[listID]
	return ok
}

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

// Arm (re)starts the wait timer for listID. On fire it removes its own entry
// and calls resume(listID).
func (s *RetryScheduler) Arm(listID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.timers[listID]; ok {
		old.Stop()
	}
	s.timers[listID] = time.AfterFunc(s.interval, func() {
		s.mu.Lock()
		delete(s.timers, listID)
		s.mu.Unlock()
		if s.resume != nil {
			s.resume(listID)
		}
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

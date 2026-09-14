package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestStart_ConcurrentCallsDoNotBothLaunchPollLoop proves Start() is safe to
// call concurrently: two racing callers arriving while neither has yet set
// started=true must not both dial getMe and both launch pollLoop — the
// second one clobbering the first's stopCh/stopOnce (see Start's own
// comment) and leaving two goroutines racing to drain the same msgCh/errCh.
// The check-then-act window (check `started` under mu, unlock, dial GetMe,
// re-lock to actually set it) used to have no serialization at all around
// it — this proves getMe is now hit exactly once no matter how many
// concurrent Start() calls arrive.
func TestStart_ConcurrentCallsDoNotBothLaunchPollLoop(t *testing.T) {
	var getMeCalls int32
	release := make(chan struct{})
	var releaseOnce sync.Once

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			n := atomic.AddInt32(&getMeCalls, 1)
			if n == 1 {
				// Hold the first call in flight so a concurrent second
				// Start() call has a real window to race into.
				<-release
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":     true,
				"result": map[string]any{"id": 1, "username": "memo_bot"},
			})
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []any{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	prevBase := apiBase
	apiBase = srv.URL + "/bot"
	defer func() { apiBase = prevBase }()

	c := NewClient("test-token")

	const n = 5
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = c.Start(context.Background())
		}(i)
	}

	// Give every goroutine a chance to reach (and block inside) getMe before
	// releasing the first one — maximizes how many actually hit the race
	// window instead of serializing trivially through scheduling luck.
	time.Sleep(50 * time.Millisecond)
	releaseOnce.Do(func() { close(release) })
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("Start() call #%d failed: %v", i, err)
		}
	}

	if got := atomic.LoadInt32(&getMeCalls); got != 1 {
		t.Errorf("getMe was called %d times for %d concurrent Start() calls, want exactly 1", got, n)
	}
	if !c.IsRunning() {
		t.Error("client should be running after concurrent Start() calls")
	}

	// Stop and wait for pollLoop to actually exit (its own deferred cleanup
	// sets started=false right as it returns) before this function's
	// deferred apiBase/srv.Close() cleanup runs — otherwise a still-running
	// pollLoop's in-flight getUpdates call can read the shared apiBase var
	// concurrently with this test restoring it, a race in the test harness
	// itself rather than in the fix under test.
	c.Stop()
	deadline := time.Now().Add(2 * time.Second)
	for c.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if c.IsRunning() {
		t.Error("pollLoop did not stop within 2s of Stop()")
	}
}

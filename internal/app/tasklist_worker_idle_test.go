package app

import (
	"strings"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/taskloop"
)

// TestWorkerIdleTimeout_FlooredAtTenMinutes: a worker turn's quiet stretches
// are whole non-streamed model calls, so the planexec-sized 300s default would
// cut healthy turns short.
func TestWorkerIdleTimeout_FlooredAtTenMinutes(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	if got := a.workerIdleTimeout(); got != 10*time.Minute {
		t.Fatalf("workerIdleTimeout() = %s, want 10m with no config", got)
	}

	a.cfg.TaskLoop.StreamIdleTimeoutSec = 300 // the planexec default
	if got := a.workerIdleTimeout(); got != 10*time.Minute {
		t.Fatalf("workerIdleTimeout() = %s, want the 10m floor to win over 300s", got)
	}

	a.cfg.TaskLoop.StreamIdleTimeoutSec = 1800 // an explicitly higher setting
	if got := a.workerIdleTimeout(); got != 30*time.Minute {
		t.Fatalf("workerIdleTimeout() = %s, want the configured 30m", got)
	}
}

// TestWorkerIdleError_NotClassifiedAsProviderFault: a wedged turn must cost
// its own item and let the list continue. If the message read as a transient
// provider fault, the engine would park the whole list on a 5-then-10-minute
// retry timer instead.
func TestWorkerIdleError_NotClassifiedAsProviderFault(t *testing.T) {
	// The exact text buildTaskLoopRunWorker produces on an idle stream.
	err := errIdleWorkerTurn(10 * time.Minute)

	if taskloop.IsTransientErr(err) {
		t.Fatalf("idle-turn error reads as transient: %v", err)
	}
	if taskloop.IsRateLimitErr(err) || taskloop.IsAuthErr(err) || taskloop.IsChatBusyErr(err) {
		t.Fatalf("idle-turn error mis-classified: %v", err)
	}
	if !strings.Contains(err.Error(), "takıldı") {
		t.Fatalf("idle-turn error should say what happened, got: %v", err)
	}
}

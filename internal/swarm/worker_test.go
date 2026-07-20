// SPDX-License-Identifier: AGPL-3.0-or-later

package swarm

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// findFreePort asks the OS for an ephemeral port, then immediately frees it
// — a standard (if inherently racy in theory, negligible in practice) way
// to get a port number likely free for a subprocess to bind next.
func findFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("findFreePort: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// writeFakeSleepBinary writes an executable shell script that just sleeps,
// standing in for rpc-server in start/stop lifecycle tests where the
// process's actual RPC behavior is irrelevant — only whether Memo's own
// process supervision (start, track, stop, force-kill) works correctly.
func writeFakeSleepBinary(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binaries aren't directly executable on Windows")
	}
	path := filepath.Join(dir, "rpc-server")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 30\n"), 0755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return path
}

func TestRPCWorker_StartRequiresBinaryPath(t *testing.T) {
	w := NewRPCWorker(findFreePort(t))
	if err := w.Start("", 0); err == nil {
		t.Error("Start(\"\", 0) = nil error, want an error for an empty binary path")
	}
	if w.IsRunning() {
		t.Error("IsRunning() = true after a failed Start, want false")
	}
}

func TestRPCWorker_StartFailsForMissingBinary(t *testing.T) {
	w := NewRPCWorker(findFreePort(t))
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if err := w.Start(missing, 0); err == nil {
		t.Errorf("Start(%q, 0) = nil error, want an error for a nonexistent binary", missing)
	}
	if w.IsRunning() {
		t.Error("IsRunning() = true after a failed Start, want false")
	}
}

func TestRPCWorker_StartStopLifecycle(t *testing.T) {
	bin := writeFakeSleepBinary(t, t.TempDir())
	w := NewRPCWorker(findFreePort(t))

	if err := w.Start(bin, 0); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !w.IsRunning() {
		t.Fatal("IsRunning() = false right after a successful Start, want true")
	}

	// A second Start while already running must fail, not spawn a second process.
	if err := w.Start(bin, 0); err == nil {
		t.Error("Start() while already running = nil error, want an error")
	}

	if err := w.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	// monitor() clears state asynchronously on process exit; give it a
	// moment rather than asserting immediately.
	deadline := time.Now().Add(2 * time.Second)
	for w.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if w.IsRunning() {
		t.Error("IsRunning() = true after Stop() + settle time, want false")
	}

	// Stop on an already-stopped worker must be a safe no-op.
	if err := w.Stop(); err != nil {
		t.Errorf("second Stop() error = %v, want nil (no-op)", err)
	}
}

// TestRPCWorker_WaitReady_SucceedsWhenPortListening is intentionally not
// implemented as a unit test: WaitReady's loop first checks IsRunning(),
// which only becomes true through a real Start() — and no portable fake
// binary can both look like a Memo-managed subprocess (IsRunning()==true)
// and actually bind the target port without shell/OS-specific tooling
// (nc's -l flag isn't portable across BSD/GNU, no assumed Python). The
// negative case below (TestRPCWorker_WaitReady_TimesOutWhenNothingListening)
// covers the timeout branch with a real running process. The success path
// against a real rpc-server is exercised in Stage 10's real-hardware
// verification instead of faked here — see PLAN_memo_swarm.md's testing
// section on what's genuinely out of reach in this environment.

func TestRPCWorker_WaitReady_TimesOutWhenNothingListening(t *testing.T) {
	bin := writeFakeSleepBinary(t, t.TempDir())
	w := NewRPCWorker(findFreePort(t))
	if err := w.Start(bin, 0); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { w.Stop() })

	// The fake binary is running (IsRunning()==true) but never actually
	// binds the port (it just sleeps) — WaitReady must time out rather
	// than hang or false-positive.
	err := w.WaitReady(300 * time.Millisecond)
	if err == nil {
		t.Error("WaitReady() = nil error, want a timeout error since nothing is actually listening")
	}
}

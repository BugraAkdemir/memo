//go:build unix

package main

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestReapInBackground_NoZombie is the regression guard for BUG-M3:
// spawnDetachedBackend used to call cmd.Process.Release() after starting
// the child, which stops Go from tracking it but does not waitpid() it —
// Setsid only puts the child in its own session, it doesn't change who its
// OS parent is, so as long as this process stays alive, an exited child
// nobody calls Wait() on sits as a zombie until this process itself exits
// (or explicitly reaps it). reapInBackground fixes this by waiting on the
// child from a goroutine as soon as it's started.
func TestReapInBackground_NoZombie(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	pid := cmd.Process.Pid

	reapInBackground(cmd)

	// kill(pid, 0) keeps succeeding for a zombie (its process-table entry
	// still exists, just unreaped) and only starts failing with ESRCH once
	// something actually calls wait() on it — the definitive signal that
	// reaping happened, not just that the child exited.
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := syscall.Kill(pid, 0)
		if err == syscall.ESRCH {
			return // reaped — test passes
		}
		if time.Now().After(deadline) {
			t.Fatalf("process %d was not reaped within the deadline (still a zombie or running)", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

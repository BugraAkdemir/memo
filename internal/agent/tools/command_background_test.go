package tools

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestRunCommand_BackgroundChildDoesNotBlockForever is the regression for the
// bug that froze a whole Self-Driving list on its first item.
//
// The model started a web server the way anyone would — "python3 blog.py &" —
// and run_command never returned. bash exits immediately, but the backgrounded
// grandchild inherits the stdout/stderr pipes os/exec created for cmd.Stdout,
// and Wait blocks until every writer closes them. The context deadline could
// not save it either: it kills the shell, which had already exited.
//
// The call must come back promptly with whatever the shell printed.
func TestRunCommand_BackgroundChildDoesNotBlockForever(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell backgrounding syntax is POSIX-specific")
	}
	dir := t.TempDir()

	// Sleep far longer than any timeout in play, in the background, and exit.
	args, _ := json.Marshal(RunCommandArgs{Command: `sleep 600 & echo started`})

	done := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		out, err := RunCommand(context.Background(), args, dir, nil)
		if err != nil {
			errCh <- err
			return
		}
		done <- out
	}()

	select {
	case out := <-done:
		if !strings.Contains(out, "started") {
			t.Fatalf("output lost the command's own stdout: %q", out)
		}
		if !strings.Contains(out, "background process") {
			t.Fatalf("output should tell the model a child is still running, got: %q", out)
		}
	case err := <-errCh:
		t.Fatalf("RunCommand returned an error for a normal backgrounded command: %v", err)
	case <-time.After(CommandWaitDelay + 20*time.Second):
		t.Fatal("RunCommand blocked on a backgrounded child — the pipes are still held open")
	}
}

// TestRunCommand_ForegroundOutputUnaffected: the WaitDelay must not clip an
// ordinary command's output.
func TestRunCommand_ForegroundOutputUnaffected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell")
	}
	dir := t.TempDir()
	args, _ := json.Marshal(RunCommandArgs{Command: `echo hello; echo oops >&2`})

	out, err := RunCommand(context.Background(), args, dir, nil)
	if err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}
	if !strings.Contains(out, "hello") || !strings.Contains(out, "oops") {
		t.Fatalf("stdout/stderr not both captured: %q", out)
	}
	if strings.Contains(out, "background process") {
		t.Fatalf("a plain command must not be reported as leaving a background process: %q", out)
	}
}

// TestRunCommand_TimeoutStillReportsTimeout: a genuinely long *foreground*
// command still times out and says so (the WaitDelay path must not swallow it).
func TestRunCommand_TimeoutStillReportsTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell")
	}
	dir := t.TempDir()
	args, _ := json.Marshal(RunCommandArgs{Command: `sleep 30`})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	out, err := RunCommand(ctx, args, dir, nil)
	if err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}
	if !strings.Contains(out, "timed out") {
		t.Fatalf("expected a timeout report, got: %q", out)
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("timeout took %s to surface", elapsed)
	}
}

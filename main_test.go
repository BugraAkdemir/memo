package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestEmptyPromptExitsCleanly is the regression test for BUG-L1: `memo -p
// ""` used to hang forever, only killable from outside (SIGINT/SIGTERM),
// because `*prompt != ""` couldn't distinguish "-p not passed at all" from
// "-p passed with an explicit empty string" — the latter silently fell
// through to the interactive/headless signal-wait loop instead of
// runPrintMode, and that loop never exits on its own when nothing sent it a
// signal. This builds the real binary and runs it as a subprocess (calling
// main() in-process would call os.Exit and kill the test runner), asserting
// it exits quickly with a clear error instead of hanging — bounded by a
// context timeout so a regression fails the test instead of hanging CI.
func TestEmptyPromptExitsCleanly(t *testing.T) {
	bin := buildTestBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "-p", "", "--port", "18199")
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("process did not exit on its own within the timeout (regressed to hanging); output so far=%s", out)
	}
	if err == nil {
		t.Fatalf("expected a non-zero exit for empty -p, got success; output=%s", out)
	}
	if !strings.Contains(string(out), "boş mesaj") {
		t.Errorf("output = %q, want a clear empty-message error", out)
	}
}

func buildTestBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "memo-test-bin")
	cmd := exec.Command("go", "build", "-tags", "sqlite_fts5", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

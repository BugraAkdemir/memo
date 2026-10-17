// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"memo/internal/updater"
)

// These tests run the REAL memo-updater binary against a fake install whose
// "memo-backend" is a tiny HTTP server, so the whole sequence — wait for the old
// process, swap, relaunch through the launcher script, health check, rollback —
// is exercised with real processes and real files.

func buildTool(t *testing.T, pkg, out string, ldflags string) {
	t.Helper()
	args := []string{"build"}
	if ldflags != "" {
		args = append(args, "-ldflags", ldflags)
	}
	args = append(args, "-o", out, pkg)
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", pkg, err, b)
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

func writeScript(t *testing.T, dir string) {
	t.Helper()
	// Mirrors the shape of the real run_memo.sh: start the backend in the
	// background, then wait (the real one runs the Flutter window here).
	script := "#!/bin/sh\nDIR=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\n\"$DIR/memo-backend\" &\nwait\n"
	if err := os.WriteFile(filepath.Join(dir, "run_memo.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func versionOf(t *testing.T, port string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return updater.HTTPHealth("http://127.0.0.1:" + port + "/api/version")(ctx)
}

func killBackend(t *testing.T, install string) {
	b, err := os.ReadFile(filepath.Join(install, "backend.pid"))
	if err != nil {
		return
	}
	if pid, _ := strconv.Atoi(strings.TrimSpace(string(b))); pid > 0 {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

type e2eEnv struct {
	updaterBin, oldBackend, newBackend, install, port string
}

func newE2E(t *testing.T) *e2eEnv {
	if runtime.GOOS == "windows" {
		t.Skip("script-install flow is unix only")
	}
	if testing.Short() {
		t.Skip("builds binaries")
	}
	bin := t.TempDir()
	e := &e2eEnv{
		updaterBin: filepath.Join(bin, "memo-updater"),
		oldBackend: filepath.Join(bin, "backend-old"),
		newBackend: filepath.Join(bin, "backend-new"),
		install:    t.TempDir(),
		port:       freePort(t),
	}
	buildTool(t, ".", e.updaterBin, "")
	buildTool(t, "./testdata/fakebackend", e.oldBackend, "-X main.version=V4.6.0")
	buildTool(t, "./testdata/fakebackend", e.newBackend, "-X main.version=V4.7.0")
	return e
}

func (e *e2eEnv) layoutOld(t *testing.T) {
	copyExec(t, e.oldBackend, filepath.Join(e.install, "memo-backend"))
	writeScript(t, e.install)
	_ = os.MkdirAll(filepath.Join(e.install, "config"), 0o755)
	_ = os.WriteFile(filepath.Join(e.install, "config", "config.yaml"), []byte("user: setting\n"), 0o600)
	_ = os.MkdirAll(filepath.Join(e.install, "data", "models"), 0o755)
	_ = os.WriteFile(filepath.Join(e.install, "data", "models", "m.gguf"), []byte("MODEL"), 0o644)
}

func (e *e2eEnv) layoutStaged(t *testing.T) (staged string) {
	staged = filepath.Join(e.install, ".update", "staging", "Memo")
	_ = os.MkdirAll(staged, 0o755)
	copyExec(t, e.newBackend, filepath.Join(staged, "memo-backend"))
	writeScript(t, staged)
	return staged
}

func copyExec(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o755); err != nil {
		t.Fatal(err)
	}
}

func (e *e2eEnv) startOld(t *testing.T) *exec.Cmd {
	cmd := exec.Command(filepath.Join(e.install, "memo-backend"))
	cmd.Env = append(os.Environ(), "FAKE_PORT="+e.port)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	waitFor(t, func() bool { v, err := versionOf(t, e.port); return err == nil && v == "V4.6.0" }, "old backend up")
	return cmd
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", what)
}

func (e *e2eEnv) runUpdater(t *testing.T, old *exec.Cmd, killOldAfter time.Duration) (string, error) {
	work := filepath.Join(e.install, ".update")
	cmd := exec.Command(e.updaterBin, "apply",
		"--install", e.install, "--work", work,
		"--staged", filepath.Join(work, "staging", "Memo"), "--backup", filepath.Join(work, "backup"),
		"--wait-pid", strconv.Itoa(old.Process.Pid), "--expect", "V4.7.0",
		"--health", "http://127.0.0.1:"+e.port+"/api/version",
		"--launch", filepath.Join(e.install, "run_memo.sh"),
		"--log", filepath.Join(e.install, "update.log"))
	cmd.Env = append(os.Environ(), "FAKE_PORT="+e.port)
	var out strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// The old Memo "shuts down" a moment after the updater started waiting.
	time.AfterFunc(killOldAfter, func() { _ = old.Process.Kill(); _, _ = old.Process.Wait() })
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return out.String(), err
	case <-time.After(60 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("memo-updater did not finish:\n%s", out.String())
		return "", nil
	}
}

func TestE2E_UpdateSucceeds(t *testing.T) {
	e := newE2E(t)
	e.layoutOld(t)
	e.layoutStaged(t)
	old := e.startOld(t)
	// A note from an earlier failed attempt must not outlive a success.
	_ = os.WriteFile(filepath.Join(e.install, updater.FailureFile), []byte("update to V4.6.9 failed\n"), 0o600)

	out, err := e.runUpdater(t, old, 300*time.Millisecond)
	defer killBackend(t, e.install)
	if err != nil {
		t.Fatalf("updater failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(e.install, updater.FailureFile)); err == nil {
		t.Error("the old failure note survived a successful update")
	}
	if v, err := versionOf(t, e.port); err != nil || v != "V4.7.0" {
		t.Fatalf("after the update the backend reports %q, %v; want V4.7.0\n%s", v, err, out)
	}
	if b, _ := os.ReadFile(filepath.Join(e.install, "config", "config.yaml")); string(b) != "user: setting\n" {
		t.Errorf("user config changed: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(e.install, "data", "models", "m.gguf")); string(b) != "MODEL" {
		t.Errorf("a model file changed: %q", b)
	}
	if _, err := os.Stat(filepath.Join(e.install, ".update")); err == nil {
		t.Error("the scratch directory was left behind after a successful update")
	}
	if b, _ := os.ReadFile(filepath.Join(e.install, "update.log")); !strings.Contains(string(b), "done") {
		t.Errorf("log lacks the final line:\n%s", b)
	}
}

func TestE2E_NewVersionThatCrashesIsRolledBack(t *testing.T) {
	e := newE2E(t)
	// The new release is built to exit as soon as it starts.
	buildTool(t, "./testdata/fakebackend", e.newBackend, "-X main.version=V4.7.0 -X main.crash=1")
	e.layoutOld(t)
	e.layoutStaged(t)
	old := e.startOld(t)

	out, err := e.runUpdater(t, old, 300*time.Millisecond)
	defer killBackend(t, e.install)
	if err == nil {
		t.Fatalf("a crashing new version was reported as success:\n%s", out)
	}
	logText, _ := os.ReadFile(filepath.Join(e.install, "update.log"))
	if !strings.Contains(string(logText), "previous version") {
		t.Errorf("update.log does not explain the rollback:\n%s", logText)
	}
	// The person is told on the next start.
	if note := updater.ReadFailure(filepath.Join(e.install, updater.FailureFile)); !strings.Contains(note, "V4.7.0") || !strings.Contains(note, "rolled back") {
		t.Errorf("failure note = %q", note)
	}
	// Each line is written once: to the log, not also to stderr.
	if strings.Count(string(logText), "relaunching") != 2 { // the new version, then the previous one
		t.Errorf("expected exactly two 'relaunching' lines (new, then restored), got:\n%s", logText)
	}
	waitFor(t, func() bool { v, err := versionOf(t, e.port); return err == nil && v == "V4.6.0" }, "the OLD backend to be back after the rollback")
	if b, _ := os.ReadFile(filepath.Join(e.install, "config", "config.yaml")); string(b) != "user: setting\n" {
		t.Errorf("user config changed: %q", b)
	}
	if _, err := os.Stat(filepath.Join(e.install, ".update")); err == nil {
		t.Error("the scratch directory was left behind after a clean rollback")
	}
}

func TestE2E_RefusesToSwapWhileTheOldMemoStillRuns(t *testing.T) {
	e := newE2E(t)
	e.layoutOld(t)
	e.layoutStaged(t)
	old := e.startOld(t)
	// Shrink nothing: the updater waits 90s by default, so use a pid that never
	// exits only briefly — kill the old one AFTER the updater gives up is not
	// possible; instead assert that nothing is replaced before the old exits.
	work := filepath.Join(e.install, ".update")
	cmd := exec.Command(e.updaterBin, "apply",
		"--install", e.install, "--work", work,
		"--staged", filepath.Join(work, "staging", "Memo"), "--backup", filepath.Join(work, "backup"),
		"--wait-pid", strconv.Itoa(old.Process.Pid), "--expect", "V4.7.0",
		"--health", "http://127.0.0.1:"+e.port+"/api/version",
		"--launch", filepath.Join(e.install, "run_memo.sh"))
	cmd.Env = append(os.Environ(), "FAKE_PORT="+e.port)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	time.Sleep(1500 * time.Millisecond)

	if v, err := versionOf(t, e.port); err != nil || v != "V4.6.0" {
		t.Fatalf("the running Memo was disturbed: %q, %v", v, err)
	}
	b, _ := os.ReadFile(filepath.Join(e.install, "memo-backend"))
	oldBytes, _ := os.ReadFile(e.oldBackend)
	if string(b) != string(oldBytes) {
		t.Fatal("memo-backend was replaced while the old process was still alive")
	}
}

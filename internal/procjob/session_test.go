// SPDX-License-Identifier: AGPL-3.0-or-later

package procjob

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestMain intercepts HELPER_MODE invocations so session tests can spawn real
// child processes (owned backend, UI, hung backend, detached external app)
// without external binaries or shell scripts.
func TestMain(m *testing.M) {
	mode := os.Getenv("MEMO_PROCJOB_HELPER")
	if mode != "" {
		os.Exit(runHelper(mode))
	}
	os.Exit(m.Run())
}

func runHelper(mode string) int {
	switch mode {
	case "ui-clean":
		// Record invocation and exit 0 immediately.
		if p := os.Getenv("HELPER_UI_MARKER"); p != "" {
			_ = os.WriteFile(p, []byte("ui-clean\n"), 0644)
		}
		return 0

	case "ui-crash":
		if p := os.Getenv("HELPER_UI_MARKER"); p != "" {
			_ = os.WriteFile(p, []byte("ui-crash\n"), 0644)
		}
		return 137

	case "ui-wait-backend":
		time.Sleep(50 * time.Millisecond)
		return 0

	case "backend-graceful":
		// Write ready file, then wait for stdin pipe EOF from WatchOwnerPipe.
		if p := os.Getenv("HELPER_READY_FILE"); p != "" {
			_ = os.WriteFile(p, []byte("ready\n"), 0644)
		}
		done := make(chan struct{})
		WatchOwnerPipe(os.Stdin, func() {
			if p := os.Getenv("HELPER_FLUSH_FILE"); p != "" {
				_ = os.WriteFile(p, []byte("flushed\n"), 0644)
			}
			close(done)
		})
		<-done
		return 0

	case "backend-hung":
		// Spawns an in-group child AND a DetachedAttr sibling, writes all PIDs
		// cleanly, then ignores stdin EOF until force-killed.
		inGroup := exec.Command(os.Args[0])
		inGroup.Env = append(os.Environ(), "MEMO_PROCJOB_HELPER=sleep-forever")
		_ = inGroup.Start()

		detached := exec.Command(os.Args[0])
		detached.Env = append(os.Environ(), "MEMO_PROCJOB_HELPER=sleep-short")
		detached.SysProcAttr = DetachedAttr()
		_ = detached.Start()

		if p := os.Getenv("HELPER_PIDS_FILE"); p != "" {
			inPid := 0
			detPid := 0
			if inGroup.Process != nil {
				inPid = inGroup.Process.Pid
			}
			if detached.Process != nil {
				detPid = detached.Process.Pid
			}
			_ = os.WriteFile(p, []byte(fmt.Sprintf("%d,%d,%d", os.Getpid(), inPid, detPid)), 0644)
		}
		if p := os.Getenv("HELPER_READY_FILE"); p != "" {
			_ = os.WriteFile(p, []byte("ready\n"), 0644)
		}
		for {
			time.Sleep(time.Hour)
		}

	case "sleep-forever":
		for {
			time.Sleep(time.Hour)
		}

	case "sleep-short":
		// Stays alive long enough for parent hung test to check survival, then exits.
		time.Sleep(1500 * time.Millisecond)
		if p := os.Getenv("HELPER_DETACHED_ALIVE_FILE"); p != "" {
			_ = os.WriteFile(p, []byte("survived\n"), 0644)
		}
		return 0
	}
	return 2
}

func TestWatchOwnerPipe_FiresOnceOnEOF(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()

	var calls atomic.Int32
	done := make(chan struct{})
	WatchOwnerPipe(pr, func() {
		calls.Add(1)
		close(done)
	})

	select {
	case <-done:
		t.Fatal("WatchOwnerPipe fired before write end closed")
	case <-time.After(30 * time.Millisecond):
	}

	_ = pw.Close()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("WatchOwnerPipe did not fire after write end closed")
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
}

func TestDesktopSession_SharedBackendPreserved(t *testing.T) {
	dir := t.TempDir()
	uiMarker := filepath.Join(dir, "ui.txt")
	backendMarker := filepath.Join(dir, "backend.txt")

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	cfg := DesktopSessionConfig{
		UIPath:          exe,
		UIEnv:           []string{"MEMO_PROCJOB_HELPER=ui-clean", "HELPER_UI_MARKER=" + uiMarker},
		BackendPath:     exe,
		BackendArgs:     []string{},
		BackendEnv:      []string{"MEMO_PROCJOB_HELPER=backend-graceful", "HELPER_FLUSH_FILE=" + backendMarker},
		ProbeHealthy:    func(context.Context) bool { return true },
		GracefulTimeout: time.Second,
	}

	code, err := RunDesktopSession(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunDesktopSession error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if _, err := os.Stat(uiMarker); err != nil {
		t.Fatalf("UI did not run: %v", err)
	}
	if _, err := os.Stat(backendMarker); !os.IsNotExist(err) {
		t.Fatalf("owned backend ran/flushed even though shared backend was already healthy")
	}
}

func TestDesktopSession_OwnedGracefulShutdownAndCrashCleanup(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name     string
		uiHelper string
		wantCode int
	}{
		{name: "normal_ui_exit", uiHelper: "ui-clean", wantCode: 0},
		{name: "ui_crash_exit", uiHelper: "ui-crash", wantCode: 137},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			readyFile := filepath.Join(dir, "ready.txt")
			flushFile := filepath.Join(dir, "flush.txt")

			cfg := DesktopSessionConfig{
				UIPath: exe,
				UIEnv:  []string{"MEMO_PROCJOB_HELPER=" + tc.uiHelper},
				BackendPath: exe,
				BackendEnv: []string{
					"MEMO_PROCJOB_HELPER=backend-graceful",
					"HELPER_READY_FILE=" + readyFile,
					"HELPER_FLUSH_FILE=" + flushFile,
				},
				ProbeHealthy: func(context.Context) bool {
					_, err := os.Stat(readyFile)
					return err == nil
				},
				ReadyTimeout:    5 * time.Second,
				GracefulTimeout: 5 * time.Second,
			}

			start := time.Now()
			code, err := RunDesktopSession(context.Background(), cfg)
			if err != nil {
				t.Fatalf("RunDesktopSession error: %v", err)
			}
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d", code, tc.wantCode)
			}
			if time.Since(start) > 4*time.Second {
				t.Fatalf("cleanup took %v, expected immediate graceful stop well inside GracefulTimeout", time.Since(start))
			}
			got, err := os.ReadFile(flushFile)
			if err != nil || strings.TrimSpace(string(got)) != "flushed" {
				t.Fatalf("expected backend flush on control pipe EOF, got %q err=%v", got, err)
			}
		})
	}
}

func TestDesktopSession_HungBackendForceKillAndDetachedSiblingPreserved(t *testing.T) {
	dir := t.TempDir()
	readyFile := filepath.Join(dir, "ready.txt")
	pidsFile := filepath.Join(dir, "pids.txt")
	detachedAliveFile := filepath.Join(dir, "detached-alive.txt")

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	cfg := DesktopSessionConfig{
		UIPath:      exe,
		UIEnv:       []string{"MEMO_PROCJOB_HELPER=ui-wait-backend"},
		BackendPath: exe,
		BackendEnv: []string{
			"MEMO_PROCJOB_HELPER=backend-hung",
			"HELPER_READY_FILE=" + readyFile,
			"HELPER_PIDS_FILE=" + pidsFile,
			"HELPER_DETACHED_ALIVE_FILE=" + detachedAliveFile,
		},
		ProbeHealthy: func(context.Context) bool {
			_, err := os.Stat(readyFile)
			return err == nil
		},
		ReadyTimeout:    3 * time.Second,
		GracefulTimeout: 150 * time.Millisecond,
	}

	code, err := RunDesktopSession(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunDesktopSession error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	raw, err := os.ReadFile(pidsFile)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(strings.TrimSpace(string(raw)), ",")
	if len(parts) != 3 {
		t.Fatalf("unexpected pids %q", raw)
	}
	backendPid, _ := strconv.Atoi(parts[0])
	inGroupPid, _ := strconv.Atoi(parts[1])
	detachedPid, _ := strconv.Atoi(parts[2])

	t.Cleanup(func() {
		if p, err := os.FindProcess(detachedPid); err == nil {
			_ = p.Kill()
		}
	})

	// Owned backend and its in-group child must both be dead after bounded timeout.
	waitDead(t, backendPid)
	waitDead(t, inGroupPid)

	// Detached external sibling (open_app / memo-updater) must finish its natural run!
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(detachedAliveFile); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("detached external sibling (pid %d) was terminated unexpectedly", detachedPid)
}

func TestDesktopSession_ContextCancelStopsUIAndOwnedBackend(t *testing.T) {
	dir := t.TempDir()
	readyFile := filepath.Join(dir, "ready.txt")
	flushFile := filepath.Join(dir, "flush.txt")

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	cfg := DesktopSessionConfig{
		UIPath:      exe,
		UIEnv:       []string{"MEMO_PROCJOB_HELPER=sleep-forever"},
		BackendPath: exe,
		BackendEnv: []string{
			"MEMO_PROCJOB_HELPER=backend-graceful",
			"HELPER_READY_FILE=" + readyFile,
			"HELPER_FLUSH_FILE=" + flushFile,
		},
		ProbeHealthy: func(context.Context) bool {
			_, err := os.Stat(readyFile)
			return err == nil
		},
		ReadyTimeout:    3 * time.Second,
		GracefulTimeout: time.Second,
		LogOut:          io.Discard,
	}

	_, _ = RunDesktopSession(ctx, cfg)
	if _, err := os.Stat(flushFile); err != nil {
		t.Fatalf("expected owned backend flush on owner cancellation, err=%v", err)
	}
}

func waitDead(t *testing.T, pid int) {
	t.Helper()
	if pid <= 0 {
		return
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("process %d still alive after kill", pid)
}

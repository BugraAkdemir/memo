// SPDX-License-Identifier: AGPL-3.0-or-later

package procjob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	defaultReadyTimeout    = 15 * time.Second
	defaultGracefulTimeout = 18 * time.Second
)

// WatchOwnerPipe reads r until EOF or read error and invokes onClose once.
// Closing or killing the parent process holding the write end immediately tears
// down the pipe in the OS kernel, triggering clean child shutdown without
// polling or waiting for stale client TTLs.
func WatchOwnerPipe(r io.Reader, onClose func()) {
	if r == nil || onClose == nil {
		return
	}
	go func() {
		buf := make([]byte, 64)
		for {
			_, err := r.Read(buf)
			if err != nil {
				onClose()
				return
			}
		}
	}()
}

// DesktopSessionConfig controls how RunDesktopSession supervises a desktop UI
// and (when no shared server is already healthy) an owned Memo backend.
type DesktopSessionConfig struct {
	UIPath          string
	UIArgs          []string
	UIEnv           []string
	UIDir           string
	BackendPath     string
	BackendArgs     []string
	BackendEnv      []string
	BackendDir      string
	ProbeHealthy    func(context.Context) bool
	ReadyTimeout    time.Duration
	GracefulTimeout time.Duration
	LogOut          io.Writer
	OnBackendReady  func()
}

// RunDesktopSession supervises a single Memo desktop session:
//
//  1. If ProbeHealthy already reports true (e.g. user started a CLI or shared
//     --headless backend), only the UI is launched and the shared backend is
//     left running when the UI exits.
//  2. Otherwise, an owned backend is started holding the read end of a control
//     pipe on stdin, Readiness is polled up to ReadyTimeout, then UI is launched.
//  3. When the UI exits (quit, crash, or kill) or ctx is cancelled, the owner
//     closes the control pipe write end so the backend flushes SQLite and stops
//     local engines within GracefulTimeout; if hung beyond GracefulTimeout, only
//     the owned backend's process group/tree is force-killed.
func RunDesktopSession(ctx context.Context, cfg DesktopSessionConfig) (int, error) {
	if strings.TrimSpace(cfg.UIPath) == "" {
		return 1, fmt.Errorf("desktop session: empty UIPath")
	}
	readyTimeout := cfg.ReadyTimeout
	if readyTimeout <= 0 {
		readyTimeout = defaultReadyTimeout
	}
	gracefulTimeout := cfg.GracefulTimeout
	if gracefulTimeout <= 0 {
		gracefulTimeout = defaultGracefulTimeout
	}
	logOut := cfg.LogOut
	if logOut == nil {
		logOut = io.Discard
	}

	if err := EnsureSelf(); err != nil {
		_, _ = fmt.Fprintf(logOut, "procjob: EnsureSelf warning: %v\n", err)
	}

	if cfg.ProbeHealthy != nil {
		probeCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
		alreadyHealthy := cfg.ProbeHealthy(probeCtx)
		cancel()
		if alreadyHealthy {
			if cfg.OnBackendReady != nil {
				cfg.OnBackendReady()
			}
			return runUIOnly(ctx, cfg, gracefulTimeout)
		}
	}

	if strings.TrimSpace(cfg.BackendPath) == "" {
		return 1, fmt.Errorf("desktop session: empty BackendPath")
	}

	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return 1, fmt.Errorf("desktop session: pipe: %w", err)
	}
	var closePipeOnce sync.Once
	closePipeW := func() {
		closePipeOnce.Do(func() {
			if pipeW != nil {
				_ = pipeW.Close()
			}
		})
	}
	defer closePipeW()

	backendCmd := exec.Command(cfg.BackendPath, cfg.BackendArgs...)
	backendCmd.Dir = cfg.BackendDir
	backendCmd.Env = mergeEnv(os.Environ(), cfg.BackendEnv)
	backendCmd.Stdin = pipeR
	backendCmd.Stdout = logOut
	backendCmd.Stderr = logOut
	backendCmd.SysProcAttr = ManagedAttr()

	if err := backendCmd.Start(); err != nil {
		_ = pipeR.Close()
		closePipeW()
		return 1, fmt.Errorf("desktop session: start backend: %w", err)
	}
	// Close the parent's copy of the read end right after Start so closing
	// pipeW delivers EOF immediately inside the child backend.
	_ = pipeR.Close()
	tree := attachManagedTree(backendCmd.Process)
	defer releaseManagedTree(tree)

	backendDone := make(chan error, 1)
	go func() {
		backendDone <- backendCmd.Wait()
	}()

	stopOwnedBackend := func() {
		closePipeW()
		timer := time.NewTimer(gracefulTimeout)
		defer timer.Stop()
		select {
		case <-backendDone:
			return
		case <-timer.C:
			killManagedTree(backendCmd, tree)
			forceTimer := time.NewTimer(2 * time.Second)
			defer forceTimer.Stop()
			select {
			case <-backendDone:
			case <-forceTimer.C:
			}
		}
	}

	if cfg.ProbeHealthy != nil {
		readyCtx, cancelReady := context.WithTimeout(ctx, readyTimeout)
		readyOK, earlyExit, bErr := waitBackendReady(readyCtx, cfg.ProbeHealthy, backendDone)
		cancelReady()
		if earlyExit {
			closePipeW()
			return 1, fmt.Errorf("desktop session: backend exited during startup: %w", bErr)
		}
		if ctx.Err() != nil {
			stopOwnedBackend()
			return 0, ctx.Err()
		}
		if readyOK && cfg.OnBackendReady != nil {
			cfg.OnBackendReady()
		}
	} else if cfg.OnBackendReady != nil {
		cfg.OnBackendReady()
	}

	uiCmd, uiDone, err := startUI(cfg)
	if err != nil {
		stopOwnedBackend()
		return 1, fmt.Errorf("desktop session: start ui: %w", err)
	}

	var (
		uiErr        error
		backendAlive = true
	)
	select {
	case uiErr = <-uiDone:
	case <-ctx.Done():
		uiErr = stopUIProcess(uiCmd, uiDone, minDuration(2*time.Second, gracefulTimeout))
	case bErr := <-backendDone:
		backendAlive = false
		_, _ = fmt.Fprintf(logOut, "procjob: owned backend exited unexpectedly: %v\n", bErr)
		select {
		case uiErr = <-uiDone:
		case <-ctx.Done():
			uiErr = stopUIProcess(uiCmd, uiDone, minDuration(2*time.Second, gracefulTimeout))
		}
	}

	if backendAlive {
		stopOwnedBackend()
	}

	return exitCodeOf(uiErr), nil
}

func runUIOnly(ctx context.Context, cfg DesktopSessionConfig, gracefulTimeout time.Duration) (int, error) {
	uiCmd, uiDone, err := startUI(cfg)
	if err != nil {
		return 1, fmt.Errorf("desktop session: start ui: %w", err)
	}
	var uiErr error
	select {
	case uiErr = <-uiDone:
	case <-ctx.Done():
		uiErr = stopUIProcess(uiCmd, uiDone, minDuration(2*time.Second, gracefulTimeout))
	}
	return exitCodeOf(uiErr), nil
}

func startUI(cfg DesktopSessionConfig) (*exec.Cmd, chan error, error) {
	uiCmd := exec.Command(cfg.UIPath, cfg.UIArgs...)
	uiCmd.Dir = cfg.UIDir
	uiCmd.Env = mergeEnv(os.Environ(), cfg.UIEnv)
	uiCmd.Stdout = os.Stdout
	uiCmd.Stderr = os.Stderr
	uiCmd.SysProcAttr = UIAttr()
	if err := uiCmd.Start(); err != nil {
		return nil, nil, err
	}
	uiDone := make(chan error, 1)
	go func() {
		uiDone <- uiCmd.Wait()
	}()
	return uiCmd, uiDone, nil
}

func stopUIProcess(uiCmd *exec.Cmd, uiDone <-chan error, waitBudget time.Duration) error {
	terminateGraceful(uiCmd)
	timer := time.NewTimer(waitBudget)
	defer timer.Stop()
	select {
	case err := <-uiDone:
		return err
	case <-timer.C:
		KillProcessTree(uiCmd)
		return <-uiDone
	}
}

func waitBackendReady(ctx context.Context, probe func(context.Context) bool, backendDone <-chan error) (ready bool, exited bool, exitErr error) {
	if probe(ctx) {
		return true, false, nil
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-backendDone:
			return false, true, err
		case <-ctx.Done():
			return false, false, nil
		case <-ticker.C:
			if probe(ctx) {
				return true, false, nil
			}
		}
	}
}

func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		if code >= 0 {
			return code
		}
	}
	return 1
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// mergeEnv overlays overrides onto base without duplicating keys.
func mergeEnv(base, overrides []string) []string {
	if len(overrides) == 0 {
		return base
	}
	idxByKey := make(map[string]int, len(base))
	out := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		k, _, ok := strings.Cut(entry, "=")
		if ok {
			idxByKey[strings.ToUpper(k)] = len(out)
		}
		out = append(out, entry)
	}
	for _, entry := range overrides {
		k, _, ok := strings.Cut(entry, "=")
		if ok {
			upper := strings.ToUpper(k)
			if pos, exists := idxByKey[upper]; exists {
				out[pos] = entry
				continue
			}
			idxByKey[upper] = len(out)
		}
		out = append(out, entry)
	}
	return out
}

// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package tools

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// launchSettle is how long startDetached watches a freshly started app for
// an immediate failure. A var so tests can shorten it.
var launchSettle = 1500 * time.Millisecond

// startDetached starts argv as a GUI application that outlives the request
// and reports a failure the app itself shows in its first moments.
//
//   - Its own session (Setsid), so it is not tied to Memo's process group
//     and survives Memo restarting, the way an app launched from the menu
//     does.
//   - Always reaped: a goroutine Waits on it. The old code called Start and
//     never Wait, which left a zombie behind for every launch (seen live as
//     "[xdg-open] <defunct>").
//   - An immediate non-zero exit — no display, a broken install, a missing
//     library — is returned as an error with the app's own stderr, instead
//     of the old unconditional "started". A zero exit inside the window is
//     success: many launchers (steam, flatpak, code) hand off to a running
//     instance or a child and exit at once.
func startDetached(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("empty command")
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return err
	}
	cmd := exec.Command(path, argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	var stderr lockedBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if len(msg) > 400 {
				msg = msg[len(msg)-400:]
			}
			if msg != "" {
				return fmt.Errorf("%w: %s", err, msg)
			}
			return err
		}
		return nil
	case <-time.After(launchSettle):
		// Still running: it is up. The goroutine above keeps waiting so the
		// process is reaped whenever it exits; stop collecting its stderr.
		stderr.stop()
		return nil
	}
}

// lockedBuffer is a bytes.Buffer safe to write from the process's stderr
// copier while startDetached reads it, and capped so a chatty app that keeps
// running cannot grow it forever.
type lockedBuffer struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	stopped bool
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.stopped && b.buf.Len() < 64*1024 {
		b.buf.Write(p)
	}
	return len(p), nil
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *lockedBuffer) stop() {
	b.mu.Lock()
	b.stopped = true
	b.buf.Reset()
	b.mu.Unlock()
}

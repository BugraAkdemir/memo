// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package updater

import (
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"
)

// StartDetached runs script (Memo's launcher) in its own session, so it
// outlives the updater, and returns a function that stops everything it
// started. dir is the working directory; output goes to out (may be nil).
//
// The process is intentionally never Wait()ed on by the caller's logic that
// follows: it is Memo, expected to run for hours. A goroutine reaps it so the
// stop function and a quick exit do not leave a zombie behind.
func StartDetached(script, dir string, env []string, out io.Writer) (*Launched, error) {
	cmd := exec.Command(script)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = nil
	if out != nil {
		cmd.Stdout = out
		cmd.Stderr = out
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("updater: starting %s: %w", script, err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()

	return &Launched{
		Exited: done,
		Stop: func() {
			// Setsid makes the child its own group leader: signal the whole group,
			// so the backend the launcher started goes too. Anything that left the
			// group (a daemonised grandchild) is not ours to chase.
			pgid := cmd.Process.Pid
			_ = syscall.Kill(-pgid, syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
			}
			// The launcher may be gone while its backend is still shutting down.
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		},
	}, nil
}

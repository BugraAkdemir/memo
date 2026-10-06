//go:build !windows

// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"os/exec"
	"syscall"
	"time"
)

// sysProcAttr puts the child in its own process group so killTree can end the
// whole tree without touching Memo itself (same pattern as internal/ngrok).
func sysProcAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }

// killTree asks the group to stop, then makes sure. The group kill is only
// safe because sysProcAttr gave the child its own group.
func killTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil || pgid != cmd.Process.Pid {
		_ = cmd.Process.Kill()
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	time.Sleep(300 * time.Millisecond)
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}

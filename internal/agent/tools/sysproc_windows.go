// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package tools

import (
	"os/exec"
	"syscall"
)

// newSysProcAttr has no process-group equivalent to set here; Windows job
// objects would be the analogue and nothing in Memo needs one yet.
func newSysProcAttr() *syscall.SysProcAttr { return nil }

// killProcessGroup falls back to killing the direct child.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

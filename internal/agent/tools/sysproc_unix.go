// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package tools

import (
	"os/exec"
	"syscall"
)

// newSysProcAttr puts the shell run_command starts in its own process group,
// the same pattern internal/agentcli and internal/llama already use, so
// killProcessGroup can take down the whole tree it spawned rather than just
// the direct `bash -c` child.
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup is cmd.Cancel: on a timeout, kill everything the command
// started, not only the shell (which for `foo &` has usually exited already).
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil || pgid == 0 {
		return cmd.Process.Kill()
	}
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

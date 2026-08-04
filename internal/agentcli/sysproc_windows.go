// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package agentcli

import (
	"os/exec"
	"syscall"
)

// newSysProcAttr: no process-group semantics on Windows via syscall (same
// limitation internal/llama's process_windows.go already accepts) — killing
// just the direct process is the same behavior this had before LK-1.
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func killProcessGroup(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}

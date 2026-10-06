//go:build windows

// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"os/exec"
	"strconv"
	"syscall"
)

// createNoWindow keeps a console window from flashing up for a background
// helper (CREATE_NO_WINDOW).
const createNoWindow = 0x08000000

func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// killTree ends the process and its children — Windows has no process groups,
// so taskkill /T walks the tree.
func killTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	hide := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
	hide.SysProcAttr = sysProcAttr()
	_ = hide.Run()
}

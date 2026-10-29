// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package llama

import (
	"os/exec"
	"syscall"
)

const sigKill = int(syscall.SIGKILL)

// exitSignal is the signal that ended the process, 0 if it exited normally.
func exitSignal(ee *exec.ExitError) int {
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return int(ws.Signal())
	}
	return 0
}

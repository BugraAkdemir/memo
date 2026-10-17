// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package updater

import (
	"errors"
	"syscall"
)

// pidAlive reports whether a process with this pid exists. EPERM means it
// exists but belongs to someone else — still alive for our purposes.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

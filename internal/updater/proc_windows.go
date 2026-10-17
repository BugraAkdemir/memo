// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package updater

// On Windows an update is the Setup.exe hand-off, not an in-place swap, so
// nothing here waits on a pid. The stub keeps the package building.
func pidAlive(pid int) bool { return false }

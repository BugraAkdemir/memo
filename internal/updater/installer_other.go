// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package updater

import "errors"

// RunInstaller is Windows-only: elsewhere an update is the in-place swap.
func RunInstaller(path string) error {
	return errors.New("updater: Setup.exe installers are Windows-only")
}

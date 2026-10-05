// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package tools

import "fmt"

// startDetached is only reached on Linux (see OpenApp); Windows launches
// through `cmd /C start`, which needs none of this.
func startDetached(argv []string) error {
	return fmt.Errorf("startDetached is not used on windows")
}

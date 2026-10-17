// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package updater

import (
	"errors"
	"io"
)

// StartDetached is not used on Windows (an update there is the Setup.exe
// hand-off); the stub keeps the package building.
func StartDetached(script, dir string, env []string, out io.Writer) (*Launched, error) {
	return nil, errors.New("updater: in-place relaunch is not supported on windows")
}

// StartDetachedArgs is not used on Windows either.
func StartDetachedArgs(script string, args []string, dir string, env []string, out io.Writer) (*Launched, error) {
	return nil, errors.New("updater: in-place relaunch is not supported on windows")
}

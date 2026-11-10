//go:build !windows

// SPDX-License-Identifier: AGPL-3.0-or-later

package procjob

import "os"

// Bind does nothing outside Windows (see the package doc).
func Bind(*os.Process, string) {}

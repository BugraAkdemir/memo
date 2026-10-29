// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package llama

import "os/exec"

// Windows has no signals; every death is an exit code.
const sigKill = -1

func exitSignal(*exec.ExitError) int { return 0 }

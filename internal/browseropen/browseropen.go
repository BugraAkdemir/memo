// SPDX-License-Identifier: AGPL-3.0-or-later

// Package browseropen opens a URL in the user's default browser. Each
// platform has exactly one reliable way to do this and no Go stdlib support,
// so this is the one shared place that shells out for it — used both by the
// CLI's --github/--bugreport/--docs flags and by the Tailscale interactive
// login flow (internal/tunnel).
package browseropen

import (
	"os/exec"
	"runtime"
)

// OpenURL opens u in the user's default browser.
func OpenURL(u string) error {
	return browserCommand(runtime.GOOS, u).Start()
}

// browserCommand builds (without starting) the platform-specific command
// that opens u — split out from OpenURL so the per-OS branch selection is
// testable without actually spawning a browser process for whichever OS
// happens to be running go test.
func browserCommand(goos, u string) *exec.Cmd {
	switch goos {
	case "windows":
		// rundll32 rather than `start`: `start` is a cmd.exe builtin, not an
		// executable, so it can't be exec'd directly.
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		return exec.Command("open", u)
	default:
		return exec.Command("xdg-open", u)
	}
}

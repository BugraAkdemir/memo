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
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// rundll32 rather than `start`: `start` is a cmd.exe builtin, not an
		// executable, so it can't be exec'd directly.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	return cmd.Start()
}

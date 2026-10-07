// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"os"
	"path/filepath"
	"runtime"

	"memo/internal/browseropen"
	"memo/internal/logx"
)

// openURL opens the sign-in page in the user's browser. A var so tests do not
// launch a real browser.
var openURL = browseropen.OpenURL

// The sidecar decides whether it can open a browser by RUNNING
// `xdg-open about:blank` and waiting for it, and only then prints the sign-in
// URL. about: has no handler on a typical desktop: under KDE `kde-open` sits on
// an error dialog forever, so the sidecar printed "Opening browser…" and nothing
// else, Memo never got the URL, and the UI said "waiting for the browser" with
// nothing to click (the same trap internal/agent/tools/openapp.go documents).
//
// So on Linux the login process gets an `xdg-open` that returns at once and
// opens nothing; Memo opens the URL itself (browseropen) once it has read it
// from the output. Other platforms' launchers (`open`, rundll32) return
// promptly and are left alone.
const xdgOpenShim = "#!/bin/sh\n# Memo: the sign-in page is opened by Memo itself (see internal/cliproxy/browser.go).\nexit 0\n"

// loginEnv returns the environment for a login process: the current one, with
// (on Linux) a directory holding the no-op xdg-open in front of PATH.
func (m *Manager) loginEnv() []string {
	env := os.Environ()
	if runtime.GOOS != "linux" {
		return env
	}
	dir := filepath.Join(m.dir, "bin")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return env
	}
	shim := filepath.Join(dir, "xdg-open")
	if b, err := os.ReadFile(shim); err != nil || string(b) != xdgOpenShim {
		if err := os.WriteFile(shim, []byte(xdgOpenShim), 0o700); err != nil {
			return env
		}
	}
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if len(kv) >= 5 && kv[:5] == "PATH=" {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// openLoginPage opens u in the browser where the sidecar could not be trusted
// to (see above). Failure is not an error: the URL is also shown for copying.
func openLoginPage(u string) {
	if runtime.GOOS != "linux" {
		return
	}
	open := openURL // read here, not in the goroutine: tests swap the var
	go func() {
		if err := open(u); err != nil {
			logx.Printf("cliproxy: could not open the browser for the sign-in page: %v", err)
		}
	}()
}

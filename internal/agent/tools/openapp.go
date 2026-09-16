// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"memo/internal/browseropen"
)

// openAppTimeout bounds the macOS/Windows launcher commands (`open`,
// `start`) — both hand off to the target app and exit almost immediately, so
// a short bound is enough to catch a genuine "app not found" failure
// without making the tool call feel slow.
const openAppTimeout = 10 * time.Second

// OpenAppArgs represents arguments for the open_app tool.
type OpenAppArgs struct {
	AppName string `json:"app_name"`
}

// browserAliases are the names that mean "the default browser, no specific
// site" rather than a literal application to launch — resolved by opening a
// blank tab (browseropen.OpenURL) instead of trying to exec a program
// literally named "browser"/"tarayıcı", which does not exist on any
// platform.
var browserAliases = map[string]bool{
	"browser":             true,
	"web browser":         true,
	"tarayıcı":            true,
	"internet tarayıcı":   true,
	"web tarayıcı":        true,
	"varsayılan tarayıcı": true,
}

// isBrowserAlias reports whether name is one of browserAliases, split out as
// its own pure function so tests can check the matching logic without
// calling OpenApp (which, for a real browser alias, actually launches a
// browser process).
func isBrowserAlias(name string) bool {
	return browserAliases[strings.ToLower(strings.TrimSpace(name))]
}

// buildAppCommand returns (without starting) the platform-specific
// executable name and arguments that launch an application literally named
// appName. Split out from OpenApp, same as browseropen's browserCommand, so
// the per-OS argv selection is testable without actually spawning a
// process.
func buildAppCommand(goos, appName string) (name string, args []string) {
	switch goos {
	case "windows":
		// `start` is a cmd.exe builtin (see browseropen.go), not an
		// executable — invoke it through cmd /C. The empty "" argument is
		// the window-title placeholder `start` expects before the target
		// when the target itself may contain spaces.
		return "cmd", []string{"/C", "start", "", appName}
	case "darwin":
		return "open", []string{"-a", appName}
	default:
		// Linux has no universal "launch by display name" mechanism the way
		// macOS/Windows do. Most packaged apps register their own binary
		// under their common name (spotify, steam, discord, code, ...), so
		// treat appName as that binary directly — exactly what a user would
		// type in a terminal to launch it themselves.
		return appName, nil
	}
}

// OpenApp launches a desktop application (or the default browser) by name.
// It is best-effort: on Linux the target IS the actual application process,
// started detached and unwaited (GUI apps run indefinitely, so waiting
// would hang the tool call); on macOS/Windows the target is a short-lived
// launcher command whose exit status is a real, synchronous signal, so it
// is run to completion under openAppTimeout instead.
func OpenApp(ctx context.Context, argsJSON json.RawMessage, _ string, _ func(string) error) (string, error) {
	var args OpenAppArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	appName := strings.TrimSpace(args.AppName)
	if appName == "" {
		return "", fmt.Errorf("app_name is required")
	}

	if isBrowserAlias(appName) {
		if err := browseropen.OpenURL("about:blank"); err != nil {
			return "", fmt.Errorf(T("tarayıcı açılamadı: %w", "failed to open the browser: %w"), err)
		}
		return T("Varsayılan tarayıcı açıldı.", "Default browser opened."), nil
	}

	name, cmdArgs := buildAppCommand(runtime.GOOS, appName)

	if runtime.GOOS == "linux" {
		cmd := exec.Command(name, cmdArgs...)
		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf(T("%q uygulaması başlatılamadı: %w", "failed to start %q: %w"), appName, err)
		}
		return fmt.Sprintf(T("%q başlatıldı.", "%q started."), appName), nil
	}

	execCtx, cancel := context.WithTimeout(ctx, openAppTimeout)
	defer cancel()
	cmd := exec.CommandContext(execCtx, name, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf(T("%q uygulamasını açma zaman aşımına uğradı", "opening %q timed out"), appName)
		}
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf(T("%q uygulaması bulunamadı ya da açılamadı: %s", "%q not found or failed to open: %s"), appName, msg)
	}
	return fmt.Sprintf(T("%q başlatıldı.", "%q started."), appName), nil
}

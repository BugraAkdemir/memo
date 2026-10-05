// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"context"
	"encoding/json"
	"errors"
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
	"default browser":     true,
	"tarayıcı":            true,
	"tarayıcıyı":          true,
	"internet tarayıcı":   true,
	"internet tarayıcısı": true,
	"web tarayıcı":        true,
	"web tarayıcısı":      true,
	"varsayılan tarayıcı": true,
}

// isBrowserAlias reports whether name is one of browserAliases, split out as
// its own pure function so tests can check the matching logic without
// calling OpenApp (which, for a real browser alias, actually launches a
// browser process).
func isBrowserAlias(name string) bool {
	return browserAliases[strings.ToLower(strings.TrimSpace(name))]
}

// buildAppCommand returns (without starting) the launcher command for the
// two platforms that have a real "open an app by its display name" call.
// Split out from OpenApp, same as browseropen's browserCommand, so the
// per-OS argv selection is testable without spawning a process. Every other
// platform goes through openAppDesktopEntry instead — see applaunch.go for
// why exec'ing the name directly was wrong there.
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
	}
	return "", nil
}

// OpenApp launches a desktop application (or the default browser) by name.
// On macOS/Windows the target is a short-lived launcher command whose exit
// status is a real, synchronous signal, so it runs to completion under
// openAppTimeout. Everywhere else the app is found in the desktop's own
// application registry (applaunch.go) and started detached, with an early
// failure reported rather than a blind "started".
func OpenApp(ctx context.Context, argsJSON json.RawMessage, _ string, _ func(string) error) (string, error) {
	var args OpenAppArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	appName := strings.TrimSpace(args.AppName)
	if appName == "" {
		return "", fmt.Errorf("app_name is required")
	}

	desktopRegistry := runtime.GOOS != "windows" && runtime.GOOS != "darwin"

	if isBrowserAlias(appName) {
		if desktopRegistry {
			return openDefaultBrowserDesktop()
		}
		if err := browseropen.OpenURL("about:blank"); err != nil {
			return "", fmt.Errorf(T("tarayıcı açılamadı: %w", "failed to open the browser: %w"), err)
		}
		return T("Varsayılan tarayıcı açıldı.", "Default browser opened."), nil
	}

	if desktopRegistry {
		return openAppDesktopEntry(appName)
	}

	name, cmdArgs := buildAppCommand(runtime.GOOS, appName)
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

// openAppDesktopEntry finds appName in the desktop's application registry
// and starts it. When no entry matches, a binary of that name (as typed,
// lowercased, or with spaces removed) on PATH is the fallback, which covers
// command-line-installed GUI apps that ship no .desktop file.
func openAppDesktopEntry(appName string) (string, error) {
	if e, ok := findDesktopEntry(appName, loadDesktopEntries(desktopEntryDirs())); ok {
		label := e.ID
		if len(e.Names) > 0 {
			label = e.Names[0]
		}
		if err := startDetached(execCommandLine(e)); err != nil {
			return "", fmt.Errorf(T("%q (%s) açılamadı: %w", "%q (%s) failed to start: %w"), label, e.ID, err)
		}
		return fmt.Sprintf(T("%s başlatıldı.", "%s started."), label), nil
	}
	lower := strings.ToLower(appName)
	for _, cand := range []string{appName, lower, strings.ReplaceAll(lower, " ", ""), strings.ReplaceAll(lower, " ", "-")} {
		if _, err := exec.LookPath(cand); err != nil {
			continue
		}
		if err := startDetached([]string{cand}); err != nil {
			return "", fmt.Errorf(T("%q açılamadı: %w", "%q failed to start: %w"), cand, err)
		}
		return fmt.Sprintf(T("%q başlatıldı.", "%q started."), cand), nil
	}
	return "", fmt.Errorf(T(
		"%q adında kurulu bir uygulama bulunamadı. Kullanıcıya uygulamanın adını sor ya da kurulu olup olmadığını kontrol etmesini söyle.",
		"no installed application named %q was found. Ask the user for the app's exact name or whether it is installed.",
	), appName)
}

// openDefaultBrowserDesktop starts the desktop's default web browser. It
// used to run `xdg-open about:blank`, but about: has no registered handler
// on a typical desktop, so that failed (exit 4, or hung on an error dialog
// under KDE) while the tool — which never waited for it — reported success.
// Starting the default browser's own entry with no URL opens its start page,
// which is what "open the browser" means.
func openDefaultBrowserDesktop() (string, error) {
	entries := loadDesktopEntries(desktopEntryDirs())
	if id := defaultBrowserID(); id != "" {
		if e, ok := findDesktopEntryByID(id, entries); ok {
			if err := startDetached(execCommandLine(e)); err != nil {
				return "", fmt.Errorf(T("tarayıcı açılamadı: %w", "failed to open the browser: %w"), err)
			}
			return T("Varsayılan tarayıcı açıldı.", "Default browser opened."), nil
		}
	}
	for _, b := range fallbackBrowsers {
		if _, err := exec.LookPath(b); err != nil {
			continue
		}
		if err := startDetached([]string{b}); err != nil {
			return "", fmt.Errorf(T("tarayıcı açılamadı: %w", "failed to open the browser: %w"), err)
		}
		return T("Tarayıcı açıldı.", "Browser opened."), nil
	}
	return "", errors.New(T("bu bilgisayarda bir web tarayıcısı bulunamadı", "no web browser was found on this computer"))
}

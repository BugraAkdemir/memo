// SPDX-License-Identifier: AGPL-3.0-or-later

package webchat

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"memo/internal/browserengine"
)

// The sign-in window must be a real, windowed browser. Memo's own resolver
// (browserengine.ResolveExecutable) is built for headless page fetching: it can
// return chrome-headless-shell, and the one-click engine in Settings downloads
// exactly that — a build with no window at all, so there would be nothing to
// sign in to. This file finds a browser the person can see and use instead.
//
// Only Chromium-family browsers can be used: Memo drives the page over the
// Chrome DevTools Protocol, which Firefox and Safari do not offer (Firefox
// dropped it; its replacement needs a separate driver program). A default
// browser that is not Chromium-based is therefore skipped, not an error.
//
// The browser always runs with Memo's own profile folder, never the person's
// everyday profile: current Chrome refuses a debugging port on its default
// profile, and mixing the two would put their real tabs and cookies in reach.
// The price is one separate sign-in, kept in data/webchat/<site>.

// browserFamilies lists, per browser, the executable names to look for.
// Order = preference when the default browser gives no hint.
var browserFamilies = []struct {
	key   string   // matched against the default-browser entry
	names []string // PATH names
}{
	{"chrome", []string{"google-chrome-stable", "google-chrome"}},
	{"chromium", []string{"chromium", "chromium-browser"}},
	{"brave", []string{"brave-browser", "brave", "brave-browser-stable"}},
	{"edge", []string{"microsoft-edge", "microsoft-edge-stable"}},
	{"vivaldi", []string{"vivaldi", "vivaldi-stable"}},
	{"opera", []string{"opera"}},
}

// absolute install locations, probed after PATH names.
var platformPaths = func() []string {
	switch runtime.GOOS {
	case "windows":
		pf, pf86, local := os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")
		var out []string
		for _, root := range []string{pf, pf86, local} {
			if root == "" {
				continue
			}
			out = append(out,
				filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(root, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
				filepath.Join(root, "Vivaldi", "Application", "vivaldi.exe"),
				filepath.Join(root, "Chromium", "Application", "chrome.exe"),
			)
		}
		return out
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Vivaldi.app/Contents/MacOS/Vivaldi",
			"/Applications/Opera.app/Contents/MacOS/Opera",
		}
	default:
		return []string{
			"/usr/bin/google-chrome-stable", "/usr/bin/google-chrome", "/opt/google/chrome/chrome",
			"/usr/bin/chromium", "/usr/bin/chromium-browser", "/snap/bin/chromium",
			"/usr/bin/brave-browser", "/usr/bin/brave", "/opt/brave.com/brave/brave",
			"/usr/bin/microsoft-edge", "/usr/bin/microsoft-edge-stable",
			"/usr/bin/vivaldi", "/usr/bin/vivaldi-stable", "/usr/bin/opera",
		}
	}
}

// familyOfDefault maps a default-browser identifier (a .desktop file name on
// Linux, e.g. "brave-browser.desktop") to a family key, or "" when it is not
// a Chromium-family browser we can drive (Firefox, Safari, a flatpak we do not
// know...).
func familyOfDefault(entry string) string {
	e := strings.ToLower(entry)
	switch {
	case strings.Contains(e, "brave"):
		return "brave"
	case strings.Contains(e, "vivaldi"):
		return "vivaldi"
	case strings.Contains(e, "opera"):
		return "opera"
	case strings.Contains(e, "edge"):
		return "edge"
	case strings.Contains(e, "chromium"):
		return "chromium"
	case strings.Contains(e, "chrome"):
		return "chrome"
	}
	return ""
}

// defaultBrowserEntry asks the OS for the default web browser's identifier.
// Linux only; elsewhere it answers "" and the fixed order applies.
var defaultBrowserEntry = func(ctx context.Context) string {
	if runtime.GOOS != "linux" {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "xdg-settings", "get", "default-web-browser").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// isHeadlessOnly reports a binary that cannot show a window.
func isHeadlessOnly(path string) bool {
	return strings.Contains(strings.ToLower(filepath.Base(path)), "headless")
}

// findBrowser returns the executable to open for sign-in, in this order: the
// MEMO_WEBCHAT_BROWSER override, the default browser when it is a
// Chromium-family one, then the first installed browser from the fixed list.
// It never returns a headless-only build.
func findBrowser(ctx context.Context) (path, name string, err error) {
	if p := strings.TrimSpace(os.Getenv("MEMO_WEBCHAT_BROWSER")); p != "" {
		if lp, e := exec.LookPath(p); e == nil && !isHeadlessOnly(lp) {
			return lp, filepath.Base(lp), nil
		}
		// An override that does not work is reported, not silently replaced.
		return "", "", newErr(KindNotInstalled, "", browserengine.ErrBrowserNotInstalled)
	}

	probe := func(family string) (string, bool) {
		for _, f := range browserFamilies {
			if f.key != family {
				continue
			}
			for _, n := range f.names {
				if p, e := exec.LookPath(n); e == nil && !isHeadlessOnly(p) {
					return p, true
				}
			}
		}
		return "", false
	}
	if fam := familyOfDefault(defaultBrowserEntry(ctx)); fam != "" {
		if p, ok := probe(fam); ok {
			return p, fam, nil
		}
	}
	for _, f := range browserFamilies {
		if p, ok := probe(f.key); ok {
			return p, f.key, nil
		}
	}
	for _, p := range platformPaths() {
		if st, e := os.Stat(p); e == nil && !st.IsDir() && !isHeadlessOnly(p) {
			return p, filepath.Base(p), nil
		}
	}
	return "", "", newErr(KindNotInstalled, "", browserengine.ErrBrowserNotInstalled)
}

// resolveBrowser is the lookup Service uses; a var-typed seam like the rest.
func resolveBrowser(ctx context.Context) (string, error) {
	p, _, err := findBrowser(ctx)
	return p, err
}

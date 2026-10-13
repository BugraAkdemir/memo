// SPDX-License-Identifier: AGPL-3.0-or-later

package webchat

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFamilyOfDefault(t *testing.T) {
	cases := map[string]string{
		"brave-browser.desktop":         "brave",
		"com.brave.Browser.desktop":     "brave",
		"google-chrome.desktop":         "chrome",
		"chromium.desktop":              "chromium",
		"org.chromium.Chromium.desktop": "chromium",
		"microsoft-edge.desktop":        "edge",
		"vivaldi-stable.desktop":        "vivaldi",
		"opera.desktop":                 "opera",
		"firefox.desktop":               "", // cannot be driven over DevTools
		"org.mozilla.firefox.desktop":   "",
		"":                              "",
	}
	for in, want := range cases {
		if got := familyOfDefault(in); got != want {
			t.Errorf("familyOfDefault(%q) = %q, want %q", in, got, want)
		}
	}
}

// fakeBin makes an executable file named name in dir.
func fakeBin(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func withBrowserEnv(t *testing.T, dir, defaultEntry string) {
	t.Helper()
	t.Setenv("PATH", dir)
	t.Setenv("MEMO_WEBCHAT_BROWSER", "")
	oldDef, oldPaths := defaultBrowserEntry, platformPaths
	defaultBrowserEntry = func(context.Context) string { return defaultEntry }
	platformPaths = func() []string { return nil }
	t.Cleanup(func() { defaultBrowserEntry, platformPaths = oldDef, oldPaths })
}

func TestFindBrowser_PrefersAChromiumFamilyDefault(t *testing.T) {
	dir := t.TempDir()
	chrome := fakeBin(t, dir, "google-chrome")
	brave := fakeBin(t, dir, "brave-browser")
	withBrowserEnv(t, dir, "brave-browser.desktop")
	got, _, err := findBrowser(context.Background())
	if err != nil || got != brave {
		t.Fatalf("got %q, %v; want the default browser %q (not %q)", got, err, brave, chrome)
	}
}

func TestFindBrowser_FirefoxDefaultFallsBackToAnInstalledChromiumBrowser(t *testing.T) {
	dir := t.TempDir()
	vivaldi := fakeBin(t, dir, "vivaldi")
	fakeBin(t, dir, "firefox")
	withBrowserEnv(t, dir, "firefox.desktop")
	got, _, err := findBrowser(context.Background())
	if err != nil || got != vivaldi {
		t.Fatalf("got %q, %v; want %q", got, err, vivaldi)
	}
}

func TestFindBrowser_NeverReturnsAHeadlessOnlyBuild(t *testing.T) {
	dir := t.TempDir()
	fakeBin(t, dir, "chrome-headless-shell")
	withBrowserEnv(t, dir, "")
	_, _, err := findBrowser(context.Background())
	if KindOf(err) != KindNotInstalled {
		t.Fatalf("err = %v, want not-installed: a headless shell has no window to sign in to", err)
	}
}

func TestFindBrowser_OverrideWinsAndABadOverrideIsReported(t *testing.T) {
	dir := t.TempDir()
	fakeBin(t, dir, "google-chrome")
	custom := fakeBin(t, dir, "my-browser")
	withBrowserEnv(t, dir, "")
	t.Setenv("MEMO_WEBCHAT_BROWSER", custom)
	if got, _, err := findBrowser(context.Background()); err != nil || got != custom {
		t.Fatalf("override: got %q, %v", got, err)
	}
	t.Setenv("MEMO_WEBCHAT_BROWSER", filepath.Join(dir, "does-not-exist"))
	if _, _, err := findBrowser(context.Background()); KindOf(err) != KindNotInstalled {
		t.Fatalf("a broken override must be reported, not silently replaced: %v", err)
	}
}

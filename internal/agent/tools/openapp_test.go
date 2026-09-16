// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

// TestBuildAppCommand_PerPlatform guards the one thing that actually
// matters in this file: picking the right launcher binary and argument
// shape per OS. Actual execution isn't exercised here — see
// TestOpenApp_UnknownAppOnLinux for why only the Linux path can be run for
// real in this suite.
func TestBuildAppCommand_PerPlatform(t *testing.T) {
	cases := []struct {
		goos     string
		wantName string
		wantArgs []string
	}{
		{"windows", "cmd", []string{"/C", "start", "", "Spotify"}},
		{"darwin", "open", []string{"-a", "Spotify"}},
		{"linux", "Spotify", nil},
		{"freebsd", "Spotify", nil}, // the default branch, any unlisted GOOS
	}
	for _, c := range cases {
		t.Run(c.goos, func(t *testing.T) {
			name, args := buildAppCommand(c.goos, "Spotify")
			if name != c.wantName {
				t.Errorf("goos=%s: got name %q, want %q", c.goos, name, c.wantName)
			}
			if !equalArgs(args, c.wantArgs) {
				t.Errorf("goos=%s: got args %v, want %v", c.goos, args, c.wantArgs)
			}
		})
	}
}

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestIsBrowserAlias covers the disambiguation surface this tool exists
// for: "open the browser" must resolve to the blank-tab special case, while
// a real application name (including one that merely contains "browser")
// must fall through to buildAppCommand instead.
func TestIsBrowserAlias(t *testing.T) {
	trueCases := []string{"browser", "Browser", "  browser  ", "tarayıcı", "Tarayıcı", "web tarayıcı", "varsayılan tarayıcı", "internet tarayıcı", "web browser"}
	for _, name := range trueCases {
		if !isBrowserAlias(name) {
			t.Errorf("isBrowserAlias(%q) = false, want true", name)
		}
	}

	falseCases := []string{"spotify", "steam", "brave browser", "Google Chrome", ""}
	for _, name := range falseCases {
		if isBrowserAlias(name) {
			t.Errorf("isBrowserAlias(%q) = true, want false", name)
		}
	}
}

func TestOpenApp_EmptyAppName(t *testing.T) {
	_, err := OpenApp(context.Background(), json.RawMessage(`{"app_name":"  "}`), "", nil)
	if err == nil {
		t.Fatal("expected error for empty app_name, got nil")
	}
	if !strings.Contains(err.Error(), "app_name") {
		t.Errorf("error %q does not mention app_name", err.Error())
	}
}

func TestOpenApp_InvalidJSON(t *testing.T) {
	_, err := OpenApp(context.Background(), json.RawMessage(`{not valid json`), "", nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// TestOpenApp_UnknownAppOnLinux is the only branch of OpenApp's real
// execution this suite can safely exercise: on Linux, a nonexistent binary
// name fails synchronously in cmd.Start() (no process is ever spawned),
// exactly like typing a typo'd command at a shell prompt. The macOS/Windows
// branches shell out to real system launcher binaries (open/cmd) that
// aren't present (or wouldn't behave identically) on this suite's runners,
// so they're covered only by TestBuildAppCommand_PerPlatform's argv check.
func TestOpenApp_UnknownAppOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("exercises the Linux exec.Command(appName).Start() path only")
	}
	argsJSON := json.RawMessage(`{"app_name":"definitely-not-a-real-app-xyz123"}`)
	_, err := OpenApp(context.Background(), argsJSON, "", nil)
	if err == nil {
		t.Fatal("expected error for a nonexistent binary, got nil")
	}
}

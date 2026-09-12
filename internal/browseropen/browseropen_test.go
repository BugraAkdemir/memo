// SPDX-License-Identifier: AGPL-3.0-or-later

package browseropen

import "testing"

// TestBrowserCommand_PerPlatform guards the one thing that actually matters
// in this package: picking the right opener binary and argument shape per
// OS. OpenURL itself isn't exercised directly — it calls cmd.Start(), which
// would genuinely try to launch a browser process on whatever machine runs
// go test.
func TestBrowserCommand_PerPlatform(t *testing.T) {
	cases := []struct {
		goos     string
		wantPath string
		wantArgs []string
	}{
		{"windows", "rundll32", []string{"rundll32", "url.dll,FileProtocolHandler", "https://example.com"}},
		{"darwin", "open", []string{"open", "https://example.com"}},
		{"linux", "xdg-open", []string{"xdg-open", "https://example.com"}},
		{"freebsd", "xdg-open", []string{"xdg-open", "https://example.com"}}, // the default branch, any unlisted GOOS
	}
	for _, c := range cases {
		t.Run(c.goos, func(t *testing.T) {
			cmd := browserCommand(c.goos, "https://example.com")
			if got := cmd.Args; !equalArgs(got, c.wantArgs) {
				t.Errorf("goos=%s: got Args %v, want %v", c.goos, got, c.wantArgs)
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

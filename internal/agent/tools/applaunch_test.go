// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeDesktop(t *testing.T, dir, id, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".desktop"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A desktop that looks like a real one: the names the model is told to pass
// ("Spotify", "VS Code", "Steam") are display names, and none of them is the
// binary's name. Every one of these used to fail with "executable file not
// found in $PATH".
func testEntries(t *testing.T) []desktopEntry {
	t.Helper()
	sys := filepath.Join(t.TempDir(), "applications")
	user := filepath.Join(t.TempDir(), "applications")
	writeDesktop(t, sys, "spotify", "[Desktop Entry]\nType=Application\nName=Spotify\nExec=spotify --uri=%u\n")
	writeDesktop(t, sys, "code", "[Desktop Entry]\nType=Application\nName=Visual Studio Code\nKeywords=vscode;\nExec=/usr/share/code/code %F\n")
	writeDesktop(t, sys, "code-url-handler", "[Desktop Entry]\nType=Application\nName=Visual Studio Code - URL Handler\nNoDisplay=true\nExec=/usr/share/code/code --open-url %U\n")
	writeDesktop(t, sys, "steam", "[Desktop Entry]\nType=Application\nName=Steam\nExec=/usr/bin/steam %U\n")
	writeDesktop(t, sys, "org.kde.kcalc", "[Desktop Entry]\nType=Application\nName=KCalc\nName[tr]=K Hesap\nGenericName=Scientific Calculator\nGenericName[tr]=Bilimsel Hesap Makinesi\nExec=kcalc\n")
	writeDesktop(t, sys, "com.discordapp.Discord", "[Desktop Entry]\nType=Application\nName=Discord\nExec=/usr/bin/flatpak run --branch=stable com.discordapp.Discord\n")
	writeDesktop(t, sys, "firefox", "[Desktop Entry]\nType=Application\nName=Firefox\nExec=/usr/lib/firefox/firefox %u\n[Desktop Action new-window]\nName=New Window\nExec=/usr/lib/firefox/firefox --new-window %u\n")
	writeDesktop(t, sys, "hidden-app", "[Desktop Entry]\nType=Application\nName=Hidden\nHidden=true\nExec=hidden\n")
	writeDesktop(t, sys, "a-link", "[Desktop Entry]\nType=Link\nName=Spotify Website\nURL=https://spotify.com\n")
	// The user's own entry shadows the system one with the same id.
	writeDesktop(t, sys, "steam-shadowed", "[Desktop Entry]\nType=Application\nName=Old\nExec=old\n")
	writeDesktop(t, user, "steam-shadowed", "[Desktop Entry]\nType=Application\nName=New\nExec=new\n")
	return loadDesktopEntries([]string{user, sys})
}

func TestFindDesktopEntry_DisplayNamesResolve(t *testing.T) {
	entries := testEntries(t)
	cases := map[string]string{
		"Spotify":            "spotify",
		"spotify":            "spotify",
		"VS Code":            "code", // keyword "vscode", and never the NoDisplay URL handler
		"Visual Studio Code": "code",
		"Steam":              "steam",
		"Discord":            "com.discordapp.Discord", // a Flatpak: not on PATH at all
		"Hesap Makinesi":     "org.kde.kcalc",          // Turkish GenericName, partial
		"K Hesap":            "org.kde.kcalc",          // Turkish Name
		"kcalc":              "org.kde.kcalc",
		"Firefox":            "firefox",
	}
	for name, want := range cases {
		e, ok := findDesktopEntry(name, entries)
		if !ok {
			t.Errorf("%q: no entry found, want %s", name, want)
			continue
		}
		if e.ID != want {
			t.Errorf("%q: got %s, want %s", name, e.ID, want)
		}
	}
	for _, name := range []string{"Hidden", "Spotify Website", "nonexistent app", ""} {
		if e, ok := findDesktopEntry(name, entries); ok && (name == "Hidden" || name == "nonexistent app" || name == "") {
			t.Errorf("%q matched %s; Hidden entries, non-applications and unknown names must not", name, e.ID)
		}
	}
	if e, ok := findDesktopEntryByID("steam-shadowed.desktop", entries); !ok || e.Exec != "new" {
		t.Errorf("the user's own entry did not shadow the system one: %+v", e)
	}
}

// A helper entry (NoDisplay=true) that happens to carry the exact name must
// not beat the visible app: launching "Foo" should open the editor the user
// sees in their menu, not its background helper.
func TestFindDesktopEntry_NoDisplayHelperNeverWins(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "applications")
	writeDesktop(t, dir, "foo", "[Desktop Entry]\nType=Application\nName=Foo Editor\nExec=foo\n")
	writeDesktop(t, dir, "foo-helper", "[Desktop Entry]\nType=Application\nName=Foo\nNoDisplay=true\nExec=foo-helper\n")
	e, ok := findDesktopEntry("Foo", loadDesktopEntries([]string{dir}))
	if !ok || e.ID != "foo" {
		t.Errorf("got %+v, want the visible entry \"foo\"", e)
	}
}

func TestExecCommandLine_DropsTargetFieldCodes(t *testing.T) {
	cases := []struct {
		exec string
		want []string
	}{
		{"spotify --uri=%u", []string{"spotify"}},
		{"/usr/lib/firefox/firefox %u", []string{"/usr/lib/firefox/firefox"}},
		{`"/opt/My App/app" --flag %F`, []string{"/opt/My App/app", "--flag"}},
		{"app --name %c --x 100%%", []string{"app", "--name", "Thing", "--x", "100%"}},
		{"env FOO=1 /usr/bin/app %U", []string{"env", "FOO=1", "/usr/bin/app"}},
	}
	for _, c := range cases {
		got := execCommandLine(desktopEntry{Exec: c.exec, Names: []string{"Thing"}})
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Exec=%q: got %q, want %q", c.exec, got, c.want)
		}
	}
	if b := execBinary("env BAMF=1 /opt/spotify/spotify %U"); b != "spotify" {
		t.Errorf("execBinary through env = %q", b)
	}
	if b := execBinary("/usr/bin/flatpak run --branch=stable com.spotify.Client"); b != "Client" {
		t.Errorf("execBinary through flatpak run = %q", b)
	}
}

func TestStartDetached_ReportsAnImmediateFailure(t *testing.T) {
	err := startDetached([]string{"sh", "-c", "echo 'cannot open display' >&2; exit 3"})
	if err == nil || !strings.Contains(err.Error(), "cannot open display") {
		t.Fatalf("err = %v, want the app's own stderr", err)
	}
	if err := startDetached([]string{"sh", "-c", "exit 0"}); err != nil {
		t.Errorf("a launcher that hands off and exits 0 is success, got %v", err)
	}
	old := launchSettle
	launchSettle = 200 * 1e6
	defer func() { launchSettle = old }()
	if err := startDetached([]string{"sleep", "2"}); err != nil {
		t.Errorf("a still-running app is success, got %v", err)
	}
	if err := startDetached([]string{"definitely-not-a-binary-xyz"}); err == nil {
		t.Error("a missing binary reported success")
	}
}

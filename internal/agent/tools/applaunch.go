// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// Linux application lookup for open_app.
//
// Linux has no "launch by display name" call the way macOS (`open -a`) and
// Windows (`start`) do, and treating the name the model extracted as a
// binary name failed for every realistic input: the tool's own description
// tells the model to pass "Spotify", "Steam", "VS Code", while the binaries
// are spotify, steam, code — and Flatpak/Snap apps are not on PATH at all.
// Verified live: all four of those names failed with "executable file not
// found in $PATH" on a machine with every one of them installed.
//
// What every Linux desktop actually uses for this is the freedesktop.org
// Desktop Entry registry (.desktop files under $XDG_DATA_HOME and
// $XDG_DATA_DIRS, which is also where Flatpak and Snap export theirs). This
// file finds the entry the user means — by its displayed name, translated
// name, keywords, desktop-file id or Exec binary — and returns the command
// line its Exec key declares.

// desktopEntry is the subset of a .desktop file open_app needs.
type desktopEntry struct {
	ID        string   // file name without .desktop, e.g. "spotify", "com.spotify.Client"
	Path      string   // full path of the .desktop file
	Names     []string // Name plus every Name[locale]
	Generic   []string // GenericName plus every GenericName[locale]
	Keywords  []string // Keywords plus every Keywords[locale], split
	Exec      string
	NoDisplay bool
}

// desktopEntryDirs returns the application directories to search, most
// specific first — the user's own entries shadow system ones with the same
// id, exactly as the desktop does. Flatpak and Snap export directories are
// added explicitly because a backend started outside a desktop session (a
// systemd user service, ssh) may not have them in XDG_DATA_DIRS.
func desktopEntryDirs() []string {
	var dataDirs []string
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		dataDirs = append(dataDirs, v)
	} else if home, err := os.UserHomeDir(); err == nil {
		dataDirs = append(dataDirs, filepath.Join(home, ".local", "share"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dataDirs = append(dataDirs, filepath.Join(home, ".local", "share", "flatpak", "exports", "share"))
	}
	if v := os.Getenv("XDG_DATA_DIRS"); v != "" {
		dataDirs = append(dataDirs, filepath.SplitList(v)...)
	} else {
		dataDirs = append(dataDirs, "/usr/local/share", "/usr/share")
	}
	dataDirs = append(dataDirs, "/var/lib/flatpak/exports/share")

	seen := map[string]bool{}
	var out []string
	for _, d := range dataDirs {
		app := filepath.Join(d, "applications")
		if !seen[app] {
			seen[app] = true
			out = append(out, app)
		}
	}
	if !seen["/var/lib/snapd/desktop/applications"] {
		out = append(out, "/var/lib/snapd/desktop/applications")
	}
	return out
}

// parseDesktopEntry reads the [Desktop Entry] group of a .desktop file.
// Returns ok=false for anything that is not a launchable application:
// another Type, Hidden=true, or no Exec.
func parseDesktopEntry(path string) (desktopEntry, bool) {
	f, err := os.Open(path)
	if err != nil {
		return desktopEntry{}, false
	}
	defer f.Close()

	e := desktopEntry{ID: strings.TrimSuffix(filepath.Base(path), ".desktop"), Path: path}
	inMain := false
	typ := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inMain = line == "[Desktop Entry]"
			continue
		}
		if !inMain {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		base, _, _ := strings.Cut(key, "[") // "Name[tr]" -> "Name"
		switch base {
		case "Type":
			typ = val
		case "Name":
			e.Names = append(e.Names, val)
		case "GenericName":
			e.Generic = append(e.Generic, val)
		case "Keywords":
			for _, k := range strings.Split(val, ";") {
				if k = strings.TrimSpace(k); k != "" {
					e.Keywords = append(e.Keywords, k)
				}
			}
		case "Exec":
			if key == "Exec" {
				e.Exec = val
			}
		case "Hidden":
			if strings.EqualFold(val, "true") {
				return desktopEntry{}, false
			}
		case "NoDisplay":
			e.NoDisplay = strings.EqualFold(val, "true")
		}
	}
	if typ != "Application" || e.Exec == "" {
		return desktopEntry{}, false
	}
	return e, true
}

// loadDesktopEntries reads every application entry in dirs. An id that
// appears in more than one directory is taken from the first (most
// specific) one, matching the spec's lookup order.
func loadDesktopEntries(dirs []string) []desktopEntry {
	seen := map[string]bool{}
	var out []desktopEntry
	for _, dir := range dirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".desktop") {
				continue
			}
			id := strings.TrimSuffix(f.Name(), ".desktop")
			if seen[id] {
				continue
			}
			seen[id] = true
			if e, ok := parseDesktopEntry(filepath.Join(dir, f.Name())); ok {
				out = append(out, e)
			}
		}
	}
	return out
}

// normalizeAppName folds case and drops everything but letters and digits,
// so "VS Code", "vs-code" and "vscode" compare equal, as do "Google Chrome"
// and "google-chrome".
func normalizeAppName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// execBinary is the base name of the first word of an Exec line, with an
// `env VAR=x` / `flatpak run` prefix seen through, e.g.
// "env BAMF=1 /opt/spotify/spotify %U" -> "spotify".
func execBinary(execLine string) string {
	args := splitExec(execLine)
	for len(args) > 0 {
		head := filepath.Base(args[0])
		switch {
		case head == "env":
			args = args[1:]
			for len(args) > 0 && strings.Contains(args[0], "=") && !strings.HasPrefix(args[0], "-") {
				args = args[1:]
			}
			continue
		case head == "flatpak" && len(args) > 2 && args[1] == "run":
			// "flatpak run [opts] com.spotify.Client" -> last segment of the app id
			for _, a := range args[2:] {
				if !strings.HasPrefix(a, "-") {
					parts := strings.Split(a, ".")
					return parts[len(parts)-1]
				}
			}
			return ""
		}
		return head
	}
	return ""
}

// scoreDesktopEntry rates how well e matches the normalized query q; 0 means
// no match. Exact matches on what the user sees (the name) beat matches on
// implementation details (the id, the binary), which beat partial matches.
func scoreDesktopEntry(e desktopEntry, q string) int {
	best := 0
	set := func(s int) {
		if s > best {
			best = s
		}
	}
	for _, n := range e.Names {
		nn := normalizeAppName(n)
		switch {
		case nn == q:
			set(100)
		case strings.HasPrefix(nn, q) && len(q) >= 3:
			set(60)
		case strings.Contains(nn, q) && len(q) >= 4:
			set(45)
		}
	}
	idParts := strings.Split(e.ID, ".")
	if normalizeAppName(e.ID) == q || normalizeAppName(idParts[len(idParts)-1]) == q {
		set(90)
	}
	if normalizeAppName(execBinary(e.Exec)) == q {
		set(85)
	}
	for _, k := range e.Keywords {
		if normalizeAppName(k) == q {
			set(75)
		}
	}
	for _, g := range e.Generic {
		gn := normalizeAppName(g)
		switch {
		case gn == q:
			set(55)
		case strings.Contains(gn, q) && len(q) >= 5:
			// "Hesap Makinesi" -> KCalc, whose Turkish GenericName is
			// "Bilimsel Hesap Makinesi": people ask for a kind of app.
			set(40)
		}
	}
	if best > 0 && e.NoDisplay {
		// Helpers like "code-url-handler" carry NoDisplay=true; never let one
		// win over the visible app it belongs to.
		best -= 30
	}
	return best
}

// findDesktopEntry returns the best entry for name, or ok=false when nothing
// matches. Ties go to the shorter (more canonical) id, then alphabetically,
// so the result is deterministic.
func findDesktopEntry(name string, entries []desktopEntry) (desktopEntry, bool) {
	q := normalizeAppName(name)
	if q == "" {
		return desktopEntry{}, false
	}
	type cand struct {
		e     desktopEntry
		score int
	}
	var cands []cand
	for _, e := range entries {
		if s := scoreDesktopEntry(e, q); s > 0 {
			cands = append(cands, cand{e, s})
		}
	}
	if len(cands) == 0 {
		return desktopEntry{}, false
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		if len(cands[i].e.ID) != len(cands[j].e.ID) {
			return len(cands[i].e.ID) < len(cands[j].e.ID)
		}
		return cands[i].e.ID < cands[j].e.ID
	})
	return cands[0].e, true
}

// findDesktopEntryByID returns the entry whose id is exactly id (with or
// without the .desktop suffix) — used for the default browser, which
// xdg-settings reports as an id.
func findDesktopEntryByID(id string, entries []desktopEntry) (desktopEntry, bool) {
	id = strings.TrimSuffix(strings.TrimSpace(id), ".desktop")
	for _, e := range entries {
		if e.ID == id {
			return e, true
		}
	}
	return desktopEntry{}, false
}

// splitExec splits an Exec value into arguments following the Desktop Entry
// spec's quoting rules: whitespace separates, double quotes group, and
// inside quotes a backslash escapes the next character.
func splitExec(s string) []string {
	var args []string
	var cur strings.Builder
	inQuote, have := false, false
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case inQuote && r == '\\' && i+1 < len(rs):
			i++
			cur.WriteRune(rs[i])
		case r == '"':
			inQuote = !inQuote
			have = true
		case !inQuote && (r == ' ' || r == '\t'):
			if have {
				args = append(args, cur.String())
				cur.Reset()
				have = false
			}
		default:
			cur.WriteRune(r)
			have = true
		}
	}
	if have {
		args = append(args, cur.String())
	}
	return args
}

// execCommandLine turns an entry's Exec value into argv: field codes that
// would take a file or URL (%f %F %u %U and the deprecated ones) are dropped
// because open_app opens the app with nothing, %c becomes the app's name,
// %k the entry's path, %i is dropped (it needs an icon), and %% is a literal
// percent sign.
func execCommandLine(e desktopEntry) []string {
	var out []string
	name := ""
	if len(e.Names) > 0 {
		name = e.Names[0]
	}
	for _, a := range splitExec(e.Exec) {
		if hasEmbeddedTargetCode(a) {
			// "--uri=%u" (Spotify): with no URL to pass, the desktop drops
			// the whole argument, not just the code.
			continue
		}
		switch a {
		case "%f", "%F", "%u", "%U", "%d", "%D", "%n", "%N", "%v", "%m", "%i":
			continue
		case "%c":
			out = append(out, name)
			continue
		case "%k":
			out = append(out, e.Path)
			continue
		}
		a = strings.ReplaceAll(a, "%c", name)
		a = strings.ReplaceAll(a, "%k", e.Path)
		a = strings.ReplaceAll(a, "%%", "%")
		out = append(out, a)
	}
	return out
}

// hasEmbeddedTargetCode reports a file/URL field code inside a longer
// argument, e.g. "--uri=%u". A bare "%u" is handled by execCommandLine's
// switch.
func hasEmbeddedTargetCode(arg string) bool {
	if len(arg) <= 2 {
		return false
	}
	for _, c := range []string{"%f", "%F", "%u", "%U"} {
		if strings.Contains(arg, c) {
			return true
		}
	}
	return false
}

// defaultBrowserID asks the desktop which entry is the default web browser
// ("firefox.desktop"). Empty when it cannot tell.
var defaultBrowserID = func() string {
	out, err := exec.Command("xdg-settings", "get", "default-web-browser").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// fallbackBrowsers is tried, in order, when the desktop names no default
// browser entry Memo can find.
var fallbackBrowsers = []string{"firefox", "chromium", "google-chrome", "google-chrome-stable", "brave-browser", "brave", "microsoft-edge", "vivaldi", "opera", "epiphany"}

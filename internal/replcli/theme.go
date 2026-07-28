package replcli

import (
	"os"
	"path/filepath"
	"strings"

	"memo/internal/config"
)

// replTheme selects which status-bar/welcome-banner style the composer
// renders. Purely a local terminal-rendering preference — unlike the
// language setting (SetLanguage), it has no backend counterpart and never
// needs one, since the desktop GUI has no equivalent concept.
type replTheme string

const (
	// themeG is the default: a compact, box-free welcome line and a bottom
	// status bar showing live data (model, memory, auto-permission state)
	// instead of static command hints.
	themeG replTheme = "g"
	// themeClassic is the original boxed welcome panel with a static
	// command-hint bar — kept for anyone who preferred it.
	themeClassic replTheme = "classic"
)

// parseTheme validates a /tema argument against the known themes.
func parseTheme(s string) (replTheme, bool) {
	switch replTheme(strings.ToLower(strings.TrimSpace(s))) {
	case themeG:
		return themeG, true
	case themeClassic:
		return themeClassic, true
	}
	return "", false
}

// themeFilePath is where the chosen theme is persisted — a one-line text
// file next to the REPL's own debug log (config.DataPath("repl.log")),
// resolved relative to whichever machine actually runs the terminal client,
// which is correct here: the theme is a property of this terminal, not of
// whatever backend it happens to be talking to.
func themeFilePath() string {
	return config.DataPath("cli_theme")
}

// loadSavedTheme returns the last theme saved via saveTheme, or themeG (the
// default) if none was ever saved, the file is unreadable, or its contents
// don't parse — every one of those cases should look like "first run",
// not an error.
func loadSavedTheme() replTheme {
	data, err := os.ReadFile(themeFilePath())
	if err != nil {
		return themeG
	}
	if th, ok := parseTheme(string(data)); ok {
		return th
	}
	return themeG
}

// saveTheme persists th so it survives a restart. Best-effort by design —
// callers should still apply the theme for the current session even if
// this fails (a read-only data dir, for instance), matching how other
// local writes in this package degrade.
func saveTheme(th replTheme) error {
	path := themeFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(th), 0644)
}

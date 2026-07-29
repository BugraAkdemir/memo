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
	// themeDefault is Memo's own default: a compact, box-free welcome line
	// and a bottom status bar showing live data (model, memory,
	// auto-permission state) instead of static command hints.
	themeDefault replTheme = "default"
	// themeClaudeCode is the original boxed welcome panel with a static
	// command-hint bar, modeled on Claude Code's own terminal client — kept
	// for anyone who preferred that look.
	themeClaudeCode replTheme = "claude-code"
)

// themeChoices lists every theme in display order — the single source of
// truth for the /theme arrow-key picker (commands.go's pickTheme).
func themeChoices() []replTheme {
	return []replTheme{themeDefault, themeClaudeCode}
}

// parseTheme validates a /theme argument against the known themes. "g" and
// "classic" are accepted as legacy aliases for themeDefault/themeClaudeCode
// — their original names before a clearer naming pass — so a preference
// file saved before that rename still parses correctly instead of silently
// resetting someone's already-made choice back to the default.
func parseTheme(s string) (replTheme, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(themeDefault), "g":
		return themeDefault, true
	case string(themeClaudeCode), "classic":
		return themeClaudeCode, true
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

// loadSavedTheme returns the last theme saved via saveTheme, or themeDefault
// if none was ever saved, the file is unreadable, or its contents don't
// parse — every one of those cases should look like "first run", not an
// error.
func loadSavedTheme() replTheme {
	data, err := os.ReadFile(themeFilePath())
	if err != nil {
		return themeDefault
	}
	if th, ok := parseTheme(string(data)); ok {
		return th
	}
	return themeDefault
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

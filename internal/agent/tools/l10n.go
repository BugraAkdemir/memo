package tools

import "sync/atomic"

// The tool packages have no access to the App or its config — they are plain
// functions over package-level state (CalendarClient, kiloModelsURL-style
// vars). Their user-visible result strings therefore get the language via
// this tiny process-wide setting instead of threading a lang parameter
// through every signature.
//
// SetUILanguage is called from internal/app at startup and on every
// SetUILanguage change; T reads it fresh per call, matching waLang's
// semantics elsewhere in this codebase: exactly "tr" yields Turkish,
// everything else (unset included) yields English — the GUI's default since
// 2026-08-13. Known carve-out: an unset language makes REPL users see
// English tool results even though replcli's own chrome defaults to Turkish;
// new surfaces deliberately default English (see whatsapp_l10n.go's doc).
var uiLang atomic.Value // string

// SetUILanguage records the active UI language for tool result strings.
func SetUILanguage(lang string) {
	uiLang.Store(lang)
}

// T picks tr when the recorded UI language is exactly "tr", otherwise en.
// fmt verbs survive: callers Sprintf/Errorf the returned template.
func T(tr, en string) string {
	if v, _ := uiLang.Load().(string); v == "tr" {
		return tr
	}
	return en
}

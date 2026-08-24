package llama

import "sync/atomic"

// Installer progress/error strings are user-facing (streamed to the install
// dialog's log view, or surfaced verbatim in the HTTP error body by
// handleLlamaInstall) but this package has no access to App or its config.
// Same pattern as internal/agent/tools/l10n.go: a process-wide language
// setting instead of threading a lang parameter through every signature.
var uiLang atomic.Value // string

// SetUILanguage records the active UI language for installer strings.
func SetUILanguage(lang string) {
	uiLang.Store(lang)
}

// T picks tr when the recorded UI language is exactly "tr", otherwise en —
// matching tools.T's semantics (unset/unknown defaults to English).
func T(tr, en string) string {
	if v, _ := uiLang.Load().(string); v == "tr" {
		return tr
	}
	return en
}

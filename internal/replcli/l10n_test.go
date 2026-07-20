package replcli

import (
	"regexp"
	"testing"
)

// These tests name the *testing.T parameter "tt", not the Go-idiomatic "t":
// this package's own l10n helper is a function named t(key string) string,
// and a parameter named t would shadow it for the whole test body, turning
// every t("key") call into a compile error ("cannot call non-function t").

func TestT_DefaultsToTurkish(tt *testing.T) {
	activeLang = "tr"
	if got := t("welcome_back"); got != tr["welcome_back"] {
		tt.Errorf("t(welcome_back) = %q, want the Turkish value %q", got, tr["welcome_back"])
	}
}

func TestT_SetLanguageEn_SwitchesActiveLanguage(tt *testing.T) {
	SetLanguage("en")
	defer SetLanguage("tr")
	if got := t("welcome_back"); got != en["welcome_back"] {
		tt.Errorf("after SetLanguage(en), t(welcome_back) = %q, want %q", got, en["welcome_back"])
	}
}

func TestSetLanguage_AnythingOtherThanEnIsTurkish(tt *testing.T) {
	SetLanguage("en")
	SetLanguage("fr") // not "en" — must fall back to "tr", not stay "en"
	defer SetLanguage("tr")
	if got := t("welcome_back"); got != tr["welcome_back"] {
		tt.Errorf("SetLanguage(fr) left active language as %q, want tr's value; got %q", activeLang, got)
	}
}

func TestT_UnknownKeyFallsBackToRawKey(tt *testing.T) {
	if got := t("this_key_does_not_exist"); got != "this_key_does_not_exist" {
		tt.Errorf("t(unknown) = %q, want the raw key back", got)
	}
}

func TestL10nMaps_TrAndEnHaveTheSameKeySet(tt *testing.T) {
	// Guards against the actual failure mode of hand-maintaining two large
	// parallel maps: a typo'd or forgotten key silently degrades that one
	// string to Turkish (or the raw key) under "en" instead of failing
	// anything visibly.
	for k := range tr {
		if _, ok := en[k]; !ok {
			tt.Errorf("key %q exists in tr but not en", k)
		}
	}
	for k := range en {
		if _, ok := tr[k]; !ok {
			tt.Errorf("key %q exists in en but not tr", k)
		}
	}
}

var fmtVerbPattern = regexp.MustCompile(`%[a-zA-Z]|%%`)

func TestL10nMaps_FormatVerbsMatchBetweenLanguages(tt *testing.T) {
	// Every t()-returned template is fed straight into fmt.Sprintf/Fprintf/
	// Errorf by its call site with a fixed argument list (same call site,
	// same args, regardless of language) — so a tr/en pair whose verb
	// sequences differ (wrong count, wrong type, or %% miscounted) would
	// panic or silently misformat only under "en", the language nobody was
	// looking at while writing the call site.
	for k, trVal := range tr {
		enVal, ok := en[k]
		if !ok {
			continue // already reported by TestL10nMaps_TrAndEnHaveTheSameKeySet
		}
		trVerbs := fmtVerbPattern.FindAllString(trVal, -1)
		enVerbs := fmtVerbPattern.FindAllString(enVal, -1)
		if len(trVerbs) != len(enVerbs) {
			tt.Errorf("key %q: tr has %d format verbs %v, en has %d %v", k, len(trVerbs), trVerbs, len(enVerbs), enVerbs)
			continue
		}
		for i := range trVerbs {
			if trVerbs[i] != enVerbs[i] {
				tt.Errorf("key %q: verb #%d differs — tr has %q, en has %q (%v vs %v)", k, i, trVerbs[i], enVerbs[i], trVerbs, enVerbs)
			}
		}
	}
}

func TestT_EnMissingKeyFallsBackToTurkish(tt *testing.T) {
	// A key present only in tr (an unfinished translation) must still
	// resolve to *something* readable under "en", not the raw key —
	// mirrors l10n.dart's own fallback order.
	const key = "l10n_test_tr_only_key"
	tr[key] = "sadece türkçe"
	defer delete(tr, key)

	SetLanguage("en")
	defer SetLanguage("tr")
	if got := t(key); got != "sadece türkçe" {
		tt.Errorf("t(%s) under en = %q, want the Turkish fallback %q", key, got, "sadece türkçe")
	}
}

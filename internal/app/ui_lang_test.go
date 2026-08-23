package app

import (
	"fmt"
	"testing"

	"memo/internal/config"
)

// The seam this closes: backend-generated strings reaching the chat window
// and API responses were Turkish no matter what language the rest of the UI
// spoke. t() is the single decision point — lock its semantics down:
// exactly "tr" picks Turkish, everything else (including unset "" and any
// unrecognized value) picks English, mirroring waLang's convention and the
// GUI's English default since 2026-08-13.
func TestT_SelectsLanguageVariants(t *testing.T) {
	cases := []struct {
		lang string
		want string
	}{
		{"tr", "yanıt durduruldu"},
		{"en", "response stopped"},
		{"", "response stopped"},
		{"de", "response stopped"},
	}
	for _, tc := range cases {
		a := &App{cfg: &config.AppConfig{
			Identity: config.IdentityConfig{UILanguage: tc.lang},
		}}
		if got := a.t("yanıt durduruldu", "response stopped"); got != tc.want {
			t.Errorf("UILanguage=%q: t() = %q, want %q", tc.lang, got, tc.want)
		}
	}
}

// fmt verbs must survive intact through t() — callers Sprintf the template
// it returns.
func TestT_ReturnsTemplateForFormatting(t *testing.T) {
	a := &App{cfg: &config.AppConfig{
		Identity: config.IdentityConfig{UILanguage: "tr"},
	}}
	got := a.t("model yüklenemedi: %v", "failed to load model: %v")
	if want := "model yüklenemedi: %v"; got != want {
		t.Fatalf("t() = %q, want %q", got, want)
	}
	wrapped := fmt.Sprintf(got, "llama-server")
	if wrapped != "model yüklenemedi: llama-server" {
		t.Errorf("Sprintf(t(...)) = %q", wrapped)
	}
}

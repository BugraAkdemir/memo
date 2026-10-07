package app

import (
	"strings"
	"testing"

	"memo/internal/config"
)

func friendlyApp(lang string) *App {
	a := &App{cfg: &config.AppConfig{}}
	a.cfg.Identity.UILanguage = lang
	return a
}

// The reported case, word for word: a vendor's safety refusal arriving as a bare
// status line with a request id.
const reportedSafetyError = "⚠️ [custom] status 400: Your request was rejected by the safety system. If you believe this is an error, contact us at help.openai.com and include the request ID 5805bed6-a5d1-4502-8e86-fab58cd61e3e. safety_violations=[sexual]."

func TestFriendlyError_ReportedSafetyRefusal(t *testing.T) {
	for lang, want := range map[string]string{"tr": "güvenlik filtresi", "en": "safety filter"} {
		got := friendlyApp(lang).FriendlyError(reportedSafetyError)
		if !strings.HasPrefix(got, "⚠️ ") || !strings.Contains(got, want) {
			t.Errorf("%s: %q should be a friendly safety sentence containing %q", lang, got, want)
		}
		for _, leak := range []string{"request ID", "safety_violations", "5805bed6", "[custom]", "help.openai.com"} {
			if strings.Contains(got, leak) {
				t.Errorf("%s: %q leaks %q", lang, got, leak)
			}
		}
		if !strings.Contains(got, "(HTTP 400)") {
			t.Errorf("%s: %q should keep the status as a small tag", lang, got)
		}
	}
}

func TestFriendlyError_KnownFailures(t *testing.T) {
	cases := []struct {
		name, raw, wantEN, wantTR string
	}{
		{"bad key", "⚠️ all providers failed: [openai] authentication error (status 401): Incorrect API key provided", "verify who you are", "kimliğini doğrulayamadı"},
		{"forbidden", "⚠️ [claude] status 403: forbidden", "not allowed to use this model", "erişim izni yok"},
		// Found live against the fake provider: the OpenAI-compatible provider
		// words a 403 as an "authentication error", which must not read as a bad key.
		{"403 worded as authentication", "⚠️ all providers failed: [custom] authentication error (status 403): simulated error", "not allowed to use this model", "erişim izni yok"},
		{"no such model", "⚠️ [custom] status 404: model not found", "was not found", "bulunamadı"},
		{"rate limit", "⚠️ [groq] status 429: Rate limit reached for requests", "too many requests", "çok fazla istek"},
		{"server error", "⚠️ [custom] status 500: Internal error encountered.", "problem on its side", "sunucu tarafında"},
		{"bad gateway", "⚠️ all providers failed: [gemini] status 503: overloaded", "problem on its side", "sunucu tarafında"},
		{"context", "⚠️ [claude] status 400: prompt is too long: 210000 tokens > 200000 maximum", "context window", "bağlam sınırına"},
		{"quota", "⚠️ [custom] status 429: You exceeded your current quota, please check your plan", "allowance has run out", "kullanım hakkı doldu"},
		{"no network", "⚠️ Post \"https://api.x.ai/v1/chat/completions\": dial tcp: lookup api.x.ai: no such host", "Could not reach the provider", "ulaşılamadı"},
		{"timeout", "⚠️ [custom] context deadline exceeded", "timed out", "zaman aşımına"},
		{"other 4xx", "⚠️ [custom] status 422: unsupported parameter", "rejected this request", "kabul etmedi"},
	}
	for _, c := range cases {
		if got := friendlyApp("en").FriendlyError(c.raw); !strings.Contains(got, c.wantEN) || !strings.HasPrefix(got, "⚠️ ") {
			t.Errorf("%s (en): got %q, want it to contain %q", c.name, got, c.wantEN)
		}
		if got := friendlyApp("tr").FriendlyError(c.raw); !strings.Contains(got, c.wantTR) {
			t.Errorf("%s (tr): got %q, want it to contain %q", c.name, got, c.wantTR)
		}
		// A second pass must leave a friendly sentence alone, in either language.
		for _, lang := range []string{"en", "tr"} {
			a := friendlyApp(lang)
			once := a.FriendlyError(c.raw)
			if twice := a.FriendlyError(once); twice != once {
				t.Errorf("%s (%s): second pass changed %q into %q", c.name, lang, once, twice)
			}
		}
	}
}

// Text that is not a provider failure — or is already written for the user —
// must come back byte for byte.
func TestFriendlyError_LeavesEverythingElseAlone(t *testing.T) {
	a := friendlyApp("en")
	for _, raw := range []string{
		"",
		"⚠️ Local model not loaded. Start a model or select an API provider.",
		"⏱️ The request was stopped because the model sent nothing for 300 seconds. It may be very slow or stuck — you can try again.",
		"⏹️ Response stopped.",
		"⚠️ Model returned an empty response",
		"just some text",
	} {
		if got := a.FriendlyError(raw); got != raw {
			t.Errorf("FriendlyError(%q) = %q, want it unchanged", raw, got)
		}
	}
}

// Applying it twice must not mangle the first result (the stream and the saved
// message are converted independently from the same raw text, but a retry path
// could pass an already-friendly string through again).
func TestFriendlyError_IsIdempotent(t *testing.T) {
	a := friendlyApp("tr")
	once := a.FriendlyError(reportedSafetyError)
	if twice := a.FriendlyError(once); twice != once {
		t.Errorf("second pass changed the text:\n once: %q\ntwice: %q", once, twice)
	}
}

func TestErrStatus(t *testing.T) {
	for raw, want := range map[string]int{
		"[custom] status 400: x": 400, "authentication error (status 401): y": 401,
		"HTTP 503 Service Unavailable": 503, "status code: 429": 429, "no code here": 0, "status 200": 0,
	} {
		if got := errStatus(raw); got != want {
			t.Errorf("errStatus(%q) = %d, want %d", raw, got, want)
		}
	}
}

package provider

// This file centralizes what every vendor calls "reasoning effort" — a
// genuinely heterogeneous landscape. The only trustworthy source for
// "which effort labels does THIS model actually accept" is a live,
// per-model capability check against the vendor's own API — a static,
// hand-maintained table is a promise to keep it in sync with every vendor's
// model roster forever, and a wrong entry doesn't just look odd in a
// dropdown: verified live (2026-08-18), sending OpenAI's reasoning_effort
// to a non-reasoning model like gpt-4o returns a hard 400 ("Unsupported
// parameter"), and OpenCode Zen's own /models endpoint carries no
// capability data at all, so an earlier static table for it showed a full
// none..max picker for models with no such concept.
//
// So: providers with a real capability-discovery endpoint are queried live,
// per selected model, in internal/webserver/handlers_oauth.go — never
// listed here:
//   - OpenRouter: GET /api/v1/models exposes each model's own
//     reasoning.supported_efforts (fetchOpenRouterModelEffortLevels).
//   - Claude: GET /v1/models/{id} exposes capabilities.effort.{low,medium,
//     high,max,xhigh}.supported per model (fetchClaudeModelEffortLevels).
//   - Gemini: GET /v1/beta/models/{id} exposes a "thinking" boolean per
//     model — gates whether EffortLevelsForGemini's names apply at all
//     (fetchGeminiModelEffortLevels).
//   - Ollama: POST /api/show (native, not the OpenAI-compat endpoint)
//     exposes a per-model capabilities array; "thinking" in that array
//     gates whether the real value set (low/medium/high/max — not "none",
//     unlike an earlier version of this file guessed) applies
//     (fetchOllamaModelEffortLevels).
//
// Every other provider type — OpenAI, Grok, Groq, llama.cpp, OpenCode
// Zen/Go, Custom, and the CLI-backed types — has no such endpoint (checked
// live against each vendor's current docs, 2026-08-18: OpenAI's own docs
// say to check each model's page in prose, nothing programmatic; Grok,
// Groq, and OpenCode's /models responses carry no capability field;
// llama.cpp serves whatever arbitrary local GGUF the user loaded, which no
// vendor registry could describe). EffortLevelsForType returns nil for all
// of them — no static list, picker hidden — rather than guess.

// EffortLevelsForType always returns nil: every provider type is either
// discovered live per-model (see package doc comment above, handled in
// internal/webserver/handlers_oauth.go) or has no known effort concept.
// Kept as a function (not deleted outright) so callers have one stable,
// documented place to ask "does this type have a static answer" without
// needing to know which category a given ProviderType falls into.
func EffortLevelsForType(t ProviderType) []string {
	return nil
}

// geminiThinkingBudget maps the same label vocabulary used everywhere else
// onto generationConfig.thinkingConfig.thinkingBudget token counts, since
// the classic generateContent REST endpoint (what gemini.go calls) has no
// named-level parameter — only Google's newer, separate Interactions API
// does, which this codebase doesn't use. Picked as round, documented-
// reasonable starting points (Gemini's own budget guidance runs roughly
// 1k-32k+ depending on task complexity), not vendor-specified constants —
// there is no "the vendor says exactly this" source for what a label
// should number to, unlike the vendor-reported capability data this file's
// package doc comment describes. Whether to offer these labels for a given
// model at all is gated live by fetchGeminiModelEffortLevels's "thinking"
// boolean check, not by anything in this file.
var geminiThinkingBudget = map[string]int{
	"minimal": 512,
	"low":     1024,
	"medium":  8192,
	"high":    24576,
	"max":     32768,
}

// GeminiThinkingBudgetForLevel resolves a label to a thinkingBudget token
// count. ok is false for an unrecognized label (including ""), in which
// case gemini.go must omit thinkingConfig entirely rather than send a
// zero-value budget — 0 is itself a meaningful value to some Gemini models
// (disables thinking), not a safe "unset" sentinel.
func GeminiThinkingBudgetForLevel(level string) (budget int, ok bool) {
	budget, ok = geminiThinkingBudget[level]
	return
}

// EffortLevelsForGemini lists the labels GeminiThinkingBudgetForLevel
// accepts, for API responses / UI dropdowns once a model has confirmed
// "thinking" support (see fetchGeminiModelEffortLevels) — kept separate
// from a generic table because Gemini's list order matters (ascending
// budget) and there's no "none"/"default" entry the way other vendors have.
func EffortLevelsForGemini() []string {
	return []string{"minimal", "low", "medium", "high", "max"}
}

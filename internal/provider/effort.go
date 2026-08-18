package provider

// This file centralizes what every vendor calls "reasoning effort" — a
// genuinely heterogeneous landscape, verified against each vendor's current
// API docs (2026-08-18): OpenAI/Grok/Groq/llama.cpp use a flat
// "reasoning_effort" string; Claude uses a nested "thinking" object whose
// shape itself depends on the model generation; Gemini has no named levels
// at the REST generateContent endpoint Memo actually calls, only an integer
// token budget; Ollama accepts either a bool or a level string depending on
// the model.
//
// Most of these vendors have no self-describing "list my capabilities"
// endpoint, so the label sets below are authored from documentation, not
// fetched at runtime — but which set applies is still selected dynamically,
// per the provider type actually configured, never a single fixed value a
// user types in once. OpenRouter is the one real exception: its own
// /api/v1/models endpoint returns each model's actual supported_efforts,
// so that path (see openrouter.go's FetchOpenRouterEffortLevels) discovers
// the true, current, per-model answer instead of guessing.
//
// OpenCode Zen/Go are deliberately absent from effortLevelsByType even
// though they're thin OpenAI-compatible wrappers (opencode_zen.go/
// opencode_go.go) that inherit whatever flat reasoning_effort openai.go
// sends on the wire: that's a statement about the REQUEST shape, not about
// which values any given proxied model actually honors. Both are
// aggregators fronting many different underlying vendors' models (Claude,
// Gemini, GPT, DeepSeek, GLM, Kimi, ...) behind one endpoint, exactly like
// OpenRouter — but unlike OpenRouter, their own /models endpoint (checked
// live, 2026-08-18) returns bare {id, object, created, owned_by} with no
// reasoning/effort field at all, so there's no way to know per-model which
// labels (if any) apply. An earlier version of this file reused the OpenAI
// table for both — verified live as wrong: e.g. a non-reasoning model
// showed a full none..max picker with nothing behind it, and
// deepseek-v4-flash (whose real OpenRouter-side capability is only
// max/high/low, per that model's own supported_efforts) showed the OpenAI
// set's "xhigh" as if it were valid. Silence (empty list, effort picker
// hidden) is the honest answer here, same as ProviderCustom/CLI types
// below — guessing a universal list for an aggregator was actively
// misleading.

// effortLevelsByType holds the vendor-documented label set for provider
// types with a flat or simple enum-shaped effort parameter. Types absent
// from this map either have no effort concept (ProviderCustom,
// ProviderClaudeCodeCLI, ProviderCodexCLI — the CLI-backed providers have
// no HTTP request for this package to shape at all), have no reliable way
// to know which labels apply (ProviderOpenCodeZen, ProviderOpenCodeGo — see
// package doc comment above), or are handled by their own dedicated logic
// (ProviderClaude's model-generation caveat lives in claude.go;
// ProviderGemini's token-budget mapping is geminiThinkingBudget below;
// ProviderOpenRouter is discovered live, never listed here).
var effortLevelsByType = map[ProviderType][]string{
	ProviderOpenAI: {"none", "minimal", "low", "medium", "high", "xhigh", "max"},
	// Adaptive-mode effort (thinking:{type:"adaptive"} + output_config:
	// {effort}) — the vendor's current, forward-looking mechanism. Only
	// low/medium/high are documented values; see claude.go for the
	// model-compatibility caveat (adaptive thinking 400s on Claude Sonnet
	// 4.5/Opus 4.5/Haiku 4.5 and earlier, which only support the
	// deprecated manual budget_tokens mode instead).
	ProviderClaude: {"low", "medium", "high"},
	ProviderGrok:   {"low", "medium", "high", "xhigh"},
	// Union across the models Groq documents support for this param
	// (Qwen: none/default; GPT-OSS: low/medium/high) — Groq has no
	// discovery endpoint, so the model actually selected may only honor a
	// subset of this list. An unsupported value is the provider's problem
	// to reject/ignore, same as picking an unsupported one on any vendor.
	ProviderGroq: {"none", "default", "low", "medium", "high"},
	// Memo talks to Ollama over its OpenAI-compatible endpoint (see
	// ollama.go), not Ollama's native /api/chat — that compat layer
	// documents this exact flat reasoning_effort field/value set as a
	// pass-through to its native "think" control.
	ProviderOllama:   {"none", "low", "medium", "high", "max"},
	ProviderLlamaCPP: {"default", "minimal", "low", "medium", "high", "xhigh", "max"},
}

// EffortLevelsForType returns the known valid effort labels for t, or nil
// if t has no effort concept (ProviderGemini and ProviderOpenRouter are
// deliberately handled elsewhere — see this file's package doc comment).
func EffortLevelsForType(t ProviderType) []string {
	levels := effortLevelsByType[t]
	if levels == nil {
		return nil
	}
	out := make([]string, len(levels))
	copy(out, levels)
	return out
}

// geminiThinkingBudget maps the same label vocabulary used everywhere else
// onto generationConfig.thinkingConfig.thinkingBudget token counts, since
// the classic generateContent REST endpoint (what gemini.go calls) has no
// named-level parameter — only Google's newer, separate Interactions API
// does, which this codebase doesn't use. Picked as round, documented-
// reasonable starting points (Gemini's own budget guidance runs roughly
// 1k-32k+ depending on task complexity), not vendor-specified constants —
// unlike every other table in this file, there is no "the vendor says
// exactly this" source for what a label should number to.
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
// accepts, for API responses / UI dropdowns — kept separate from
// effortLevelsByType because Gemini's list order matters (ascending
// budget) and "none"/"medium-default" don't apply the same way here.
func EffortLevelsForGemini() []string {
	return []string{"minimal", "low", "medium", "high", "max"}
}

package livemode

// ModelInfo is the provider-agnostic shape model discovery returns to
// Flutter — one ID + a human-readable label, regardless of which engine
// it came from (Google Live's "models/…" resource name, OpenAI's plain
// model ID, or ElevenLabs' model_id). Per-engine discovery lives in this
// package's google/openai_realtime subpackages and in internal/tts's
// ElevenLabs discovery (internal/app.ListLiveModeEngineModels maps each
// engine's own response shape into this one, see PLAN_live_mode_v2.md §5.1).
type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

package app

import (
	"testing"

	"memo/internal/config"
)

func ptrFloat64(v float64) *float64 { return &v }
func ptrInt(v int) *int             { return &v }

// TestUpdateLlamaConfig_ExplicitZero is a regression test for BUG-QM2:
// UpdateLlamaConfig used to merge Temperature/TopP/MaxTokens with a plain
// `if cfg.Field != 0` check, which silently ignored a request that
// explicitly wanted the field set to 0 (greedy decoding for Temperature/
// TopP, "no limit" for MaxTokens) — indistinguishable from the field simply
// not being included in the request. Pointer fields on LlamaConfigUpdate
// fix that: nil means "not provided", a non-nil pointer to 0 means
// "explicitly set to zero".
// TestResolveLocalLlamaRouter_CarriesRealContextTokens guards a live bug: an
// agent-mode turn on a small local model (Phi-3-mini-4k-instruct, 4096 ctx)
// produced an 18147-token request against its real 4096-token window.
// modelContextWindow (internal/agent/executor.go) reads ContextTokens off
// the router's active provider config; when nothing ever set it for the
// local llama.cpp provider it fell through to that function's 128*1024
// fallback, so the agent pipeline's own per-turn token budget thought a
// tiny local model had a 128K window. resolveLocalLlamaRouter now takes the
// real ctx-size explicitly (llama.Server.CtxSize(), the same value
// buildMessagesForSession's non-agent path already budgets the plain-chat
// prompt against) and must carry it through onto the constructed router's
// provider config.
func TestResolveLocalLlamaRouter_CarriesRealContextTokens(t *testing.T) {
	router := resolveLocalLlamaRouter("http://127.0.0.1:9999", "phi-3-mini-4k", 4096)
	active := router.ActiveProviders()
	if len(active) != 1 {
		t.Fatalf("ActiveProviders() = %d entries, want 1", len(active))
	}
	if active[0].ContextTokens != 4096 {
		t.Errorf("ContextTokens = %d, want 4096 — modelContextWindow will otherwise fall back to its 128K default for this small local model", active[0].ContextTokens)
	}
}

func TestUpdateLlamaConfig_ExplicitZero(t *testing.T) {
	a := &App{cfg: &config.AppConfig{
		Llama: config.LlamaConfig{
			Temperature: 0.7,
			TopP:        0.9,
			MaxTokens:   256,
		},
	}}

	if err := a.UpdateLlamaConfig(config.LlamaConfigUpdate{
		Temperature: ptrFloat64(0),
		TopP:        ptrFloat64(0),
		MaxTokens:   ptrInt(0),
	}); err != nil {
		t.Fatalf("UpdateLlamaConfig() error = %v", err)
	}

	if a.cfg.Llama.Temperature != 0 {
		t.Errorf("Temperature = %v, want 0 (explicit zero must be applied)", a.cfg.Llama.Temperature)
	}
	if a.cfg.Llama.TopP != 0 {
		t.Errorf("TopP = %v, want 0 (explicit zero must be applied)", a.cfg.Llama.TopP)
	}
	if a.cfg.Llama.MaxTokens != 0 {
		t.Errorf("MaxTokens = %v, want 0 (explicit zero must be applied)", a.cfg.Llama.MaxTokens)
	}
}

// TestUpdateLlamaConfig_NilFieldsLeaveExistingValues confirms the other half
// of the fix: omitting a field (nil pointer) must still preserve whatever
// was already configured, not reset it to zero.
func TestUpdateLlamaConfig_NilFieldsLeaveExistingValues(t *testing.T) {
	a := &App{cfg: &config.AppConfig{
		Llama: config.LlamaConfig{
			Temperature: 0.7,
			TopP:        0.9,
			MaxTokens:   256,
		},
	}}

	if err := a.UpdateLlamaConfig(config.LlamaConfigUpdate{
		EngineMode: "cpu",
	}); err != nil {
		t.Fatalf("UpdateLlamaConfig() error = %v", err)
	}

	if a.cfg.Llama.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want unchanged 0.7", a.cfg.Llama.Temperature)
	}
	if a.cfg.Llama.TopP != 0.9 {
		t.Errorf("TopP = %v, want unchanged 0.9", a.cfg.Llama.TopP)
	}
	if a.cfg.Llama.MaxTokens != 256 {
		t.Errorf("MaxTokens = %v, want unchanged 256", a.cfg.Llama.MaxTokens)
	}
	if a.cfg.Llama.EngineMode != "cpu" {
		t.Errorf("EngineMode = %q, want %q", a.cfg.Llama.EngineMode, "cpu")
	}
}

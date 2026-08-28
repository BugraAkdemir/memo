package app

import (
	"context"
	"fmt"

	"memo/internal/config"
	"memo/internal/livemode"
	"memo/internal/livemode/google"
	"memo/internal/livemode/openai_realtime"
	"memo/internal/tts"
)

// liveModeValidEngines/liveModeValidWorkModes/liveModeValidPermissionPolicies
// are the only accepted values for their respective LiveModeConfig fields —
// validated here rather than left to whatever a client happens to send, since
// this config also selects which code path handleLiveModeSession (a later
// phase) takes.
var (
	liveModeValidEngines            = map[string]bool{"local": true, "google_live": true, "openai_realtime": true, "elevenlabs": true, "custom": true}
	liveModeValidWorkModes          = map[string]bool{"delegate": true, "standalone": true}
	liveModeValidPermissionPolicies = map[string]bool{"voice_prompt": true, "auto_allow_once": true}
	liveModeValidBargeInLevels      = map[string]bool{"high": true, "low": true}
)

// GetLiveModeConfig returns the current top-level Live Mode selector
// (Enabled/ActiveEngine/WorkMode/AgentPermissionPolicy). Per-engine
// credentials/model/voice config is a later phase's own
// internal/livemode.ConfigManager-backed addition, not part of this struct.
func (a *App) GetLiveModeConfig() config.LiveModeConfig {
	if a.cfg == nil {
		return config.LiveModeConfig{}
	}
	return a.cfg.LiveMode
}

// UpdateLiveModeConfig validates and persists a new Live Mode selector.
// Mirrors SetBeta's simplicity (internal/app/remote_tailscale.go) — this is
// a single small config struct, not a multi-field endpoint like remote
// access, so one validate-then-save method covers the whole thing rather
// than one setter per field.
func (a *App) UpdateLiveModeConfig(cfg config.LiveModeConfig) error {
	if a.cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	if !liveModeValidEngines[cfg.ActiveEngine] {
		return fmt.Errorf("invalid active_engine: %q", cfg.ActiveEngine)
	}
	if !liveModeValidWorkModes[cfg.WorkMode] {
		return fmt.Errorf("invalid work_mode: %q", cfg.WorkMode)
	}
	if !liveModeValidPermissionPolicies[cfg.AgentPermissionPolicy] {
		return fmt.Errorf("invalid agent_permission_policy: %q", cfg.AgentPermissionPolicy)
	}
	// Empty = the "high" default (a config written before this field existed,
	// or a client that doesn't send it yet) — normalize rather than reject.
	if cfg.BargeInSensitivity == "" {
		cfg.BargeInSensitivity = "high"
	}
	if !liveModeValidBargeInLevels[cfg.BargeInSensitivity] {
		return fmt.Errorf("invalid barge_in_sensitivity: %q", cfg.BargeInSensitivity)
	}
	a.cfg.LiveMode = cfg
	if err := config.Save(a.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

// initLiveModeEngines loads data/livemode_engines.json — Phase 3 of
// docs/plans/PLAN_live_mode_v2.md. Called once from Startup, alongside
// initTTSProviders/initSTTProviders. Unlike those two, there is no
// router to (re)build here — see internal/livemode's package doc comment
// on why engine selection isn't a priority-fallback router.
func (a *App) initLiveModeEngines() {
	cfgMgr := livemode.NewConfigManager(config.DataPath("livemode_engines.json"), nil)

	a.liveModeMu.Lock()
	a.liveModeEngineCfgMgr = cfgMgr
	a.liveModeMu.Unlock()
}

// GetLiveModeEngines returns every saved non-local engine config.
func (a *App) GetLiveModeEngines() []livemode.EngineConfig {
	a.liveModeMu.RLock()
	cfgMgr := a.liveModeEngineCfgMgr
	a.liveModeMu.RUnlock()
	if cfgMgr == nil {
		return nil
	}
	return cfgMgr.GetAll()
}

// UpdateLiveModeEngine saves one engine's config (add or update, keyed by
// Type). Does not touch internal/tts/internal/stt's own provider systems —
// that sync lands in a later phase, once TranscribeAudio/SynthesizeSpeech
// actually dispatch through the active Live Mode engine (see
// docs/plans/PLAN_live_mode_v2.md's Phase 5).
func (a *App) UpdateLiveModeEngine(cfg livemode.EngineConfig) error {
	a.liveModeMu.RLock()
	cfgMgr := a.liveModeEngineCfgMgr
	a.liveModeMu.RUnlock()
	if cfgMgr == nil {
		return fmt.Errorf("Live Mode engine system not initialized")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	cfgMgr.Set(cfg)
	return nil
}

// DeleteLiveModeEngine removes one engine's saved config.
func (a *App) DeleteLiveModeEngine(t livemode.EngineType) error {
	a.liveModeMu.RLock()
	cfgMgr := a.liveModeEngineCfgMgr
	a.liveModeMu.RUnlock()
	if cfgMgr == nil {
		return fmt.Errorf("Live Mode engine system not initialized")
	}
	cfgMgr.Delete(t)
	return nil
}

// ListLiveModeEngineModels fetches the live model list for one engine type
// directly from that provider's own API — per docs/plans/PLAN_live_mode_v2.md
// §5.1's "never hardcode a model list" requirement. "local" and "custom"
// have no discovery endpoint (custom is an opaque, arbitrary
// OpenAI-compatible server) and return a clear "not supported" error rather
// than a fabricated list.
func (a *App) ListLiveModeEngineModels(ctx context.Context, t livemode.EngineType, apiKey string) ([]livemode.ModelInfo, error) {
	switch t {
	case livemode.EngineGoogleLive:
		models, err := google.ListLiveModels(ctx, apiKey)
		if err != nil {
			return nil, err
		}
		out := make([]livemode.ModelInfo, 0, len(models))
		for _, m := range models {
			out = append(out, livemode.ModelInfo{ID: m.Name, DisplayName: m.DisplayName})
		}
		return out, nil
	case livemode.EngineOpenAIRealtime:
		models, err := openai_realtime.ListRealtimeModels(ctx, apiKey)
		if err != nil {
			return nil, err
		}
		out := make([]livemode.ModelInfo, 0, len(models))
		for _, m := range models {
			out = append(out, livemode.ModelInfo{ID: m.ID, DisplayName: m.ID})
		}
		return out, nil
	case livemode.EngineElevenLabs:
		// Reuses internal/tts's existing ElevenLabs discovery (Phase 2) —
		// same TTS-capable filter ListTTSProviderModels already applies.
		models, err := tts.ListElevenLabsModels(ctx, apiKey)
		if err != nil {
			return nil, err
		}
		out := make([]livemode.ModelInfo, 0, len(models))
		for _, m := range models {
			if m.CanDoTextToSpeech {
				out = append(out, livemode.ModelInfo{ID: m.ModelID, DisplayName: m.Name})
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("model discovery not supported for %q", t)
	}
}

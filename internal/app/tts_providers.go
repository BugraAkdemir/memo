package app

import (
	"context"
	"fmt"
	"memo/internal/config"
	"memo/internal/tts"
)

// initTTSProviders loads data/tts_providers.json and builds the initial
// router. Called once from Startup, right after initTTS (the local Piper
// synthesizer) — mirrors reinitProviderAndOrchestra's role for the chat
// provider system, but TTS has no health-check loop or "active provider"
// restriction to restore (see docs/plans/PLAN_voice_live_mode_faz2.md's
// 2.1 note on why HealthCheck was deliberately left out).
func (a *App) initTTSProviders() {
	cfgMgr := tts.NewConfigManager(config.DataPath("tts_providers.json"), nil)
	router := tts.NewRouter(cfgMgr.GetEnabled())

	a.ttsRouterMu.Lock()
	a.ttsProviderCfgMgr = cfgMgr
	a.ttsRouter = router
	a.ttsRouterMu.Unlock()
}

// GetTTSProviders returns all configured external TTS providers.
func (a *App) GetTTSProviders() []tts.ProviderConfig {
	a.ttsRouterMu.RLock()
	cfgMgr := a.ttsProviderCfgMgr
	a.ttsRouterMu.RUnlock()
	if cfgMgr == nil {
		return nil
	}
	return cfgMgr.GetAll()
}

// UpdateTTSProvider saves a TTS provider config and rebuilds the router.
func (a *App) UpdateTTSProvider(cfg tts.ProviderConfig) error {
	a.ttsRouterMu.RLock()
	cfgMgr := a.ttsProviderCfgMgr
	a.ttsRouterMu.RUnlock()
	if cfgMgr == nil {
		return fmt.Errorf("TTS provider system not initialized")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	cfgMgr.Set(cfg)

	a.ttsRouterMu.Lock()
	a.ttsRouter = tts.NewRouter(cfgMgr.GetEnabled())
	a.ttsRouterMu.Unlock()
	return nil
}

// DeleteTTSProvider removes a TTS provider config and rebuilds the router.
func (a *App) DeleteTTSProvider(pt tts.ProviderType, name ...string) error {
	a.ttsRouterMu.RLock()
	cfgMgr := a.ttsProviderCfgMgr
	a.ttsRouterMu.RUnlock()
	if cfgMgr == nil {
		return fmt.Errorf("TTS provider system not initialized")
	}
	cfgMgr.Delete(pt, name...)

	a.ttsRouterMu.Lock()
	a.ttsRouter = tts.NewRouter(cfgMgr.GetEnabled())
	a.ttsRouterMu.Unlock()
	return nil
}

// TestTTSProviderConnection tests connectivity for the given TTS provider
// config (a real, short synthesis call — see tts.ConfigManager.TestConnection).
func (a *App) TestTTSProviderConnection(cfg tts.ProviderConfig) error {
	a.ttsRouterMu.RLock()
	cfgMgr := a.ttsProviderCfgMgr
	a.ttsRouterMu.RUnlock()
	if cfgMgr == nil {
		return fmt.Errorf("TTS provider system not initialized")
	}
	return cfgMgr.TestConnection(&cfg)
}

// ListTTSProviderModels fetches the live, TTS-capable model list for a
// provider type directly from that provider's own API — per
// docs/plans/PLAN_live_mode_v2.md's "never hardcode a model list"
// requirement. Only ElevenLabs currently exposes model discovery; OpenAI's
// TTS voices are a small fixed set with no discovery endpoint, and Custom
// is opaque (an arbitrary OpenAI-compatible server) — both return a clear
// "not supported" error rather than a fabricated list.
func (a *App) ListTTSProviderModels(ctx context.Context, pt tts.ProviderType, apiKey string) ([]tts.ElevenLabsModel, error) {
	if pt != tts.ProviderElevenLabs {
		return nil, fmt.Errorf("model discovery not supported for %q", pt)
	}
	all, err := tts.ListElevenLabsModels(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	ttsCapable := make([]tts.ElevenLabsModel, 0, len(all))
	for _, m := range all {
		if m.CanDoTextToSpeech {
			ttsCapable = append(ttsCapable, m)
		}
	}
	return ttsCapable, nil
}

// ListTTSProviderVoices fetches the live voice list for a provider type.
// Same "only ElevenLabs supports discovery" scope as ListTTSProviderModels.
func (a *App) ListTTSProviderVoices(ctx context.Context, pt tts.ProviderType, apiKey string) ([]tts.ElevenLabsVoice, error) {
	if pt != tts.ProviderElevenLabs {
		return nil, fmt.Errorf("voice discovery not supported for %q", pt)
	}
	return tts.ListElevenLabsVoices(ctx, apiKey)
}

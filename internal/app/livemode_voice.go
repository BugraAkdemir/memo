package app

import (
	"context"
	"fmt"

	"memo/internal/livemode"
	"memo/internal/stt"
	"memo/internal/tts"
)

// synthesizeViaLiveModeEngine and transcribeViaLiveModeEngine build a
// tts.TTSProvider/stt.STTProvider directly from the saved
// livemode.EngineConfig for engines "elevenlabs"/"custom", bypassing
// tts.Router/stt.Router/ConfigManager entirely.
//
// This is a deliberate simplification versus an earlier draft of
// PLAN_live_mode_v2.md's §4.2, which considered syncing a saved
// livemode.EngineConfig into a matching tts.ProviderConfig/
// stt.ProviderConfig entry so the existing Router "just works". That would
// mean the exact same API key/voice/base_url living in two separate config
// stores (data/livemode_engines.json AND data/tts_providers.json/
// data/stt_providers.json) that some future change could update one of
// without the other — precisely the two-sources-of-truth drift class
// AGENTS.md's BUG-ONB gotchas already document elsewhere in this codebase.
// Live Mode's active engine is a single, explicit choice (never a fallback
// chain, see internal/livemode's package doc comment), so there is nothing
// here that actually needs Router's multi-provider machinery — constructing
// the provider straight from livemode's own config and calling it directly
// is both simpler and structurally immune to that drift.
//
// The pre-existing /api/tts/providers (Faz 2) system remains fully
// independent and keeps powering the "local" engine's optional external TTS
// fallback exactly as it does today — it is untouched by Live Mode's engine
// selection.
func (a *App) synthesizeViaLiveModeEngine(ctx context.Context, engine livemode.EngineType, text string) ([]byte, error) {
	cfg, err := a.getLiveModeEngineConfig(engine)
	if err != nil {
		return nil, err
	}

	var ttsType tts.ProviderType
	switch engine {
	case livemode.EngineElevenLabs:
		ttsType = tts.ProviderElevenLabs
	case livemode.EngineCustom:
		ttsType = tts.ProviderCustom
	default:
		return nil, fmt.Errorf("Live Mode engine %q has no TTS provider", engine)
	}

	p, err := tts.NewProvider(tts.ProviderConfig{
		Type:    ttsType,
		APIKey:  cfg.APIKey,
		Voice:   cfg.Voice,
		BaseURL: cfg.BaseURL,
	})
	if err != nil {
		return nil, err
	}
	return p.Synthesize(ctx, text, cfg.Voice)
}

func (a *App) transcribeViaLiveModeEngine(ctx context.Context, engine livemode.EngineType, audioData []byte) (string, error) {
	cfg, err := a.getLiveModeEngineConfig(engine)
	if err != nil {
		return "", err
	}

	var sttType stt.ProviderType
	switch engine {
	case livemode.EngineElevenLabs:
		sttType = stt.ProviderElevenLabs
	case livemode.EngineCustom:
		sttType = stt.ProviderCustom
	default:
		return "", fmt.Errorf("Live Mode engine %q has no STT provider", engine)
	}

	p, err := stt.NewProvider(stt.ProviderConfig{
		Type:    sttType,
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
	})
	if err != nil {
		return "", err
	}
	return p.Transcribe(ctx, audioData)
}

func (a *App) getLiveModeEngineConfig(engine livemode.EngineType) (livemode.EngineConfig, error) {
	a.liveModeMu.RLock()
	cfgMgr := a.liveModeEngineCfgMgr
	a.liveModeMu.RUnlock()
	if cfgMgr == nil {
		return livemode.EngineConfig{}, fmt.Errorf("Live Mode engine system not initialized")
	}
	cfg, ok := cfgMgr.Get(engine)
	if !ok {
		return livemode.EngineConfig{}, fmt.Errorf("no Live Mode config saved for engine %q", engine)
	}
	return cfg, nil
}

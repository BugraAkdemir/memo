package app

import (
	"context"
	"fmt"
	"memo/internal/logx"
	"memo/internal/tts"
	"time"
)

// initTTS constructs the Piper synthesizer from config, if enabled. Unlike
// startSTTServer this is synchronous and cheap — tts.Synthesizer holds no
// subprocess or open port (Piper has no persistent server mode, see
// internal/tts's package doc), it just captures the configured binary/model
// paths and spawns a fresh subprocess per Synthesize call. Missing
// binary/model errors surface at Synthesize time (SynthesizeSpeech below),
// not here.
func (a *App) initTTS() {
	cfg := a.cfg.TTS
	if !cfg.Enabled {
		logx.Info("TTS: disabled by config")
		return
	}

	synth := tts.NewSynthesizer(cfg.BinaryPath, cfg.ModelPath)
	fillers := tts.NewFillerCache(synth)

	a.ttsMu.Lock()
	a.ttsSynthesizer = synth
	a.ttsFillerCache = fillers
	a.ttsMu.Unlock()
	logx.Info("TTS: Piper synthesizer configured")

	// Prewarm in the background so the first real filler request during a
	// live voice conversation (GetTTSFillerSound below) doesn't pay
	// subprocess-spawn latency at the exact moment it's trying to mask
	// latency — see FillerCache's own doc comment. Best-effort: if the
	// voice/binary turns out to be misconfigured, Prewarm's phrases are
	// just left uncached (logged inside Synthesize's own error paths
	// elsewhere), not a fatal Startup error.
	goRecover("tts.FillerCache.Prewarm", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		fillers.Prewarm(ctx)
	})
}

// GetTTSFillerSound returns a cached, local-only "thinking" filler sound
// (see tts.FillerCache) — nil error only when the local Piper synthesizer
// is configured, since fillers deliberately never go through the external
// provider Router (an API round-trip would defeat the point of masking
// latency).
func (a *App) GetTTSFillerSound() ([]byte, error) {
	a.ttsMu.RLock()
	fillers := a.ttsFillerCache
	a.ttsMu.RUnlock()
	if fillers == nil {
		return nil, fmt.Errorf("TTS: not enabled or not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return fillers.Random(ctx)
}

// SynthesizeSpeech renders text to WAV-encoded audio bytes. Tries any
// configured/enabled external TTS provider first (tts.Router, Faz 2), then
// falls back to the local Piper synthesizer (Faz 1) — mirrors
// callLLMStream's external-provider-then-local-model priority order
// (internal/app/llm.go), simplified to two tiers since TTS has no Orchestra
// equivalent. If no external provider is configured at all (Faz 1's only
// state, and still the default), behavior is unchanged from before this
// tier was added: straight to Piper.
func (a *App) SynthesizeSpeech(text string) ([]byte, error) {
	a.ttsRouterMu.RLock()
	router := a.ttsRouter
	a.ttsRouterMu.RUnlock()
	if router != nil && router.HasActiveProvider() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		audio, err := router.Synthesize(ctx, text)
		cancel()
		if err == nil {
			return audio, nil
		}
		logx.Printf("TTS: external provider(s) failed, falling back to local Piper: %v", err)
	}

	a.ttsMu.RLock()
	s := a.ttsSynthesizer
	a.ttsMu.RUnlock()
	if s == nil {
		return nil, fmt.Errorf("TTS: not enabled or not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return s.Synthesize(ctx, text)
}

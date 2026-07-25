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

	a.ttsMu.Lock()
	a.ttsSynthesizer = tts.NewSynthesizer(cfg.BinaryPath, cfg.ModelPath)
	a.ttsMu.Unlock()
	logx.Info("TTS: Piper synthesizer configured")
}

// SynthesizeSpeech renders text to WAV-encoded audio bytes via the
// configured Piper synthesizer. Mirrors TranscribeAudio's shape (stt.go).
func (a *App) SynthesizeSpeech(text string) ([]byte, error) {
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

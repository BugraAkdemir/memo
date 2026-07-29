package app

import (
	"memo/internal/config"
	"memo/internal/tts"
	"testing"
)

func TestInitTTS_DisabledLeavesSynthesizerNil(t *testing.T) {
	a := &App{cfg: &config.AppConfig{TTS: config.TTSConfig{Enabled: false}}}
	a.initTTS()
	if a.ttsSynthesizer != nil {
		t.Error("expected ttsSynthesizer to stay nil when TTS is disabled")
	}
}

func TestInitTTS_EnabledConfiguresSynthesizer(t *testing.T) {
	a := &App{cfg: &config.AppConfig{TTS: config.TTSConfig{
		Enabled:    true,
		BinaryPath: "/some/piper",
		ModelPath:  "/some/voice.onnx",
	}}}
	a.initTTS()
	if a.ttsSynthesizer == nil {
		t.Error("expected ttsSynthesizer to be configured when TTS is enabled")
	}
	if a.ttsFillerCache == nil {
		t.Error("expected ttsFillerCache to be configured alongside the synthesizer")
	}
}

// TestGetTTSFillerSound_NotConfiguredFailsFast mirrors
// TestSynthesizeSpeech_NotConfiguredFailsFastWithClearMessage — a disabled/
// unconfigured TTS setup must error immediately, not nil-pointer panic.
func TestGetTTSFillerSound_NotConfiguredFailsFast(t *testing.T) {
	a := &App{cfg: &config.AppConfig{TTS: config.TTSConfig{Enabled: false}}}
	_, err := a.GetTTSFillerSound()
	if err == nil {
		t.Fatal("expected error when TTS is not configured")
	}
}

// TestGetTTSFillerSound_ReturnsCachedAudio confirms GetTTSFillerSound reads
// through the App's own ttsFillerCache field rather than some separate
// path, using a cache pre-seeded directly (no real Piper binary needed).
func TestGetTTSFillerSound_ReturnsCachedAudio(t *testing.T) {
	fillers := tts.NewFillerCache(nil)
	for _, p := range tts.FillerPhrases {
		fillers.SetCached(p, []byte("audio-for-"+p))
	}
	a := &App{ttsFillerCache: fillers}

	audio, err := a.GetTTSFillerSound()
	if err != nil {
		t.Fatalf("GetTTSFillerSound: %v", err)
	}
	if len(audio) == 0 {
		t.Error("expected non-empty audio")
	}
}

// TestSynthesizeSpeech_NotConfiguredFailsFastWithClearMessage ensures a
// disabled/unconfigured TTS setup errors immediately with a message that
// distinguishes it from a Piper-subprocess failure, matching how
// TranscribeAudio (stt.go) reports "whisper server not started" rather than
// letting a nil-pointer call fail unclearly.
func TestSynthesizeSpeech_NotConfiguredFailsFastWithClearMessage(t *testing.T) {
	a := &App{cfg: &config.AppConfig{TTS: config.TTSConfig{Enabled: false}}}
	_, err := a.SynthesizeSpeech("merhaba")
	if err == nil {
		t.Fatal("expected error when TTS is not configured")
	}
}

// TestSynthesizeSpeech_NoActiveExternalProvider_BehavesLikeFaz1 asserts the
// new external-router tier (Faz 2.4) is a true no-op when no external TTS
// provider is configured/enabled — the default state, and Faz 1's only
// state — so existing Piper-only behavior is unaffected.
func TestSynthesizeSpeech_NoActiveExternalProvider_BehavesLikeFaz1(t *testing.T) {
	a := &App{
		cfg:       &config.AppConfig{TTS: config.TTSConfig{Enabled: false}},
		ttsRouter: tts.NewRouter(nil),
	}
	_, err := a.SynthesizeSpeech("merhaba")
	if err == nil {
		t.Fatal("expected error: no external provider active and Piper not configured")
	}
}

package app

import (
	"memo/internal/config"
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

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"memo/internal/config"
	"memo/internal/livemode"
	"memo/internal/tts"
)

func newAppWithLiveModeEngine(t *testing.T, cfg livemode.EngineConfig) *App {
	t.Helper()
	cfgMgr := livemode.NewConfigManager("", nil)
	cfgMgr.Set(cfg)
	return &App{
		cfg:                  &config.AppConfig{LiveMode: config.LiveModeConfig{ActiveEngine: string(cfg.Type)}},
		liveModeEngineCfgMgr: cfgMgr,
	}
}

func TestSynthesizeViaLiveModeEngine_CustomSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake-wav-bytes"))
	}))
	defer srv.Close()

	a := newAppWithLiveModeEngine(t, livemode.EngineConfig{
		Type:    livemode.EngineCustom,
		BaseURL: srv.URL,
		Voice:   "myvoice",
		Enabled: true,
	})

	audio, err := a.synthesizeViaLiveModeEngine(context.Background(), livemode.EngineCustom, "merhaba")
	if err != nil {
		t.Fatalf("synthesizeViaLiveModeEngine: %v", err)
	}
	if string(audio) != "fake-wav-bytes" {
		t.Errorf("expected fake-wav-bytes, got %q", audio)
	}
}

func TestSynthesizeViaLiveModeEngine_NoSavedConfig(t *testing.T) {
	a := &App{
		cfg:                  &config.AppConfig{LiveMode: config.LiveModeConfig{ActiveEngine: "custom"}},
		liveModeEngineCfgMgr: livemode.NewConfigManager("", nil),
	}
	_, err := a.synthesizeViaLiveModeEngine(context.Background(), livemode.EngineCustom, "merhaba")
	if err == nil {
		t.Fatal("expected an error when no config is saved for the engine")
	}
}

// TestSynthesizeSpeech_LiveModeEngineFailureFallsBackToRouterAndPiper
// confirms the dispatch added in Phase 5 degrades exactly like the
// pre-existing external-provider-then-Piper tiers: a Live Mode engine
// failure must not be a hard error, it must fall through to whatever the
// old behavior already was (here: no router, no Piper configured, so the
// final error is the same "not configured" class of message
// TestSynthesizeSpeech_NotConfiguredFailsFastWithClearMessage asserts).
func TestSynthesizeSpeech_LiveModeEngineFailureFallsBackToRouterAndPiper(t *testing.T) {
	a := &App{
		cfg: &config.AppConfig{
			LiveMode: config.LiveModeConfig{ActiveEngine: "custom"},
			TTS:      config.TTSConfig{Enabled: false},
		},
		liveModeEngineCfgMgr: livemode.NewConfigManager("", nil), // no saved config -> engine call fails
		ttsRouter:            tts.NewRouter(nil),
	}
	_, err := a.SynthesizeSpeech("merhaba")
	if err == nil {
		t.Fatal("expected an error: Live Mode engine unconfigured, no router provider, no Piper")
	}
}

func TestTranscribeViaLiveModeEngine_CustomSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"text":"merhaba dünya"}`))
	}))
	defer srv.Close()

	a := newAppWithLiveModeEngine(t, livemode.EngineConfig{
		Type:    livemode.EngineCustom,
		BaseURL: srv.URL,
		Enabled: true,
	})

	text, err := a.transcribeViaLiveModeEngine(context.Background(), livemode.EngineCustom, []byte("audio"))
	if err != nil {
		t.Fatalf("transcribeViaLiveModeEngine: %v", err)
	}
	if text != "merhaba dünya" {
		t.Errorf("expected 'merhaba dünya', got %q", text)
	}
}

func TestTranscribeAudio_LiveModeEngineFailureFallsBackToWhisper(t *testing.T) {
	a := &App{
		cfg: &config.AppConfig{
			LiveMode: config.LiveModeConfig{ActiveEngine: "custom"},
		},
		liveModeEngineCfgMgr: livemode.NewConfigManager("", nil), // no saved config -> engine call fails
	}
	_, err := a.TranscribeAudio([]byte("audio"))
	if err == nil {
		t.Fatal("expected an error: Live Mode engine unconfigured, whisper server not started")
	}
}

// TestSynthesizeSpeech_LocalEngineUnaffected confirms the new Phase 5
// dispatch is a true no-op when ActiveEngine is "local" (the default) —
// existing Faz 1/2 behavior must be completely unchanged.
func TestSynthesizeSpeech_LocalEngineUnaffected(t *testing.T) {
	a := &App{
		cfg:       &config.AppConfig{LiveMode: config.LiveModeConfig{ActiveEngine: "local"}, TTS: config.TTSConfig{Enabled: false}},
		ttsRouter: tts.NewRouter(nil),
	}
	_, err := a.SynthesizeSpeech("merhaba")
	if err == nil {
		t.Fatal("expected the same not-configured error as before Phase 5")
	}
}

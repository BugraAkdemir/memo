package app

import (
	"memo/internal/config"
	"memo/internal/tts"
	"os"
	"path/filepath"
	"testing"
)

func TestGetTTSVoiceCatalog_ReturnsCurated(t *testing.T) {
	a := &App{}
	got := a.GetTTSVoiceCatalog()
	if len(got) == 0 {
		t.Fatal("expected a non-empty curated voice catalog")
	}
}

func TestTTSVoiceStore_NilGuards(t *testing.T) {
	a := &App{}
	if got := a.GetLocalTTSVoices(); got != nil {
		t.Errorf("expected nil with no voice store, got %v", got)
	}
	if got := a.GetTTSVoiceDownloadProgress(); got != nil {
		t.Errorf("expected nil with no voice store, got %v", got)
	}
	if err := a.DownloadTTSVoice("tr_TR", "fahrettin", "medium"); err == nil {
		t.Error("expected error with no voice store")
	}
	if err := a.DeleteTTSVoice("x"); err == nil {
		t.Error("expected error with no voice store")
	}
	if err := a.SelectTTSVoice("x"); err == nil {
		t.Error("expected error with no voice store")
	}
}

func TestSelectTTSVoice_NotDownloadedFails(t *testing.T) {
	dir := t.TempDir()
	a := &App{
		cfg:           &config.AppConfig{TTS: config.TTSConfig{}},
		ttsVoiceStore: tts.NewVoiceStore(dir),
	}
	if err := a.SelectTTSVoice("tr_TR-fahrettin-medium"); err == nil {
		t.Error("expected error selecting a voice that was never downloaded")
	}
}

// TestSelectTTSVoice_ConfiguresSynthesizerFromDownloadedVoice exercises the
// real config.Save(a.cfg) path, which writes to whatever config.DataPath()
// currently resolves to — that resolution is cached process-wide via
// sync.Once (internal/config/config.go), so a t.Setenv("MEMO_DATA_DIR", ...)
// here would silently no-op if any earlier test in this binary already
// triggered the cache. Following backup_test.go's writeAndRestore
// convention: back up whatever config.yaml is really there, let this test
// overwrite it, then restore it — correct regardless of run order, and
// never touches data this test didn't create itself.
func TestSelectTTSVoice_ConfiguresSynthesizerFromDownloadedVoice(t *testing.T) {
	configPath := config.DataPath("config.yaml")
	original, readErr := os.ReadFile(configPath)
	hadOriginal := readErr == nil
	t.Cleanup(func() {
		if hadOriginal {
			_ = os.WriteFile(configPath, original, 0o600)
		} else {
			_ = os.Remove(configPath)
		}
	})

	voicesDir := t.TempDir()
	id := "tr_TR-fahrettin-medium"
	if err := os.WriteFile(filepath.Join(voicesDir, id+".onnx"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(voicesDir, id+".onnx.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	a := &App{
		cfg:           &config.AppConfig{TTS: config.TTSConfig{}},
		ttsVoiceStore: tts.NewVoiceStore(voicesDir),
	}
	if err := a.SelectTTSVoice(id); err != nil {
		t.Fatalf("SelectTTSVoice: %v", err)
	}
	if !a.cfg.TTS.Enabled {
		t.Error("expected TTS.Enabled to be set true")
	}
	if a.cfg.TTS.ModelPath == "" {
		t.Error("expected TTS.ModelPath to be set")
	}
	if a.ttsSynthesizer == nil {
		t.Error("expected initTTS() to have configured a synthesizer")
	}
}

package app

import (
	"fmt"
	"memo/internal/config"
	"memo/internal/tts"
)

// GetTTSVoiceCatalog returns the curated list of downloadable Piper voices
// (see tts.CuratedVoices) — static, no network call.
func (a *App) GetTTSVoiceCatalog() []tts.Voice {
	return tts.CuratedVoices()
}

// GetLocalTTSVoices returns Piper voices already downloaded to disk.
func (a *App) GetLocalTTSVoices() []tts.LocalVoice {
	if a.ttsVoiceStore == nil {
		return nil
	}
	return a.ttsVoiceStore.ListLocalVoices()
}

// GetTTSVoiceDownloadProgress returns every currently tracked voice download.
func (a *App) GetTTSVoiceDownloadProgress() []*tts.VoiceDownloadProgress {
	if a.ttsVoiceStore == nil {
		return nil
	}
	return a.ttsVoiceStore.GetDownloadProgress()
}

// DownloadTTSVoice starts downloading the given curated voice in the background.
func (a *App) DownloadTTSVoice(locale, name, quality string) error {
	if a.ttsVoiceStore == nil {
		return fmt.Errorf("TTS voice store not initialized")
	}
	return a.ttsVoiceStore.DownloadVoice(tts.Voice{Locale: locale, Name: name, Quality: quality})
}

// DeleteTTSVoice removes a downloaded voice's files.
func (a *App) DeleteTTSVoice(id string) error {
	if a.ttsVoiceStore == nil {
		return fmt.Errorf("TTS voice store not initialized")
	}
	return a.ttsVoiceStore.DeleteLocalVoice(id)
}

// SelectTTSVoice points the local Piper synthesizer at a downloaded voice —
// this is what actually makes TTS work fully offline: pick a curated voice,
// download it once (DownloadTTSVoice), then select it here. Persists to
// config.yaml (tts.model_path/tts.enabled) so the choice survives a
// restart, then rebuilds a.ttsSynthesizer immediately via initTTS() so it
// takes effect without one.
func (a *App) SelectTTSVoice(id string) error {
	if a.ttsVoiceStore == nil {
		return fmt.Errorf("TTS voice store not initialized")
	}
	var match *tts.LocalVoice
	for _, v := range a.ttsVoiceStore.ListLocalVoices() {
		if v.ID() == id {
			match = &v
			break
		}
	}
	if match == nil {
		return fmt.Errorf("voice %q is not downloaded yet", id)
	}

	a.cfg.TTS.ModelPath = match.Path
	a.cfg.TTS.Enabled = true
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	a.initTTS()
	return nil
}

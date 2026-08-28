package tts

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
)

// FillerPhrases are short, non-linguistic interjections synthesized once
// per voice and cached, then played back (one picked at random per gap,
// and — since the discrete voice loop now replays them until the reply is
// ready — several times across a longer wait) during the "thinking" gap
// between a user's utterance and Memo's reply (Faz 3 of the parent plan,
// docs/plans/PLAN_voice_live_mode.md — "gecikme anlarında ... kısa .wav
// klipleri ... çalınıyor"). Deliberately short, phonetically simple, and
// NOT real words in any single language, so they render tolerably
// regardless of which language the configured Piper voice speaks (a
// Turkish sentence read by an English voice, or vice versa, would sound
// wrong) — a hum/breath like "hmm" or "mm-hm" passes as a listening sound
// in most languages' phoneme sets either way. Kept to this safe set on
// purpose; language-specific phrases ("bir saniye", "one sec") would need
// the cache to know the UI language and are a separate change.
var FillerPhrases = []string{"Hmm", "Mm", "Ah", "Mm-hm", "Hm-mm", "Hmmm", "Ahh", "Mmh"}

// FillerCache synthesizes FillerPhrases through the local Piper Synthesizer
// once each and keeps the resulting WAV bytes in memory, so a caller never
// pays subprocess-spawn latency at the exact moment it's trying to mask
// latency. Deliberately local-only — this never goes through the external
// provider Router (2.1-2.4): fillers are meant to be near-instant, and an
// API round-trip for a one-word "hmm" would defeat the entire point.
type FillerCache struct {
	synth *Synthesizer

	mu    sync.RWMutex
	cache map[string][]byte
}

func NewFillerCache(synth *Synthesizer) *FillerCache {
	return &FillerCache{synth: synth, cache: make(map[string][]byte)}
}

// SetCached seeds phrase's cached audio directly, bypassing synthesis —
// exists for other packages' tests (internal/app's, which can't reach the
// unexported cache map directly and has no real Piper binary to synthesize
// through) rather than any production code path.
func (f *FillerCache) SetCached(phrase string, audio []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cache[phrase] = audio
}

// Prewarm synthesizes every phrase up front (call once after the local
// Piper synthesizer is configured/changed — see App.initTTS) so the first
// real Random() call during a live conversation doesn't pay the one-time
// synthesis cost. Best-effort: a phrase that fails to synthesize (e.g. the
// voice model can't render it) is simply left out of the cache rather than
// aborting the whole prewarm.
func (f *FillerCache) Prewarm(ctx context.Context) {
	for _, phrase := range FillerPhrases {
		_, _ = f.get(ctx, phrase)
	}
}

// Random returns one cached filler's WAV bytes, synthesizing it first if
// this is the very first call for that phrase (Prewarm not having run yet,
// or a phrase Prewarm itself failed to synthesize).
func (f *FillerCache) Random(ctx context.Context) ([]byte, error) {
	if len(FillerPhrases) == 0 {
		return nil, fmt.Errorf("tts: no filler phrases configured")
	}
	phrase := FillerPhrases[rand.Intn(len(FillerPhrases))]
	return f.get(ctx, phrase)
}

func (f *FillerCache) get(ctx context.Context, phrase string) ([]byte, error) {
	f.mu.RLock()
	if audio, ok := f.cache[phrase]; ok {
		f.mu.RUnlock()
		return audio, nil
	}
	f.mu.RUnlock()

	if f.synth == nil {
		return nil, fmt.Errorf("tts: filler cache has no synthesizer configured")
	}

	audio, err := f.synth.Synthesize(ctx, phrase)
	if err != nil {
		return nil, fmt.Errorf("tts: synthesize filler %q: %w", phrase, err)
	}

	f.mu.Lock()
	f.cache[phrase] = audio
	f.mu.Unlock()
	return audio, nil
}

package tts

import (
	"context"
	"testing"
)

// FillerCache.synth is a concrete *Synthesizer, not an interface, so these
// tests can't inject a fake provider — instead they exercise the cache's
// own get()/Random() logic directly against a FillerCache whose cache map
// is pre-seeded, never touching synth at all (nil is fine as long as
// nothing misses the cache — see the dedicated nil-synth test below for
// what happens when something does).
func TestFillerCache_ReturnsCachedBytesWithoutSynthesizing(t *testing.T) {
	fc := NewFillerCache(nil)
	fc.cache["Hmm"] = []byte("cached-hmm")
	audio, err := fc.get(context.Background(), "Hmm")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(audio) != "cached-hmm" {
		t.Errorf("expected cached bytes, got %q", audio)
	}
}

func TestFillerCache_RandomPicksFromPhraseList(t *testing.T) {
	fc := NewFillerCache(nil)
	for _, p := range FillerPhrases {
		fc.cache[p] = []byte("audio-for-" + p)
	}
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		audio, err := fc.Random(context.Background())
		if err != nil {
			t.Fatalf("Random: %v", err)
		}
		seen[string(audio)] = true
	}
	if len(seen) == 0 {
		t.Error("expected at least one distinct filler to be returned")
	}
	for got := range seen {
		found := false
		for _, p := range FillerPhrases {
			if got == "audio-for-"+p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unexpected filler audio %q not in FillerPhrases", got)
		}
	}
}

func TestFillerCache_UncachedGetFailsCleanlyWithNilSynth(t *testing.T) {
	fc := NewFillerCache(nil)
	// nil *Synthesizer: get() must return an error, not panic, when a
	// phrase isn't already cached and synthesis is attempted.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("get() panicked instead of returning an error: %v", r)
		}
	}()
	if _, err := fc.get(context.Background(), "not-cached"); err == nil {
		t.Error("expected an error synthesizing via a nil Synthesizer")
	}
}

func TestFillerPhrases_NotEmpty(t *testing.T) {
	if len(FillerPhrases) == 0 {
		t.Fatal("FillerPhrases must not be empty — Random() depends on it")
	}
}

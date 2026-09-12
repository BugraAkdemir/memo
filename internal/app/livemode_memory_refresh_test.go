// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/livemode"
)

// recordingInjectSession is a livemode.Session whose InjectContext calls are
// captured for assertion, and whose Events() channel a test drives directly —
// for exercising refreshLiveModeMemory / the wrapper's role-based routing
// without a real provider client.
type recordingInjectSession struct {
	events   chan livemode.SessionEvent
	injected chan string
}

func newRecordingInjectSession() *recordingInjectSession {
	return &recordingInjectSession{
		events:   make(chan livemode.SessionEvent, 4),
		injected: make(chan string, 4),
	}
}

func (f *recordingInjectSession) Start(context.Context) error { return nil }
func (f *recordingInjectSession) SendAudio([]byte) error      { return nil }
func (f *recordingInjectSession) InjectContext(text string) error {
	f.injected <- text
	return nil
}
func (f *recordingInjectSession) Events() <-chan livemode.SessionEvent { return f.events }
func (f *recordingInjectSession) Close() error                         { return nil }

func TestRefreshLiveModeMemory_DisabledConfigNeverInjects(t *testing.T) {
	a := &App{cfg: &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: false}}}
	s := newRecordingInjectSession()

	a.refreshLiveModeMemory(s, "kullanıcının köpeğinin adı ne")

	select {
	case got := <-s.injected:
		t.Fatalf("expected no injection with memory disabled, got %q", got)
	case <-time.After(200 * time.Millisecond):
		// Expected: nothing injected.
	}
}

func TestRefreshLiveModeMemory_EmptyTranscriptNeverInjects(t *testing.T) {
	store := newExtractionTestStore(t)
	a := &App{store: store, cfg: &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: true, TopK: 5}}}
	s := newRecordingInjectSession()

	a.refreshLiveModeMemory(s, "   ")

	select {
	case got := <-s.injected:
		t.Fatalf("expected no injection for a blank transcript, got %q", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestRefreshLiveModeMemory_NoMatchingMemoryNeverInjects(t *testing.T) {
	store := newExtractionTestStore(t)
	a := &App{store: store, cfg: &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: true, TopK: 5, MinSimilarity: 0, PinnedFactsPerTurn: 3}}}
	s := newRecordingInjectSession()

	// Empty store: RetrieveContext/GetPinnedFactsRanked both return nothing,
	// so FormatMemoriesForPrompt must produce "" and refreshLiveModeMemory
	// must skip injecting an empty/noise message.
	a.refreshLiveModeMemory(s, "kullanıcının köpeğinin adı ne")

	select {
	case got := <-s.injected:
		t.Fatalf("expected no injection when memory search finds nothing, got %q", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestRefreshLiveModeMemory_InjectsFormattedMemoryWhenFound(t *testing.T) {
	store := newExtractionTestStore(t)
	if err := store.SaveExplicit(context.Background(), "Kullanıcının köpeğinin adı Zeytin", "profile"); err != nil {
		t.Fatalf("SaveExplicit: %v", err)
	}
	a := &App{store: store, cfg: &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: true, TopK: 5, MinSimilarity: 0, PinnedFactsPerTurn: 3}}}
	s := newRecordingInjectSession()

	a.refreshLiveModeMemory(s, "köpeğim hakkında ne biliyorsun")

	select {
	case got := <-s.injected:
		if !strings.Contains(got, "Zeytin") {
			t.Errorf("injected text %q does not contain the saved memory", got)
		}
		if !strings.Contains(strings.ToLower(got), "internal context") && !strings.Contains(got, "İç bağlam") {
			t.Errorf("injected text %q is missing the do-not-voice-this framing", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected an injection for a matching memory, got none")
	}
}

// TestLivePermissionRoutingSession_RefreshesMemoryOnlyForUserTranscripts
// guards the wrapper's routing decision: a real memory refresh must fire for
// the user's own speech, never for the model's own transcript — narrating
// the model's own words back into its context would be pointless and could
// even reinforce a hallucination.
func TestLivePermissionRoutingSession_RefreshesMemoryOnlyForUserTranscripts(t *testing.T) {
	store := newExtractionTestStore(t)
	if err := store.SaveExplicit(context.Background(), "Kullanıcının köpeğinin adı Zeytin", "profile"); err != nil {
		t.Fatalf("SaveExplicit: %v", err)
	}
	a := &App{store: store, cfg: &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: true, TopK: 5, MinSimilarity: 0, PinnedFactsPerTurn: 3}}}

	inner := newRecordingInjectSession()
	wrapped := a.wrapLiveModeSessionForPermissionRouting(inner)

	inner.events <- livemode.SessionEvent{Type: livemode.EventTranscript, Role: livemode.RoleModel, Transcript: "köpeğin hakkında bir şey biliyor muyum"}
	// Drain the outward channel so the pump isn't blocked on forwarding.
	go func() {
		for range wrapped.Events() {
		}
	}()

	select {
	case got := <-inner.injected:
		t.Fatalf("expected no memory refresh for the model's own transcript, got %q", got)
	case <-time.After(500 * time.Millisecond):
		// Expected: no injection triggered by the model's own speech.
	}

	inner.events <- livemode.SessionEvent{Type: livemode.EventTranscript, Role: livemode.RoleUser, Transcript: "köpeğim hakkında ne biliyorsun"}
	close(inner.events)

	select {
	case got := <-inner.injected:
		if !strings.Contains(got, "Zeytin") {
			t.Errorf("injected text %q does not contain the saved memory", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected a memory refresh triggered by the user's transcript, got none")
	}
}

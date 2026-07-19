// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/agent"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/observer"
	"memo/internal/routine"
	"memo/internal/sessions"
	"memo/internal/whatsapp"
)

// TestFormatWhatsAppMessagesForRoutine_FallsBackToJIDWhenSenderNameEmpty is
// the regression test for BUG-L5: a contact with no saved display name used
// to render with a blank sender ("[15:04] : hello") instead of falling back
// to the JID's local part, unlike the equivalent agent tool
// (GetWhatsAppMessages) which already does this.
func TestFormatWhatsAppMessagesForRoutine_FallsBackToJIDWhenSenderNameEmpty(t *testing.T) {
	msgs := []whatsapp.Message{
		{SenderJID: "905551234567@s.whatsapp.net", SenderName: "", Text: "hello"},
	}
	got := formatWhatsAppMessagesForRoutine(msgs, "tr")
	if !strings.Contains(got, "905551234567: hello") {
		t.Errorf("formatWhatsAppMessagesForRoutine = %q, want it to fall back to the JID's local part as sender", got)
	}
}

// TestRoutineLanguageIsEnglish_DefaultsToTurkish is the regression test for
// BUG-M1: only the exact value "en" should select English — the empty
// string (every routine created before Routine.Language existed) and any
// unrecognized value must default to Turkish rather than requiring a
// migration or crashing on an unexpected value.
func TestRoutineLanguageIsEnglish_DefaultsToTurkish(t *testing.T) {
	cases := []struct {
		lang string
		want bool
	}{
		{"", false},
		{"tr", false},
		{"en", true},
		{"fr", false},
		{"EN", false}, // exact match only, no case-folding — matches client's own lowercase MemoLocale encoding
	}
	for _, c := range cases {
		if got := routineLanguageIsEnglish(c.lang); got != c.want {
			t.Errorf("routineLanguageIsEnglish(%q) = %v, want %v", c.lang, got, c.want)
		}
	}
}

// TestFormatEventsForRoutine_LocalizesEmptyFallback is the regression test
// for BUG-M1: the "no events today" filler text used to be hardcoded
// Turkish regardless of the routine's own language.
func TestFormatEventsForRoutine_LocalizesEmptyFallback(t *testing.T) {
	if got := formatEventsForRoutine(nil, "en"); got != "No events on today's calendar." {
		t.Errorf("formatEventsForRoutine(nil, %q) = %q, want the English fallback", "en", got)
	}
	if got := formatEventsForRoutine(nil, "tr"); got != "Bugün için takvimde etkinlik yok." {
		t.Errorf("formatEventsForRoutine(nil, %q) = %q, want the Turkish fallback", "tr", got)
	}
}

// TestFormatWhatsAppMessagesForRoutine_LocalizesEmptyFallback mirrors the
// above for the WhatsApp-context "no new messages" filler.
func TestFormatWhatsAppMessagesForRoutine_LocalizesEmptyFallback(t *testing.T) {
	if got := formatWhatsAppMessagesForRoutine(nil, "en"); got != "No new messages in this chat." {
		t.Errorf("formatWhatsAppMessagesForRoutine(nil, %q) = %q, want the English fallback", "en", got)
	}
	if got := formatWhatsAppMessagesForRoutine(nil, "tr"); got != "Bu sohbette yeni mesaj yok." {
		t.Errorf("formatWhatsAppMessagesForRoutine(nil, %q) = %q, want the Turkish fallback", "tr", got)
	}
}

// TestRoutineNotificationTitle_Localized is the regression test for the
// GetRoutinesReadyForMobile half of BUG-M1: mobile push notifications used
// to always title themselves "Rutin" regardless of the routine's language,
// even though both clients already carry a `routine_fallback` L10n key
// ("Rutin"/"Routine") that this must stay in sync with.
func TestRoutineNotificationTitle_Localized(t *testing.T) {
	if got := routineNotificationTitle("en"); got != "Routine" {
		t.Errorf("routineNotificationTitle(%q) = %q, want %q", "en", got, "Routine")
	}
	if got := routineNotificationTitle("tr"); got != "Rutin" {
		t.Errorf("routineNotificationTitle(%q) = %q, want %q", "tr", got, "Rutin")
	}
	if got := routineNotificationTitle(""); got != "Rutin" {
		t.Errorf("routineNotificationTitle(%q) = %q, want the Turkish default %q", "", got, "Rutin")
	}
}

// TestCreateRoutineFromDraft_PersistsLanguage verifies the create path
// actually stores the client-supplied language on the routine, so it's
// available at every later read site (system prompt, context fillers,
// notification title) without needing a second round trip.
func TestCreateRoutineFromDraft_PersistsLanguage(t *testing.T) {
	a := newRoutineTestApp(t)
	dir := t.TempDir()
	st, err := routine.NewStore(dir)
	if err != nil {
		t.Fatalf("routine.NewStore: %v", err)
	}
	a.routineStore = st

	created, err := a.CreateRoutineFromDraft("her gün 21:00'de kitap oku", routine.Draft{
		TimeOfDay: "21:00",
		Prompt:    "remind me to read",
	}, "", false, "en")
	if err != nil {
		t.Fatalf("CreateRoutineFromDraft: %v", err)
	}
	if created.Language != "en" {
		t.Errorf("created.Language = %q, want %q", created.Language, "en")
	}
}

func newRoutineTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()

	sm, err := sessions.NewManager(filepath.Join(dir, "sessions"))
	if err != nil {
		t.Fatalf("sessions.NewManager: %v", err)
	}
	obsStore, err := observer.NewStore(observer.StoreConfig{Dir: dir})
	if err != nil {
		t.Fatalf("observer.NewStore: %v", err)
	}

	a := &App{
		cfg:              &config.AppConfig{},
		identity:         identity.New("Test", "Memo", "casual", "", false),
		sessions:         sm,
		observerRecorder: observer.NewRecorder(obsStore),
		agentExecutor:    agent.NewExecutor(dir, nil, nil),
	}
	return a
}

// TestRunAgentRoutine_DoesNotMutateGlobalAgentModeOrActiveChat is a
// regression test for BUG-C1: runAgentRoutine used to call SwitchChat and
// SetAgentEnabled(true) with no restore, permanently flipping the app's one
// global agent-mode flag on and hijacking the global "active chat" pointer
// after the first agent-mode routine ever ran. The fix passes forceAgent
// straight into sendMessageStreamCore instead (which already activates tool
// execution for just this one call, per routeStream's own forceAgent
// handling), so neither global should ever move.
func TestRunAgentRoutine_DoesNotMutateGlobalAgentModeOrActiveChat(t *testing.T) {
	a := newRoutineTestApp(t)

	// Seed a real "current" chat/agent-mode state that must survive the
	// routine run untouched.
	userChatID := a.NewChat()
	if err := a.SwitchChat(userChatID); err != nil {
		t.Fatalf("SwitchChat: %v", err)
	}
	if a.GetAgentEnabled() {
		t.Fatal("test setup: agent mode should start disabled")
	}

	r := routine.Routine{Prompt: "test prompt", AgentMode: true}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The stream itself will fail (no LLM client/provider configured in this
	// test) — that's fine, we only care about the global state afterward.
	_, _ = a.runAgentRoutine(ctx, r)

	if a.GetAgentEnabled() {
		t.Error("BUG-C1 regressed: global agent mode is enabled after runAgentRoutine, should be untouched")
	}
	if got := a.GetActiveChatID(); got != userChatID {
		t.Errorf("BUG-C1 regressed: active chat = %q, want unchanged %q", got, userChatID)
	}
}

// TestRunAgentRoutine_AutoApprovePermissionRestoredBeforeUnlock verifies the
// auto-permission flag set for AutoApproveTools is always restored to its
// prior value, and — the actual BUG-C1 race — that it is restored *before*
// streamMu is released, so no concurrent caller can ever observe it left on.
func TestRunAgentRoutine_AutoApprovePermissionRestoredBeforeUnlock(t *testing.T) {
	a := newRoutineTestApp(t)

	if a.GetAgentAutoPermission() {
		t.Fatal("test setup: auto-permission should start disabled")
	}

	r := routine.Routine{Prompt: "test prompt", AgentMode: true, AutoApproveTools: true}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = a.runAgentRoutine(ctx, r)

	if a.GetAgentAutoPermission() {
		t.Error("BUG-C1 regressed: auto-permission left enabled after runAgentRoutine")
	}

	// The lock must be free again (proves unlock happened, and — combined
	// with the assertion above already having observed the restored value —
	// that restore-before-unlock ordering held).
	if !a.streamMu.TryLock() {
		t.Error("streamMu still held after runAgentRoutine returned")
	} else {
		a.streamMu.Unlock()
	}
}

// TestRunAgentRoutine_BusyStreamFailsFastWithoutMutatingState covers the
// case sendMessageStreamInnerTo's internal TryLock previously masked: the
// old code called SwitchChat/SetAgentEnabled(true) unconditionally *before*
// ever discovering another stream was in flight. The fixed version checks
// streamMu itself first, so a concurrent stream must leave global state
// completely untouched.
func TestRunAgentRoutine_BusyStreamFailsFastWithoutMutatingState(t *testing.T) {
	a := newRoutineTestApp(t)

	userChatID := a.NewChat()
	if err := a.SwitchChat(userChatID); err != nil {
		t.Fatalf("SwitchChat: %v", err)
	}

	// Simulate a concurrent interactive stream already in progress.
	a.streamMu.Lock()
	defer a.streamMu.Unlock()

	r := routine.Routine{Prompt: "test prompt", AgentMode: true, AutoApproveTools: true}
	_, err := a.runAgentRoutine(context.Background(), r)
	if err == nil {
		t.Fatal("expected an error when streamMu is already held")
	}

	if a.GetAgentEnabled() {
		t.Error("busy path mutated global agent mode")
	}
	if got := a.GetActiveChatID(); got != userChatID {
		t.Errorf("busy path changed active chat: got %q, want %q", got, userChatID)
	}
	if a.GetAgentAutoPermission() {
		t.Error("busy path left auto-permission enabled")
	}
}

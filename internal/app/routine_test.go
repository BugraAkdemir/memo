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
	"memo/internal/telegram"
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

	offset := 180
	created, err := a.CreateRoutineFromDraft("her gün 21:00'de kitap oku", routine.Draft{
		TimeOfDay: "21:00",
		Prompt:    "remind me to read",
	}, "", false, "en", &offset)
	if err != nil {
		t.Fatalf("CreateRoutineFromDraft: %v", err)
	}
	if created.Language != "en" {
		t.Errorf("created.Language = %q, want %q", created.Language, "en")
	}
	if created.Schedule.UTCOffsetMinutes == nil || *created.Schedule.UTCOffsetMinutes != 180 {
		t.Errorf("created.Schedule.UTCOffsetMinutes = %v, want pointer to 180", created.Schedule.UTCOffsetMinutes)
	}
}

// TestResolveRoutineDeliveryTarget_SelfChatSourceIsAlwaysForced is the
// actual security-boundary test for create_routine (the user's own
// reported concern: they must never be able to pick an arbitrary WhatsApp
// contact via the conversational tool the way the Routines-tab UI's human
// picker can). A WhatsApp/Telegram self-chat source on ctx must force that
// exact target, completely independent of whatever connectivity state
// waClient/tgStore happen to be in — confirmed here with both left nil
// entirely, which would panic if the function ever fell through to reading
// them in this branch.
func TestResolveRoutineDeliveryTarget_SelfChatSourceIsAlwaysForced(t *testing.T) {
	a := &App{} // waClient, tgStore both nil — must never be touched below

	t.Run("whatsapp_source_forces_that_exact_jid", func(t *testing.T) {
		ctx := withSelfChatSource(context.Background(), SelfChatSource{WhatsApp: true, WhatsAppJID: "905551234567@s.whatsapp.net"})
		jid, wa, tg := a.resolveRoutineDeliveryTarget(ctx)
		if jid != "905551234567@s.whatsapp.net" || !wa || tg {
			t.Errorf("resolveRoutineDeliveryTarget() = (%q, %v, %v), want (the self-chat JID, true, false)", jid, wa, tg)
		}
	})

	t.Run("telegram_source_forces_telegram_only", func(t *testing.T) {
		ctx := withSelfChatSource(context.Background(), SelfChatSource{Telegram: true, TelegramChatID: 555})
		jid, wa, tg := a.resolveRoutineDeliveryTarget(ctx)
		if jid != "" || wa || !tg {
			t.Errorf("resolveRoutineDeliveryTarget() = (%q, %v, %v), want (\"\", false, true)", jid, wa, tg)
		}
	})
}

// TestResolveRoutineDeliveryTarget_NormalChatDefaultsToConnectedSurfaces
// covers the "no self-chat source at all" case (a normal, non-self-chat
// agent conversation calling create_routine) — delivery should default to
// whichever surfaces are actually connected, never require the model to
// supply a target.
func TestResolveRoutineDeliveryTarget_NormalChatDefaultsToConnectedSurfaces(t *testing.T) {
	t.Run("nothing_connected_enables_nothing", func(t *testing.T) {
		a := &App{}
		jid, wa, tg := a.resolveRoutineDeliveryTarget(context.Background())
		if jid != "" || wa || tg {
			t.Errorf("resolveRoutineDeliveryTarget() = (%q, %v, %v), want all zero/false with nothing connected", jid, wa, tg)
		}
	})

	t.Run("linked_telegram_bot_is_enabled", func(t *testing.T) {
		store := telegram.NewStore(filepath.Join(t.TempDir(), "telegram.json"), testMasterKey)
		store.SetOwner(999, "Bugra")
		a := &App{tgStore: store}

		jid, wa, tg := a.resolveRoutineDeliveryTarget(context.Background())
		if jid != "" || wa {
			t.Errorf("resolveRoutineDeliveryTarget() WhatsApp side = (%q, %v), want (\"\", false) — no WhatsApp client at all", jid, wa)
		}
		if !tg {
			t.Error("expected Telegram delivery to default on when a bot is linked")
		}
	})
}

// TestRunRoutineDeliver_FiresEachChannelIndependently confirms both
// channels are attempted even if one is missing its target/fails, and a
// failure on one doesn't prevent (or hide) delivery on the other — see
// runRoutineDeliver's own doc comment.
func TestRunRoutineDeliver_FiresEachChannelIndependently(t *testing.T) {
	a := newRoutineTestApp(t)

	t.Run("whatsapp_only_no_target_configured", func(t *testing.T) {
		r := routine.Routine{DeliveryWhatsApp: true, WhatsAppTargetJID: ""}
		err := a.runRoutineDeliver(context.Background(), r, "content")
		if err == nil {
			t.Error("expected an error when DeliveryWhatsApp is set but WhatsAppTargetJID is empty")
		}
	})

	t.Run("neither_channel_enabled_is_a_no_op", func(t *testing.T) {
		r := routine.Routine{}
		if err := a.runRoutineDeliver(context.Background(), r, "content"); err != nil {
			t.Errorf("expected no error when neither channel is enabled, got %v", err)
		}
	})

	t.Run("telegram_only_no_target_configured", func(t *testing.T) {
		r := routine.Routine{DeliveryTelegram: true, TelegramTargetChatID: 0}
		err := a.runRoutineDeliver(context.Background(), r, "content")
		if err == nil {
			t.Error("expected an error when DeliveryTelegram is set but TelegramTargetChatID is 0")
		}
	})

	t.Run("both_missing_targets_reports_both_errors", func(t *testing.T) {
		r := routine.Routine{DeliveryWhatsApp: true, DeliveryTelegram: true}
		err := a.runRoutineDeliver(context.Background(), r, "content")
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "whatsapp") || !strings.Contains(err.Error(), "no telegram target") {
			t.Errorf("expected errors.Join to report both failures, got: %v", err)
		}
	})
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

// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/agent"
	"memo/internal/calendar"
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

// TestListRoutinesForChat_FormatsEveryRoutineWithItsRealID confirms the
// list_routines tool's actual purpose: cancel_routine can never guess an
// ID, so the listing must expose the real one for every routine, alongside
// enough detail (time, days, channels, prompt) for the model to tell them
// apart when the user refers to "the news one" or similar.
func TestListRoutinesForChat_FormatsEveryRoutineWithItsRealID(t *testing.T) {
	a := newRoutineTestApp(t)
	st, err := routine.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("routine.NewStore: %v", err)
	}
	a.routineStore = st

	t.Run("empty_store", func(t *testing.T) {
		out, err := a.ListRoutinesForChat(context.Background())
		if err != nil {
			t.Fatalf("ListRoutinesForChat: %v", err)
		}
		if out == "" {
			t.Error("expected a non-empty 'no routines' message for an empty store")
		}
	})

	created, err := a.CreateRoutineFromDraft("haberleri getir", routine.Draft{
		TimeOfDay:        "09:00",
		Prompt:           "yapay zeka haberlerini getir",
		DeliveryWhatsApp: true,
	}, "905551234567@s.whatsapp.net", false, "tr", nil)
	if err != nil {
		t.Fatalf("CreateRoutineFromDraft: %v", err)
	}

	out, err := a.ListRoutinesForChat(context.Background())
	if err != nil {
		t.Fatalf("ListRoutinesForChat: %v", err)
	}
	if !strings.Contains(out, created.ID) {
		t.Errorf("listing = %q, want it to contain the real id %q", out, created.ID)
	}
	if !strings.Contains(out, "09:00") || !strings.Contains(out, "yapay zeka haberlerini getir") || !strings.Contains(out, "WhatsApp") {
		t.Errorf("listing = %q, missing expected detail (time/prompt/channel)", out)
	}
}

// TestDeleteRoutineForChat_ActuallyDeletesAndRejectsUnknownID is the
// cancellation regression test — "bu rutinimi iptal et" must actually
// remove it from the store, not just claim to, and a made-up/stale id must
// fail loudly rather than silently no-op.
func TestDeleteRoutineForChat_ActuallyDeletesAndRejectsUnknownID(t *testing.T) {
	a := newRoutineTestApp(t)
	st, err := routine.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("routine.NewStore: %v", err)
	}
	a.routineStore = st

	created, err := a.CreateRoutineFromDraft("haberleri getir", routine.Draft{TimeOfDay: "09:00", Prompt: "x"}, "", false, "tr", nil)
	if err != nil {
		t.Fatalf("CreateRoutineFromDraft: %v", err)
	}

	if _, err := a.DeleteRoutineForChat(context.Background(), "does-not-exist"); err == nil {
		t.Error("expected an error for an unknown id")
	}
	if got := a.ListRoutines(); len(got) != 1 {
		t.Fatalf("expected the real routine to be untouched by the failed delete, got %d routines", len(got))
	}

	summary, err := a.DeleteRoutineForChat(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("DeleteRoutineForChat: %v", err)
	}
	if summary == "" {
		t.Error("expected a non-empty confirmation summary")
	}
	if got := a.ListRoutines(); len(got) != 0 {
		t.Errorf("expected the routine to actually be gone, got %d routines still present", len(got))
	}
}

// TestResolveRoutineDeliveryTarget_SelfChatSourceIsAlwaysForced is the
// actual security-boundary test for create_routine (the user's own
// reported concern: they must never be able to pick an arbitrary WhatsApp
// contact via the conversational tool the way the Routines-tab UI's human
// picker can). A WhatsApp/Telegram self-chat source on ctx must force that
// exact target on, completely independent of whatever connectivity state
// waClient/tgStore happen to be in — confirmed here with both left nil
// entirely (draftWants*=false), which would panic if the function ever
// fell through to reading them in this branch.
func TestResolveRoutineDeliveryTarget_SelfChatSourceIsAlwaysForced(t *testing.T) {
	a := &App{} // waClient, tgStore both nil — must never be touched below

	t.Run("whatsapp_source_forces_that_exact_jid", func(t *testing.T) {
		ctx := withSelfChatSource(context.Background(), SelfChatSource{WhatsApp: true, WhatsAppJID: "905551234567@s.whatsapp.net"})
		jid, wa, tg := a.resolveRoutineDeliveryTarget(ctx, false, false)
		if jid != "905551234567@s.whatsapp.net" || !wa || tg {
			t.Errorf("resolveRoutineDeliveryTarget() = (%q, %v, %v), want (the self-chat JID, true, false)", jid, wa, tg)
		}
	})

	t.Run("telegram_source_forces_telegram_only", func(t *testing.T) {
		ctx := withSelfChatSource(context.Background(), SelfChatSource{Telegram: true, TelegramChatID: 555})
		jid, wa, tg := a.resolveRoutineDeliveryTarget(ctx, false, false)
		if jid != "" || wa || !tg {
			t.Errorf("resolveRoutineDeliveryTarget() = (%q, %v, %v), want (\"\", false, true)", jid, wa, tg)
		}
	})
}

// TestResolveRoutineDeliveryTarget_ExplicitlyRequestedOtherChannelIsAdded
// is the regression test for the user's own live test scenario: "buradan
// (WhatsApp self-chat) hem Telegram hem WhatsApp'a gönder" must actually
// enable both, not silently drop the one that isn't the current surface —
// as long as the added channel still only ever resolves to the user's own
// already-connected surface, never anything the model could have invented.
func TestResolveRoutineDeliveryTarget_ExplicitlyRequestedOtherChannelIsAdded(t *testing.T) {
	t.Run("whatsapp_self_chat_also_wants_telegram", func(t *testing.T) {
		store := telegram.NewStore(filepath.Join(t.TempDir(), "telegram.json"), testMasterKey)
		store.SetOwner(999, "Bugra")
		a := &App{tgStore: store}

		ctx := withSelfChatSource(context.Background(), SelfChatSource{WhatsApp: true, WhatsAppJID: "x@s.whatsapp.net"})
		jid, wa, tg := a.resolveRoutineDeliveryTarget(ctx, false, true)
		if jid != "x@s.whatsapp.net" || !wa {
			t.Errorf("WhatsApp side = (%q, %v), want the forced self-chat JID still on", jid, wa)
		}
		if !tg {
			t.Error("expected Telegram to be added on top since the text explicitly asked for it and a bot is linked")
		}
	})

	t.Run("whatsapp_self_chat_wants_telegram_but_none_linked_stays_whatsapp_only", func(t *testing.T) {
		a := &App{} // no tgStore at all — nothing linked to add
		ctx := withSelfChatSource(context.Background(), SelfChatSource{WhatsApp: true, WhatsAppJID: "x@s.whatsapp.net"})
		_, wa, tg := a.resolveRoutineDeliveryTarget(ctx, false, true)
		if !wa {
			t.Error("expected WhatsApp to stay on")
		}
		if tg {
			t.Error("expected Telegram to stay off — asking for it doesn't conjure a link that doesn't exist")
		}
	})

	t.Run("not_requesting_the_other_channel_leaves_it_off", func(t *testing.T) {
		store := telegram.NewStore(filepath.Join(t.TempDir(), "telegram.json"), testMasterKey)
		store.SetOwner(999, "Bugra")
		a := &App{tgStore: store}

		ctx := withSelfChatSource(context.Background(), SelfChatSource{WhatsApp: true, WhatsAppJID: "x@s.whatsapp.net"})
		_, _, tg := a.resolveRoutineDeliveryTarget(ctx, false, false)
		if tg {
			t.Error("expected Telegram to stay off when the text never asked for it, even though a bot is linked")
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
		jid, wa, tg := a.resolveRoutineDeliveryTarget(context.Background(), false, false)
		if jid != "" || wa || tg {
			t.Errorf("resolveRoutineDeliveryTarget() = (%q, %v, %v), want all zero/false with nothing connected", jid, wa, tg)
		}
	})

	t.Run("linked_telegram_bot_is_enabled", func(t *testing.T) {
		store := telegram.NewStore(filepath.Join(t.TempDir(), "telegram.json"), testMasterKey)
		store.SetOwner(999, "Bugra")
		a := &App{tgStore: store}

		jid, wa, tg := a.resolveRoutineDeliveryTarget(context.Background(), false, false)
		if jid != "" || wa {
			t.Errorf("resolveRoutineDeliveryTarget() WhatsApp side = (%q, %v), want (\"\", false) — no WhatsApp client at all", jid, wa)
		}
		if !tg {
			t.Error("expected Telegram delivery to default on when a bot is linked")
		}
	})
}

// TestCreateRoutineFromDraft_AgentModeAlwaysOnAutoApprovePassesThrough is the
// BUG-M6 regression test: AgentMode used to come straight from the
// extractor's NeedsAgentMode guess (and AutoApproveTools was force-false
// whenever that guess was false) — now every routine gets AgentMode=true
// unconditionally, regardless of what the draft says, and AutoApproveTools
// passes straight through from the caller with no gate at all.
func TestCreateRoutineFromDraft_AgentModeAlwaysOnAutoApprovePassesThrough(t *testing.T) {
	a := newRoutineTestApp(t)
	st, err := routine.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("routine.NewStore: %v", err)
	}
	a.routineStore = st

	notNeeded, err := a.CreateRoutineFromDraft("x", routine.Draft{TimeOfDay: "09:00", NeedsAgentMode: false}, "", true, "tr", nil)
	if err != nil {
		t.Fatalf("CreateRoutineFromDraft: %v", err)
	}
	if !notNeeded.AgentMode {
		t.Error("AgentMode should be true even when the draft's NeedsAgentMode guess was false")
	}
	if !notNeeded.AutoApproveTools {
		t.Error("AutoApproveTools should pass through true from the caller regardless of NeedsAgentMode")
	}

	needed, err := a.CreateRoutineFromDraft("x", routine.Draft{TimeOfDay: "09:00", NeedsAgentMode: true}, "", false, "tr", nil)
	if err != nil {
		t.Fatalf("CreateRoutineFromDraft: %v", err)
	}
	if !needed.AgentMode {
		t.Error("AgentMode should be true when the draft's NeedsAgentMode guess was true")
	}
	if needed.AutoApproveTools {
		t.Error("AutoApproveTools should pass through false from the caller")
	}
}

// TestRoutinePermissionCallbacks_WiresToWhicheverChannelIsConfigured
// confirms a routine-triggered permission question actually gets sent
// through the routine's own delivery target, and that a routine with no
// live delivery channel fails safe (sendQuestion errors immediately, which
// resolveSelfChatPermission treats as deny) rather than hanging forever
// with nowhere to ask.
func TestRoutinePermissionCallbacks_WiresToWhicheverChannelIsConfigured(t *testing.T) {
	a := newRoutineTestApp(t)

	t.Run("no_channel_configured_sendQuestion_fails_fast", func(t *testing.T) {
		send, await := a.routinePermissionCallbacks(context.Background(), routine.Routine{})
		if err := send("soru"); err == nil {
			t.Error("expected sendQuestion to fail when the routine has no delivery channel")
		}
		if _, ok := await(context.Background()); ok {
			t.Error("expected awaitAnswer to report no answer when there's no channel to ask through")
		}
	})

	t.Run("whatsapp_channel_attempts_a_real_send", func(t *testing.T) {
		r := routine.Routine{DeliveryWhatsApp: true, WhatsAppTargetJID: "905551234567@s.whatsapp.net"}
		send, _ := a.routinePermissionCallbacks(context.Background(), r)
		// a.waClient is nil in this test app, so the send itself fails —
		// what's under test is that it actually TRIES WhatsApp (the right
		// error), not that it silently no-ops or picks the wrong channel.
		err := send("soru")
		if err == nil || !strings.Contains(err.Error(), "not initialized") {
			t.Errorf("expected a WhatsApp-not-initialized error (proving it tried WhatsApp), got %v", err)
		}
	})

	t.Run("telegram_channel_attempts_a_real_send", func(t *testing.T) {
		r := routine.Routine{DeliveryTelegram: true, TelegramTargetChatID: 555}
		send, _ := a.routinePermissionCallbacks(context.Background(), r)
		err := send("soru")
		if err == nil || !strings.Contains(err.Error(), "not initialized") {
			t.Errorf("expected a Telegram-not-initialized error (proving it tried Telegram), got %v", err)
		}
	})
}

func TestRoutinePermissionQuestion_PicksChannelAppropriatePhrasing(t *testing.T) {
	ev := agent.AgentEvent{ToolName: "run_command", Preview: "ls -la"}

	wa := routinePermissionQuestion("tr", true)(ev)
	if !strings.Contains(wa, "run_command") || !strings.Contains(wa, "ls -la") {
		t.Errorf("WhatsApp question missing tool/preview: %q", wa)
	}

	tg := routinePermissionQuestion("tr", false)(ev)
	if !strings.Contains(tg, "run_command") || !strings.Contains(tg, "ls -la") {
		t.Errorf("Telegram question missing tool/preview: %q", tg)
	}
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

// TestBuildRoutinePrompt_NoContextStillAddsNotificationFraming covers the
// ContextNone case: no deterministic context to merge, but the
// "standalone scheduled notification" framing (previously only sent via the
// now-removed non-agent path's dedicated system message) must still be
// present, since runAgentRoutine's own agent chat has no idea this is a
// routine rather than a live user turn.
func TestBuildRoutinePrompt_NoContextStillAddsNotificationFraming(t *testing.T) {
	a := newRoutineTestApp(t)
	r := routine.Routine{Prompt: "sistem durumunu özetle", Language: "tr"}

	got := a.buildRoutinePrompt(context.Background(), r)

	if !strings.Contains(got, routineSystemPrompt("tr")) {
		t.Errorf("buildRoutinePrompt = %q, want it to contain the notification framing", got)
	}
	if !strings.Contains(got, "sistem durumunu özetle") {
		t.Errorf("buildRoutinePrompt = %q, want it to contain the routine's own prompt", got)
	}
	if strings.Contains(got, "Bağlam:") {
		t.Errorf("buildRoutinePrompt = %q, should not add a context block when there is no context to merge", got)
	}
}

// TestBuildRoutinePrompt_MergesCalendarContext is the BUG-M6 regression test
// for the actual merge behavior: a routine's deterministic ContextSource
// pre-fetch (calendar agenda here) must still end up in the final prompt
// runAgentRoutine sends, exactly as it did via the old, now-removed
// runSimplePromptRoutine — this is what keeps a routine's guaranteed context
// from becoming dependent on the agent model deciding to fetch the
// equivalent data itself via a tool call.
func TestBuildRoutinePrompt_MergesCalendarContext(t *testing.T) {
	a := newRoutineTestApp(t)
	store, err := calendar.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("calendar.NewStore: %v", err)
	}
	defer store.Close()
	a.calendarStore = store

	ctx := context.Background()
	// Anchor the event inside today's agenda window regardless of when the
	// suite runs. The original now.Add(2*time.Hour) crossed midnight whenever
	// the test ran after ~22:00 local: the event landed *tomorrow*, outside
	// buildRoutinePrompt's [today 00:00, today+24h) fetch window, and the test
	// failed with "Bugün için takvimde etkinlik yok" — a time-of-day flake,
	// not a behavior change. Noon today is inside the window at any hour and
	// List(start,end) applies no past-event filter.
	n := time.Now()
	eventTime := time.Date(n.Year(), n.Month(), n.Day(), 12, 0, 0, 0, n.Location())
	if _, err := store.Add(ctx, calendar.Event{Title: "Diş randevusu", StartTime: eventTime}); err != nil {
		t.Fatalf("calendar.Add: %v", err)
	}

	r := routine.Routine{
		Prompt:        "günün ajandasını gönder",
		Language:      "tr",
		ContextSource: routine.ContextSource{Type: routine.ContextCalendar},
	}
	got := a.buildRoutinePrompt(ctx, r)

	if !strings.Contains(got, "Diş randevusu") {
		t.Errorf("buildRoutinePrompt = %q, want the pre-fetched calendar event merged in", got)
	}
	if !strings.Contains(got, "Bağlam:") {
		t.Errorf("buildRoutinePrompt = %q, want a context label when context was actually fetched", got)
	}
	if !strings.Contains(got, "günün ajandasını gönder") {
		t.Errorf("buildRoutinePrompt = %q, want the routine's own prompt still present", got)
	}
}

// TestRunRoutineGenerate_IgnoresStaleAgentModeFalse is the BUG-M6 regression
// test for legacy data: a routine persisted before this fix (AgentMode:
// false, from the old extractor-gated behavior) must still run through the
// full agent pipeline today, not the removed tool-less path — runRoutineGenerate
// no longer branches on Routine.AgentMode at all. Verified the same way
// TestRunAgentRoutine_DoesNotMutateGlobalAgentModeOrActiveChat verifies
// runAgentRoutine ran: NewAgentChat's active-chat side effect happens (and is
// then restored), which only happens on the agent path.
func TestRunRoutineGenerate_IgnoresStaleAgentModeFalse(t *testing.T) {
	a := newRoutineTestApp(t)

	userChatID := a.NewChat()
	if err := a.SwitchChat(userChatID); err != nil {
		t.Fatalf("SwitchChat: %v", err)
	}

	r := routine.Routine{Prompt: "test prompt", AgentMode: false}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The stream itself will fail (no LLM client/provider configured in this
	// test) — that's fine, we only care that it went through the agent path
	// (active chat restored to the pre-existing one afterward) rather than
	// erroring immediately from some other, tool-less code path.
	_, _ = a.runRoutineGenerate(ctx, r)

	if got := a.GetActiveChatID(); got != userChatID {
		t.Errorf("active chat = %q, want restored to %q — runRoutineGenerate should still go through the agent path even with AgentMode=false", got, userChatID)
	}
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

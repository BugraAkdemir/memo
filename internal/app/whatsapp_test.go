// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/config"
	"memo/internal/sessions"
	"memo/internal/whatsapp"
)

// TestWhatsAppAssistantSystemPrompt_InstructsAcceptingExplicitSendRequests is
// the regression test for BUG-L2: live testing (Session 40) found the
// WhatsApp-chat assistant refusing whatsapp_send on its own conversational
// judgment across three separate, increasingly direct rephrasings of an
// explicit user request — even with the tool's own permission gate already
// set to auto-allow, so the refusal happened before the tool was ever
// called. This asserts the prompt actually carries the fix's guidance
// (rather than testing live model behavior, which this environment can't
// exercise): the model should treat a clear, direct user request as
// sufficient and not add its own extra layer of refusal on top of it.
func TestWhatsAppAssistantSystemPrompt_InstructsAcceptingExplicitSendRequests(t *testing.T) {
	prompt := whatsAppAssistantSystemPrompt
	if !strings.Contains(prompt, "whatsapp_send") {
		t.Fatal("prompt no longer mentions whatsapp_send at all")
	}
	if !strings.Contains(prompt, "own judgment") {
		t.Error("prompt is missing the BUG-L2 explicit-consent guidance (should tell the model not to second-guess a clear, direct user request)")
	}
}

// TestIsSelfChatMessage_MatchesEitherOwnJIDForm is the regression test for
// the actual reported bug: the self-chat assistant was enabled and
// connected but never replied to anything. Root cause, confirmed live
// against a real account: a genuine "Message Yourself" message's ChatJID
// arrived as "<n>@lid" (WhatsApp's Linked-ID form), not
// "<phone>@s.whatsapp.net" — the only form the original, pre-fix code
// compared against (Client.OwnJID, singular). isSelfChatMessage must match
// against every JID Client.OwnJIDs returns, phone-number and @lid alike.
// Real values from that live account (client.go's OwnJIDs doc comment /
// TestOwnJIDs_IncludesLID has the same shape).
func TestIsSelfChatMessage_MatchesEitherOwnJIDForm(t *testing.T) {
	ownJIDs := []string{"905373154237@s.whatsapp.net", "110874714980365@lid"}

	t.Run("matches_lid_form", func(t *testing.T) {
		msg := whatsapp.Message{ChatJID: "110874714980365@lid", Text: "selam", FromMe: true}
		if !isSelfChatMessage(msg, ownJIDs) {
			t.Error("expected true for a ChatJID matching the @lid form — this is the exact case that silently never fired before the fix")
		}
	})

	t.Run("matches_phone_form", func(t *testing.T) {
		msg := whatsapp.Message{ChatJID: "905373154237@s.whatsapp.net", Text: "selam", FromMe: true}
		if !isSelfChatMessage(msg, ownJIDs) {
			t.Error("expected true for a ChatJID matching the phone-number form")
		}
	})

	t.Run("rejects_someone_elses_chat", func(t *testing.T) {
		msg := whatsapp.Message{ChatJID: "905555555555@s.whatsapp.net", Text: "selam", FromMe: true}
		if isSelfChatMessage(msg, ownJIDs) {
			t.Error("expected false for a ChatJID that matches neither own JID form")
		}
	})
}

// TestShouldAutoReplyToWhatsApp_Guards locks in the safe-default behavior
// of the self-chat auto-reply gate: it must stay false unless every
// condition genuinely holds, so a future refactor can't accidentally make
// Memo reply to someone else's message, or reply while the feature is
// still off. The "genuine self-chat match" true case is covered by
// TestIsSelfChatMessage_MatchesEitherOwnJIDForm above (the pure matching
// logic) rather than here — this function still needs a live OwnJIDs(),
// which requires a connected whatsmeow.Client that internal/app has no way
// to construct.
func TestShouldAutoReplyToWhatsApp_Guards(t *testing.T) {
	baseMsg := whatsapp.Message{
		ID:      "msg-1",
		ChatJID: "905555555555@s.whatsapp.net",
		Text:    "selam naber",
		FromMe:  true,
	}

	t.Run("feature_disabled", func(t *testing.T) {
		a := &App{
			cfg:      &config.AppConfig{WhatsApp: config.WhatsAppConfig{SelfChatAssistant: false}},
			waClient: whatsapp.NewClient(whatsapp.Config{}),
		}
		if a.shouldAutoReplyToWhatsApp(baseMsg) {
			t.Error("expected false when SelfChatAssistant is disabled")
		}
	})

	t.Run("not_from_me", func(t *testing.T) {
		a := &App{cfg: &config.AppConfig{WhatsApp: config.WhatsAppConfig{SelfChatAssistant: true}}}
		msg := baseMsg
		msg.FromMe = false
		if a.shouldAutoReplyToWhatsApp(msg) {
			t.Error("expected false for a message that isn't FromMe — self-chat is always FromMe")
		}
	})

	t.Run("empty_text", func(t *testing.T) {
		a := &App{cfg: &config.AppConfig{WhatsApp: config.WhatsAppConfig{SelfChatAssistant: true}}}
		msg := baseMsg
		msg.Text = "   "
		if a.shouldAutoReplyToWhatsApp(msg) {
			t.Error("expected false for whitespace-only text")
		}
	})

	t.Run("no_whatsapp_client", func(t *testing.T) {
		a := &App{cfg: &config.AppConfig{WhatsApp: config.WhatsAppConfig{SelfChatAssistant: true}}}
		if a.shouldAutoReplyToWhatsApp(baseMsg) {
			t.Error("expected false when waClient is nil (WhatsApp never connected)")
		}
	})

	t.Run("client_not_yet_connected", func(t *testing.T) {
		// A freshly constructed Client has no live session yet, so OwnJID()
		// returns "" — must fail safe rather than treating "" == "" as a
		// match against some hypothetical empty ChatJID.
		a := &App{
			cfg:      &config.AppConfig{WhatsApp: config.WhatsAppConfig{SelfChatAssistant: true}},
			waClient: whatsapp.NewClient(whatsapp.Config{}),
		}
		if a.shouldAutoReplyToWhatsApp(baseMsg) {
			t.Error("expected false when the WhatsApp client isn't connected yet")
		}
	})
}

// TestGetWhatsAppSelfChatAssistant confirms the getter reads the config
// field directly (default false, matching the opt-in-only intent — an
// existing user's config.yaml missing this key must not silently start
// auto-replying to their WhatsApp self-chat).
func TestGetWhatsAppSelfChatAssistant(t *testing.T) {
	t.Run("default_disabled", func(t *testing.T) {
		a := &App{cfg: &config.AppConfig{}}
		if a.GetWhatsAppSelfChatAssistant() {
			t.Error("expected false by default")
		}
	})

	t.Run("enabled_in_config", func(t *testing.T) {
		a := &App{cfg: &config.AppConfig{WhatsApp: config.WhatsAppConfig{SelfChatAssistant: true}}}
		if !a.GetWhatsAppSelfChatAssistant() {
			t.Error("expected true when config has it enabled")
		}
	})
}

// TestStartWhatsAppComposing_NoClientReturnsNoOpStop guards the pre-connect
// state: must return a harmless no-op stop function rather than panic when
// there's no WhatsApp client to send a presence update through.
func TestStartWhatsAppComposing_NoClientReturnsNoOpStop(t *testing.T) {
	a := &App{}
	stop := a.startWhatsAppComposing(context.Background(), "905555555555@s.whatsapp.net")
	if stop == nil {
		t.Fatal("expected a non-nil stop function even with no WhatsApp client")
	}
	stop() // must not panic
}

// TestHandleWhatsAppSelfChatCommand_NonCommandNotHandled confirms ordinary
// chat text is left alone (handled=false) so the caller routes it to the
// LLM as normal — a command handler that's too eager would swallow real
// messages that happen to contain a "/" somewhere.
func TestHandleWhatsAppSelfChatCommand_NonCommandNotHandled(t *testing.T) {
	a := &App{}
	for _, text := range []string{"selam kanka", "bugün 3/4 oranında bitti", ""} {
		if _, handled := a.handleWhatsAppSelfChatCommand(text); handled {
			t.Errorf("handleWhatsAppSelfChatCommand(%q) handled = true, want false", text)
		}
	}
}

// TestHandleWhatsAppSelfChatCommand_New confirms /new creates a fresh
// background session (not the globally active one — see
// NewBackgroundChat's doc comment) and points waSelfChatSessionID at it.
func TestHandleWhatsAppSelfChatCommand_New(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	a := &App{cfg: &config.AppConfig{}, sessions: sm, waSelfChatSessionID: "stale-id"}

	reply, handled := a.handleWhatsAppSelfChatCommand("/new")
	if !handled {
		t.Fatal("expected /new to be handled")
	}
	if reply == "" {
		t.Error("expected a non-empty confirmation reply")
	}
	if a.waSelfChatSessionID == "" || a.waSelfChatSessionID == "stale-id" {
		t.Errorf("waSelfChatSessionID = %q, want a fresh session id", a.waSelfChatSessionID)
	}
	if !sm.SessionExists(a.waSelfChatSessionID) {
		t.Error("the new session id doesn't actually exist in the session manager")
	}
}

// TestHandleWhatsAppSelfChatCommand_Agent covers /agent on, /agent off, and
// /agent with no argument (status query) — SetAgentEnabled/GetAgentEnabled
// don't touch config.Save, so this is safe to exercise directly without a
// temp config path.
func TestHandleWhatsAppSelfChatCommand_Agent(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}

	if _, handled := a.handleWhatsAppSelfChatCommand("/agent on"); !handled {
		t.Fatal("expected /agent on to be handled")
	}
	if !a.GetAgentEnabled() {
		t.Error("expected agent mode enabled after /agent on")
	}

	reply, handled := a.handleWhatsAppSelfChatCommand("/agent")
	if !handled {
		t.Fatal("expected /agent (no arg) to be handled")
	}
	// Language-agnostic: the reply's wording differs by UILanguage (see
	// TestHandleWhatsAppSelfChatCommand_FollowsUILanguage), so this only
	// checks it's non-empty and reflects the real state via GetAgentEnabled
	// above, not by matching a specific language's word for "on".
	if reply == "" {
		t.Error("expected a non-empty status reply")
	}

	if _, handled := a.handleWhatsAppSelfChatCommand("/agent off"); !handled {
		t.Fatal("expected /agent off to be handled")
	}
	if a.GetAgentEnabled() {
		t.Error("expected agent mode disabled after /agent off")
	}
}

// TestHandleWhatsAppSelfChatCommand_Web covers /web on and /web off — unlike
// /agent, UpdateWebSearchConfig persists via config.Save, so this needs a
// temp config path (same pattern as remote_auth_test.go) to avoid writing
// to the real repo's config.yaml as a test side effect.
func TestHandleWhatsAppSelfChatCommand_Web(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.Load(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a := &App{cfg: config.Get()}

	if _, handled := a.handleWhatsAppSelfChatCommand("/web on"); !handled {
		t.Fatal("expected /web on to be handled")
	}
	if !a.GetWebSearchEnabled() {
		t.Error("expected web search enabled after /web on")
	}

	if _, handled := a.handleWhatsAppSelfChatCommand("/web off"); !handled {
		t.Fatal("expected /web off to be handled")
	}
	if a.GetWebSearchEnabled() {
		t.Error("expected web search disabled after /web off")
	}
}

// TestHandleWhatsAppSelfChatCommand_AutoPerm covers /auto-perm on, off, and
// the no-arg status query — the actual bug this exists for: without it,
// agent mode was silently broken for any Medium/Dangerous tool call from
// WhatsApp self-chat (the permission prompt had nowhere to go), and default
// off means it stays broken-as-in-safely-asks unless explicitly opted into
// auto-approve.
func TestHandleWhatsAppSelfChatCommand_AutoPerm(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.Load(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a := &App{cfg: config.Get()}

	if a.GetWhatsAppAutoApprovePermissions() {
		t.Fatal("test assumption violated: auto-approve should default to false")
	}

	if _, handled := a.handleWhatsAppSelfChatCommand("/auto-perm on"); !handled {
		t.Fatal("expected /auto-perm on to be handled")
	}
	if !a.GetWhatsAppAutoApprovePermissions() {
		t.Error("expected auto-approve enabled after /auto-perm on")
	}

	reply, handled := a.handleWhatsAppSelfChatCommand("/auto-perm")
	if !handled || reply == "" {
		t.Errorf("/auto-perm (no arg): handled=%v reply=%q, want handled=true and non-empty", handled, reply)
	}

	if _, handled := a.handleWhatsAppSelfChatCommand("/auto-perm off"); !handled {
		t.Fatal("expected /auto-perm off to be handled")
	}
	if a.GetWhatsAppAutoApprovePermissions() {
		t.Error("expected auto-approve disabled after /auto-perm off")
	}
}

// TestSetWhatsAppSelfChatAssistant_TurnsOnWebSearch confirms enabling the
// self-chat assistant also turns web search on — a self-chat conversation
// has no other UI affordance to flip it on (unlike normal chat's toggle
// button) — and that disabling the assistant leaves web search untouched
// (only the "on" direction is coupled; turning the assistant off isn't a
// reason to also turn web search off, since the user might still want it
// on for normal chat).
func TestSetWhatsAppSelfChatAssistant_TurnsOnWebSearch(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.Load(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a := &App{cfg: config.Get()}

	if a.GetWebSearchEnabled() {
		t.Fatal("test assumption violated: web search should start off")
	}
	if err := a.SetWhatsAppSelfChatAssistant(true); err != nil {
		t.Fatalf("SetWhatsAppSelfChatAssistant: %v", err)
	}
	if !a.GetWebSearchEnabled() {
		t.Error("expected web search on after enabling the self-chat assistant")
	}

	if err := a.SetWhatsAppSelfChatAssistant(false); err != nil {
		t.Fatalf("SetWhatsAppSelfChatAssistant(false): %v", err)
	}
	if !a.GetWebSearchEnabled() {
		t.Error("disabling the self-chat assistant should not also turn web search back off")
	}
}

// TestHandleWhatsAppSelfChatCommand_StatusAndHelp confirms /status and
// /help are recognized and reply with non-empty text — the exact content
// is free to change, so this only locks in that they're wired up and don't
// panic against a minimally-populated App.
func TestHandleWhatsAppSelfChatCommand_StatusAndHelp(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}

	if reply, handled := a.handleWhatsAppSelfChatCommand("/status"); !handled || reply == "" {
		t.Errorf("/status: handled=%v reply=%q, want handled=true and non-empty", handled, reply)
	}
	if reply, handled := a.handleWhatsAppSelfChatCommand("/help"); !handled || reply != waT("en", "wa_help") {
		t.Errorf("/help: handled=%v reply=%q, want handled=true and reply == waT(\"en\", \"wa_help\")", handled, reply)
	}
}

// TestHandleWhatsAppSelfChatCommand_UnknownCommand confirms a typo'd or
// unrecognized "/word" is still treated as a command attempt (handled=true,
// with guidance back to the user) rather than silently falling through to
// the LLM as if it were a genuine chat message.
func TestHandleWhatsAppSelfChatCommand_UnknownCommand(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	reply, handled := a.handleWhatsAppSelfChatCommand("/statuss")
	if !handled {
		t.Fatal("expected an unrecognized slash command to still be handled")
	}
	if !strings.Contains(reply, "/statuss") {
		t.Errorf("reply %q should echo back the unrecognized command", reply)
	}
}

// TestHandleWhatsAppSelfChatCommand_FollowsUILanguage is the direct answer
// to a user question: are these replies actually following the app's
// language setting (Identity.UILanguage, same as the Flutter GUI's own
// toggle), or hardcoded Turkish regardless? They must not be hardcoded —
// /help's reply in particular must differ by language, and an unset
// UILanguage (GetUILanguage()'s documented "" when the GUI's toggle was
// never touched) must default to English, matching the Flutter GUI's own
// 2026-08-13 default.
func TestHandleWhatsAppSelfChatCommand_FollowsUILanguage(t *testing.T) {
	t.Run("turkish", func(t *testing.T) {
		a := &App{cfg: &config.AppConfig{Identity: config.IdentityConfig{UILanguage: "tr"}}}
		reply, _ := a.handleWhatsAppSelfChatCommand("/help")
		if !strings.Contains(reply, "Komutlar") {
			t.Errorf("expected the Turkish help text (contains \"Komutlar\"), got %q", reply)
		}
		if strings.Contains(reply, "Commands") {
			t.Errorf("got English text back for UILanguage=\"tr\": %q", reply)
		}
	})

	t.Run("english", func(t *testing.T) {
		a := &App{cfg: &config.AppConfig{Identity: config.IdentityConfig{UILanguage: "en"}}}
		reply, _ := a.handleWhatsAppSelfChatCommand("/help")
		if !strings.Contains(reply, "Commands") {
			t.Errorf("expected the English help text (contains \"Commands\"), got %q", reply)
		}
		if strings.Contains(reply, "Komutlar") {
			t.Errorf("got Turkish text back for UILanguage=\"en\": %q", reply)
		}
	})

	t.Run("unset_defaults_to_english", func(t *testing.T) {
		a := &App{cfg: &config.AppConfig{}}
		reply, _ := a.handleWhatsAppSelfChatCommand("/help")
		if !strings.Contains(reply, "Commands") {
			t.Errorf("expected an unset UILanguage to default to English, got %q", reply)
		}
	})
}

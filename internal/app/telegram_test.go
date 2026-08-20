// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/config"
	"memo/internal/sessions"
	"memo/internal/telegram"
)

// testMasterKey is a fixed, non-nil AES key for telegram.NewStore in these
// tests — passing nil would fall back to provider.DefaultMachineKey(),
// which resolves config.DataDir() and can create a stray data/machine.key
// under this package's own directory when tests run in isolation (the
// process cwd, not a temp dir). t.TempDir() alone doesn't prevent this,
// since that fallback is keyed off cwd, not the store's own file path.
var testMasterKey = make([]byte, 32)

// TestIsTelegramOwnerMessage is the pure matching logic behind
// shouldReplyToTelegram — mirrors TestIsSelfChatMessage_MatchesEitherOwnJIDForm
// in whatsapp_test.go.
func TestIsTelegramOwnerMessage(t *testing.T) {
	if !isTelegramOwnerMessage(999, 999) {
		t.Error("expected true when chatID matches ownerChatID")
	}
	if isTelegramOwnerMessage(111, 999) {
		t.Error("expected false when chatID differs from ownerChatID")
	}
}

// TestShouldReplyToTelegram_LinksFirstSenderThenGuards is the Telegram
// equivalent of TestShouldAutoReplyToWhatsApp_Guards — but where WhatsApp's
// self-chat identity is implicit (only the account owner can write to their
// own "Message Yourself" chat), a Telegram bot token is reachable by
// anyone, so the very first message is what establishes ownership, and
// every message after that is checked against it.
func TestShouldReplyToTelegram_LinksFirstSenderThenGuards(t *testing.T) {
	t.Run("no_store_never_replies", func(t *testing.T) {
		a := &App{}
		if a.shouldReplyToTelegram(telegram.Message{ChatID: 1}) {
			t.Error("expected false with no tgStore at all")
		}
	})

	t.Run("first_message_links_owner", func(t *testing.T) {
		store := telegram.NewStore(filepath.Join(t.TempDir(), "telegram.json"), testMasterKey)
		a := &App{tgStore: store}
		msg := telegram.Message{ChatID: 555, FromName: "Bugra", Text: "selam"}

		if !a.shouldReplyToTelegram(msg) {
			t.Fatal("expected the first-ever message to link its sender as owner and reply")
		}
		st := store.Get()
		if st.OwnerChatID != 555 || st.OwnerName != "Bugra" {
			t.Errorf("owner not linked correctly: %+v", st)
		}
	})

	t.Run("stranger_rejected_once_owner_linked", func(t *testing.T) {
		store := telegram.NewStore(filepath.Join(t.TempDir(), "telegram.json"), testMasterKey)
		store.SetOwner(555, "Bugra")
		a := &App{tgStore: store}

		stranger := telegram.Message{ChatID: 111, FromName: "Someone Else", Text: "merhaba"}
		if a.shouldReplyToTelegram(stranger) {
			t.Error("expected a message from a chat ID other than the linked owner to be rejected")
		}

		owner := telegram.Message{ChatID: 555, FromName: "Bugra", Text: "merhaba"}
		if !a.shouldReplyToTelegram(owner) {
			t.Error("expected a message from the already-linked owner to still be accepted")
		}
	})
}

// TestHandleTelegramCommand_NonCommandNotHandled mirrors
// TestHandleWhatsAppSelfChatCommand_NonCommandNotHandled.
func TestHandleTelegramCommand_NonCommandNotHandled(t *testing.T) {
	a := &App{}
	for _, text := range []string{"selam kanka", "bugün 3/4 oranında bitti", ""} {
		if _, handled := a.handleTelegramCommand(text); handled {
			t.Errorf("handleTelegramCommand(%q) handled = true, want false", text)
		}
	}
}

// TestHandleTelegramCommand_New mirrors TestHandleWhatsAppSelfChatCommand_New.
func TestHandleTelegramCommand_New(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	a := &App{cfg: &config.AppConfig{}, sessions: sm, tgSelfChatSessionID: "stale-id"}

	reply, handled := a.handleTelegramCommand("/new")
	if !handled {
		t.Fatal("expected /new to be handled")
	}
	if reply == "" {
		t.Error("expected a non-empty confirmation reply")
	}
	if a.tgSelfChatSessionID == "" || a.tgSelfChatSessionID == "stale-id" {
		t.Errorf("tgSelfChatSessionID = %q, want a fresh session id", a.tgSelfChatSessionID)
	}
	if !sm.SessionExists(a.tgSelfChatSessionID) {
		t.Error("the new session id doesn't actually exist in the session manager")
	}
}

// TestHandleTelegramCommand_Agent mirrors TestHandleWhatsAppSelfChatCommand_Agent.
func TestHandleTelegramCommand_Agent(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}

	if _, handled := a.handleTelegramCommand("/agent on"); !handled {
		t.Fatal("expected /agent on to be handled")
	}
	if !a.GetAgentEnabled() {
		t.Error("expected agent mode enabled after /agent on")
	}

	reply, handled := a.handleTelegramCommand("/agent")
	if !handled || reply == "" {
		t.Errorf("/agent (no arg): handled=%v reply=%q, want handled=true and non-empty", handled, reply)
	}

	if _, handled := a.handleTelegramCommand("/agent off"); !handled {
		t.Fatal("expected /agent off to be handled")
	}
	if a.GetAgentEnabled() {
		t.Error("expected agent mode disabled after /agent off")
	}
}

// TestHandleTelegramCommand_Web mirrors TestHandleWhatsAppSelfChatCommand_Web.
func TestHandleTelegramCommand_Web(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.Load(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a := &App{cfg: config.Get()}

	if _, handled := a.handleTelegramCommand("/web on"); !handled {
		t.Fatal("expected /web on to be handled")
	}
	if !a.GetWebSearchEnabled() {
		t.Error("expected web search enabled after /web on")
	}

	if _, handled := a.handleTelegramCommand("/web off"); !handled {
		t.Fatal("expected /web off to be handled")
	}
	if a.GetWebSearchEnabled() {
		t.Error("expected web search disabled after /web off")
	}
}

// TestHandleTelegramCommand_StatusAndHelp mirrors
// TestHandleWhatsAppSelfChatCommand_StatusAndHelp.
func TestHandleTelegramCommand_StatusAndHelp(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}

	if reply, handled := a.handleTelegramCommand("/status"); !handled || reply == "" {
		t.Errorf("/status: handled=%v reply=%q, want handled=true and non-empty", handled, reply)
	}
	if reply, handled := a.handleTelegramCommand("/help"); !handled || reply != tgT("en", "tg_help") {
		t.Errorf("/help: handled=%v reply=%q, want handled=true and reply == tgT(\"en\", \"tg_help\")", handled, reply)
	}
}

// TestHandleTelegramCommand_UnknownCommand mirrors
// TestHandleWhatsAppSelfChatCommand_UnknownCommand.
func TestHandleTelegramCommand_UnknownCommand(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	reply, handled := a.handleTelegramCommand("/statuss")
	if !handled {
		t.Fatal("expected an unrecognized slash command to still be handled")
	}
	if !strings.Contains(reply, "/statuss") {
		t.Errorf("reply %q should echo back the unrecognized command", reply)
	}
}

// TestHandleTelegramCommand_FollowsUILanguage mirrors
// TestHandleWhatsAppSelfChatCommand_FollowsUILanguage — same requirement:
// no hardcoded language, unset defaults to English.
func TestHandleTelegramCommand_FollowsUILanguage(t *testing.T) {
	t.Run("turkish", func(t *testing.T) {
		a := &App{cfg: &config.AppConfig{Identity: config.IdentityConfig{UILanguage: "tr"}}}
		reply, _ := a.handleTelegramCommand("/help")
		if !strings.Contains(reply, "Komutlar") {
			t.Errorf("expected the Turkish help text (contains \"Komutlar\"), got %q", reply)
		}
		if strings.Contains(reply, "Commands") {
			t.Errorf("got English text back for UILanguage=\"tr\": %q", reply)
		}
	})

	t.Run("english", func(t *testing.T) {
		a := &App{cfg: &config.AppConfig{Identity: config.IdentityConfig{UILanguage: "en"}}}
		reply, _ := a.handleTelegramCommand("/help")
		if !strings.Contains(reply, "Commands") {
			t.Errorf("expected the English help text (contains \"Commands\"), got %q", reply)
		}
		if strings.Contains(reply, "Komutlar") {
			t.Errorf("got Turkish text back for UILanguage=\"en\": %q", reply)
		}
	})

	t.Run("unset_defaults_to_english", func(t *testing.T) {
		a := &App{cfg: &config.AppConfig{}}
		reply, _ := a.handleTelegramCommand("/help")
		if !strings.Contains(reply, "Commands") {
			t.Errorf("expected an unset UILanguage to default to English, got %q", reply)
		}
	})
}

// TestStartTelegramComposing_NoClientReturnsNoOpStop mirrors
// TestStartWhatsAppComposing_NoClientReturnsNoOpStop.
func TestStartTelegramComposing_NoClientReturnsNoOpStop(t *testing.T) {
	a := &App{}
	stop := a.startTelegramComposing(t.Context(), 555)
	if stop == nil {
		t.Fatal("expected a non-nil stop function even with no Telegram client")
	}
	stop() // must not panic
}

// TestGetTelegramStatus_NoStoreReturnsUnconfigured guards the pre-connect
// state so the Settings UI gets a stable, well-formed response before the
// user has ever pasted a bot token.
func TestGetTelegramStatus_NoStoreReturnsUnconfigured(t *testing.T) {
	a := &App{}
	status := a.GetTelegramStatus()
	if status["configured"] != false || status["connected"] != false {
		t.Errorf("expected an unconfigured/disconnected status with no store, got %+v", status)
	}
}

// TestDisconnectTelegram_WipesStoredState confirms Disconnect (the
// Telegram equivalent of LogoutWhatsApp) actually clears the token/owner —
// not just pauses polling like Stop does.
func TestDisconnectTelegram_WipesStoredState(t *testing.T) {
	store := telegram.NewStore(filepath.Join(t.TempDir(), "telegram.json"), testMasterKey)
	store.Set(telegram.State{Enabled: true, BotToken: "123:abc"})
	store.SetOwner(555, "Bugra")

	a := &App{tgStore: store, tgSelfChatSessionID: "some-session"}
	if err := a.DisconnectTelegram(); err != nil {
		t.Fatalf("DisconnectTelegram: %v", err)
	}

	st := store.Get()
	if st.Linked() || st.BotToken != "" || st.Enabled {
		t.Errorf("expected DisconnectTelegram to fully wipe stored state, got %+v", st)
	}
	if a.tgSelfChatSessionID != "" {
		t.Errorf("expected tgSelfChatSessionID reset, got %q", a.tgSelfChatSessionID)
	}
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"strings"
	"testing"

	"memo/internal/config"
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

// TestShouldAutoReplyToWhatsApp_Guards locks in the safe-default behavior
// of the self-chat auto-reply gate: it must stay false unless every
// condition genuinely holds, so a future refactor can't accidentally make
// Memo reply to someone else's message, or reply while the feature is
// still off. The "genuine self-chat match" true case isn't covered here —
// it needs a live OwnJID(), which requires a connected whatsmeow.Client
// that internal/app has no way to construct (see client.go's OwnJID and
// its own TestOwnJID_StripsDeviceSuffix in internal/whatsapp for that half).
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

// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"strings"
	"testing"
)

// TestIncognito_ChatIDSendIsNotSaved was found live: the Flutter client sends
// chat_id with every message, and that path ignored incognito mode — the
// "private" message was written to the chat's history on disk.
func TestIncognito_ChatIDSendIsNotSaved(t *testing.T) {
	h := NewHarness(t)
	h.SetWebSearchEnabled(false)
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		return FakeChatResponse{Text: "ok"}
	}
	chatID := h.NewChat()
	h.SendMessageStream(chatID, "an earlier, normal message")

	h.postJSON("/api/incognito", map[string]bool{"enabled": true}).Body.Close()
	h.SendMessageStream(chatID, "PRIVATE-MARKER-42")
	h.postJSON("/api/incognito", map[string]bool{"enabled": false}).Body.Close()

	for _, m := range h.storedMessages(chatID) {
		if strings.Contains(m.Content, "PRIVATE-MARKER-42") {
			t.Fatalf("an incognito message was saved to the chat: %+v", m)
		}
	}
	// And the incognito turn must not have been given the chat's history.
	for _, req := range h.Fake.Requests() {
		raw := string(req.Raw)
		if strings.Contains(raw, "PRIVATE-MARKER-42") && strings.Contains(raw, "an earlier, normal message") {
			t.Errorf("the incognito turn was sent the regular chat's history")
		}
	}
}

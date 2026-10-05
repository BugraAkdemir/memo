// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"net/http"
	"strings"
	"testing"
)

// TestMessages_ChatIDIsHonouredNotTheActiveChat was found live: GET
// /api/messages ignored chat_id and always answered with the globally active
// chat, and update/delete-by-index acted on the active chat too. With two
// clients on one backend (desktop + phone), one switching chats between the
// other's switch and read made the other show — or delete from — the wrong
// chat; asking for a chat another device had deleted returned some other
// chat's history.
func TestMessages_ChatIDIsHonouredNotTheActiveChat(t *testing.T) {
	h := NewHarness(t)
	h.SetWebSearchEnabled(false)
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		return FakeChatResponse{Text: "ack"}
	}
	chatA := h.NewChat()
	chatB := h.NewChat()
	h.SendMessageStream(chatA, "message in A")
	h.SendMessageStream(chatB, "message in B")
	// Another client switches the backend to A.
	h.postJSON("/api/chats/switch", map[string]string{"id": chatA}).Body.Close()

	msgs := h.storedMessages(chatB)
	if len(msgs) == 0 || !strings.Contains(msgs[0].Content, "message in B") {
		t.Fatalf("GET /api/messages?chat_id=B while A is active = %+v, want B's history", msgs)
	}

	// Delete B's first message by index, explicitly in B — A must be untouched.
	r := h.postJSON("/api/messages/delete", map[string]any{"chat_id": chatB, "index": 0})
	r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("delete in B: status %d", r.StatusCode)
	}
	if a := h.storedMessages(chatA); len(a) == 0 || !strings.Contains(a[0].Content, "message in A") {
		t.Errorf("chat A after deleting from B = %+v, want A intact", a)
	}
	if b := h.storedMessages(chatB); len(b) != len(msgs)-1 {
		t.Errorf("chat B has %d messages after the delete, want %d", len(b), len(msgs)-1)
	}

	// A deleted chat is a 404, not someone else's history.
	h.postJSON("/api/chats/delete", map[string]string{"id": chatB}).Body.Close()
	r = h.getJSON("/api/messages?chat_id=" + chatB)
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Errorf("GET /api/messages for a deleted chat: status %d, want 404", r.StatusCode)
	}
}

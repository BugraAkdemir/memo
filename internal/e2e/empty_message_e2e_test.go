// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"net/http"
	"testing"
)

// TestSend_EmptyMessageIsRejected was found live: an empty or whitespace-only
// message was stored as an empty user bubble and spent a full LLM turn.
func TestSend_EmptyMessageIsRejected(t *testing.T) {
	h := NewHarness(t)
	chatID := h.NewChat()
	for _, msg := range []string{"", "   \n\t"} {
		resp := h.postJSON("/api/send/stream", map[string]string{"chat_id": chatID, "message": msg})
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("POST /api/send/stream with %q: status %d, want 400", msg, resp.StatusCode)
		}
	}
	if n := len(h.Fake.Requests()); n != 0 {
		t.Errorf("the provider was called %d time(s) for empty messages, want 0", n)
	}
	if msgs := h.storedMessages(chatID); len(msgs) != 0 {
		t.Errorf("stored history = %+v, want nothing saved", msgs)
	}
}

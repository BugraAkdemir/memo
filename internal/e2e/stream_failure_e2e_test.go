// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"strings"
	"testing"
)

// storedMessages reads a chat's persisted history the way a client reload
// does: GET /api/messages?chat_id=...
func (h *Harness) storedMessages(chatID string) []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
} {
	h.t.Helper()
	var msgs []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	decodeInto(h.t, h.getJSON("/api/messages?chat_id="+chatID), &msgs)
	return msgs
}

// TestChat_StreamCutMidAnswerKeepsWhatTheUserSaw was found live: an upstream
// that drops the connection mid-answer showed the user the text as it
// arrived, then persisted only "⚠️ unexpected EOF" — so on the next reload
// the part they had already read was gone.
func TestChat_StreamCutMidAnswerKeepsWhatTheUserSaw(t *testing.T) {
	h := NewHarness(t)
	h.SetWebSearchEnabled(false) // plain chat → the provider's SSE path
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		if !req.Stream {
			return FakeChatResponse{Text: "title"}
		}
		return FakeChatResponse{Text: "the first half of a long answer", CutAfterText: true}
	}
	chatID := h.NewChat()
	events := h.SendMessageStream(chatID, "explain something long")

	last := events[len(events)-1]
	if last.Error == "" {
		t.Fatalf("last event = %+v, want the stream failure surfaced as an error", last)
	}
	msgs := h.storedMessages(chatID)
	if len(msgs) == 0 || msgs[len(msgs)-1].Role != "assistant" {
		t.Fatalf("stored history = %+v, want a trailing assistant message", msgs)
	}
	got := msgs[len(msgs)-1].Content
	if !strings.Contains(got, "the first half of a long answer") {
		t.Errorf("stored reply = %q, want the partial answer the user already saw", got)
	}
	if !strings.Contains(got, last.Error) {
		t.Errorf("stored reply = %q, want the error %q appended", got, last.Error)
	}
}

// TestChat_EmptyReplyIsExplainedNotSavedBlank was found live: a provider
// that finished normally with no text left an empty assistant bubble and
// no hint of what happened.
func TestChat_EmptyReplyIsExplainedNotSavedBlank(t *testing.T) {
	h := NewHarness(t)
	h.SetWebSearchEnabled(false)
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		if !req.Stream {
			return FakeChatResponse{Text: "title"}
		}
		return FakeChatResponse{Text: ""}
	}
	chatID := h.NewChat()
	events := h.SendMessageStream(chatID, "hello?")

	last := events[len(events)-1]
	if last.Error == "" {
		t.Fatalf("last event = %+v, want an empty-response error", last)
	}
	msgs := h.storedMessages(chatID)
	if got := msgs[len(msgs)-1]; got.Role != "assistant" || strings.TrimSpace(got.Content) == "" {
		t.Errorf("stored reply = %+v, want a non-empty explanation, not a blank bubble", got)
	}
}

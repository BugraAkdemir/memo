// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"strings"
	"testing"
)

// TestChat_PlainMessageRoundTrip is the simplest possible real E2E proof:
// a real app.App, behind a real HTTP server, streaming a real SSE reply
// assembled from a real (fake-backed) provider call — the exact path a
// Flutter client's "type a message, see a reply" exercises, with nothing
// mocked below the provider's own HTTP boundary.
func TestChat_PlainMessageRoundTrip(t *testing.T) {
	h := NewHarness(t)
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		return FakeChatResponse{Text: "merhaba, nasıl yardımcı olabilirim?"}
	}

	chatID := h.NewChat()
	events := h.SendMessageStream(chatID, "selam")

	if len(events) == 0 {
		t.Fatal("got zero SSE events")
	}
	last := events[len(events)-1]
	if !last.Done {
		t.Errorf("last event is not marked done: %+v", last)
	}
	if got := FinalContent(events); got != "merhaba, nasıl yardımcı olabilirim?" {
		t.Errorf("assembled reply = %q, want the fake provider's exact text", got)
	}

	// The fake provider must have actually been called — a real request
	// really left the app and really came back, not a bypassed/cached path.
	// Deliberately not asserting Stream true/false here: that's an internal
	// implementation choice of callLLMStream's provider path, not part of
	// the client-visible contract this test is proving.
	reqs := h.Fake.Requests()
	if len(reqs) == 0 {
		t.Fatal("FakeProvider never received a request — the app didn't actually call out")
	}
}

// TestChat_TwoChatsStayIndependent proves the explicit chat_id routing this
// harness always uses (SendMessageStream) actually keeps two chats'
// history separate, rather than both silently landing on whatever chat
// happens to be globally "active" — the exact class of bug
// PLAN_chatid_refactor.md's Faz 4 (quoted in handlers_flutter.go) was built
// to close. Asserts on the raw request bytes the app actually sent to the
// provider for chat B's turn — the real proof of isolation is that chat A's
// message never appears in chat B's history, not just that the two replies
// differ.
func TestChat_TwoChatsStayIndependent(t *testing.T) {
	h := NewHarness(t)
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		return FakeChatResponse{Text: "ack"}
	}

	chatA := h.NewChat()
	chatB := h.NewChat()
	if chatA == chatB {
		t.Fatalf("two calls to /api/chats/new returned the same id: %q", chatA)
	}

	h.SendMessageStream(chatA, "gizli-mesaj-A-icin-anahtar-kelime")
	h.SendMessageStream(chatB, "mesaj B")

	// Not asserting an exact call count: a new chat's first message also
	// fires an async, best-effort chat-title-generation call (see
	// internal/app/chat.go) that can legitimately land as its own provider
	// request depending on scheduling — real, harmless, and orthogonal to
	// what this test checks. Instead: find the request that's chat B's own
	// turn (contains "mesaj B") and confirm chat A's secret text never
	// appears anywhere else in that same request's message list — that's
	// the actual leak this test guards against.
	found := false
	for _, req := range h.Fake.Requests() {
		hasB := false
		for _, m := range req.Messages {
			if strings.Contains(string(m), "mesaj B") {
				hasB = true
				break
			}
		}
		if !hasB {
			continue
		}
		found = true
		for _, m := range req.Messages {
			if strings.Contains(string(m), "gizli-mesaj-A-icin-anahtar-kelime") {
				t.Fatalf("chat B's own request also carries chat A's message — history leaked across chats: %s", m)
			}
		}
	}
	if !found {
		t.Fatal("never found a provider request containing chat B's message")
	}
}

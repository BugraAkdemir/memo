package webserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memo/internal/api"
)

// TestHandleSendStream_WithChatID_UsesSendMessageStreamTo is the regression
// test for PLAN_chatid_refactor.md Faz 4 / the "GUI and internal/replcli
// fighting over one global active chat" bug report: before this fix,
// POST /api/send/stream carried no chat identifier at all, so the backend
// always wrote into whichever chat sessions.Manager happened to consider
// globally "active" at that instant — a second client (a terminal `memo`
// session, or a second browser tab) switching chats out from under a
// request in flight could silently redirect it into the wrong chat. A
// request that supplies chat_id must now be routed through
// SendMessageStreamTo(chatID, ...), never the implicit-active variant.
func TestHandleSendStream_WithChatID_UsesSendMessageStreamTo(t *testing.T) {
	var gotChatID, gotMsg string
	var streamToCalled, streamCalled bool
	bridge := &swarmStubBridge{
		sendMessageStreamTo: func(ctx context.Context, chatID, userMsg string) <-chan api.StreamChunk {
			streamToCalled = true
			gotChatID = chatID
			gotMsg = userMsg
			ch := make(chan api.StreamChunk, 1)
			ch <- api.StreamChunk{Content: "ok", Done: true}
			close(ch)
			return ch
		},
		sendMessageStream: func(ctx context.Context, userMsg string) <-chan api.StreamChunk {
			streamCalled = true
			ch := make(chan api.StreamChunk, 1)
			ch <- api.StreamChunk{Content: "wrong path", Done: true}
			close(ch)
			return ch
		},
	}
	s := &Server{fullBridge: bridge}

	req := httptest.NewRequest(http.MethodPost, "/api/send/stream",
		strings.NewReader(`{"message":"hello","chat_id":"chat-A"}`))
	rec := httptest.NewRecorder()
	s.handleSendStream(rec, req)

	if !streamToCalled {
		t.Fatal("expected SendMessageStreamTo to be called when chat_id is present")
	}
	if streamCalled {
		t.Fatal("SendMessageStream (implicit-active) must not be called when chat_id is present")
	}
	if gotChatID != "chat-A" {
		t.Errorf("chatID passed to SendMessageStreamTo = %q, want %q", gotChatID, "chat-A")
	}
	if gotMsg != "hello" {
		t.Errorf("userMsg passed to SendMessageStreamTo = %q, want %q", gotMsg, "hello")
	}
	if !strings.Contains(rec.Body.String(), `"content":"ok"`) {
		t.Errorf("response body missing expected content, got: %s", rec.Body.String())
	}
}

// TestHandleSendStream_WithoutChatID_FallsBackToImplicitActive covers the
// backward-compatibility half of the same fix: an older client (or one that
// genuinely has no specific chat in mind) that omits chat_id must keep
// working exactly as before — routed through the implicit-active
// SendMessageStream, not rejected or forced through SendMessageStreamTo("").
func TestHandleSendStream_WithoutChatID_FallsBackToImplicitActive(t *testing.T) {
	var streamToCalled, streamCalled bool
	bridge := &swarmStubBridge{
		sendMessageStreamTo: func(ctx context.Context, chatID, userMsg string) <-chan api.StreamChunk {
			streamToCalled = true
			ch := make(chan api.StreamChunk)
			close(ch)
			return ch
		},
		sendMessageStream: func(ctx context.Context, userMsg string) <-chan api.StreamChunk {
			streamCalled = true
			ch := make(chan api.StreamChunk, 1)
			ch <- api.StreamChunk{Content: "legacy path", Done: true}
			close(ch)
			return ch
		},
	}
	s := &Server{fullBridge: bridge}

	req := httptest.NewRequest(http.MethodPost, "/api/send/stream",
		strings.NewReader(`{"message":"hello"}`))
	rec := httptest.NewRecorder()
	s.handleSendStream(rec, req)

	if streamToCalled {
		t.Fatal("SendMessageStreamTo must not be called when chat_id is omitted")
	}
	if !streamCalled {
		t.Fatal("expected SendMessageStream (implicit-active) to be called when chat_id is omitted")
	}
	if !strings.Contains(rec.Body.String(), `"content":"legacy path"`) {
		t.Errorf("response body missing expected content, got: %s", rec.Body.String())
	}
}

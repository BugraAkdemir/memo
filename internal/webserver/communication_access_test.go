package webserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memo/internal/agent/tools"
	"memo/internal/api"
)

func TestCommunicationRoleSurvivesStreamingAndSynchronousChat(t *testing.T) {
	for _, path := range []string{"/api/send", "/api/send/stream"} {
		for _, token := range []string{"user-session", "admin-session"} {
			t.Run(path+token, func(t *testing.T) {
				b := restrictedBridge()
				seen := false
				b.sendMessageStream = func(ctx context.Context, _ string) <-chan api.StreamChunk {
					seen = true
					if got, want := tools.CommunicationAllowed(ctx), token == "admin-session"; got != want {
						t.Errorf("private tool access=%v want=%v", got, want)
					}
					ch := make(chan api.StreamChunk, 2)
					ch <- api.StreamChunk{FinishReason: "agent_event", Content: "private metadata"}
					ch <- api.StreamChunk{Content: "reply", Done: true}
					close(ch)
					return ch
				}
				s := New(b)
				s.fullBridge = b
				s.listenAddr = "127.0.0.1"
				handler := http.HandlerFunc(s.handleSend)
				if path == "/api/send/stream" {
					handler = s.handleSendStream
				}
				req := httptest.NewRequest("POST", path, strings.NewReader(`{"message":"hello"}`))
				req.Header.Set("X-Memo-Token", token)
				w := httptest.NewRecorder()
				s.remoteAuthMiddleware(handler).ServeHTTP(w, req)
				if w.Code != 200 || !seen {
					t.Fatalf("request failed: %d %s", w.Code, w.Body.String())
				}
				if path == "/api/send" && strings.Contains(w.Body.String(), "private metadata") {
					t.Fatal("SSE marker leaked into synchronous reply")
				}
			})
		}
	}
}

package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/provider"
)

func shortenStreamTimers(t *testing.T, idle, total time.Duration) {
	t.Helper()
	oldIdle, oldTotal := streamIdleTimeout, streamTotalCap
	streamIdleTimeout, streamTotalCap = idle, total
	t.Cleanup(func() { streamIdleTimeout, streamTotalCap = oldIdle, oldTotal })
}

func providerApp(t *testing.T, h http.HandlerFunc) *App {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	router := provider.NewRouter([]provider.ProviderConfig{{
		Type: provider.ProviderCustom, Name: "test", BaseURL: srv.URL, Model: "m", Enabled: true,
	}})
	return &App{providerRouter: router, activeProviderName: "test", cfg: &config.AppConfig{}}
}

func collect(t *testing.T, ch <-chan api.StreamChunk, within time.Duration) (content string, last api.StreamChunk) {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case c, ok := <-ch:
			if !ok {
				return content, last
			}
			if c.FinishReason == "" {
				content += c.Content
			}
			last = c
		case <-deadline:
			t.Fatal("stream did not finish in time")
		}
	}
}

// TestCallLLMStream_LongButActiveReplyIsNotCut is the live finding: the
// plain-chat stream ran under a 300s TOTAL deadline, so a reply that was
// still arriving was cut at that mark (and saved as if the user had pressed
// stop). Here the total is 5× the idle timeout and chunks keep coming — it
// must finish whole.
func TestCallLLMStream_LongButActiveReplyIsNotCut(t *testing.T) {
	shortenStreamTimers(t, 150*time.Millisecond, 5*time.Second)
	a := providerApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		for i := 0; i < 8; i++ { // 8 × 100ms = 800ms > 5 × the idle timeout
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"w%d \"}}]}\n\n", i)
			fl.Flush()
			time.Sleep(100 * time.Millisecond)
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	})
	content, last := collect(t, a.callLLMStream(context.Background(), []api.Message{api.NewTextMessage("user", "hi")}, "hi", "", "", ""), 5*time.Second)
	if last.Error != "" || !strings.Contains(content, "w7") {
		t.Fatalf("content=%q last=%+v, want the whole reply with no error", content, last)
	}
}

// TestCallLLMStream_SilentStreamEndsWithATimeoutNotAStop: a stream that
// really goes quiet must end — and must say it timed out, instead of the
// "⏹️ Response stopped." the old code saved as if the user had stopped it.
func TestCallLLMStream_SilentStreamEndsWithATimeoutNotAStop(t *testing.T) {
	shortenStreamTimers(t, 200*time.Millisecond, 10*time.Second)
	a := providerApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial answer\"}}]}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done() // then silence
	})
	_, last := collect(t, a.callLLMStream(context.Background(), []api.Message{api.NewTextMessage("user", "hi")}, "hi", "", "", ""), 5*time.Second)
	if !last.Done || !strings.Contains(last.Error, "⏱️") {
		t.Fatalf("terminal chunk = %+v, want a timeout error", last)
	}
	if strings.Contains(last.Error, a.stopMarker()) {
		t.Errorf("a timeout was reported as a user stop: %q", last.Error)
	}
}

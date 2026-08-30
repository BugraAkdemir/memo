package app

import (
	"encoding/json"
	"testing"
	"time"

	"memo/internal/config"
)

// TestPublishTaskEvent_FanOut is the v4.6.0 Faz D check: raw engine events
// reach every subscriber as an enriched JSON line, non-taskloop events are
// ignored, and unsubscribing stops delivery.
func TestPublishTaskEvent_FanOut(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}

	ch, unsub := a.SubscribeTaskEvents()

	a.publishTaskEvent("taskloop:executing", "L1")
	select {
	case line := <-ch:
		var ev taskChatEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("bad JSON line %q: %v", line, err)
		}
		if ev.Event != "executing" || ev.ListID != "L1" {
			t.Fatalf("got %+v, want event=executing list_id=L1", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber never received the event")
	}

	// A non-taskloop event must not be forwarded.
	a.publishTaskEvent("chat:done", "whatever")
	select {
	case line := <-ch:
		t.Fatalf("non-taskloop event was forwarded: %q", line)
	case <-time.After(100 * time.Millisecond):
	}

	unsub()
	a.publishTaskEvent("tasklist:item_done", "L1:item-1")
	select {
	case line := <-ch:
		t.Fatalf("event delivered after unsubscribe: %q", line)
	case <-time.After(100 * time.Millisecond):
	}
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/telegram"
	"memo/internal/whatsapp"
)

func waMsg(t *testing.T, chatJID, text string) whatsapp.Message {
	t.Helper()
	return whatsapp.Message{ChatJID: chatJID, Text: text, FromMe: true}
}

func tgMsg(chatID int64, text string) telegram.Message {
	return telegram.Message{ChatID: chatID, Text: text}
}

func TestIsAffirmativeAnswer(t *testing.T) {
	yes := []string{"y", "Y", "yes", "YES", "evet", "e", "onay", "onayla", "onaylıyorum", "tamam", "  y  "}
	no := []string{"n", "no", "hayır", "nope", "", "yesplease", "evets", "maybe"}

	for _, s := range yes {
		if !isAffirmativeAnswer(s) {
			t.Errorf("isAffirmativeAnswer(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if isAffirmativeAnswer(s) {
			t.Errorf("isAffirmativeAnswer(%q) = true, want false", s)
		}
	}
}

// TestDrainSelfChatReply_AccumulatesTextAndStopsOnError confirms the plain
// (no permission events involved) path behaves exactly like drainToReply —
// text chunks accumulate, an error chunk short-circuits, agent_event/other
// FinishReason chunks that aren't permission_request are skipped.
func TestDrainSelfChatReply_AccumulatesTextAndStopsOnError(t *testing.T) {
	a := &App{}

	t.Run("accumulates_text_and_skips_non_permission_events", func(t *testing.T) {
		ch := make(chan api.StreamChunk, 8)
		ch <- api.StreamChunk{Content: "Hello, "}
		ch <- api.StreamChunk{Content: `{"type":"tool_executing","tool":"web_search"}`, FinishReason: "agent_event"}
		ch <- api.StreamChunk{Content: "world!"}
		ch <- api.StreamChunk{Content: "9", FinishReason: "status"}
		close(ch)

		got := a.drainSelfChatReply(ch, false, nil, nil, nil)
		if got != "Hello, world!" {
			t.Errorf("drainSelfChatReply() = %q, want %q", got, "Hello, world!")
		}
	})

	t.Run("returns_error_immediately", func(t *testing.T) {
		ch := make(chan api.StreamChunk, 4)
		ch <- api.StreamChunk{Content: "partial"}
		ch <- api.StreamChunk{Error: "boom", Done: true}
		close(ch)

		got := a.drainSelfChatReply(ch, false, nil, nil, nil)
		if got != "boom" {
			t.Errorf("drainSelfChatReply() = %q, want %q", got, "boom")
		}
	})
}

// TestDrainSelfChatReply_AutoApprove confirms that with autoApprove=true, a
// permission_request event is resolved without ever calling
// buildQuestion/sendQuestion/awaitAnswer — those must stay nil-safe to call
// (passing nil funcs and expecting a panic if they were ever invoked is the
// actual assertion here).
func TestDrainSelfChatReply_AutoApprove(t *testing.T) {
	a := &App{}
	ev := agent.AgentEvent{Type: agent.EventPermissionRequest, RequestID: "req-1", ToolName: "run_command"}
	raw, _ := json.Marshal(ev)

	ch := make(chan api.StreamChunk, 4)
	ch <- api.StreamChunk{Content: string(raw), FinishReason: "agent_event"}
	ch <- api.StreamChunk{Content: "done"}
	close(ch)

	// nil buildQuestion/sendQuestion/awaitAnswer would panic if actually
	// called — reaching "done" without panicking proves autoApprove's
	// early-return path was taken.
	got := a.drainSelfChatReply(ch, true, nil, nil, nil)
	if got != "done" {
		t.Errorf("drainSelfChatReply() = %q, want %q", got, "done")
	}
}

// TestDrainSelfChatReply_AsksAndWaitsForAnswer confirms the off-autoApprove
// path: buildQuestion formats the prompt, sendQuestion delivers it, and
// awaitAnswer's return value determines the policy passed onward (verified
// indirectly — HandleAgentPermission errors here since there's no real
// agentExecutor, which is fine; what's under test is that the callbacks
// were invoked with the right event/text, not the final policy delivery,
// which is covered separately by the routing tests).
func TestDrainSelfChatReply_AsksAndWaitsForAnswer(t *testing.T) {
	a := &App{}
	ev := agent.AgentEvent{Type: agent.EventPermissionRequest, RequestID: "req-2", ToolName: "delete_file", Preview: "rm foo.txt"}
	raw, _ := json.Marshal(ev)

	ch := make(chan api.StreamChunk, 4)
	ch <- api.StreamChunk{Content: string(raw), FinishReason: "agent_event"}
	ch <- api.StreamChunk{Content: "after permission"}
	close(ch)

	var builtFor agent.AgentEvent
	var sentQuestion string
	var awaitCalled bool

	got := a.drainSelfChatReply(
		ch,
		false,
		func(e agent.AgentEvent) string {
			builtFor = e
			return "question text"
		},
		func(q string) error {
			sentQuestion = q
			return nil
		},
		func(ctx context.Context) (string, bool) {
			awaitCalled = true
			return "y", true
		},
	)

	if got != "after permission" {
		t.Errorf("drainSelfChatReply() = %q, want %q", got, "after permission")
	}
	if builtFor.RequestID != "req-2" || builtFor.ToolName != "delete_file" {
		t.Errorf("buildQuestion called with wrong event: %+v", builtFor)
	}
	if sentQuestion != "question text" {
		t.Errorf("sendQuestion got %q, want %q", sentQuestion, "question text")
	}
	if !awaitCalled {
		t.Error("awaitAnswer was never called")
	}
}

// TestDrainSelfChatReply_SendQuestionFailure confirms a failure to even
// deliver the question (e.g. WhatsAppSend/TelegramSend erroring) doesn't
// hang waiting for an answer that can never arrive — it must fail safe
// (deny) and move on, not call awaitAnswer at all.
func TestDrainSelfChatReply_SendQuestionFailure(t *testing.T) {
	a := &App{}
	ev := agent.AgentEvent{Type: agent.EventPermissionRequest, RequestID: "req-3", ToolName: "run_command"}
	raw, _ := json.Marshal(ev)

	ch := make(chan api.StreamChunk, 4)
	ch <- api.StreamChunk{Content: string(raw), FinishReason: "agent_event"}
	ch <- api.StreamChunk{Content: "still finishes"}
	close(ch)

	awaitCalled := false
	got := a.drainSelfChatReply(
		ch,
		false,
		func(e agent.AgentEvent) string { return "q" },
		func(q string) error { return context.DeadlineExceeded },
		func(ctx context.Context) (string, bool) {
			awaitCalled = true
			return "y", true
		},
	)

	if got != "still finishes" {
		t.Errorf("drainSelfChatReply() = %q, want %q", got, "still finishes")
	}
	if awaitCalled {
		t.Error("awaitAnswer must not be called when sendQuestion itself failed")
	}
}

// TestDrainSelfChatReply_AwaitTimesOut confirms that when awaitAnswer
// reports no reply arrived in time (ok=false), the loop simply continues
// without panicking or blocking further — the pipeline's own 60s timer is
// what actually resolves this case.
func TestDrainSelfChatReply_AwaitTimesOut(t *testing.T) {
	a := &App{}
	ev := agent.AgentEvent{Type: agent.EventPermissionRequest, RequestID: "req-4", ToolName: "run_command"}
	raw, _ := json.Marshal(ev)

	ch := make(chan api.StreamChunk, 4)
	ch <- api.StreamChunk{Content: string(raw), FinishReason: "agent_event"}
	ch <- api.StreamChunk{Content: "eventually"}
	close(ch)

	got := a.drainSelfChatReply(
		ch,
		false,
		func(e agent.AgentEvent) string { return "q" },
		func(q string) error { return nil },
		func(ctx context.Context) (string, bool) { return "", false },
	)

	if got != "eventually" {
		t.Errorf("drainSelfChatReply() = %q, want %q", got, "eventually")
	}
}

// TestWhatsAppPermissionRouting_AwaitAndRouteRoundTrip exercises
// awaitWhatsAppPermissionAnswer and routeWhatsAppPermissionAnswer together,
// the real end-to-end plumbing runWhatsAppIntentLoop/handleWhatsAppSelfChatMessage
// rely on: a message on the exact pending chat JID is delivered to the
// waiting goroutine; a message on any other JID is left unconsumed.
func TestWhatsAppPermissionRouting_AwaitAndRouteRoundTrip(t *testing.T) {
	a := &App{}
	const jid = "905551234567@s.whatsapp.net"

	resultCh := make(chan struct {
		text string
		ok   bool
	}, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		text, ok := a.awaitWhatsAppPermissionAnswer(ctx, jid)
		resultCh <- struct {
			text string
			ok   bool
		}{text, ok}
	}()

	// Give the goroutine a moment to register the pending channel before
	// routing anything at it.
	time.Sleep(20 * time.Millisecond)

	if consumed := a.routeWhatsAppPermissionAnswer(waMsg(t, "someone-else@s.whatsapp.net", "y")); consumed {
		t.Error("a message on an unrelated JID must not be consumed as the pending answer")
	}
	if consumed := a.routeWhatsAppPermissionAnswer(waMsg(t, jid, "  evet  ")); !consumed {
		t.Fatal("expected the message on the pending JID to be consumed")
	}

	select {
	case res := <-resultCh:
		if !res.ok || res.text != "evet" {
			t.Errorf("await result = %+v, want ok=true text=%q", res, "evet")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("awaitWhatsAppPermissionAnswer never returned")
	}
}

// TestTelegramPermissionRouting_AwaitAndRouteRoundTrip is
// TestWhatsAppPermissionRouting_AwaitAndRouteRoundTrip's Telegram
// counterpart.
func TestTelegramPermissionRouting_AwaitAndRouteRoundTrip(t *testing.T) {
	a := &App{}
	const chatID int64 = 555

	resultCh := make(chan struct {
		text string
		ok   bool
	}, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		text, ok := a.awaitTelegramPermissionAnswer(ctx, chatID)
		resultCh <- struct {
			text string
			ok   bool
		}{text, ok}
	}()

	time.Sleep(20 * time.Millisecond)

	if consumed := a.routeTelegramPermissionAnswer(tgMsg(111, "y")); consumed {
		t.Error("a message on an unrelated chat ID must not be consumed as the pending answer")
	}
	if consumed := a.routeTelegramPermissionAnswer(tgMsg(chatID, "  y  ")); !consumed {
		t.Fatal("expected the message on the pending chat ID to be consumed")
	}

	select {
	case res := <-resultCh:
		if !res.ok || res.text != "y" {
			t.Errorf("await result = %+v, want ok=true text=%q", res, "y")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("awaitTelegramPermissionAnswer never returned")
	}
}

// TestRouteWhatsAppPermissionAnswer_NoneOutstanding confirms routing is a
// harmless no-op when nothing is actually pending — the common case for
// every ordinary incoming self-chat message.
func TestRouteWhatsAppPermissionAnswer_NoneOutstanding(t *testing.T) {
	a := &App{}
	if a.routeWhatsAppPermissionAnswer(waMsg(t, "905551234567@s.whatsapp.net", "selam")) {
		t.Error("expected false when no permission question is outstanding")
	}
}

// TestRouteTelegramPermissionAnswer_NoneOutstanding mirrors the above for
// Telegram.
func TestRouteTelegramPermissionAnswer_NoneOutstanding(t *testing.T) {
	a := &App{}
	if a.routeTelegramPermissionAnswer(tgMsg(555, "selam")) {
		t.Error("expected false when no permission question is outstanding")
	}
}

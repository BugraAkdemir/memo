package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type recordingChatSubAgents struct {
	action string
	args   json.RawMessage
	calls  int
}

func (s *recordingChatSubAgents) SubAgentAction(_ context.Context, action string, args json.RawMessage) (string, error) {
	s.action = action
	s.args = append(json.RawMessage(nil), args...)
	s.calls++
	return `{"status":"queued"}`, nil
}

func TestChatSubAgentActionsAreContextScoped(t *testing.T) {
	a, b := &recordingChatSubAgents{}, &recordingChatSubAgents{}
	ctxA, ctxB := WithChatSubAgentService(context.Background(), a), WithChatSubAgentService(context.Background(), b)
	if !HasChatSubAgentService(ctxA) || HasChatSubAgentService(context.Background()) {
		t.Fatal("service presence is not context scoped")
	}
	args := json.RawMessage(`{"prompt":"Inspect parser","title":"Parser","skills":["go"],"write":false,"isolate":false}`)
	out, err := ChatSubAgentAction(ctxA, "spawn", args)
	if err != nil || out != `{"status":"queued"}` || a.calls != 1 || b.calls != 0 || a.action != "spawn" || string(a.args) != string(args) {
		t.Fatalf("wrong dispatch: %s %v %#v %#v", out, err, a, b)
	}
	if _, err := ChatSubAgentAction(ctxB, "list", json.RawMessage(`{}`)); err != nil || b.calls != 1 {
		t.Fatalf("second scope: %v", err)
	}
	if _, err := ChatSubAgentAction(context.Background(), "list", json.RawMessage(`{}`)); err == nil {
		t.Fatal("missing service accepted")
	}
}

func TestChatSubAgentActionsRejectInvalidOrCrossChatArguments(t *testing.T) {
	s := &recordingChatSubAgents{}
	ctx := WithChatSubAgentService(context.Background(), s)
	for _, tc := range []struct{ action, args string }{
		{"spawn", `{"prompt":" "}`}, {"spawn", `{"prompt":"x","chat_id":"other"}`}, {"spawn", `{"prompt":"x","skills":[""]}`},
		{"send", `{"id":"x","prompt":""}`}, {"send", `{"prompt":"x"}`}, {"merge", `{"id":""}`},
		{"list", `{"id":"x"}`}, {"close", `{"id":"x","chat_id":"other"}`}, {"unknown", `{}`}, {"list", `{} {}`}, {"list", `null`},
	} {
		if _, err := ChatSubAgentAction(ctx, tc.action, json.RawMessage(tc.args)); err == nil {
			t.Errorf("accepted %s %s", tc.action, tc.args)
		}
	}
	if s.calls != 0 {
		t.Fatalf("invalid args reached service: %d", s.calls)
	}
	for _, action := range []string{"send", "merge", "close", "cancel"} {
		args := `{"id":"child"}`
		if action == "send" {
			args = `{"id":"child","prompt":"Fix test"}`
		}
		if _, err := ChatSubAgentAction(ctx, action, json.RawMessage(args)); err != nil {
			t.Errorf("%s: %v", action, err)
		}
	}
}

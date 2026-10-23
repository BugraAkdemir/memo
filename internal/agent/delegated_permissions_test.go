package agent

import (
	"context"
	"encoding/json"
	"memo/internal/provider"
	"testing"
)

func TestDelegatedPermissionsPreserveExplicitDenies(t *testing.T) {
	var ran bool
	reg := NewChatSubAgentRegistry(true)
	reg.Register(ToolDef{Name: "write_file", DangerLevel: Medium, ExecuteFn: func(context.Context, json.RawMessage, string, func(string) error) (string, error) {
		ran = true
		return "wrote", nil
	}})
	pm := NewPermissionManager(t.TempDir())
	pm.HandleResponse("write_file", json.RawMessage(`{}`), DenyForever)
	fake := &scriptedProvider{responses: []provider.ChatResponse{{ToolCalls: []provider.ToolCall{mustToolCall(t, "write", "write_file", map[string]string{"path": "a", "content": "x"})}}, {Content: "blocked"}}}
	p := NewPipeline(reg, pm, NewSandbox(DefaultSandboxConfig(t.TempDir())), fake, nil)
	p.delegatedPermissions = true
	p.codeSubMode = "build"
	ch, err := p.RunStream(context.Background(), nil, "fake", func(AgentEvent) {}, func(string, AgentEvent) (PermissionPolicy, error) {
		t.Error("invisible permission prompt")
		return DenyOnce, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}
	if ran {
		t.Fatal("delegated child overrode user's permanent deny")
	}
}
func TestMutationGuardDoesNotBlockReads(t *testing.T) {
	r := &ToolRegistry{tools: map[string]ToolDef{}}
	for _, name := range []string{"read_file", "write_file"} {
		r.Register(ToolDef{Name: name, ExecuteFn: func(context.Context, json.RawMessage, string, func(string) error) (string, error) { return "ok", nil }})
	}
	calls := 0
	ctx := WithMutationGuard(context.Background(), func(context.Context, string) (func(), error) { calls++; return func() {}, nil })
	if _, err := r.Execute(ctx, "read_file", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("read blocked by writer")
	}
	if _, err := r.Execute(ctx, "write_file", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("mutation not guarded")
	}
}

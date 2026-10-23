package agent

import (
	"context"
	"encoding/json"
	"memo/internal/agent/tools"
	"testing"
)

type chatSubAgentTestService struct{ action string }

func (s *chatSubAgentTestService) SubAgentAction(_ context.Context, action string, _ json.RawMessage) (string, error) {
	s.action = action
	return `{"ok":true}`, nil
}
func TestChatSubAgentToolsRegisteredAndScoped(t *testing.T) {
	r := NewRegistry()
	s := &chatSubAgentTestService{}
	ctx := tools.WithChatSubAgentService(context.Background(), s)
	for name, action := range map[string]string{"spawn_agent": "spawn", "send_agent": "send", "list_agents": "list", "close_agent": "close", "cancel_agent": "cancel", "merge_agent": "merge"} {
		def, ok := r.Get(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if !json.Valid(def.Parameters) {
			t.Fatalf("bad schema %s", name)
		}
		expected := Medium
		if action == "list" {
			expected = Safe
		}
		if def.DangerLevel != expected {
			t.Errorf("%s danger %s", name, def.DangerLevel)
		}
		args := `{"id":"child"}`
		if action == "spawn" {
			args = `{"prompt":"Implement parser"}`
		}
		if action == "send" {
			args = `{"id":"child","prompt":"Fix parser"}`
		}
		if action == "list" {
			args = `{}`
		}
		if _, err := r.Execute(ctx, name, json.RawMessage(args), "", nil); err != nil || s.action != action {
			t.Errorf("%s dispatched %s: %v", name, s.action, err)
		}
	}
	offered := map[string]bool{}
	for _, d := range ForCodeMode(r.ToOpenAITools(nil)) {
		offered[d.Function.Name] = true
	}
	if !offered["spawn_agent"] {
		t.Fatal("Code Mode omitted child tools")
	}
}
func TestChatSubAgentRegistryHasOnlyTaskTools(t *testing.T) {
	for _, write := range []bool{false, true} {
		r := NewChatSubAgentRegistry(write)
		for _, name := range []string{"read_file", "search_content", "search_files", "list_directory", "get_file_info", "web_search", "fetch_page"} {
			if _, ok := r.Get(name); !ok {
				t.Errorf("missing %s", name)
			}
		}
		for _, name := range []string{"spawn_agent", "send_agent", "list_agents", "close_agent", "cancel_agent", "merge_agent", "change_directory", "self_clone", "configure_provider", "get_calendar_events", "read_env", "use_skill", "start_self_driving_task", "run_command_readonly"} {
			if _, ok := r.Get(name); ok {
				t.Errorf("child exposes %s", name)
			}
		}
		for _, name := range []string{"write_file", "edit_file", "insert_line", "delete_lines", "delete_file", "run_command"} {
			if _, ok := r.Get(name); ok != write {
				t.Errorf("write=%v %s presence=%v", write, name, ok)
			}
		}
	}
}

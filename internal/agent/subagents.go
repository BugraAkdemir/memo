package agent

import (
	"context"
	"encoding/json"
	"memo/internal/agent/tools"
)

func (r *ToolRegistry) registerChatSubAgentTools() {
	for _, entry := range []struct{ name, action, description, schema string }{
		{"spawn_agent", "spawn", "Spawn an async child agent for a bounded task (resumes main chat automatically when finished; do not poll). Set write=true to edit files, isolate=true for a separate memo- Git branch/worktree.", `{"type":"object","properties":{"prompt":{"type":"string","minLength":1},"title":{"type":"string"},"write":{"type":"boolean","default":false},"isolate":{"type":"boolean","description":"Separate memo- branch/worktree for parallel writers (defaults to write)"},"skills":{"type":"array","items":{"type":"string","minLength":1}}},"required":["prompt"],"additionalProperties":false}`},
		{"send_agent", "send", "Send a follow-up instruction to an existing child agent (runs async, resumes automatically on completion).", `{"type":"object","properties":{"id":{"type":"string","minLength":1},"prompt":{"type":"string","minLength":1}},"required":["id","prompt"],"additionalProperties":false}`},
		{"list_agents", "list", "Show live sub-agent status (pass id for full output). Do not poll in a loop; check before reporting child status.", `{"type":"object","properties":{"id":{"type":"string","description":"Optional child ID to include full output"}},"additionalProperties":false}`},
		{"close_agent", "close", "Accept and close a completed child task after reviewing/merging its results.", `{"type":"object","properties":{"id":{"type":"string","minLength":1}},"required":["id"],"additionalProperties":false}`},
		{"cancel_agent", "cancel", "Cancel a queued or running child task while keeping unmerged work.", `{"type":"object","properties":{"id":{"type":"string","minLength":1}},"required":["id"],"additionalProperties":false}`},
		{"merge_agent", "merge", "Merge a reviewed isolated child's memo- branch into main and clean its worktree.", `{"type":"object","properties":{"id":{"type":"string","minLength":1}},"required":["id"],"additionalProperties":false}`},
	} {
		action := entry.action
		danger := Medium
		if action == "list" {
			danger = Safe
		}
		r.Register(ToolDef{Name: entry.name, Description: entry.description, Parameters: json.RawMessage(entry.schema), DangerLevel: danger, ExecuteFn: func(ctx context.Context, args json.RawMessage, _ string, _ func(string) error) (string, error) {
			return tools.ChatSubAgentAction(ctx, action, args)
		}})
	}
}

// NewChatSubAgentRegistry exposes task tools only. An allowlist prevents newly
// added main-agent tools from silently giving children personal or delegation
// capabilities. Read-only children have no shell: the legacy shell prefix
// allowlist cannot guarantee that compound commands do not write files.
func NewChatSubAgentRegistry(write bool) *ToolRegistry {
	source := NewReadOnlyRegistry()
	if write {
		source = NewRegistry()
	}
	allowed := map[string]bool{"read_file": true, "list_directory": true, "search_files": true, "search_content": true, "get_file_info": true, "web_search": true, "fetch_page": true}
	if write {
		for _, name := range []string{"write_file", "edit_file", "insert_line", "delete_lines", "delete_file", "run_command"} {
			allowed[name] = true
		}
	}
	child := &ToolRegistry{tools: make(map[string]ToolDef)}
	for name := range allowed {
		if def, ok := source.Get(name); ok {
			child.Register(def)
		}
	}
	return child
}

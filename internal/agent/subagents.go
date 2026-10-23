package agent

import (
	"context"
	"encoding/json"
	"memo/internal/agent/tools"
)

func (r *ToolRegistry) registerChatSubAgentTools() {
	for _, entry := range []struct{ name, action, description, schema string }{
		{"spawn_agent", "spawn", "Delegate a bounded task to an asynchronous sub-agent. Returns its ID and queued/running status immediately. Results automatically resume this chat when its current turn ends; do not poll. Reuse the same child for follow-up work on the same task. Children receive only their task, Code Mode instructions and explicitly selected skills, without personal memory or persona. Set write for coding; use isolate for a memo- Git branch and separate worktree, required for parallel writers. Validate results before merge_agent or close_agent.", `{"type":"object","properties":{"prompt":{"type":"string","minLength":1},"title":{"type":"string"},"write":{"type":"boolean","default":false},"isolate":{"type":"boolean","description":"Defaults to write. True gives a separate memo- branch/worktree for parallel coding; false uses a serialized shared writer."},"skills":{"type":"array","items":{"type":"string","minLength":1}}},"required":["prompt"],"additionalProperties":false}`},
		{"send_agent", "send", "Give an existing child a follow-up instruction for the same task, preserving its task history. Runs asynchronously; completion automatically resumes the main agent. For an independent task create a new child.", `{"type":"object","properties":{"id":{"type":"string","minLength":1},"prompt":{"type":"string","minLength":1}},"required":["id","prompt"],"additionalProperties":false}`},
		{"list_agents", "list", "Live status of this chat's sub-agents: for each, its state (queued/running/failed/...), what it is doing right now, elapsed and idle time, tool-call counts, recent tools, the error if it failed, the tail of its output and what you can do next. Compact; completion is delivered automatically, so do not poll in a loop, but DO call this (or read the <memo-subagent-status> block) before telling the user how a child is doing.", `{"type":"object","properties":{"id":{"type":"string","description":"Optional. A child's ID: also returns its full final output."}},"additionalProperties":false}`},
		{"close_agent", "close", "Accept and close a completed child task after reviewing its results. Merge isolated coding work first; unmerged or conflicted changes must be preserved.", `{"type":"object","properties":{"id":{"type":"string","minLength":1}},"required":["id"],"additionalProperties":false}`},
		{"cancel_agent", "cancel", "Cancel a queued or running child task in this chat, preserving its results and any unmerged coding work.", `{"type":"object","properties":{"id":{"type":"string","minLength":1}},"required":["id"],"additionalProperties":false}`},
		{"merge_agent", "merge", "After reviewing a completed isolated child's changes and verification, merge its memo- branch into main through the serialized merge queue. Successful merges clean the branch/worktree; conflicts and unmerged changes are retained for resolution.", `{"type":"object","properties":{"id":{"type":"string","minLength":1}},"required":["id"],"additionalProperties":false}`},
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

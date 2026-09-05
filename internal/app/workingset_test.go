package app

import (
	"encoding/json"
	"strings"
	"testing"

	"memo/internal/agent"
)

func toolResult(name string, args map[string]any, result string) agent.AgentEvent {
	raw, _ := json.Marshal(args)
	return agent.AgentEvent{Type: agent.EventToolResult, ToolName: name, Args: raw, Result: result}
}

func toolError(name string, args map[string]any, errStr string) agent.AgentEvent {
	raw, _ := json.Marshal(args)
	return agent.AgentEvent{Type: agent.EventToolError, ToolName: name, Args: raw, Error: errStr}
}

func TestWorkingSet_RecordsFilesAndCommands(t *testing.T) {
	ws := newWorkingSet()
	ws.record(toolResult("read_file", map[string]any{"path": "internal/app/llm.go"}, "line1\nline2\nline3"))
	ws.record(toolResult("write_file", map[string]any{"path": "internal/app/new.go", "content": "a\nb"}, "ok"))
	ws.record(toolResult("run_command", map[string]any{"command": "go build ./..."}, ""))
	ws.record(toolError("run_command", map[string]any{"command": "go test ./x"}, "FAIL x/y_test.go:42"))
	// ignored tools
	ws.record(toolResult("search_files", map[string]any{"query": "foo"}, "many hits"))
	ws.record(toolResult("list_directory", map[string]any{"path": "."}, "a\nb\nc"))

	out := ws.render(600)
	for _, want := range []string{
		"internal/app/llm.go (3 lines) — read",
		"internal/app/new.go (2 lines) — wrote",
		"`go build ./...` → ok",
		"`go test ./x` → failed: FAIL x/y_test.go:42",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q\n---\n%s", want, out)
		}
	}
	if strings.Contains(out, "search_files") || strings.Contains(out, "list_directory") {
		t.Errorf("render should ignore search/list tools:\n%s", out)
	}
}

func TestWorkingSet_EmptyRendersNothing(t *testing.T) {
	if got := newWorkingSet().render(600); got != "" {
		t.Errorf("empty working set rendered %q, want empty", got)
	}
}

func TestWorkingSet_MostRecentFileActionWins(t *testing.T) {
	ws := newWorkingSet()
	ws.record(toolResult("read_file", map[string]any{"path": "a.go"}, "x\ny"))
	ws.record(toolResult("edit_file", map[string]any{"path": "a.go"}, "ok"))
	out := ws.render(600)
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "— edited") {
		t.Errorf("latest action should win (edited):\n%s", out)
	}
	if strings.Count(out, "a.go") != 1 {
		t.Errorf("same file must appear once, got:\n%s", out)
	}
}

func TestWorkingSet_CapsFileCount(t *testing.T) {
	ws := newWorkingSet()
	for i := 0; i < wsMaxFiles+8; i++ {
		ws.record(toolResult("read_file", map[string]any{"path": "f" + string(rune('a'+i)) + ".go"}, "x"))
	}
	out := ws.render(0) // no token cap, only the file-count cap
	if n := strings.Count(out, ".go"); n > wsMaxFiles {
		t.Errorf("file list not capped: %d entries\n%s", n, out)
	}
}

func TestWorkingSet_TokenCapTruncates(t *testing.T) {
	ws := newWorkingSet()
	for i := 0; i < wsMaxFiles; i++ {
		ws.record(toolResult("read_file", map[string]any{"path": "some/long/path/to/file" + string(rune('a'+i)) + ".go"}, "x\ny\nz"))
	}
	out := ws.render(20) // absurdly tight
	if !strings.Contains(out, "working set truncated") {
		t.Errorf("expected truncation marker under a tight token cap:\n%s", out)
	}
}

package replcli

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"memo/internal/api"
)

// lockedBuffer: the status spinner writes from its own goroutine.
type lockedBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func liveSession(out *lockedBuffer) *session {
	s := &session{ctx: context.Background(), out: out, webTools: true}
	s.sp = newSpinner(out)
	return s
}

func agentChunk(t *testing.T, ev map[string]any) api.StreamChunk {
	t.Helper()
	b, _ := json.Marshal(ev)
	return api.StreamChunk{Content: string(b), FinishReason: "agent_event"}
}

func feed(t *testing.T, s *session, chunks ...api.StreamChunk) {
	t.Helper()
	for _, c := range chunks {
		if err := s.handleChunk(c); err != nil {
			t.Fatal(err)
		}
	}
	s.stopSpinner()
	s.endLine()
}

// The whole turn, as the backend streams it now: thinking, the words the model
// writes next to a tool call, the tool (one permanent line), then the answer.
func TestLiveRender_ThinkingNarrationToolAndAnswerEachOnceAndInOrder(t *testing.T) {
	out := &lockedBuffer{}
	s := liveSession(out)
	feed(t, s,
		api.StreamChunk{Thinking: "önce testlere bakmalıyım"},
		agentChunk(t, map[string]any{"type": "narration_delta", "content": "Testleri "}),
		agentChunk(t, map[string]any{"type": "narration_delta", "content": "çalıştırıyorum."}),
		agentChunk(t, map[string]any{"type": "narration", "content": "Testleri çalıştırıyorum."}),
		agentChunk(t, map[string]any{"type": "tool_executing", "tool": "run_command", "args": map[string]any{"command": "go test ./..."}}),
		agentChunk(t, map[string]any{"type": "tool_result", "tool": "run_command", "args": map[string]any{"command": "go test ./..."}, "duration_ms": 1200}),
		agentChunk(t, map[string]any{"type": "narration_delta", "content": "Hepsi "}),
		agentChunk(t, map[string]any{"type": "narration_delta", "content": "geçti."}),
		agentChunk(t, map[string]any{"type": "final_response", "content": "Hepsi geçti."}),
		api.StreamChunk{Content: "Hepsi geçti."},
		api.StreamChunk{Done: true, FinishReason: "stop"},
	)
	got := out.String()

	for _, want := range []string{"önce testlere bakmalıyım", "Testleri çalıştırıyorum.", "go test ./...", "1200ms", "Hepsi geçti."} {
		if strings.Count(got, want) != 1 {
			t.Errorf("%q appears %d times, want exactly once:\n%s", want, strings.Count(got, want), got)
		}
	}
	order := []string{"önce testlere bakmalıyım", "Testleri çalıştırıyorum.", "go test ./...", "Hepsi geçti."}
	last := -1
	for _, w := range order {
		i := strings.Index(got, w)
		if i < last {
			t.Errorf("%q is out of order:\n%s", w, got)
		}
		last = i
	}
}

// A backend or provider that does not stream live (older backend, blocking
// fallback): narration arrives whole as one event and the answer as one chunk —
// both still shown, once.
func TestLiveRender_WithoutLiveDeltasPrintsNarrationAndAnswerWhole(t *testing.T) {
	out := &lockedBuffer{}
	s := liveSession(out)
	feed(t, s,
		agentChunk(t, map[string]any{"type": "narration", "content": "Dosyayı okuyorum."}),
		agentChunk(t, map[string]any{"type": "tool_result", "tool": "read_file", "args": map[string]any{"path": "a.go"}}),
		api.StreamChunk{Content: "Bitti."},
	)
	got := out.String()
	for _, want := range []string{"Dosyayı okuyorum.", "read_file a.go", "Bitti."} {
		if strings.Count(got, want) != 1 {
			t.Errorf("%q appears %d times:\n%s", want, strings.Count(got, want), got)
		}
	}
}

func TestLiveRender_FailedToolShowsTheReason(t *testing.T) {
	out := &lockedBuffer{}
	s := liveSession(out)
	feed(t, s, agentChunk(t, map[string]any{"type": "tool_error", "tool": "edit_file", "args": map[string]any{"path": "x.go"}, "error": "old_string not found"}))
	got := out.String()
	if !strings.Contains(got, "✗") || !strings.Contains(got, "edit_file x.go") || !strings.Contains(got, "old_string not found") {
		t.Errorf("failed tool line:\n%s", got)
	}
}

func TestToolArgSummary(t *testing.T) {
	cases := []struct {
		tool string
		args string
		want string
	}{
		{"run_command", `{"command":"go  test\n./..."}`, "go test ./..."},
		{"read_file", `{"path":"a/b.go"}`, "a/b.go"},
		{"search_content", `{"pattern":"func main","path":"."}`, "func main"},
		{"web_search", `{"query":"dart sealed"}`, "dart sealed"},
		{"read_file", `"{\"path\":\"a.go\"}"`, "a.go"},
		{"read_file", `not json`, ""},
		{"read_file", ``, ""},
	}
	for _, c := range cases {
		if got := toolArgSummary(c.tool, json.RawMessage(c.args)); got != c.want {
			t.Errorf("toolArgSummary(%s, %s) = %q, want %q", c.tool, c.args, got, c.want)
		}
	}
	long := toolArgSummary("run_command", json.RawMessage(`{"command":"`+strings.Repeat("x", 400)+`"}`))
	if len([]rune(long)) > 121 {
		t.Errorf("summary not bounded: %d runes", len([]rune(long)))
	}
}

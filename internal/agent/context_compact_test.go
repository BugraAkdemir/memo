package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/provider"
)

func toolExchange(t *testing.T, id, name, args, result string) []provider.Message {
	t.Helper()
	return []provider.Message{
		{Role: "assistant", ToolCalls: []provider.ToolCall{{ID: id, Type: "function", Function: provider.ToolCallFunction{Name: name, Arguments: json.RawMessage(args)}}}},
		{Role: "tool", ToolCallID: id, Content: result},
	}
}

// The reported failure: a long turn forgot what it was asked. The seed (system
// prompt, history, the user's request) must survive any compaction untouched.
func TestCompactTurn_NeverTouchesTheSeed(t *testing.T) {
	seed := []provider.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "earlier question"},
		{Role: "assistant", Content: "earlier answer"},
		{Role: "user", Content: "THE TASK: refactor the parser"},
	}
	msgs := append([]provider.Message(nil), seed...)
	for i := 0; i < 12; i++ {
		msgs = append(msgs, toolExchange(t, string(rune('a'+i)), "read_file", `"{\"path\":\"f.go\"}"`, strings.Repeat("x", 30000))...)
	}
	out, changed := compactTurn(msgs, len(seed), 20000, 0, 2)
	if !changed {
		t.Fatal("nothing was compacted")
	}
	for i, m := range seed {
		if out[i].Content != m.Content || out[i].Role != m.Role {
			t.Fatalf("seed message %d was changed: %+v", i, out[i])
		}
	}
	if estimateMessagesTokens(out) > 20000 {
		t.Errorf("still %d tokens, target 20000", estimateMessagesTokens(out))
	}
}

// Oldest tool outputs go first, the newest stay whole, and every tool call
// keeps its result (an orphan is a hard 400 on most providers).
func TestCompactTurn_ElidesOldestOutputsFirstAndKeepsPairing(t *testing.T) {
	seed := []provider.Message{{Role: "system", Content: "s"}, {Role: "user", Content: "go"}}
	msgs := append([]provider.Message(nil), seed...)
	for i := 0; i < 10; i++ {
		msgs = append(msgs, toolExchange(t, string(rune('a'+i)), "run_command", `"{\"command\":\"ls\"}"`, strings.Repeat("o", 9000))...)
	}
	out, changed := compactTurn(msgs, len(seed), 12000, 0, 3)
	if !changed {
		t.Fatal("nothing compacted")
	}
	var toolResults []string
	calls := map[string]bool{}
	for _, m := range out {
		for _, tc := range m.ToolCalls {
			calls[tc.ID] = true
		}
		if m.Role == "tool" {
			if !calls[m.ToolCallID] {
				t.Fatalf("orphan tool result %q", m.ToolCallID)
			}
			toolResults = append(toolResults, m.Content.(string))
		}
	}
	// A call is only dropped together with its result, never alone.
	for id := range calls {
		found := false
		for _, m := range out {
			if m.Role == "tool" && m.ToolCallID == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("tool call %q lost its result", id)
		}
	}
	last := toolResults[len(toolResults)-1]
	if strings.HasPrefix(last, contextTrimMarker) {
		t.Errorf("the newest tool output must stay whole, got %q", last)
	}
	if !strings.HasPrefix(toolResults[0], contextTrimMarker) && len(toolResults) == 10 {
		t.Errorf("the oldest output should be the first to be elided, got %q", toolResults[0][:20])
	}
}

// Big arguments of OLD tool calls (a write_file body) are elided too, and the
// call stays valid JSON in both argument shapes.
func TestCompactTurn_ElidesHugeArgumentsAsValidJSON(t *testing.T) {
	body := strings.Repeat("code ", 6000)
	objArgs, _ := json.Marshal(map[string]string{"path": "a.go", "content": body})
	strArgs, _ := json.Marshal(string(objArgs)) // OpenAI-compatible shape: a JSON string holding the object
	for name, args := range map[string]json.RawMessage{"object": objArgs, "string": strArgs} {
		t.Run(name, func(t *testing.T) {
			seed := []provider.Message{{Role: "system", Content: "s"}, {Role: "user", Content: "go"}}
			msgs := append([]provider.Message(nil), seed...)
			msgs = append(msgs, provider.Message{Role: "assistant", ToolCalls: []provider.ToolCall{{ID: "w1", Type: "function", Function: provider.ToolCallFunction{Name: "write_file", Arguments: args}}}},
				provider.Message{Role: "tool", ToolCallID: "w1", Content: "ok"})
			for i := 0; i < 8; i++ {
				msgs = append(msgs, toolExchange(t, string(rune('a'+i)), "read_env", `"{}"`, "small")...)
			}
			out, changed := compactTurn(msgs, len(seed), 3000, 0, 2)
			if !changed {
				t.Fatal("nothing compacted")
			}
			got := out[len(seed)].ToolCalls[0].Function.Arguments
			if len(got) > 2000 {
				t.Fatalf("arguments still %d bytes", len(got))
			}
			var inner json.RawMessage = got
			var asStr string
			if json.Unmarshal(got, &asStr) == nil {
				inner = json.RawMessage(asStr)
			}
			var obj map[string]string
			if err := json.Unmarshal(inner, &obj); err != nil {
				t.Fatalf("arguments are no longer valid JSON: %s (%v)", got, err)
			}
			if obj["path"] != "a.go" || !strings.Contains(obj["content"], "elided") {
				t.Errorf("obj = %+v", obj)
			}
		})
	}
}

// When eliding is not enough, whole old exchanges go — but never the newest.
func TestCompactTurn_LastResortDropsOldExchangesNotTheNewest(t *testing.T) {
	seed := []provider.Message{{Role: "system", Content: "s"}, {Role: "user", Content: "go"}}
	msgs := append([]provider.Message(nil), seed...)
	for i := 0; i < 8; i++ {
		msgs = append(msgs, toolExchange(t, string(rune('a'+i)), "read_file", `"{\"path\":\"p\"}"`, strings.Repeat("y", 100))...)
	}
	out, changed := compactTurn(msgs, len(seed), 60, 0, 2)
	if !changed {
		t.Fatal("nothing compacted")
	}
	lastID := ""
	for _, m := range out {
		for _, tc := range m.ToolCalls {
			lastID = tc.ID
		}
	}
	if lastID != "h" {
		t.Errorf("newest exchange was dropped, last call = %q", lastID)
	}
	note := 0
	for _, m := range out {
		if s, ok := m.Content.(string); ok && strings.Contains(s, "Earlier steps of this turn were dropped") {
			note++
		}
	}
	if note != 1 {
		t.Errorf("want exactly one drop note, got %d", note)
	}
}

func TestCompactTurn_UnderTargetChangesNothing(t *testing.T) {
	msgs := []provider.Message{{Role: "system", Content: "s"}, {Role: "user", Content: "go"}}
	out, changed := compactTurn(msgs, 2, 100000, 0, 2)
	if changed || len(out) != 2 {
		t.Fatalf("changed=%v len=%d", changed, len(out))
	}
}

// Tool-call arguments count toward the estimate — the old guard ignored them
// and undercounted a write-heavy turn by most of its size.
func TestEstimateMessageTokens_CountsToolCallArguments(t *testing.T) {
	m := provider.Message{Role: "assistant", ToolCalls: []provider.ToolCall{{Function: provider.ToolCallFunction{Name: "write_file", Arguments: json.RawMessage(`"` + strings.Repeat("a", 30000) + `"`)}}}}
	if got := estimateMessageTokens(m); got < 9000 {
		t.Errorf("estimate = %d, want the 30k-char argument counted (~10000)", got)
	}
}

// Pipeline: the provider's own prompt-token count triggers compaction, the
// task survives, and the model is told how to recover elided output.
func TestRunStream_CompactsFromRealPromptTokensAndKeepsTheTask(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(strings.Repeat("line of text\n", 3000)), 0o644); err != nil {
		t.Fatal(err)
	}
	var responses []provider.ChatResponse
	for i := 0; i < 5; i++ {
		responses = append(responses, provider.ChatResponse{
			ToolCalls: []provider.ToolCall{mustToolCall(t, string(rune('a'+i)), "read_file", map[string]string{"path": "a.txt"})},
			// The provider says the context is already huge.
			Usage: &provider.Usage{PromptTokens: 90000, CompletionTokens: 10, TotalTokens: 90010},
		})
	}
	responses = append(responses, provider.ChatResponse{Content: "done"})
	prov := &recordingProvider{responses: responses}
	pl := NewPipelineWithBudget(NewRegistry(), NewPermissionManager(t.TempDir()), NewSandbox(DefaultSandboxConfig(dir)), prov, nil, 80000)
	pl.autoPermission = true

	seed := []provider.Message{{Role: "system", Content: "sys"}, {Role: "user", Content: "THE TASK"}}
	ch, err := pl.RunStream(context.Background(), seed, "m", func(AgentEvent) {}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}
	last := prov.seen[len(prov.seen)-1]
	if s, _ := last[1].Content.(string); s != "THE TASK" {
		t.Fatalf("the user's request was lost: %+v", last[1])
	}
	elided := 0
	for _, m := range last {
		if s, ok := m.Content.(string); ok && m.Role == "tool" && strings.HasPrefix(s, contextTrimMarker) {
			elided++
		}
	}
	if elided == 0 {
		t.Errorf("no tool output was elided although the provider reported 90000 prompt tokens against an 80000 budget")
	}
}

// A 400 "prompt too long" mid-turn is recovered by compacting and retrying,
// not by ending the turn.
func TestRunStream_ContextOverflowIsRecoveredByCompacting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(strings.Repeat("z", 20000)), 0o644); err != nil {
		t.Fatal(err)
	}
	prov := &overflowOnceProvider{
		toolSteps: 4,
		file:      "a.txt",
	}
	pl := NewPipelineWithBudget(NewRegistry(), NewPermissionManager(t.TempDir()), NewSandbox(DefaultSandboxConfig(dir)), prov, nil, 10_000_000)
	pl.autoPermission = true
	seed := []provider.Message{{Role: "system", Content: "sys"}, {Role: "user", Content: "THE TASK"}}
	ch, err := pl.RunStream(context.Background(), seed, "m", func(AgentEvent) {}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var content string
	var last provider.StreamChunk
	for c := range ch {
		content += c.Content
		last = c
	}
	if last.Error != "" || content != "finished" {
		t.Fatalf("turn did not recover: content=%q last=%+v", content, last)
	}
	if prov.overflows != 1 {
		t.Errorf("overflow raised %d times, want exactly 1 (then recovered)", prov.overflows)
	}
	if !prov.retryWasSmaller {
		t.Error("the retry was not smaller than the refused request")
	}
}

// overflowOnceProvider makes toolSteps read_file calls, then refuses the next
// request once as "too long", then answers if (and only if) it got smaller.
type overflowOnceProvider struct {
	toolSteps       int
	file            string
	calls           int
	overflows       int
	refusedSize     int
	retryWasSmaller bool
}

func (p *overflowOnceProvider) ChatCompletion(_ context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	p.calls++
	size := estimateMessagesTokens(req.Messages)
	if p.calls <= p.toolSteps {
		return &provider.ChatResponse{ToolCalls: []provider.ToolCall{{ID: string(rune('a' + p.calls)), Type: "function", Function: provider.ToolCallFunction{Name: "read_file", Arguments: json.RawMessage(`"{\"path\":\"` + p.file + `\"}"`)}}}}, nil
	}
	if p.overflows == 0 {
		p.overflows++
		p.refusedSize = size
		return nil, errors.New("all providers failed: [custom] status 400: The input token count exceeds the maximum number of tokens allowed (1048576).")
	}
	p.retryWasSmaller = size < p.refusedSize
	return &provider.ChatResponse{Content: "finished"}, nil
}

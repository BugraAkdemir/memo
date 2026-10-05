package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"memo/internal/provider"
)

// TestRunStream_ClaudeThinkingBlocksSurviveTheToolLoop is S4 (BUG_REPORT
// 2026-09-28), driven through the real agent pipeline and the real Claude
// provider against a fake Messages API that enforces the rule the real one
// documents: with thinking on, the assistant turn that made the tool call
// must come back with its thinking block unchanged — signature included —
// when the tool result is sent. The fake answers 400 otherwise.
//
// The pipeline used to rebuild that turn from text + tool_use only, so on an
// effort-level (adaptive thinking) agent turn — and on Opus 5, where
// thinking is on by default — the second iteration of any tool loop lost it.
func TestRunStream_ClaudeThinkingBlocksSurviveTheToolLoop(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	const sig = "sig-from-server-1"
	var mu sync.Mutex
	calls := 0
	var secondReqAssistant []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Thinking any `json:"thinking"`
			Messages []struct {
				Role    string           `json:"role"`
				Content []map[string]any `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(raw, &req)
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			// Thinking (empty text — the default display on current models)
			// then a tool call.
			w.Write([]byte(`{"model":"claude-opus-5","stop_reason":"tool_use","content":[` +
				`{"type":"thinking","thinking":"","signature":"` + sig + `"},` +
				`{"type":"tool_use","id":"toolu_1","name":"read_file","input":{"path":"note.txt"}}],` +
				`"usage":{"input_tokens":10,"output_tokens":5}}`))
			return
		}
		for _, m := range req.Messages {
			if m.Role == "assistant" {
				secondReqAssistant = m.Content
			}
		}
		if req.Thinking != nil {
			ok := len(secondReqAssistant) > 0 &&
				secondReqAssistant[0]["type"] == "thinking" &&
				secondReqAssistant[0]["signature"] == sig
			if _, has := secondReqAssistant[0]["thinking"]; !has {
				ok = false
			}
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"messages.1.content.0.type: Expected ` + "`thinking`" + ` or ` + "`redacted_thinking`" + `, but found ` + "`tool_use`" + `. When thinking is enabled, a final assistant message must start with a thinking block."}}`))
				return
			}
		}
		w.Write([]byte(`{"model":"claude-opus-5","stop_reason":"end_turn","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":20,"output_tokens":2}}`))
	}))
	defer srv.Close()

	prov, err := provider.NewProvider(provider.ProviderConfig{
		Name: "claude", Type: provider.ProviderClaude, BaseURL: srv.URL, APIKey: "k", Model: "claude-opus-5", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	pipeline := NewPipeline(NewRegistry(), NewPermissionManager(t.TempDir()), NewSandbox(DefaultSandboxConfig(dir)), prov, nil)
	pipeline.autoPermission = true
	pipeline.effortLevel = "high" // adaptive thinking on

	ch, err := pipeline.RunStream(context.Background(),
		[]provider.Message{{Role: "system", Content: "sys"}, {Role: "user", Content: "read note.txt"}},
		"claude-opus-5", func(AgentEvent) {}, nil)
	if err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	var final string
	var streamErr string
	for c := range ch {
		final += c.Content
		if c.Error != "" {
			streamErr = c.Error
		}
	}
	if streamErr != "" {
		t.Fatalf("agent turn failed: %s", streamErr)
	}
	if final != "done" {
		t.Errorf("final content = %q, want done", final)
	}
	if calls != 2 {
		t.Errorf("provider saw %d requests, want 2", calls)
	}
}

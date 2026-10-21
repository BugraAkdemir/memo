// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestAgent_StoppingATurnKeepsWhatItAlreadyDid is the real-connection proof for
// the stop button on an agent turn. The model has read a file (a tool call
// that already finished and was shown) and is still working on its next step
// when the client drops the connection — which is what pressing stop does.
// The saved reply used to be the bare "stopped" marker: every tool call the
// turn had shown vanished with it. Now they stay on the saved message.
func TestAgent_StoppingATurnKeepsWhatItAlreadyDid(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)

	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "notes.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	release := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	var agentCalls atomic.Int32
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		if len(req.Tools) == 0 {
			return FakeChatResponse{Text: "aux"}
		}
		if agentCalls.Add(1) == 1 {
			return FakeChatResponse{ToolCalls: []FakeToolCall{{ID: "call_1", Name: "read_file", Arguments: `{"path":"notes.txt"}`}}}
		}
		<-release // still thinking when the user stops
		return FakeChatResponse{Text: "late answer that nobody sees"}
	}

	chatID := h.NewAgentChat(projectDir)
	resp := h.postJSON("/api/send/stream", map[string]any{"chat_id": chatID, "message": "notes.txt dosyasını oku"})
	sawResult := false
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	deadline := time.After(10 * time.Second)
	lines := make(chan string)
	go func() {
		defer close(lines)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
read:
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				break read
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var ev SSEEvent
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev) != nil {
				continue
			}
			if ev.FinishReason == "agent_event" && strings.Contains(ev.Content, `"tool_result"`) && strings.Contains(ev.Content, "read_file") {
				sawResult = true
				break read
			}
		case <-deadline:
			t.Fatal("the tool result never reached the client")
		}
	}
	if !sawResult {
		t.Fatal("stream ended before the tool result")
	}
	resp.Body.Close() // the stop button: the client goes away mid-turn

	type stored struct {
		Role        string        `json:"role"`
		Content     string        `json:"content"`
		AgentEvents []interface{} `json:"agent_events"`
	}
	var assistant *stored
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		var msgs []stored
		decodeInto(t, h.getJSON("/api/messages?chat_id="+chatID), &msgs)
		for i := range msgs {
			if msgs[i].Role == "assistant" {
				assistant = &msgs[i]
			}
		}
		if assistant != nil {
			break
		}
	}
	if assistant == nil {
		t.Fatal("the stopped turn was never saved")
	}
	if !strings.Contains(assistant.Content, "durduruldu") && !strings.Contains(assistant.Content, "stopped") {
		t.Errorf("saved reply has no stop marker: %q", assistant.Content)
	}
	if strings.Contains(assistant.Content, "late answer") {
		t.Errorf("text the model produced after the stop was saved: %q", assistant.Content)
	}
	kept := false
	for _, ev := range assistant.AgentEvents {
		if m, ok := ev.(map[string]interface{}); ok && m["tool"] == "read_file" {
			kept = true
		}
	}
	if !kept {
		t.Errorf("the tool call that had already run vanished from the stopped turn: %+v", assistant.AgentEvents)
	}
}

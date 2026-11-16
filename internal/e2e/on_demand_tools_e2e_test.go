// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/config"
)

func reqToolNames(req FakeChatRequest) []string {
	var out []string
	for _, raw := range req.Tools {
		var item struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		}
		if json.Unmarshal(raw, &item) == nil && item.Function.Name != "" {
			out = append(out, item.Function.Name)
		}
	}
	return out
}

func TestOnDemandTools_StartsWithLoadToolsAndLoadsOnDemand(t *testing.T) {
	h := newHarness(t, func(cfg *config.AppConfig) {
		cfg.AgentMode.OnDemandTools = true
	})
	h.SetAgentEnabled(true)

	wsDir := filepath.Join(config.DataDir(), "workspace")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "hello.txt"), []byte("on-demand-ok"), 0644); err != nil {
		t.Fatalf("write hello.txt: %v", err)
	}

	var offeredPerCall [][]string
	var loadDesc string
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		if len(req.Tools) == 0 {
			// Background chat-title generation call arrives with no tools.
			return FakeChatResponse{Text: "test title"}
		}
		offeredPerCall = append(offeredPerCall, reqToolNames(req))
		switch len(offeredPerCall) {
		case 1:
			for _, raw := range req.Tools {
				var item struct {
					Function struct {
						Name        string `json:"name"`
						Description string `json:"description"`
					} `json:"function"`
				}
				if json.Unmarshal(raw, &item) == nil && item.Function.Name == "load_tools" {
					loadDesc = item.Function.Description
				}
			}
			// Pinned core tool (read_file) executes directly on call 1 with zero load_tools roundtrip.
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "tc-1",
				Name:      "read_file",
				Arguments: `{"path":"hello.txt"}`,
			}}}
		case 2:
			// Deferred group (tasks) is loaded dynamically via load_tools.
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "tc-2",
				Name:      "load_tools",
				Arguments: `{"groups":["tasks"]}`,
			}}}
		default:
			var sawFileContent bool
			for _, m := range req.Messages {
				if strings.Contains(string(m), "on-demand-ok") {
					sawFileContent = true
					break
				}
			}
			if !sawFileContent {
				return FakeChatResponse{Text: "missing file content"}
			}
			return FakeChatResponse{Text: "read: on-demand-ok"}
		}
	}

	chatID := h.NewChat()
	var reply strings.Builder
	for chunk := range h.SendMessageStreamAsync(chatID, "read hello.txt and check tasks") {
		if chunk.Error != "" {
			t.Fatalf("stream error: %s", chunk.Error)
		}
		reply.WriteString(chunk.Content)
	}
	if !strings.Contains(reply.String(), "on-demand-ok") {
		t.Fatalf("reply = %q, want it to contain on-demand-ok", reply.String())
	}
	if len(offeredPerCall) != 3 {
		t.Fatalf("model calls = %d, want 3 (%v)", len(offeredPerCall), offeredPerCall)
	}
	call1Set := make(map[string]bool, len(offeredPerCall[0]))
	for _, n := range offeredPerCall[0] {
		call1Set[n] = true
	}
	for _, core := range []string{"load_tools", "read_file", "list_directory", "search_content", "web_search"} {
		if !call1Set[core] {
			t.Fatalf("call 1 missing hybrid core tool %q: %v", core, offeredPerCall[0])
		}
	}
	for _, deferred := range []string{"create_task_md", "edit_file", "run_command", "mail_send", "whatsapp_send"} {
		if call1Set[deferred] {
			t.Fatalf("call 1 should not offer deferred/disconnected tool %q upfront", deferred)
		}
	}
	if !strings.Contains(loadDesc, "tasks =") {
		t.Errorf("load_tools description missing tasks: %q", loadDesc)
	}
	for _, absent := range []string{"mail =", "contacts ="} {
		if strings.Contains(loadDesc, absent) {
			t.Errorf("load_tools should omit disconnected group %q: %q", absent, loadDesc)
		}
	}
	var hasCreateTaskMd bool
	for _, name := range offeredPerCall[2] {
		if name == "create_task_md" {
			hasCreateTaskMd = true
		}
		if name == "mail_send" || name == "whatsapp_send" {
			t.Errorf("disconnected tool %q should never be offered", name)
		}
	}
	if !hasCreateTaskMd {
		t.Fatalf("call 3 tools = %v, want create_task_md after loading tasks", offeredPerCall[2])
	}
}

func TestOnDemandTools_SettingsEndpointRoundTrip(t *testing.T) {
	h := newHarness(t, func(cfg *config.AppConfig) {
		cfg.AgentMode.OnDemandTools = true
	})
	resp, err := h.client.Get(h.BaseURL + "/api/agent/on-demand-tools")
	if err != nil {
		t.Fatalf("GET on-demand-tools: %v", err)
	}
	var state struct {
		Enabled bool `json:"enabled"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&state)
	resp.Body.Close()
	if !state.Enabled {
		t.Fatalf("initial GET enabled = false, want true")
	}

	req, _ := http.NewRequest(http.MethodPut, h.BaseURL+"/api/agent/on-demand-tools", bytes.NewReader([]byte(`{"enabled":false}`)))
	putResp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("PUT on-demand-tools: %v", err)
	}
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200", putResp.StatusCode)
	}
	if h.App.GetOnDemandTools() {
		t.Fatalf("GetOnDemandTools = true after PUT false")
	}
}

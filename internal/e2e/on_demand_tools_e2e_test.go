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

	proj := t.TempDir()
	if err := os.WriteFile(filepath.Join(proj, "hello.txt"), []byte("on-demand-ok"), 0644); err != nil {
		t.Fatalf("write hello.txt: %v", err)
	}

	var offeredPerCall [][]string
	var loadDesc string
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		offeredPerCall = append(offeredPerCall, reqToolNames(req))
		switch callNum {
		case 1:
			if len(req.Tools) == 1 {
				var item struct {
					Function struct {
						Description string `json:"description"`
					} `json:"function"`
				}
				_ = json.Unmarshal(req.Tools[0], &item)
				loadDesc = item.Function.Description
			}
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "tc-1",
				Name:      "load_tools",
				Arguments: `{"groups":["files_read"]}`,
			}}}
		case 2:
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "tc-2",
				Name:      "read_file",
				Arguments: `{"path":"hello.txt"}`,
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

	chatID := h.NewAgentChat(proj)
	var reply strings.Builder
	for chunk := range h.SendMessageStreamAsync(chatID, "read hello.txt") {
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
	if len(offeredPerCall[0]) != 1 || offeredPerCall[0][0] != "load_tools" {
		t.Fatalf("call 1 tools = %v, want only [load_tools]", offeredPerCall[0])
	}
	if !strings.Contains(loadDesc, "files_read =") {
		t.Errorf("load_tools description missing files_read: %q", loadDesc)
	}
	for _, absent := range []string{"mail =", "contacts ="} {
		if strings.Contains(loadDesc, absent) {
			t.Errorf("load_tools should omit disconnected group %q: %q", absent, loadDesc)
		}
	}
	var hasReadFile bool
	for _, name := range offeredPerCall[1] {
		if name == "read_file" {
			hasReadFile = true
		}
		if name == "mail_send" || name == "whatsapp_send" {
			t.Errorf("disconnected tool %q should never be offered", name)
		}
	}
	if !hasReadFile {
		t.Fatalf("call 2 tools = %v, want read_file after loading files_read", offeredPerCall[1])
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

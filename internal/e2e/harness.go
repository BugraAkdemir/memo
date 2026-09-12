// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"memo/internal/app"
	"memo/internal/config"
	"memo/internal/provider"
)

//go:embed testdata/embed_placeholder.txt
var emptyBinaries embed.FS

// Harness boots a real app.App behind a real HTTP server (exactly the
// app.NewApp -> a.Startup -> a.StartWebServerHTTP sequence main.go uses for
// a headless backend) on an OS-assigned loopback port, wired to a
// FakeProvider instead of a real LLM. Every helper method below drives it
// through the same public REST API a real Flutter client uses — nothing
// here reaches into App's private fields.
type Harness struct {
	App     *app.App
	Fake    *FakeProvider
	BaseURL string

	t      *testing.T
	client *http.Client
}

// NewHarness starts a fresh, fully isolated Harness: its own MEMO_DATA_DIR
// (t.TempDir(), so no state survives or leaks between tests), a fresh
// FakeProvider set as the sole active provider, and a real HTTP server
// bound to 127.0.0.1 on a free port. t.Cleanup shuts everything down.
//
// Memory/Whisper/TTS/WhatsApp/Proactive/Dream all stay off (see
// config.Default's own off-by-default choices, reinforced explicitly below
// where Default already leaves them on) — this harness is for proving real
// chat/agent/task-loop behavior fast and deterministically, not for
// exercising those other subsystems, which would need their own real
// dependencies (a real embedding model, a real WhatsApp pairing, etc.) to
// test honestly rather than being incidentally half-started here.
func NewHarness(t *testing.T) *Harness {
	t.Helper()
	t.Setenv("MEMO_DATA_DIR", t.TempDir())

	// Written before Startup so config.Load picks it up on first read —
	// keeps every test fast and its provider-call count predictable by
	// turning off every subsystem that would otherwise make its own
	// background LLM/network calls around a real chat turn: memory (needs a
	// real embedding model this harness never provides — every retrieval/
	// save would just fail loudly into the log), auto fact extraction
	// (a second, real LLM call after some turns), WhatsApp (its "connect"
	// attempt alone added ~5s to every single test here before this was
	// disabled), proactive suggestions, and dream consolidation.
	cfg := config.Default()
	cfg.Memory.MemoryEnabled = false
	cfg.WhatsApp.Enabled = false
	cfg.Proactive.Enabled = false
	cfg.Memory.DreamEnabled = false
	if err := config.Save(cfg); err != nil {
		t.Fatalf("write initial e2e config: %v", err)
	}

	fp := NewFakeProvider(t)

	a := app.NewApp(emptyBinaries, "e2e-test")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	a.Startup(ctx)
	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		a.Shutdown(shutCtx)
	})

	if err := a.UpdateProvider(provider.ProviderConfig{
		Type:    provider.ProviderCustom,
		Name:    "e2e-fake",
		BaseURL: fp.Srv.URL,
		Model:   "fake-model",
		APIKey:  "e2e-test-key",
		Enabled: true,
	}); err != nil {
		t.Fatalf("UpdateProvider: %v", err)
	}
	a.SetActiveProvider("e2e-fake")

	port := freePort(t)
	if err := a.StartWebServerHTTP(port); err != nil {
		t.Fatalf("StartWebServerHTTP: %v", err)
	}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForListening(t, port)

	return &Harness{
		App:     a,
		Fake:    fp,
		BaseURL: baseURL,
		t:       t,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func waitForListening(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server never started listening on %s", addr)
}

// --- generic JSON HTTP helpers ---

func (h *Harness) postJSON(path string, body any) *http.Response {
	h.t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		h.t.Fatalf("marshal request body for %s: %v", path, err)
	}
	resp, err := h.client.Post(h.BaseURL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		h.t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func (h *Harness) putJSON(path string, body any) *http.Response {
	h.t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		h.t.Fatalf("marshal request body for %s: %v", path, err)
	}
	req, err := http.NewRequest(http.MethodPut, h.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		h.t.Fatalf("build PUT %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("PUT %s: %v", path, err)
	}
	return resp
}

func (h *Harness) getJSON(path string) *http.Response {
	h.t.Helper()
	resp, err := h.client.Get(h.BaseURL + path)
	if err != nil {
		h.t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func decodeInto(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

// --- chat helpers ---

// NewChat creates a real chat via POST /api/chats/new and returns its id.
func (h *Harness) NewChat() string {
	h.t.Helper()
	resp := h.postJSON("/api/chats/new", map[string]any{})
	var out struct {
		ID string `json:"id"`
	}
	decodeInto(h.t, resp, &out)
	if out.ID == "" {
		h.t.Fatalf("POST /api/chats/new returned an empty id")
	}
	return out.ID
}

// NewAgentChat creates a chat scoped to projectPath via POST /api/agent/chat
// — the real way an agent/project chat gets a working directory (as opposed
// to the plain /api/chats/new, which has none). Does not by itself enable
// tool-calling for the turn; combine with SetAgentEnabled(true).
func (h *Harness) NewAgentChat(projectPath string) string {
	h.t.Helper()
	resp := h.postJSON("/api/agent/chat", map[string]string{"project_path": projectPath})
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		h.t.Fatalf("POST /api/agent/chat: status %d", resp.StatusCode)
	}
	var out struct {
		ID string `json:"id"`
	}
	decodeInto(h.t, resp, &out)
	if out.ID == "" {
		h.t.Fatalf("POST /api/agent/chat returned an empty id")
	}
	return out.ID
}

// SetAgentEnabled toggles the global agent-mode flag via PUT
// /api/agent/enabled — Memo has one global agent-mode flag (see AGENTS.md's
// concurrency gotcha), there is no per-request override on /api/send/stream.
func (h *Harness) SetAgentEnabled(enabled bool) {
	h.t.Helper()
	resp := h.postAgentEnabled(enabled)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		h.t.Fatalf("PUT /api/agent/enabled(%v): status %d", enabled, resp.StatusCode)
	}
}

func (h *Harness) postAgentEnabled(enabled bool) *http.Response {
	h.t.Helper()
	req, err := http.NewRequest(http.MethodPut, h.BaseURL+"/api/agent/enabled",
		bytes.NewReader(mustJSON(h.t, map[string]bool{"enabled": enabled})))
	if err != nil {
		h.t.Fatalf("build PUT /api/agent/enabled: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("PUT /api/agent/enabled: %v", err)
	}
	return resp
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// SSEEvent is one parsed `data: {...}` line from a real /api/send/stream
// response — the same JSON shape as api.StreamChunk
// (internal/api/types.go), decoded loosely here so a test can assert on
// whichever fields it cares about without importing the internal/api
// package's exact type (this package is an external, black-box test
// layer).
type SSEEvent struct {
	Content      string `json:"content"`
	Thinking     string `json:"thinking"`
	Done         bool   `json:"done"`
	Error        string `json:"error"`
	FinishReason string `json:"finish_reason"`
}

// SendMessageStream POSTs to /api/send/stream with the given chatID
// (always explicit — see PLAN_chatid_refactor.md's reasoning, quoted in
// handlers_flutter.go, on why a caller that knows its chat should never
// rely on whichever chat happens to be globally "active") and returns every
// parsed SSE event in order, blocking until the stream closes.
//
// Only useful when nothing mid-stream needs a reply from the test (e.g. a
// permission_request) — the agent pipeline blocks server-side waiting for
// /api/agent/permission, so a turn that raises one never reaches Done until
// something calls that endpoint while this stream is still open. For that
// case, use SendMessageStreamAsync instead and resolve the permission
// concurrently.
func (h *Harness) SendMessageStream(chatID, message string) []SSEEvent {
	h.t.Helper()
	var events []SSEEvent
	for ev := range h.SendMessageStreamAsync(chatID, message) {
		events = append(events, ev)
	}
	return events
}

// SendMessageStreamAsync POSTs to /api/send/stream and returns a channel
// that receives each parsed SSE event as it arrives off the real HTTP
// response body, closed once the stream ends (a Done event, the body
// closing, or a decode error — the latter two also fail the test via
// t.Error, since the caller has no other way to see them once the channel
// closes). Use this whenever the test needs to react to a mid-stream event
// (most importantly permission_request) before the turn can finish.
func (h *Harness) SendMessageStreamAsync(chatID, message string) <-chan SSEEvent {
	h.t.Helper()
	resp := h.postJSON("/api/send/stream", map[string]string{
		"chat_id": chatID,
		"message": message,
	})
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		h.t.Fatalf("POST /api/send/stream: status %d", resp.StatusCode)
	}

	out := make(chan SSEEvent)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			var ev SSEEvent
			if err := json.Unmarshal([]byte(payload), &ev); err != nil {
				h.t.Errorf("decode SSE chunk %q: %v", payload, err)
				return
			}
			out <- ev
			if ev.Done {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			h.t.Errorf("reading SSE stream: %v", err)
		}
	}()
	return out
}

// FinalContent concatenates every non-agent-event Content field across
// events, in order — the assembled plain-text reply, the same thing a chat
// bubble ends up showing.
func FinalContent(events []SSEEvent) string {
	var sb strings.Builder
	for _, ev := range events {
		if ev.FinishReason == "agent_event" {
			continue
		}
		sb.WriteString(ev.Content)
	}
	return sb.String()
}

// AgentEvent is the decoded payload of an SSE chunk whose FinishReason is
// "agent_event" (internal/agent/pipeline.go's AgentEvent, redeclared here
// for the same black-box-package reason as SSEEvent above).
type AgentEvent struct {
	Type        string          `json:"type"`
	RequestID   string          `json:"request_id"`
	ToolName    string          `json:"tool"`
	Args        json.RawMessage `json:"args"`
	Result      string          `json:"result"`
	Error       string          `json:"error"`
	DangerLevel string          `json:"danger_level"`
	Content     string          `json:"content"`
	Preview     string          `json:"preview"`
}

// AgentEvents extracts and decodes every agent_event chunk from a stream, in
// order.
func AgentEvents(t *testing.T, events []SSEEvent) []AgentEvent {
	t.Helper()
	var out []AgentEvent
	for _, ev := range events {
		if ev.FinishReason != "agent_event" {
			continue
		}
		var ae AgentEvent
		if err := json.Unmarshal([]byte(ev.Content), &ae); err != nil {
			t.Fatalf("decode agent_event %q: %v", ev.Content, err)
		}
		out = append(out, ae)
	}
	return out
}

// ResolveAgentPermission POSTs to /api/agent/permission — policy is one of
// "prompt"/"allow_once"/"allow_session"/"allow_forever"/"deny_once"/
// "deny_forever" (internal/agent/permissions.go's PermissionPolicy values).
func (h *Harness) ResolveAgentPermission(requestID, policy string) {
	h.t.Helper()
	resp := h.postJSON("/api/agent/permission", map[string]string{
		"request_id": requestID,
		"policy":     policy,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		h.t.Fatalf("POST /api/agent/permission: status %d", resp.StatusCode)
	}
}

// --- task list helpers ---

// TaskList mirrors the fields of taskloop.TaskList this package's tests
// need — decoded loosely for the same black-box-package reason as
// SSEEvent/AgentEvent above.
type TaskList struct {
	ID     string     `json:"id"`
	ChatID string     `json:"chat_id"`
	Title  string     `json:"title"`
	Status string     `json:"status"`
	Mode   string     `json:"mode"`
	Items  []TaskItem `json:"items"`
}

type TaskItem struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
	Note   string `json:"note"`
}

// CreateTaskList POSTs to /api/tasklists with an explicit item list (as
// opposed to a task_md_path) and returns the created list.
func (h *Harness) CreateTaskList(chatID, title string, items []string) TaskList {
	h.t.Helper()
	resp := h.postJSON("/api/tasklists", map[string]any{
		"chat_id": chatID,
		"title":   title,
		"items":   items,
	})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		h.t.Fatalf("POST /api/tasklists: status %d, body: %s", resp.StatusCode, body)
	}
	var tl TaskList
	decodeInto(h.t, resp, &tl)
	return tl
}

// StartTaskList POSTs to /api/tasklists/{id}/start — creating a list does
// not implicitly start it running.
func (h *Harness) StartTaskList(id string) {
	h.t.Helper()
	resp := h.postJSON("/api/tasklists/"+id+"/start", map[string]any{})
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		h.t.Fatalf("POST /api/tasklists/%s/start: status %d, body: %s", id, resp.StatusCode, body)
	}
}

// GetTaskList GETs the current state of one task list.
func (h *Harness) GetTaskList(id string) TaskList {
	h.t.Helper()
	resp := h.getJSON("/api/tasklists/" + id)
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		h.t.Fatalf("GET /api/tasklists/%s: status %d", id, resp.StatusCode)
	}
	var tl TaskList
	decodeInto(h.t, resp, &tl)
	return tl
}

// ApprovePlan POSTs to /api/tasklists/{id}/approve-plan.
func (h *Harness) ApprovePlan(id string) {
	h.t.Helper()
	resp := h.postJSON("/api/tasklists/"+id+"/approve-plan", map[string]any{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		h.t.Fatalf("POST /api/tasklists/%s/approve-plan: status %d", id, resp.StatusCode)
	}
}

// WaitForTaskStatus polls GetTaskList until Status is one of want, or
// timeout elapses (fatal). Real task-loop work happens on background
// goroutines (engine.go's run()), so a caller can never assume a status
// transition already happened just because an HTTP call returned.
func (h *Harness) WaitForTaskStatus(id string, timeout time.Duration, want ...string) TaskList {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	var last TaskList
	for time.Now().Before(deadline) {
		last = h.GetTaskList(id)
		for _, w := range want {
			if last.Status == w {
				return last
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	h.t.Fatalf("task list %s never reached status %v, stuck at %q (items=%+v)", id, want, last.Status, last.Items)
	return last
}

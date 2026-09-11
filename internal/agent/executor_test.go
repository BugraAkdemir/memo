package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"memo/internal/config"
	"memo/internal/provider"
)

// TestExecutor_PermissionFlagsConcurrentAccess guards against the bug where
// RunStream read e.bypassPermissions/e.autoPermission directly instead of
// through GetBypassPermissions/GetAutoPermission — SetBypassPermissions/
// SetAutoPermission write under e.mu (an HTTP handler goroutine), so an
// unsynchronized read elsewhere has no happens-before guarantee and can
// observe a stale value. Run with -race to catch a regression.
func TestExecutor_PermissionFlagsConcurrentAccess(t *testing.T) {
	e := NewExecutor(t.TempDir(), nil, nil, nil)

	var wg sync.WaitGroup
	for i := range 100 {
		v := i%2 == 0
		wg.Add(4)
		go func() {
			defer wg.Done()
			e.SetAutoPermission(v)
		}()
		go func() {
			defer wg.Done()
			e.SetBypassPermissions(v)
		}()
		go func() {
			defer wg.Done()
			_ = e.GetAutoPermission()
		}()
		go func() {
			defer wg.Done()
			_ = e.GetBypassPermissions()
		}()
	}
	wg.Wait()
}

// TestExecutor_LogEventPersistsToAuditFile is the H10 regression: before
// this fix, e.logs was the only copy of a tool call's audit trail — capped
// at 1000 in-memory entries, entirely gone on restart. logEvent must now
// also durably append each entry to disk.
func TestExecutor_LogEventPersistsToAuditFile(t *testing.T) {
	path := config.DataPath("agent-audit.jsonl")
	// Other tests in this package share the same process-memoized
	// config.DataDir(), so this file is a single shared log across the
	// whole test binary run — record the size before, and only inspect
	// the bytes appended after, instead of assuming the file starts empty.
	var startSize int64
	if fi, err := os.Stat(path); err == nil {
		startSize = fi.Size()
	}

	e := NewExecutor(t.TempDir(), nil, nil, nil)
	if e.auditLogFile == nil {
		t.Fatal("auditLogFile is nil — could not open the audit log")
	}
	t.Cleanup(func() { e.auditLogFile.Close() })

	e.logEvent("session-h10", AgentEvent{
		Type:       EventToolResult,
		ToolName:   "write_file",
		DurationMs: 42,
	})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	appended := string(data[startSize:])
	lines := strings.Split(strings.TrimSpace(appended), "\n")
	if len(lines) == 0 || lines[len(lines)-1] == "" {
		t.Fatalf("expected at least one new line appended to %s, got %q", path, appended)
	}

	var entry AgentLogEntry
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("last appended line is not valid JSON: %v (line: %q)", err, lines[len(lines)-1])
	}
	if entry.SessionID != "session-h10" || entry.ToolName != "write_file" || entry.DurationMs != 42 {
		t.Errorf("got entry %+v, want SessionID=session-h10 ToolName=write_file DurationMs=42", entry)
	}
}

func TestModelContextWindow(t *testing.T) {
	if got := modelContextWindow(nil, "anything"); got != 128*1024 {
		t.Errorf("nil router: got %d, want 128K", got)
	}

	explicit := provider.NewRouter([]provider.ProviderConfig{
		{Type: provider.ProviderClaude, Name: "c", Model: "claude-x", Enabled: true, APIKey: "k", ContextTokens: 200000},
	})
	if got := modelContextWindow(explicit, "claude-x"); got != 200000 {
		t.Errorf("explicit ContextTokens: got %d, want 200000", got)
	}
	// modelName mismatch still falls back to "any provider with a window".
	if got := modelContextWindow(explicit, "some-other-model"); got != 200000 {
		t.Errorf("mismatched model: got %d, want 200000 (any-window fallback)", got)
	}

	typeFallback := provider.NewRouter([]provider.ProviderConfig{
		{Type: provider.ProviderClaude, Name: "c", Model: "claude-y", Enabled: true, APIKey: "k"},
	})
	if got := modelContextWindow(typeFallback, "claude-y"); got != 200*1024 {
		t.Errorf("claude type fallback: got %d, want 200K", got)
	}

	geminiFallback := provider.NewRouter([]provider.ProviderConfig{
		{Type: provider.ProviderGemini, Name: "g", Model: "gem", Enabled: true, APIKey: "k"},
	})
	if got := modelContextWindow(geminiFallback, "gem"); got != 1024*1024 {
		t.Errorf("gemini type fallback: got %d, want 1M", got)
	}
}

// fakeChatCompletionServer returns an httptest server that answers every
// POST with a fixed assistant reply, for building a distinguishable
// *provider.Router in tests without a real LLM backend.
func fakeChatCompletionServer(t *testing.T, reply string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"` + reply + `"}}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestExecutor_RunStreamWithRouter_UsesGivenRouterNotSyncedOne guards O5:
// RunStream used to read whatever router SyncRouter last stored on the
// Executor. For a *shared* Executor instance (a.agentExecutor in
// internal/app, used directly by ordinary interactive agent chats — unlike
// every other caller, which constructs its own private Executor), two
// concurrent interactive turns are only serialized per chat ID, not
// globally, so a second call's SyncRouter could land between this call's
// own SyncRouter and RunStream — silently running this call's tools
// against the OTHER chat's router. RunStreamWithRouter takes the router as
// an explicit per-call argument instead, closing that window regardless of
// what SyncRouter last set. Simulates the race scenario directly: the
// Executor is constructed with (or synced to) router A, matching "another
// chat already synced its router onto this shared executor", then
// RunStreamWithRouter is called with a *different* router B — the reply
// must come from B, never from the synced A.
func TestExecutor_RunStreamWithRouter_UsesGivenRouterNotSyncedOne(t *testing.T) {
	srvA := fakeChatCompletionServer(t, "reply-from-A")
	srvB := fakeChatCompletionServer(t, "reply-from-B")

	routerA := provider.NewRouter([]provider.ProviderConfig{
		{Type: provider.ProviderCustom, Name: "a", BaseURL: srvA.URL, Model: "m", Enabled: true},
	})
	routerA.SetActiveProvider("a")
	routerB := provider.NewRouter([]provider.ProviderConfig{
		{Type: provider.ProviderCustom, Name: "b", BaseURL: srvB.URL, Model: "m", Enabled: true},
	})
	routerB.SetActiveProvider("b")

	// Constructing with routerA (and SyncRouter would do the same) stands
	// in for another concurrent interactive chat having last synced its own
	// router onto this shared executor.
	e := NewExecutor(t.TempDir(), routerA, nil, nil)

	ch, err := e.RunStreamWithRouter(context.Background(), routerB, "sess", "m", "", []provider.Message{{Role: "user", Content: "hi"}}, func(AgentEvent) {})
	if err != nil {
		t.Fatalf("RunStreamWithRouter() error = %v", err)
	}
	var content string
	for chunk := range ch {
		content += chunk.Content
	}
	if strings.Contains(content, "reply-from-A") {
		t.Errorf("content = %q — used the synced router A instead of the explicitly passed router B (O5)", content)
	}
	if !strings.Contains(content, "reply-from-B") {
		t.Errorf("content = %q, want the reply to come from the explicitly passed router B", content)
	}
}

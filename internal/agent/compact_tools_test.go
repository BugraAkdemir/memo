package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/provider"
)

// toolRecordingProvider plays scripted responses and records the tool names
// offered on every request.
type toolRecordingProvider struct {
	responses []provider.ChatResponse
	offered   [][]string
}

func (p *toolRecordingProvider) ChatCompletion(_ context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	var names []string
	for _, d := range req.Tools {
		names = append(names, d.Function.Name)
	}
	p.offered = append(p.offered, names)
	resp := p.responses[len(p.offered)-1]
	return &resp, nil
}

func hasName(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func runCompactTestPipeline(t *testing.T, dir string, compact *CompactTools, responses []provider.ChatResponse) (*toolRecordingProvider, []AgentEvent) {
	t.Helper()
	prov := &toolRecordingProvider{responses: responses}
	p := NewPipeline(NewRegistry(), NewPermissionManager(t.TempDir()), NewSandbox(DefaultSandboxConfig(dir)), prov, NewBackupManager(t.TempDir()))
	p.autoPermission = true
	p.compactTools = compact
	var events []AgentEvent
	ch, err := p.RunStream(context.Background(), nil, "test-model", func(ev AgentEvent) { events = append(events, ev) }, nil)
	if err != nil {
		t.Fatalf("RunStream() error = %v", err)
	}
	for chunk := range ch {
		if chunk.Error != "" {
			t.Fatalf("unexpected error chunk: %s", chunk.Error)
		}
	}
	return prov, events
}

func TestCompactRun_StartsWithLoadToolsAndLoadsOnDemand(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("compact-ok"), 0644); err != nil {
		t.Fatal(err)
	}
	var stored []string
	compact := NewCompactTools(2000, nil, func(g string) { stored = append(stored, g) })

	prov, events := runCompactTestPipeline(t, dir, compact, []provider.ChatResponse{
		{ToolCalls: []provider.ToolCall{mustToolCall(t, "c1", "read_file", map[string]string{"path": "note.txt"})}},
		{ToolCalls: []provider.ToolCall{mustToolCall(t, "c2", LoadToolsName, map[string][]string{"groups": {"files_read"}})}},
		{ToolCalls: []provider.ToolCall{mustToolCall(t, "c3", "read_file", map[string]string{"path": "note.txt"})}},
		{Content: "done"},
	})

	if got := prov.offered[0]; len(got) != 1 || got[0] != LoadToolsName {
		t.Fatalf("first request offered %v, want only %s", got, LoadToolsName)
	}
	if !hasName(prov.offered[2], "read_file") || hasName(prov.offered[2], "write_file") {
		t.Fatalf("after loading files_read the request offered %v; want read_file and no other group", prov.offered[2])
	}
	if len(stored) != 1 || stored[0] != "files_read" {
		t.Errorf("OnLoad saw %v, want [files_read] so the chat keeps it", stored)
	}

	var notLoaded, readResult bool
	for _, ev := range events {
		if ev.Type == EventToolError && ev.ToolName == "read_file" && strings.Contains(ev.Error, `"files_read"`) {
			notLoaded = true
		}
		if ev.Type == EventToolResult && ev.ToolName == "read_file" && strings.Contains(ev.Result, "compact-ok") {
			readResult = true
		}
	}
	if !notLoaded {
		t.Error("calling read_file before loading it must be refused with the group to load")
	}
	if !readResult {
		t.Error("read_file did not run after its group was loaded")
	}
}

func TestCompactTools_LoadRefusesWhatDoesNotFitTheBudget(t *testing.T) {
	all := NewRegistry().ToOpenAITools(nil)
	budget := DefinitionTokens([]provider.ToolDefinition{LoadToolsDefinition()}) + 50
	c := NewCompactTools(budget, nil, nil)

	got := c.Load([]byte(`{"groups":["files_read"]}`), all, nil)

	if !strings.Contains(got, "Not loaded: files_read") {
		t.Fatalf("Load over budget = %q, want a plain refusal", got)
	}
	if len(c.Loaded()) != 0 {
		t.Errorf("a refused group stayed loaded: %v", c.Loaded())
	}
}

func TestCompactTools_UnknownGroupListsTheRealOnes(t *testing.T) {
	c := NewCompactTools(0, nil, nil)
	got := c.Load([]byte(`{"groups":["dosyalar"]}`), NewRegistry().ToOpenAITools(nil), nil)
	if !strings.Contains(got, "Unknown group: dosyalar") || !strings.Contains(got, "files_read") {
		t.Fatalf("Load(unknown) = %q, want the valid group names", got)
	}
}

func TestCompactTools_LoadToolsDefinitionIsSmall(t *testing.T) {
	// ~264 by the len/3 estimate on 2026-11-10 (most of it the JSON schema
	// wrapper), against ~10k for the full list; a 4k window's tool share is 819.
	if n := DefinitionTokens([]provider.ToolDefinition{LoadToolsDefinition()}); n > 300 {
		t.Errorf("load_tools costs ~%d tokens, want <= 300 (it is all a 4k model starts with)", n)
	}
}

func TestCompactTools_BuiltInGroupToolsExist(t *testing.T) {
	reg := NewRegistry()
	for _, g := range ToolGroups {
		switch g.Name {
		case "files_read", "files_write", "web", "plan":
		default:
			continue // registered by the app only when that feature is wired
		}
		for _, name := range g.Tools {
			if _, ok := reg.Get(name); !ok {
				t.Errorf("group %s lists %q, which the registry does not have", g.Name, name)
			}
		}
	}
}

func TestRegularRun_HasNoLoadTools(t *testing.T) {
	prov, _ := runCompactTestPipeline(t, t.TempDir(), nil, []provider.ChatResponse{{Content: "hi"}})
	if hasName(prov.offered[0], LoadToolsName) || !hasName(prov.offered[0], "read_file") {
		t.Fatalf("a regular run offered %v; want the full list and no %s", prov.offered[0], LoadToolsName)
	}
}

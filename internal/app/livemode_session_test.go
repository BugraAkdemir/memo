package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/agent"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/livemode"
	"memo/internal/livemode/google"
	"memo/internal/sessions"
)

func newTestAppForLiveModeSession(t *testing.T) *App {
	t.Helper()
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager: %v", err)
	}
	cfgMgr := livemode.NewConfigManager(filepath.Join(t.TempDir(), "livemode_engines.json"), nil)
	return &App{
		cfg: &config.AppConfig{
			Memory:   config.MemoryConfig{MemoryEnabled: false},
			Llama:    config.LlamaConfig{CtxSize: 4096},
			LiveMode: config.LiveModeConfig{ActiveEngine: "local", WorkMode: "delegate", AgentPermissionPolicy: "voice_prompt"},
		},
		identity:             identity.New("Test", "Memo", "casual", "", false),
		sessions:             sm,
		liveModeEngineCfgMgr: cfgMgr,
	}
}

func TestNewLiveModeSession_FallsBackToEchoForLocalEngine(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	s := a.NewLiveModeSession(context.Background())
	if _, ok := s.(*livemode.EchoSession); !ok {
		t.Errorf("expected EchoSession for the 'local' engine, got %T", s)
	}
}

func TestNewLiveModeSession_FallsBackToEchoWhenGoogleLiveUnconfigured(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	a.cfg.LiveMode.ActiveEngine = "google_live"
	// No engine config saved at all.
	s := a.NewLiveModeSession(context.Background())
	if _, ok := s.(*livemode.EchoSession); !ok {
		t.Errorf("expected EchoSession fallback when no engine config is saved, got %T", s)
	}
}

func TestNewLiveModeSession_FallsBackToEchoWhenModelMissing(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	a.cfg.LiveMode.ActiveEngine = "google_live"
	a.liveModeEngineCfgMgr.Set(livemode.EngineConfig{Type: livemode.EngineGoogleLive, APIKey: "g-key", Enabled: true}) // no Model
	s := a.NewLiveModeSession(context.Background())
	if _, ok := s.(*livemode.EchoSession); !ok {
		t.Errorf("expected EchoSession fallback when model is empty, got %T", s)
	}
}

func TestNewLiveModeSession_BuildsRealGoogleClientWhenConfigured(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	a.cfg.LiveMode.ActiveEngine = "google_live"
	a.liveModeEngineCfgMgr.Set(livemode.EngineConfig{
		Type: livemode.EngineGoogleLive, APIKey: "g-key", Model: "models/gemini-3.1-flash-live-preview", Enabled: true,
	})
	s := a.NewLiveModeSession(context.Background())
	if _, ok := s.(*google.Client); !ok {
		t.Errorf("expected a real *google.Client, got %T", s)
	}
}

func TestBuildLiveModeToolList_DelegateModeHasExactlyDelegateTool(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	tools := a.buildLiveModeToolList("delegate")
	if len(tools) != 1 || tools[0].Name != livemode.DelegateToolName {
		t.Errorf("expected exactly [%s], got %+v", livemode.DelegateToolName, tools)
	}
}

func TestBuildLiveModeToolList_StandaloneModeHasFullRegistry(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	a.agentExecutor = agent.NewExecutor(t.TempDir(), nil, nil, nil)

	tools := a.buildLiveModeToolList("standalone")
	if len(tools) == 0 {
		t.Fatal("expected a non-empty tool list for standalone mode")
	}
	found := false
	for _, tl := range tools {
		if tl.Name == "read_file" {
			found = true
		}
		if tl.Name == livemode.DelegateToolName {
			t.Error("standalone mode should not include delegate_to_main_model — it has direct tool access instead")
		}
	}
	if !found {
		t.Error("expected the standalone tool list to include the registry's real tools (e.g. read_file)")
	}
}

func TestBuildLiveModeToolList_StandaloneModeNoExecutorReturnsNil(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	if tools := a.buildLiveModeToolList("standalone"); tools != nil {
		t.Errorf("expected nil when agentExecutor is not initialized, got %+v", tools)
	}
}

func TestBuildLiveModeToolCallHandler_DelegateMode_UnknownToolErrors(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	sessionID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat: %v", err)
	}
	handler := a.buildLiveModeToolCallHandler("delegate", sessionID)

	_, err = handler(context.Background(), "some_other_tool", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected an error for a tool name other than delegate_to_main_model")
	}
}

func TestBuildLiveModeToolCallHandler_DelegateMode_MissingInstructionErrors(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	sessionID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat: %v", err)
	}
	handler := a.buildLiveModeToolCallHandler("delegate", sessionID)

	_, err = handler(context.Background(), livemode.DelegateToolName, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected an error when instruction is missing from args")
	}
}

func TestBuildLiveModeToolCallHandler_DelegateMode_RoutesThroughDelegation(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	sessionID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat: %v", err)
	}
	handler := a.buildLiveModeToolCallHandler("delegate", sessionID)

	// No local/external model is configured in this test App, so the
	// delegated call resolves to some error-shaped reply rather than a real
	// answer — what's under test is that the handler actually calls
	// SendLiveDelegatedMessageStream/drainLiveDelegatedReply (proven by
	// getting a non-empty string back without hanging or panicking) rather
	// than short-circuiting on the tool name / args parsing this time.
	result, err := handler(context.Background(), livemode.DelegateToolName, json.RawMessage(`{"instruction":"fix the bug"}`))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if result == "" {
		t.Error("expected a non-empty reply string (even if it's an error-shaped one from the unconfigured model)")
	}
}

func TestBuildLiveModeToolCallHandler_StandaloneMode_ExecutesRealTool(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi there"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	a.agentExecutor = agent.NewExecutor(dir, nil, nil, nil)

	sessionID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat: %v", err)
	}
	handler := a.buildLiveModeToolCallHandler("standalone", sessionID)

	result, err := handler(context.Background(), "read_file", json.RawMessage(`{"path":"hello.txt"}`))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if result != "hi there" {
		t.Errorf("expected file contents, got %q", result)
	}
}

func TestBuildLiveModeToolCallHandler_StandaloneMode_NoExecutorErrors(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	handler := a.buildLiveModeToolCallHandler("standalone", "sess-1")
	if _, err := handler(context.Background(), "read_file", json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected an error when agentExecutor is not initialized")
	}
}

func TestBuildLiveModeSystemPrompt_DiffersByWorkMode(t *testing.T) {
	a := newTestAppForLiveModeSession(t)

	delegatePrompt := a.buildLiveModeSystemPrompt(context.Background(), "delegate")
	standalonePrompt := a.buildLiveModeSystemPrompt(context.Background(), "standalone")

	if delegatePrompt == standalonePrompt {
		t.Error("expected the capability text to differ between delegate and standalone WorkModes")
	}
	if !strings.Contains(delegatePrompt, livemode.DelegateToolName) {
		t.Errorf("expected the delegate-mode prompt to mention %s, got: %s", livemode.DelegateToolName, delegatePrompt)
	}
}

func TestBuildLiveModeSystemPrompt_NoIdentityReturnsEmpty(t *testing.T) {
	a := &App{}
	if got := a.buildLiveModeSystemPrompt(context.Background(), "delegate"); got != "" {
		t.Errorf("expected empty string when identity is nil, got %q", got)
	}
}

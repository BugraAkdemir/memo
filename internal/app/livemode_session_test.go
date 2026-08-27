package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	wrapped, ok := s.(*livePermissionRoutingSession)
	if !ok {
		t.Fatalf("expected a *livePermissionRoutingSession wrapping the real client, got %T", s)
	}
	if _, ok := wrapped.Session.(*google.Client); !ok {
		t.Errorf("expected the wrapped session to be a real *google.Client, got %T", wrapped.Session)
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
	var foundReadFile, foundDelegate bool
	for _, tl := range tools {
		if tl.Name == "read_file" {
			foundReadFile = true
		}
		if tl.Name == livemode.DelegateToolName {
			foundDelegate = true
		}
	}
	if !foundReadFile {
		t.Error("expected the standalone tool list to include the registry's real tools (e.g. read_file)")
	}
	// Standalone also carries delegate_to_main_model: its own tools never
	// touch long-term memory and the session's memory context is a one-time
	// snapshot, so delegation is the only route to a real per-turn recall.
	if !foundDelegate {
		t.Error("expected standalone mode to also include delegate_to_main_model for memory recall")
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
	handler := a.buildLiveModeToolCallHandler("delegate", sessionID, nil)

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
	handler := a.buildLiveModeToolCallHandler("delegate", sessionID, nil)

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
	handler := a.buildLiveModeToolCallHandler("delegate", sessionID, nil)

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

// TestBuildLiveModeToolCallHandler_DelegateMode_InjectsWaitQuietlyHint is a
// regression test for a real bug found via real-world testing: while a
// delegated task was still running, the live model kept trying to generate
// more speech (it has no natural sense of "I'm waiting on something"),
// producing choppy, repeatedly-interrupted-sounding audio. Confirms the
// handler now injects a "say one thing, then wait quietly" hint through
// injectContext right when delegation starts, before the reply comes back.
func TestBuildLiveModeToolCallHandler_DelegateMode_InjectsWaitQuietlyHint(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	sessionID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat: %v", err)
	}

	var injected []string
	injectContext := func(text string) error {
		injected = append(injected, text)
		return nil
	}
	handler := a.buildLiveModeToolCallHandler("delegate", sessionID, injectContext)

	if _, err := handler(context.Background(), livemode.DelegateToolName, json.RawMessage(`{"instruction":"fix the bug"}`)); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(injected) != 1 {
		t.Fatalf("expected exactly one injectContext call before the delegated reply, got %d: %v", len(injected), injected)
	}
	if !strings.Contains(injected[0], "delegate_to_main_model") {
		t.Errorf("expected the injected hint to reference delegate_to_main_model, got %q", injected[0])
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
	handler := a.buildLiveModeToolCallHandler("standalone", sessionID, nil)

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
	handler := a.buildLiveModeToolCallHandler("standalone", "sess-1", nil)
	if _, err := handler(context.Background(), "read_file", json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected an error when agentExecutor is not initialized")
	}
}

// TestBuildLiveModeToolCallHandler_StandaloneMode_DelegateToolRoutesToDelegation
// confirms standalone mode also honors delegate_to_main_model (added so it
// has a route to real per-turn memory recall — its own tools never touch
// the vector store), routing it through the same delegation path delegate
// WorkMode uses rather than trying to run it as a registry tool.
func TestBuildLiveModeToolCallHandler_StandaloneMode_DelegateToolRoutesToDelegation(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	a.agentExecutor = agent.NewExecutor(t.TempDir(), nil, nil, nil)
	sessionID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat: %v", err)
	}

	var injected []string
	handler := a.buildLiveModeToolCallHandler("standalone", sessionID, func(text string) error {
		injected = append(injected, text)
		return nil
	})

	result, err := handler(context.Background(), livemode.DelegateToolName, json.RawMessage(`{"instruction":"adimi hatirliyor musun"}`))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if result == "" {
		t.Error("expected a non-empty reply from the delegation path (even an error-shaped one from the unconfigured model)")
	}
	// The "say one thing, then wait quietly" hint is the delegation path's
	// tell — an ExecuteToolCall path would never inject it.
	if len(injected) != 1 || !strings.Contains(injected[0], "delegate_to_main_model") {
		t.Errorf("expected the delegation wait-hint to be injected once, got %v", injected)
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

// ─── Phase 12: voice-based permission prompting ─────────────────────────

func TestLiveModeVoicePermissionCallbacks_SendQuestionUsesInjectContext(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	var got string
	injectContext := func(text string) error { got = text; return nil }
	_, sendQuestion, _ := a.liveModeVoicePermissionCallbacks(injectContext)

	if err := sendQuestion("İzin gerekiyor mu?"); err != nil {
		t.Fatalf("sendQuestion: %v", err)
	}
	if got != "İzin gerekiyor mu?" {
		t.Errorf("expected injectContext to receive the question text, got %q", got)
	}
}

func TestLiveModeVoicePermissionCallbacks_SendQuestionFailsWhenInjectContextNil(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	_, sendQuestion, _ := a.liveModeVoicePermissionCallbacks(nil)
	if err := sendQuestion("anything"); err == nil {
		t.Error("expected an error when injectContext is nil (session not ready / never started)")
	}
}

func TestAwaitLivePermissionAnswer_RoutedByTranscript(t *testing.T) {
	a := &App{}

	type answer struct {
		text string
		ok   bool
	}
	resultCh := make(chan answer, 1)
	go func() {
		text, ok := a.awaitLivePermissionAnswer(context.Background())
		resultCh <- answer{text, ok}
	}()

	waitForPendingLivePermAnswer(t, a)
	if !a.routeLiveTranscriptToPermissionAnswer("evet") {
		t.Error("expected routeLiveTranscriptToPermissionAnswer to report a pending question")
	}

	select {
	case res := <-resultCh:
		if !res.ok || res.text != "evet" {
			t.Errorf("expected (\"evet\", true), got (%q, %v)", res.text, res.ok)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("awaitLivePermissionAnswer never returned")
	}
}

func TestAwaitLivePermissionAnswer_TimesOutOnCtxDone(t *testing.T) {
	a := &App{}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, ok := a.awaitLivePermissionAnswer(ctx); ok {
		t.Error("expected ok=false when the context times out with no answer routed")
	}
}

func TestRouteLiveTranscriptToPermissionAnswer_NoPendingReturnsFalse(t *testing.T) {
	a := &App{}
	if a.routeLiveTranscriptToPermissionAnswer("evet") {
		t.Error("expected false when no permission question is pending")
	}
}

// TestBuildLiveModeToolCallHandler_StandaloneMode_AutoApprovePermission is a
// regression test for a real bug found while wiring Phase 12: ExecuteToolCall
// calls onEvent synchronously with the permission_request event *before* it
// registers the request in e.pendingPerms (its very next statement) — an
// onEvent that resolved the request inline, on that same goroutine, would
// deterministically lose every answer. Reusing "write_file" (Medium,
// confirmed to trigger a real permission_request in
// execute_tool_call_test.go) against a fresh Executor with no
// bypass/auto-permission flags set exercises the real path end to end; this
// test would hang for 60s and fail before the fix (resolveLivePermission
// run in its own goroutine, with a short retry loop).
func TestBuildLiveModeToolCallHandler_StandaloneMode_AutoApprovePermission(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	a.cfg.LiveMode.AgentPermissionPolicy = "auto_allow_once"
	dir := t.TempDir()
	a.agentExecutor = agent.NewExecutor(dir, nil, nil, nil)

	sessionID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat: %v", err)
	}
	handler := a.buildLiveModeToolCallHandler("standalone", sessionID, nil)

	args := json.RawMessage(`{"path":"gated.txt","content":"approved"}`)
	if _, err := handler(context.Background(), "write_file", args); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "gated.txt")); err != nil || string(got) != "approved" {
		t.Errorf("expected the write to happen after auto-approval, got %q err=%v", got, err)
	}
}

func TestBuildLiveModeToolCallHandler_StandaloneMode_VoicePromptApproved(t *testing.T) {
	a := newTestAppForLiveModeSession(t) // AgentPermissionPolicy defaults to "voice_prompt"
	dir := t.TempDir()
	a.agentExecutor = agent.NewExecutor(dir, nil, nil, nil)

	sessionID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat: %v", err)
	}

	var injectedQuestion string
	injectContext := func(text string) error {
		injectedQuestion = text
		// Must be async: awaitLivePermissionAnswer only registers the
		// pending-answer channel after sendQuestion (this call) returns.
		go func() {
			if waitForPendingLivePermAnswerAsync(a) {
				a.routeLiveTranscriptToPermissionAnswer("evet")
			}
		}()
		return nil
	}
	handler := a.buildLiveModeToolCallHandler("standalone", sessionID, injectContext)

	args := json.RawMessage(`{"path":"gated.txt","content":"approved"}`)
	if _, err := handler(context.Background(), "write_file", args); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if injectedQuestion == "" || !strings.Contains(injectedQuestion, "write_file") {
		t.Errorf("expected the injected question to mention the tool name, got %q", injectedQuestion)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "gated.txt")); err != nil || string(got) != "approved" {
		t.Errorf("expected the write to happen after a spoken 'evet', got %q err=%v", got, err)
	}
}

func TestBuildLiveModeToolCallHandler_StandaloneMode_VoicePromptDenied(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	dir := t.TempDir()
	a.agentExecutor = agent.NewExecutor(dir, nil, nil, nil)

	sessionID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat: %v", err)
	}

	injectContext := func(string) error {
		go func() {
			if waitForPendingLivePermAnswerAsync(a) {
				a.routeLiveTranscriptToPermissionAnswer("hayır")
			}
		}()
		return nil
	}
	handler := a.buildLiveModeToolCallHandler("standalone", sessionID, injectContext)

	args := json.RawMessage(`{"path":"gated.txt","content":"should not appear"}`)
	if _, err := handler(context.Background(), "write_file", args); err == nil {
		t.Fatal("expected an error when the spoken answer denies the tool")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "gated.txt")); !os.IsNotExist(statErr) {
		t.Error("expected the write to be denied, but the file exists")
	}
}

func TestBuildLiveModeToolCallHandler_StandaloneMode_VoicePromptSendQuestionFailureDenies(t *testing.T) {
	a := newTestAppForLiveModeSession(t)
	dir := t.TempDir()
	a.agentExecutor = agent.NewExecutor(dir, nil, nil, nil)

	sessionID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat: %v", err)
	}
	// nil injectContext: liveModeVoicePermissionCallbacks' sendQuestion
	// always fails, mirroring "the session isn't actually ready yet".
	handler := a.buildLiveModeToolCallHandler("standalone", sessionID, nil)

	args := json.RawMessage(`{"path":"gated.txt","content":"should not appear"}`)
	if _, err := handler(context.Background(), "write_file", args); err == nil {
		t.Fatal("expected permission denied when the question can't be asked at all")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "gated.txt")); !os.IsNotExist(statErr) {
		t.Error("expected the write to be denied, but the file exists")
	}
}

// waitForPendingLivePermAnswerAsync polls for a.livePendingPermAnswerCh to
// be registered (awaitLivePermissionAnswer has been called and is waiting),
// up to 2s. Safe to call from a background goroutine (unlike t.Fatal,
// which Go's testing package requires be called only from the test's own
// goroutine) — used by tests that simulate an async spoken answer arriving
// shortly after sendQuestion is invoked.
func waitForPendingLivePermAnswerAsync(a *App) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		a.livePermMu.Lock()
		pending := a.livePendingPermAnswerCh != nil
		a.livePermMu.Unlock()
		if pending {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// waitForPendingLivePermAnswer is waitForPendingLivePermAnswerAsync for use
// directly from a test's own goroutine, failing the test on timeout.
func waitForPendingLivePermAnswer(t *testing.T, a *App) {
	t.Helper()
	if !waitForPendingLivePermAnswerAsync(a) {
		t.Fatal("timed out waiting for a pending live permission answer channel")
	}
}

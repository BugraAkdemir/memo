package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/agent/tools"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/provider"
	"memo/internal/sessions"
	"os"
	"sync"
	"time"
)

// PermissionRequest encapsulates a pending permission request.
type PermissionRequest struct {
	ID    string
	Event AgentEvent
	ResCh chan PermissionPolicy
}

// AgentLogEntry represents an execution step for auditing.
type AgentLogEntry struct {
	Timestamp  time.Time        `json:"timestamp"`
	SessionID  string           `json:"session_id"`
	ToolName   string           `json:"tool_name"`
	Args       json.RawMessage  `json:"args"`
	Result     string           `json:"result,omitempty"`
	Error      string           `json:"error,omitempty"`
	DurationMs int64            `json:"duration_ms"`
	Permission PermissionPolicy `json:"permission,omitempty"`
}

// Executor is the top-level orchestration engine for the agent.
type Executor struct {
	basePath       string
	providerRouter *provider.Router
	providerCfgMgr *provider.ConfigManager
	registry       *ToolRegistry
	permissions    *PermissionManager
	sandbox        *Sandbox
	backup         *BackupManager
	// sessionManager lets change_directory (internal/agent/tools/changedir.go)
	// persist a directory switch past the current turn — see
	// tools.WithProjectPathSetter in RunStream below. May be nil (background/
	// test executors that never construct one); persistence is then simply
	// skipped, not an error.
	sessionManager *sessions.Manager

	mu                sync.Mutex
	pendingPerms      map[string]*PermissionRequest
	// logs holds the most recent entries in memory (see logEvent's H10 doc
	// comment for why they're also written to auditLogFile) — a cap here
	// is fine precisely because the file is now the durable copy; nothing
	// currently reads this slice, but it's kept as a ready-made "recent
	// activity" source for a future in-app view without needing to parse
	// the file.
	logs              []AgentLogEntry
	auditLogFile      *os.File // nil if it couldn't be opened; logging then falls back to logx only
	bypassPermissions bool // sistem yönetimi açıkken true
	autoPermission    bool // kullanıcı Shift+Tab ile açtığında tüm izinleri otomatik onayla
}

// openAuditLogFile opens the agent's append-only audit trail
// (config.DataDir()/agent-audit.jsonl, one JSON object per line). Returns
// nil (not an error) on failure — the executor must keep working even if
// the audit log can't be opened (permission issue, read-only filesystem);
// callers just log a warning and lose durability, not availability.
func openAuditLogFile() *os.File {
	path := config.DataPath("agent-audit.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		logx.Printf("agent: could not open audit log %s: %v — tool-call auditing will not persist across restarts", path, err)
		return nil
	}
	return f
}

// NewExecutor creates a new agent executor. sessionManager may be nil (e.g.
// in tests) — change_directory then simply can't persist a switch past the
// current turn, but the turn-local switch (the more important half) still
// works via the sandbox alone.
func NewExecutor(basePath string, providerRouter *provider.Router, providerCfgMgr *provider.ConfigManager, sessionManager *sessions.Manager) *Executor {
	return &Executor{
		basePath:       basePath,
		providerRouter: providerRouter,
		providerCfgMgr: providerCfgMgr,
		registry:       NewRegistry(),
		permissions:    NewPermissionManager(config.DataDir()),
		sandbox:        NewSandbox(DefaultSandboxConfig(basePath)),
		backup:         NewBackupManager(config.DataDir()),
		sessionManager: sessionManager,
		pendingPerms:   make(map[string]*PermissionRequest),
		logs:           make([]AgentLogEntry, 0),
		auditLogFile:   openAuditLogFile(),
	}
}

// NewWhatsAppExecutor creates a lightweight executor with only WhatsApp tools.
// Reuses sandbox/permissions/the audit log file handle from an existing
// executor to avoid re-init — a WhatsApp-triggered tool call is still the
// same agent, so it belongs in the same single audit trail, not a second
// file.
func NewWhatsAppExecutor(existing *Executor) *Executor {
	return &Executor{
		basePath:       existing.basePath,
		providerRouter: existing.providerRouter,
		providerCfgMgr: existing.providerCfgMgr,
		registry:       NewWhatsAppRegistry(),
		permissions:    existing.permissions,
		sandbox:        existing.sandbox,
		backup:         existing.backup,
		sessionManager: existing.sessionManager,
		pendingPerms:   make(map[string]*PermissionRequest),
		logs:           make([]AgentLogEntry, 0),
		auditLogFile:   existing.auditLogFile,
	}
}

// NewWebSearchExecutor creates a lightweight executor scoped to web_search +
// fetch_page (see NewWebSearchRegistry). Used by App.routeStream's non-agent
// "web search mode": the model decides per message, via native
// function-calling, whether it needs to search at all — at zero extra
// network/LLM cost when it decides not to — instead of a separate "should I
// search" call or a blind search run on every message regardless of
// content. Once it does search, the normal tool-call loop (same
// maxIters-40 pipeline full agent mode uses) lets it read a result with
// fetch_page, judge relevance, and try another one if needed — same
// search-then-verify flow as full agent mode, just without file/command/
// WhatsApp access. Reuses sandbox/permissions/backup/audit-log from an
// existing executor exactly like NewWhatsAppExecutor. Safe to share: both
// tools' DangerLevel is Safe, which PermissionManager.Check always
// auto-allows with no prompt, so this executor never touches the
// permission-prompt flow at all.
func NewWebSearchExecutor(existing *Executor) *Executor {
	return &Executor{
		basePath:       existing.basePath,
		providerRouter: existing.providerRouter,
		providerCfgMgr: existing.providerCfgMgr,
		registry:       NewWebSearchRegistry(),
		permissions:    existing.permissions,
		sandbox:        existing.sandbox,
		backup:         existing.backup,
		sessionManager: existing.sessionManager,
		pendingPerms:   make(map[string]*PermissionRequest),
		logs:           make([]AgentLogEntry, 0),
		auditLogFile:   existing.auditLogFile,
	}
}

// NewTaskExecutor creates a per-task-list executor for the Self-Driving loop.
// It reuses the shared registry/permissions/backup/audit/session-manager (a
// task worker turn is still the same agent, one audit trail) but owns its own
// providerRouter and its own bypassPermissions flag, so a running task can
// switch provider (self-heal) or run with permissions bypassed without
// disturbing the executor interactive chat uses. Mirrors NewWhatsAppExecutor.
func NewTaskExecutor(existing *Executor, router *provider.Router) *Executor {
	return &Executor{
		basePath:       existing.basePath,
		providerRouter: router,
		providerCfgMgr: existing.providerCfgMgr,
		registry:       existing.registry,
		permissions:    existing.permissions,
		sandbox:        existing.sandbox,
		backup:         existing.backup,
		sessionManager: existing.sessionManager,
		pendingPerms:   make(map[string]*PermissionRequest),
		logs:           make([]AgentLogEntry, 0),
		auditLogFile:   existing.auditLogFile,
	}
}

// ActiveRouter returns the executor's current provider router (nil if none).
func (e *Executor) ActiveRouter() *provider.Router {
	return e.getRouter()
}

// NewSubAgentExecutor creates an ephemeral executor for one Self-Driving
// sub-agent run: caller-supplied registry (full for the "coder", read-only
// for everyone else), caller-supplied router, its own bypass flag,
// sessionManager nil (no cd persistence, no session), and a sandbox scoped to
// projectPath. It reuses the shared permissions/backup/audit trail.
func NewSubAgentExecutor(existing *Executor, registry *ToolRegistry, router *provider.Router, projectPath string) *Executor {
	base := existing.basePath
	if projectPath != "" {
		base = projectPath
	}
	return &Executor{
		basePath:       base,
		providerRouter: router,
		providerCfgMgr: existing.providerCfgMgr,
		registry:       registry,
		permissions:    existing.permissions,
		sandbox:        NewSandbox(DefaultSandboxConfig(base)),
		backup:         existing.backup,
		sessionManager: nil,
		pendingPerms:   make(map[string]*PermissionRequest),
		logs:           make([]AgentLogEntry, 0),
		auditLogFile:   existing.auditLogFile,
	}
}

// Registry exposes the executor's tool registry so callers outside this
// package (the skill system) can register/unregister additional tools
// into the exact registry the agent pipeline executes against. The field
// itself is never reassigned after construction, so this needs no locking.
func (e *Executor) Registry() *ToolRegistry {
	return e.registry
}

// IsAvailable checks if the agent can run (needs a provider router).
func (e *Executor) IsAvailable() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.providerRouter != nil
}

// SyncRouter updates the providerRouter reference to match the app's current router.
// This is needed because the router is replaced when providers change.
func (e *Executor) SyncRouter(r *provider.Router) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.providerRouter = r
}

// getRouter returns the current providerRouter under the mutex.
func (e *Executor) getRouter() *provider.Router {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.providerRouter
}

// modelContextWindow resolves modelName's context-window size (in tokens)
// from the router's active provider configs, falling back to a conservative
// per-type default and finally 128K. Mirrors app.contextBudgetFor but lives
// here so the agent pipeline's intra-turn budget can track the real model
// instead of a flat constant.
func modelContextWindow(router *provider.Router, modelName string) int {
	if router != nil {
		active := router.ActiveProviders()
		for _, p := range active {
			if p.ContextTokens > 0 && (modelName == "" || p.Model == modelName) {
				return p.ContextTokens
			}
		}
		for _, p := range active {
			if p.ContextTokens > 0 {
				return p.ContextTokens
			}
		}
		for _, p := range active {
			switch p.Type {
			case provider.ProviderGemini:
				return 1024 * 1024
			case provider.ProviderClaude, provider.ProviderCustomAnthropic:
				return 200 * 1024
			}
		}
	}
	return 128 * 1024
}

// RunStream starts the agent execution and streams back chunks and events.
// effortLevel is the active provider's resolved EffortLevel (empty = let
// the provider/model use its own default) — callers resolve it the same
// way they resolve modelName, since both come from the same active
// provider config; see provider.ChatRequest.EffortLevel's doc comment.
func (e *Executor) RunStream(ctx context.Context, sessionID string, modelName string, effortLevel string, messages []provider.Message, onEvent func(AgentEvent), projectPath ...string) (<-chan provider.StreamChunk, error) {
	router := e.getRouter()
	if router == nil {
		return nil, fmt.Errorf("agent mode requires an active provider (external API or local model)")
	}

	// Resolve effective base path for this invocation.
	effectiveBase := e.basePath
	if len(projectPath) > 0 && projectPath[0] != "" {
		effectiveBase = projectPath[0]
	}

	// Create a per-call sandbox so concurrent RunStream invocations cannot
	// overwrite each other's basePath on the shared sandbox.
	sessionSandbox := NewSandbox(DefaultSandboxConfig(effectiveBase))

	// Estimate token budget: sum of all initial messages' tokens as base,
	// plus headroom for tool-call iterations, but derived from the real model
	// context window instead of a flat 64K. The flat floor meant a 200K-window
	// model kept only 64K of accumulated tool results before dropping the
	// oldest — needlessly re-reading files it had already read this turn —
	// while a tiny local window got a budget it couldn't honour anyway.
	baseTokens := 0
	for _, m := range messages {
		var content string
		switch v := m.Content.(type) {
		case string:
			content = v
		default:
		}
		baseTokens += len(content) / 3
	}
	window := modelContextWindow(router, modelName)
	maxTokens := int(float64(window) * 0.75) // leave room for the model's own output + margin
	if maxTokens < 32*1024 {
		maxTokens = 32 * 1024
	}
	// A large seed still gets the historical 32K of working headroom on top.
	if seedPlus := baseTokens + 32*1024; seedPlus > maxTokens {
		maxTokens = seedPlus
	}

	pipeline := NewPipelineWithBudget(e.registry, e.permissions, sessionSandbox, router, e.backup, maxTokens)
	// Read through the locked getters, not the bare fields — SetBypassPermissions/
	// SetAutoPermission write under e.mu from a different goroutine (an HTTP
	// handler), and a plain field read here has no happens-before guarantee with
	// that write. Symptom when this raced: toggling Shift+Tab auto-permission
	// looked like it took effect (the PUT succeeded, the UI updated) but the very
	// next tool call still prompted, because RunStream's goroutine could still see
	// the pre-toggle value.
	pipeline.bypassPermissions = e.GetBypassPermissions()
	pipeline.autoPermission = e.GetAutoPermission()
	pipeline.autoPermissionFn = e.GetAutoPermission
	pipeline.effortLevel = effortLevel

	wrappedOnEvent := func(ev AgentEvent) {
		// Log the event
		e.logEvent(sessionID, ev)
		// Pass to frontend
		onEvent(ev)
	}

	waitFn := func(requestID string, ev AgentEvent) (PermissionPolicy, error) {
		resCh := make(chan PermissionPolicy, 1)

		e.mu.Lock()
		e.pendingPerms[requestID] = &PermissionRequest{
			ID:    requestID,
			Event: ev,
			ResCh: resCh,
		}
		e.mu.Unlock()

		defer func() {
			e.mu.Lock()
			delete(e.pendingPerms, requestID)
			e.mu.Unlock()
		}()

		// Auto-deny after 60 seconds if the user doesn't respond.
		permTimer := time.NewTimer(60 * time.Second)
		defer permTimer.Stop()

		select {
		case <-ctx.Done():
			return DenyOnce, ctx.Err()
		case <-permTimer.C:
			logx.Printf("AGENT: permission request %s timed out (60s), auto-denied", requestID)
			return DenyOnce, fmt.Errorf("permission timed out")
		case policy := <-resCh:
			return policy, nil
		}
	}

	// Let change_directory persist a directory switch past this turn (see
	// tools.WithSandboxSetter in Pipeline.RunStream for the turn-local half).
	// Every RunStream caller reaches this — Flutter chat, WhatsApp, Telegram
	// — so persistence isn't a per-channel concern.
	if e.sessionManager != nil {
		ctx = tools.WithProjectPathSetter(ctx, e.sessionManager, sessionID)
	}

	return pipeline.RunStream(ctx, messages, modelName, wrappedOnEvent, waitFn)
}

// HandlePermissionResponse routes the user's permission response to the waiting pipeline.
func (e *Executor) HandlePermissionResponse(requestID string, policy PermissionPolicy) error {
	e.mu.Lock()
	req, ok := e.pendingPerms[requestID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("permission request %s not found or already answered", requestID)
	}
	// Delete before sending: if this function is called a second time for the
	// same request (e.g. user double-taps), it will get "not found" instead of
	// blocking on a full channel while still holding the mutex (deadlock).
	delete(e.pendingPerms, requestID)
	e.mu.Unlock()

	// Channel has buffer 1 but the receiver may have already gone away
	// (context cancellation / timeout). Use a non-blocking send with a
	// short timeout so the HTTP handler never blocks forever.
	select {
	case req.ResCh <- policy:
	case <-time.After(time.Second):
		logx.Printf("AGENT: permission response for %s abandoned (no listener)", requestID)
	}
	return nil
}

// GetAgentPermissions returns permanent permissions.
func (e *Executor) GetAgentPermissions() []PermissionRecord {
	return e.permissions.ListPermanent()
}

// RevokeAgentPermission revokes a permanent permission.
func (e *Executor) RevokeAgentPermission(id string) error {
	return e.permissions.Revoke(id)
}

// SetBypassPermissions sistem yönetimi modu için izin bypass'ını ayarlar.
func (e *Executor) SetBypassPermissions(v bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bypassPermissions = v
}

// GetBypassPermissions sistem yönetimi izin bypass'ının durumunu döndürür.
func (e *Executor) GetBypassPermissions() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.bypassPermissions
}

// SetAutoPermission kullanıcının Shift+Tab ile açtığı otomatik izin modunu ayarlar.
//
// Turning it ON also resolves every permission request that is currently
// waiting for an answer: the user's whole intent when they flip auto-permission
// while a prompt is on screen is "stop asking me — including this one". Without
// this the on-screen dialog just sat there until its 60s auto-deny, because the
// pipeline goroutine had already committed to blocking on that request's
// channel before the toggle flipped (BUG-PERM auto-permission: "açıkken neden
// hâlâ soruyor").
func (e *Executor) SetAutoPermission(v bool) {
	e.mu.Lock()
	e.autoPermission = v
	var draining []*PermissionRequest
	if v {
		for id, req := range e.pendingPerms {
			draining = append(draining, req)
			delete(e.pendingPerms, id)
		}
	}
	e.mu.Unlock()

	for _, req := range draining {
		logx.Printf("AGENT: [AUTO] resolving pending permission %s (auto-permission just enabled)", req.ID)
		select {
		case req.ResCh <- AllowOnce:
		case <-time.After(time.Second):
			logx.Printf("AGENT: pending permission %s had no listener when draining", req.ID)
		}
	}
}

// GetAutoPermission otomatik izin modunun durumunu döndürür.
func (e *Executor) GetAutoPermission() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.autoPermission
}

// ClearAgentPermissions clears all permanent permissions.
func (e *Executor) ClearAgentPermissions() {
	e.permissions.ClearAll()
}

// UndoLast reverts the last agent edit.
func (e *Executor) UndoLast() error {
	if e.backup == nil {
		return fmt.Errorf("backup manager not initialized")
	}
	return e.backup.UndoLast()
}

func (e *Executor) logEvent(sessionID string, ev AgentEvent) {
	if ev.Type == EventPermissionRequest || ev.Type == EventFinalResponse {
		return // we only log tool executions (results/errors)
	}

	entry := AgentLogEntry{
		Timestamp:  time.Now(),
		SessionID:  sessionID,
		ToolName:   ev.ToolName,
		Args:       ev.Args,
		Result:     ev.Result,
		Error:      ev.Error,
		DurationMs: ev.DurationMs,
	}

	e.mu.Lock()
	e.logs = append(e.logs, entry)
	// Keep last 1000 in memory — safe to cap now that auditLogFile below is
	// the durable copy; before H10's fix this in-memory slice was the
	// *only* copy, so every entry past 1000 (or any restart) was gone for
	// good, with nothing to fall back on for what an agent session with
	// file/command access had actually done.
	if len(e.logs) > 1000 {
		e.logs = e.logs[1:]
	}
	e.appendAuditLogLocked(entry)
	e.mu.Unlock()

	// Print to console for debugging
	if ev.Error != "" {
		logx.Printf("AGENT [%s] ERROR: %v", ev.ToolName, ev.Error)
	} else {
		logx.Printf("AGENT [%s] SUCCESS %dms", ev.ToolName, ev.DurationMs)
	}
}

// appendAuditLogLocked writes entry to auditLogFile as one JSON line.
// Called with e.mu already held — piggybacks on the same lock already
// guarding e.logs rather than adding a second one, since both protect the
// same "one entry at a time" invariant. Best-effort: a write failure is
// logged and otherwise ignored, matching every other fire-and-forget
// persistence path in this codebase (e.g. recordUsageEvent) — a disk error
// must not break the tool call it's describing.
func (e *Executor) appendAuditLogLocked(entry AgentLogEntry) {
	if e.auditLogFile == nil {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		logx.Printf("agent: marshal audit log entry: %v", err)
		return
	}
	if _, err := e.auditLogFile.Write(append(data, '\n')); err != nil {
		logx.Printf("agent: write audit log entry: %v", err)
	}
}

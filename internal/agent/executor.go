package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"memo/internal/provider"
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

	mu             sync.Mutex
	pendingPerms   map[string]*PermissionRequest
	logs           []AgentLogEntry
}

// NewExecutor creates a new agent executor.
func NewExecutor(basePath string, providerRouter *provider.Router, providerCfgMgr *provider.ConfigManager) *Executor {
	return &Executor{
		basePath:       basePath,
		providerRouter: providerRouter,
		providerCfgMgr: providerCfgMgr,
		registry:       NewRegistry(),
		permissions:    NewPermissionManager("data"),
		sandbox:        NewSandbox(DefaultSandboxConfig(basePath)),
		backup:         NewBackupManager("data"),
		pendingPerms:   make(map[string]*PermissionRequest),
		logs:           make([]AgentLogEntry, 0),
	}
}

// NewWhatsAppExecutor creates a lightweight executor with only WhatsApp tools.
// Reuses sandbox/permissions from an existing executor to avoid re-init.
func NewWhatsAppExecutor(existing *Executor) *Executor {
	return &Executor{
		basePath:       existing.basePath,
		providerRouter: existing.providerRouter,
		providerCfgMgr: existing.providerCfgMgr,
		registry:       NewWhatsAppRegistry(),
		permissions:    existing.permissions,
		sandbox:        existing.sandbox,
		backup:         existing.backup,
		pendingPerms:   make(map[string]*PermissionRequest),
		logs:           make([]AgentLogEntry, 0),
	}
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

// RunStream starts the agent execution and streams back chunks and events.
func (e *Executor) RunStream(ctx context.Context, sessionID string, modelName string, messages []provider.Message, onEvent func(AgentEvent), projectPath ...string) (<-chan provider.StreamChunk, error) {
	router := e.getRouter()
	if router == nil {
		return nil, fmt.Errorf("agent mode requires an active provider (external API or local model)")
	}

	// If a project path is provided for agent chat, update sandbox base path.
	// If not provided (regular chat), restore the default base path.
	if len(projectPath) > 0 && projectPath[0] != "" {
		e.sandbox.SetBasePath(projectPath[0])
	} else {
		e.sandbox.SetBasePath(e.basePath)
	}

	// Estimate token budget: sum of all initial messages' tokens as base,
	// plus generous headroom for tool call iterations.
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
	// Give 32K headroom for tool call iterations on top of the message budget.
	// The total maxTokens should still be well within the model's context window
	// because buildMessages() already truncated the history to fit CtxSize/MaxContextTokens.
	maxTokens := baseTokens + 32*1024
	if maxTokens < 64*1024 {
		maxTokens = 64 * 1024
	}

	pipeline := NewPipelineWithBudget(e.registry, e.permissions, e.sandbox, router, e.backup, maxTokens)

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
			log.Printf("AGENT: permission request %s timed out (60s), auto-denied", requestID)
			return DenyOnce, fmt.Errorf("permission timed out")
		case policy := <-resCh:
			return policy, nil
		}
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

	// Channel has buffer 1 and is empty at this point — send never blocks.
	req.ResCh <- policy
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
	// Keep last 1000 logs
	if len(e.logs) > 1000 {
		e.logs = e.logs[1:]
	}
	e.mu.Unlock()
	
	// Print to console for debugging
	if ev.Error != "" {
		log.Printf("AGENT [%s] ERROR: %v", ev.ToolName, ev.Error)
	} else {
		log.Printf("AGENT [%s] SUCCESS %dms", ev.ToolName, ev.DurationMs)
	}
}

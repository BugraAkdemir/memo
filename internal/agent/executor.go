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

// IsAvailable checks if the agent can run (needs external provider).
func (e *Executor) IsAvailable() bool {
	if e.providerRouter == nil {
		return false
	}
	// We could also check if activeProvider is external, but since router only
	// has external providers, if router exists, we have an external provider available.
	return true
}

// SyncRouter updates the providerRouter reference to match the app's current router.
// This is needed because the router is replaced when providers change.
func (e *Executor) SyncRouter(r *provider.Router) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.providerRouter = r
}

// RunStream starts the agent execution and streams back chunks and events.
func (e *Executor) RunStream(ctx context.Context, sessionID string, modelName string, messages []provider.Message, onEvent func(AgentEvent), projectPath ...string) (<-chan provider.StreamChunk, error) {
	if !e.IsAvailable() {
		return nil, fmt.Errorf("agent mode requires an active external provider")
	}

	// If a project path is provided for agent chat, update sandbox base path.
	// If not provided (regular chat), restore the default base path.
	if len(projectPath) > 0 && projectPath[0] != "" {
		e.sandbox.SetBasePath(projectPath[0])
	} else {
		e.sandbox.SetBasePath(e.basePath)
	}

	pipeline := NewPipeline(e.registry, e.permissions, e.sandbox, e.providerRouter, e.backup)

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

		select {
		case <-ctx.Done():
			return DenyOnce, ctx.Err()
		case policy := <-resCh:
			return policy, nil
		}
	}

	return pipeline.RunStream(ctx, messages, modelName, wrappedOnEvent, waitFn)
}

// HandlePermissionResponse routes the user's permission response to the waiting pipeline.
func (e *Executor) HandlePermissionResponse(requestID string, policy PermissionPolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	req, ok := e.pendingPerms[requestID]
	if !ok {
		return fmt.Errorf("permission request %s not found or already answered", requestID)
	}

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

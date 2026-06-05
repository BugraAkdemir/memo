package main

import (
	"fmt"
	"memo/internal/agent"
)

// ─── Agent Mode Bridge ──────────────────────────────────────────

func (a *App) GetAgentEnabled() bool {
	a.agentMu.RLock()
	defer a.agentMu.RUnlock()
	return a.agentEnabled
}

func (a *App) SetAgentEnabled(enabled bool) error {
	a.agentMu.Lock()
	a.agentEnabled = enabled
	a.agentMu.Unlock()
	return nil
}

func (a *App) HandleAgentPermission(requestID string, policy string) error {
	if a.agentExecutor == nil {
		return fmt.Errorf("agent executor not initialized")
	}
	return a.agentExecutor.HandlePermissionResponse(requestID, agent.PermissionPolicy(policy))
}

func (a *App) GetAgentPermissions() []agent.PermissionRecord {
	if a.agentExecutor == nil {
		return []agent.PermissionRecord{}
	}
	return a.agentExecutor.GetAgentPermissions()
}

func (a *App) RevokeAgentPermission(id string) error {
	if a.agentExecutor == nil {
		return fmt.Errorf("agent executor not initialized")
	}
	return a.agentExecutor.RevokeAgentPermission(id)
}

func (a *App) ClearAgentPermissions() {
	if a.agentExecutor != nil {
		a.agentExecutor.ClearAgentPermissions()
	}
}

func (a *App) UndoLastAgentEdit() error {
	if a.agentExecutor == nil {
		return fmt.Errorf("agent not enabled or initialized")
	}
	return a.agentExecutor.UndoLast()
}

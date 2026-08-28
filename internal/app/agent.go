package app

import (
	"fmt"

	"memo/internal/agent"
	"memo/internal/config"
)

// GetAgentEnabled reports whether agent mode is active.
func (a *App) GetAgentEnabled() bool {
	a.agentMu.RLock()
	defer a.agentMu.RUnlock()
	return a.agentEnabled
}

// SetAgentEnabled enables or disables agent mode and persists the choice to
// config.yaml so it survives a backend restart — mirrors
// UpdateWebSearchConfig. Without persistence, a restart silently reverted
// agent mode to off while every client still showed it on, and messages
// routed as plain toolless replies ("agent mode is off, turn it on...").
func (a *App) SetAgentEnabled(enabled bool) error {
	a.agentMu.Lock()
	a.agentEnabled = enabled
	a.agentMu.Unlock()

	a.cfgMu.Lock()
	cfg := a.cfg
	if cfg != nil {
		cfg.AgentMode.Enabled = enabled
	}
	a.cfgMu.Unlock()
	if cfg == nil {
		return nil
	}
	return config.Save(cfg)
}

// HandleAgentPermission records the user's policy decision for a pending permission request.
func (a *App) HandleAgentPermission(requestID string, policy string) error {
	if a.agentExecutor == nil {
		return fmt.Errorf("agent executor not initialized")
	}
	return a.agentExecutor.HandlePermissionResponse(requestID, agent.PermissionPolicy(policy))
}

// GetAgentPermissions returns the list of recorded agent permission decisions.
func (a *App) GetAgentPermissions() []agent.PermissionRecord {
	if a.agentExecutor == nil {
		return []agent.PermissionRecord{}
	}
	return a.agentExecutor.GetAgentPermissions()
}

// RevokeAgentPermission revokes a previously granted permission.
func (a *App) RevokeAgentPermission(id string) error {
	if a.agentExecutor == nil {
		return fmt.Errorf("agent executor not initialized")
	}
	return a.agentExecutor.RevokeAgentPermission(id)
}

// ClearAgentPermissions removes all agent permission records.
func (a *App) ClearAgentPermissions() {
	if a.agentExecutor != nil {
		a.agentExecutor.ClearAgentPermissions()
	}
}

// UndoLastAgentEdit reverts the most recent file edit performed by the agent.
func (a *App) UndoLastAgentEdit() error {
	if a.agentExecutor == nil {
		return fmt.Errorf("agent not enabled or initialized")
	}
	return a.agentExecutor.UndoLast()
}

// SetAgentAutoPermission toggles the Shift+Tab auto-permission mode.
func (a *App) SetAgentAutoPermission(enabled bool) error {
	if a.agentExecutor == nil {
		return fmt.Errorf("agent executor not initialized")
	}
	a.agentExecutor.SetAutoPermission(enabled)
	return nil
}

// GetAgentAutoPermission returns the current auto-permission mode state.
func (a *App) GetAgentAutoPermission() bool {
	if a.agentExecutor == nil {
		return false
	}
	return a.agentExecutor.GetAutoPermission()
}

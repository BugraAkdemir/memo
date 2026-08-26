package app

import (
	"fmt"

	"memo/internal/config"
)

// liveModeValidEngines/liveModeValidWorkModes/liveModeValidPermissionPolicies
// are the only accepted values for their respective LiveModeConfig fields —
// validated here rather than left to whatever a client happens to send, since
// this config also selects which code path handleLiveModeSession (a later
// phase) takes.
var (
	liveModeValidEngines            = map[string]bool{"local": true, "google_live": true, "openai_realtime": true, "elevenlabs": true, "custom": true}
	liveModeValidWorkModes          = map[string]bool{"delegate": true, "standalone": true}
	liveModeValidPermissionPolicies = map[string]bool{"voice_prompt": true, "auto_allow_once": true}
)

// GetLiveModeConfig returns the current top-level Live Mode selector
// (Enabled/ActiveEngine/WorkMode/AgentPermissionPolicy). Per-engine
// credentials/model/voice config is a later phase's own
// internal/livemode.ConfigManager-backed addition, not part of this struct.
func (a *App) GetLiveModeConfig() config.LiveModeConfig {
	if a.cfg == nil {
		return config.LiveModeConfig{}
	}
	return a.cfg.LiveMode
}

// UpdateLiveModeConfig validates and persists a new Live Mode selector.
// Mirrors SetBeta's simplicity (internal/app/remote_tailscale.go) — this is
// a single small config struct, not a multi-field endpoint like remote
// access, so one validate-then-save method covers the whole thing rather
// than one setter per field.
func (a *App) UpdateLiveModeConfig(cfg config.LiveModeConfig) error {
	if a.cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	if !liveModeValidEngines[cfg.ActiveEngine] {
		return fmt.Errorf("invalid active_engine: %q", cfg.ActiveEngine)
	}
	if !liveModeValidWorkModes[cfg.WorkMode] {
		return fmt.Errorf("invalid work_mode: %q", cfg.WorkMode)
	}
	if !liveModeValidPermissionPolicies[cfg.AgentPermissionPolicy] {
		return fmt.Errorf("invalid agent_permission_policy: %q", cfg.AgentPermissionPolicy)
	}
	a.cfg.LiveMode = cfg
	if err := config.Save(a.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

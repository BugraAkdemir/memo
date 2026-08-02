package app

import (
	"context"
	"fmt"
	"memo/internal/logx"
	"time"

	// Blank-imported for its init(), which registers claude-code-cli with
	// provider.RegisterConstructor — see internal/agentcli's package doc
	// for why it can't be imported by internal/provider directly.
	_ "memo/internal/agentcli"
	"memo/internal/config"
	"memo/internal/orchestra"
	"memo/internal/provider"
)

// GetProviders returns all configured providers with their connection status.
func (a *App) GetProviders() []provider.ProviderConfig {
	if a.providerCfgMgr == nil {
		return nil
	}
	configs := a.providerCfgMgr.GetAll()
	a.providerMu.RLock()
	router := a.providerRouter
	a.providerMu.RUnlock()
	if router != nil {
		active := router.ActiveProviders()
		activeMap := make(map[string]bool)
		for _, cfg := range active {
			activeMap[cfg.Name] = true
		}
		for i, cfg := range configs {
			configs[i].Connected = activeMap[cfg.Name]
		}
	}
	return configs
}

// UpdateProvider saves a provider config and rebuilds the router.
func (a *App) UpdateProvider(cfg provider.ProviderConfig) error {
	if a.providerCfgMgr == nil {
		return fmt.Errorf("provider system not initialized")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	a.providerCfgMgr.Set(cfg)
	configs := a.providerCfgMgr.GetEnabled()

	a.providerMu.Lock()
	a.providerRouter = provider.NewRouter(configs)
	// Re-apply the active provider restriction: NewRouter starts with no active
	// provider, so without this the router would forget the user's selection after
	// any provider edit and silently fall back to routing across all enabled ones.
	if a.activeProviderName != "" {
		a.providerRouter.SetActiveProvider(a.activeProviderName)
	}
	// Cancel the previous health-check goroutine so we don't accumulate one per
	// UpdateProvider call; each call creates a fresh router and needs exactly one.
	if a.healthCheckCancel != nil {
		a.healthCheckCancel()
		a.healthCheckCancel = nil
	}
	var (
		rt      = a.providerRouter
		hctx    context.Context
		hcancel context.CancelFunc
	)
	if len(configs) > 0 {
		hctx, hcancel = context.WithCancel(a.lifecycleCtx)
		a.healthCheckCancel = hcancel
	}
	a.providerMu.Unlock()

	if hctx != nil {
		goRecover("providerRouter.HealthCheck", func() { rt.HealthCheck(hctx, 5*time.Minute) })
	}
	return nil
}

// DeleteProvider removes a provider configuration and rebuilds the router.
// If the deleted provider was the active one, activeProviderName is cleared so
// the next chat request will not try to use a non-existent provider.
func (a *App) DeleteProvider(pt provider.ProviderType, name ...string) error {
	if a.providerCfgMgr == nil {
		return fmt.Errorf("provider config manager not initialized")
	}

	// Determine the name of the provider being deleted, so we can clear it if it
	// matches the current active provider.
	deletedName := ""
	if len(name) > 0 && name[0] != "" {
		deletedName = name[0]
	} else {
		for _, p := range a.providerCfgMgr.GetAll() {
			if p.Type == pt {
				deletedName = p.Name
				break
			}
		}
	}

	a.providerCfgMgr.Delete(pt, name...)

	// Rebuild router so the deleted provider stops serving immediately.
	configs := a.providerCfgMgr.GetEnabled()

	a.providerMu.Lock()
	if a.healthCheckCancel != nil {
		a.healthCheckCancel()
		a.healthCheckCancel = nil
	}
	a.providerRouter = provider.NewRouter(configs)
	if deletedName != "" && a.activeProviderName == deletedName {
		a.activeProviderName = ""
	}
	if a.activeProviderName != "" {
		a.providerRouter.SetActiveProvider(a.activeProviderName)
	}
	var (
		hctx    context.Context
		hcancel context.CancelFunc
	)
	if len(configs) > 0 {
		hctx, hcancel = context.WithCancel(a.lifecycleCtx)
		a.healthCheckCancel = hcancel
	}
	a.providerMu.Unlock()

	if hctx != nil {
		router := a.providerRouter
		goRecover("providerRouter.HealthCheck", func() { router.HealthCheck(hctx, 5*time.Minute) })
	}
	return nil
}

// TestProviderConnection tests connectivity for the given provider config.
func (a *App) TestProviderConnection(cfg provider.ProviderConfig) error {
	if a.providerCfgMgr == nil {
		return fmt.Errorf("provider system not initialized")
	}
	return a.providerCfgMgr.TestConnection(&cfg)
}

// reinitProviderAndOrchestra reloads providers.json and orchestra.json from
// disk and rebuilds providerCfgMgr/providerRouter/orchestraConductor from
// them, restoring activeProviderName from a.cfg.ActiveProvider the same way
// Startup() does. Used after ImportData and cloud restore replace these
// files on disk — without it, the running app kept serving chats through
// the pre-import in-memory router/conductor (and any provider the import
// removed would still look "active") until the next full restart.
func (a *App) reinitProviderAndOrchestra() {
	newCfgMgr := provider.NewConfigManager(config.DataPath("providers.json"), nil)
	configs := newCfgMgr.GetEnabled()

	activeProviderName := a.cfg.ActiveProvider
	if activeProviderName != "" {
		matchesName := false
		for _, p := range configs {
			if p.Name == activeProviderName {
				matchesName = true
				break
			}
		}
		if !matchesName {
			for _, p := range configs {
				if string(p.Type) == activeProviderName {
					activeProviderName = p.Name
					a.cfg.ActiveProvider = p.Name
					break
				}
			}
		}
	}

	a.providerMu.Lock()
	if a.healthCheckCancel != nil {
		a.healthCheckCancel()
		a.healthCheckCancel = nil
	}
	a.providerCfgMgr = newCfgMgr
	a.activeProviderName = activeProviderName
	var (
		newRouter *provider.Router
		hctx      context.Context
		hcancel   context.CancelFunc
	)
	if len(configs) > 0 {
		newRouter = provider.NewRouter(configs)
		if activeProviderName != "" {
			newRouter.SetActiveProvider(activeProviderName)
		}
		hctx, hcancel = context.WithCancel(a.lifecycleCtx)
		a.healthCheckCancel = hcancel
	}
	a.providerRouter = newRouter

	orchestraCfg := orchestra.LoadConfig(config.DataPath("orchestra.json"))
	a.orchestraConductor = orchestra.NewConductor(
		orchestraCfg,
		func(cfg provider.ProviderConfig) (provider.Provider, error) {
			if a.providerRouter == nil {
				return nil, fmt.Errorf("provider router not initialized, cannot create %s/%s", cfg.Type, cfg.Model)
			}
			p, ok := a.providerRouter.GetProvider(cfg.Name)
			if !ok {
				return nil, fmt.Errorf("provider %s not found in router (disabled or not configured), enable it in API Providers", cfg.Name)
			}
			return p, nil
		},
		func() []provider.ProviderConfig {
			if a.providerCfgMgr == nil {
				return nil
			}
			return a.providerCfgMgr.GetAll()
		},
	)
	a.providerMu.Unlock()

	if hctx != nil {
		goRecover("providerRouter.HealthCheck", func() { newRouter.HealthCheck(hctx, 5*time.Minute) })
	}
	logx.Printf("provider/orchestra config reloaded (%d enabled provider(s), orchestra enabled=%v)", len(configs), orchestraCfg.Enabled)
}

// SetActiveProvider selects which provider to use for chat (by provider Name).
func (a *App) SetActiveProvider(name string) {
	a.providerMu.Lock()
	a.activeProviderName = name
	if a.providerRouter != nil {
		a.providerRouter.SetActiveProvider(name)
	}
	a.providerMu.Unlock()

	a.cfg.ActiveProvider = name
	if err := config.Save(a.cfg); err != nil {
		logx.Printf("WARN: failed to persist active provider: %v", err)
	}
	logx.Printf("Active provider set to: %s", name)
}

// GetActiveProvider returns the currently active provider name.
func (a *App) GetActiveProvider() string {
	a.providerMu.RLock()
	defer a.providerMu.RUnlock()
	return a.activeProviderName
}

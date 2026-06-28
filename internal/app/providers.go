package app

import (
	"context"
	"fmt"
	"memo/internal/logx"
	"time"

	"memo/internal/config"
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
		rt     = a.providerRouter
		hctx   context.Context
		hcancel context.CancelFunc
	)
	if len(configs) > 0 {
		hctx, hcancel = context.WithCancel(a.lifecycleCtx)
		a.healthCheckCancel = hcancel
	}
	a.providerMu.Unlock()

	if hctx != nil {
		go rt.HealthCheck(hctx, 5*time.Minute)
	}
	return nil
}

// DeleteProvider removes a provider configuration.
func (a *App) DeleteProvider(pt provider.ProviderType, name ...string) error {
	if a.providerCfgMgr == nil {
		return fmt.Errorf("provider config manager not initialized")
	}
	a.providerCfgMgr.Delete(pt, name...)
	return nil
}

// TestProviderConnection tests connectivity for the given provider config.
func (a *App) TestProviderConnection(cfg provider.ProviderConfig) error {
	if a.providerCfgMgr == nil {
		return fmt.Errorf("provider system not initialized")
	}
	return a.providerCfgMgr.TestConnection(&cfg)
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

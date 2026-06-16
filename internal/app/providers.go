package app

import (
	"fmt"
	"log"
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
		activeMap := make(map[provider.ProviderType]bool)
		for _, cfg := range active {
			activeMap[cfg.Type] = true
		}
		for i, cfg := range configs {
			configs[i].Connected = activeMap[cfg.Type]
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
	a.providerMu.Unlock()
	if len(configs) > 0 {
		a.providerMu.RLock()
		rt := a.providerRouter
		a.providerMu.RUnlock()
		go rt.HealthCheck(a.lifecycleCtx, 5*time.Minute)
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

// SetActiveProvider selects which provider to use for chat.
func (a *App) SetActiveProvider(pt provider.ProviderType) {
	a.providerMu.Lock()
	a.activeProvider = pt
	if a.providerRouter != nil {
		a.providerRouter.SetActiveProvider(pt)
	}
	a.providerMu.Unlock()

	a.cfg.ActiveProvider = string(pt)
	if err := config.Save(a.cfg); err != nil {
		log.Printf("WARN: failed to persist active provider: %v", err)
	}
	log.Printf("Active provider set to: %s", pt)
}

// GetActiveProvider returns the currently active provider type.
func (a *App) GetActiveProvider() string {
	a.providerMu.RLock()
	defer a.providerMu.RUnlock()
	return string(a.activeProvider)
}

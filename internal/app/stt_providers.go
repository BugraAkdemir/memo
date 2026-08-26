package app

import (
	"fmt"
	"memo/internal/config"
	"memo/internal/stt"
)

// initSTTProviders loads data/stt_providers.json and builds the initial
// router. Called once from Startup, right after initTTSProviders — mirrors
// that function's role one-for-one (see docs/plans/PLAN_live_mode_v2.md §2).
func (a *App) initSTTProviders() {
	cfgMgr := stt.NewConfigManager(config.DataPath("stt_providers.json"), nil)
	router := stt.NewRouter(cfgMgr.GetEnabled())

	a.sttRouterMu.Lock()
	a.sttProviderCfgMgr = cfgMgr
	a.sttRouter = router
	a.sttRouterMu.Unlock()
}

// GetSTTProviders returns all configured external STT providers.
func (a *App) GetSTTProviders() []stt.ProviderConfig {
	a.sttRouterMu.RLock()
	cfgMgr := a.sttProviderCfgMgr
	a.sttRouterMu.RUnlock()
	if cfgMgr == nil {
		return nil
	}
	return cfgMgr.GetAll()
}

// UpdateSTTProvider saves an STT provider config and rebuilds the router.
func (a *App) UpdateSTTProvider(cfg stt.ProviderConfig) error {
	a.sttRouterMu.RLock()
	cfgMgr := a.sttProviderCfgMgr
	a.sttRouterMu.RUnlock()
	if cfgMgr == nil {
		return fmt.Errorf("STT provider system not initialized")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	cfgMgr.Set(cfg)

	a.sttRouterMu.Lock()
	a.sttRouter = stt.NewRouter(cfgMgr.GetEnabled())
	a.sttRouterMu.Unlock()
	return nil
}

// DeleteSTTProvider removes an STT provider config and rebuilds the router.
func (a *App) DeleteSTTProvider(pt stt.ProviderType, name ...string) error {
	a.sttRouterMu.RLock()
	cfgMgr := a.sttProviderCfgMgr
	a.sttRouterMu.RUnlock()
	if cfgMgr == nil {
		return fmt.Errorf("STT provider system not initialized")
	}
	cfgMgr.Delete(pt, name...)

	a.sttRouterMu.Lock()
	a.sttRouter = stt.NewRouter(cfgMgr.GetEnabled())
	a.sttRouterMu.Unlock()
	return nil
}

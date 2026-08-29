package app

import (
	"context"

	"memo/internal/provider"
	"memo/internal/taskloop"
)

const maxTaskTransientRetries = 2

// healTaskProvider is the engine's WithSelfHeal callback. It reacts to a
// worker-turn error for a running task list:
//
//   - rate-limit / quota  -> return false (the retry scheduler owns this;
//     the loop should wait, not switch)
//   - transient 5xx/timeout -> retry the same provider up to
//     maxTaskTransientRetries times, then fall through to a switch
//   - auth / bad key / missing model -> switch this task to the next enabled
//     provider and retry
//
// A successful switch returns true and never touches the user's global active
// provider or a neighbouring task's config.
func (a *App) healTaskProvider(ctx context.Context, listID string, workerErr error) bool {
	if ctx.Err() != nil {
		return false
	}
	if taskloop.IsRateLimitErr(workerErr) {
		return false
	}

	a.taskRunMu.Lock()
	trc := a.taskRunCfgs[listID]
	if trc == nil {
		a.taskRunMu.Unlock()
		return false
	}

	if taskloop.IsTransientErr(workerErr) && trc.transientRetries < maxTaskTransientRetries {
		trc.transientRetries++
		a.taskRunMu.Unlock()
		return true // same provider, try again
	}

	next, ok := a.nextHealthyProviderLocked(trc)
	if !ok {
		a.taskRunMu.Unlock()
		return false
	}

	enabled := a.enabledProviderConfigs()
	router := provider.NewRouter(enabled)
	router.SetActiveProvider(next.Name)
	trc.exec.SyncRouter(router)
	trc.providerName = next.Name
	trc.model = next.Model
	trc.effortLevel = next.EffortLevel
	trc.transientRetries = 0
	trc.triedProviders[next.Name] = true
	a.taskRunMu.Unlock()

	a.emitEvent("taskloop:provider_switched", listID+":"+next.Name)
	return true
}

// nextHealthyProviderLocked picks the first enabled provider config this task
// hasn't tried yet. Caller holds a.taskRunMu.
func (a *App) nextHealthyProviderLocked(trc *taskRunConfig) (provider.ProviderConfig, bool) {
	for _, p := range a.enabledProviderConfigs() {
		if p.Name == "" || trc.triedProviders[p.Name] {
			continue
		}
		return p, true
	}
	return provider.ProviderConfig{}, false
}

func (a *App) enabledProviderConfigs() []provider.ProviderConfig {
	a.providerMu.RLock()
	cfgMgr := a.providerCfgMgr
	a.providerMu.RUnlock()
	if cfgMgr == nil {
		return nil
	}
	return cfgMgr.GetEnabled()
}

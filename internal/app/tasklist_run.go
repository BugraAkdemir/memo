package app

import (
	"context"

	"memo/internal/agent"
	"memo/internal/provider"
)

// taskRunConfig is a running task list's private view of "which provider and
// model am I using". It is snapshotted from the global active provider when
// the list starts, carried to the worker turn via ctx, and mutated only by
// self-heal — never touching the user's global setting or a neighbouring
// task's config.
type taskRunConfig struct {
	exec             *agent.Executor
	providerName     string
	model            string
	effortLevel      string
	triedProviders   map[string]bool // providers self-heal has already ruled out
	transientRetries int             // consecutive 5xx/timeout retries on the current provider
}

type taskRunCtxKey struct{}

func withTaskRunConfig(ctx context.Context, c *taskRunConfig) context.Context {
	return context.WithValue(ctx, taskRunCtxKey{}, c)
}

// taskRunConfigFromCtx returns the task-local provider config if this ctx
// belongs to a Self-Driving worker turn, or nil for an ordinary interactive
// call (in which case callAgentStream behaves exactly as before).
func taskRunConfigFromCtx(ctx context.Context) *taskRunConfig {
	c, _ := ctx.Value(taskRunCtxKey{}).(*taskRunConfig)
	return c
}

// buildTaskRunConfig snapshots the current global provider/model into a
// task-private executor + router. The executor reuses the shared
// registry/permissions/audit trail but has its own router, so concurrent
// task lists (and interactive chat) never race SyncRouter on one executor.
func (a *App) buildTaskRunConfig() (*taskRunConfig, error) {
	router, model, effort, err := a.resolveAgentProvider()
	if err != nil {
		return nil, err
	}

	a.providerMu.RLock()
	name := a.activeProviderName
	cfgMgr := a.providerCfgMgr
	a.providerMu.RUnlock()

	// Give the task its own router rather than sharing a.providerRouter, so a
	// later self-heal SyncRouter on this task can't move the interactive
	// chat's provider. The local-llama path already returns a fresh
	// single-provider router from resolveAgentProvider, so reuse that.
	taskRouter := router
	if name != "" && cfgMgr != nil {
		if enabled := cfgMgr.GetEnabled(); len(enabled) > 0 {
			r := provider.NewRouter(enabled)
			r.SetActiveProvider(name)
			taskRouter = r
		}
	}

	return &taskRunConfig{
		exec:           agent.NewTaskExecutor(a.agentExecutor, taskRouter),
		providerName:   name,
		model:          model,
		effortLevel:    effort,
		triedProviders: map[string]bool{name: true},
	}, nil
}

// taskRunConfigFor returns (creating and caching if needed) the task-local
// provider config for a running list. Wired into the engine as its
// WorkerConfigHook.
func (a *App) taskRunConfigFor(ctx context.Context, listID string) context.Context {
	a.taskRunMu.RLock()
	trc := a.taskRunCfgs[listID]
	a.taskRunMu.RUnlock()

	if trc == nil {
		built, err := a.buildTaskRunConfig()
		if err != nil {
			// No usable provider snapshot: run the worker turn on the global
			// path (callAgentStream will surface the same error to the user).
			return ctx
		}
		a.taskRunMu.Lock()
		if existing := a.taskRunCfgs[listID]; existing != nil {
			trc = existing
		} else {
			a.taskRunCfgs[listID] = built
			trc = built
		}
		a.taskRunMu.Unlock()
	}
	return withTaskRunConfig(ctx, trc)
}

// clearTaskRunConfig drops a finished/paused list's provider snapshot so a
// later resume re-snapshots from the then-current global provider.
func (a *App) clearTaskRunConfig(listID string) {
	a.taskRunMu.Lock()
	delete(a.taskRunCfgs, listID)
	a.taskRunMu.Unlock()
}

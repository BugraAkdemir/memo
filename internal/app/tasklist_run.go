package app

import (
	"context"
	"strings"

	"memo/internal/agent"
	"memo/internal/logx"
	"memo/internal/provider"
)

// taskRunConfig is a running task list's private view of "which provider and
// model am I using". It is snapshotted from the global active provider when
// the list starts, carried to the worker turn via ctx, and mutated only by
// self-heal — never touching the user's global setting or a neighbouring
// task's config.
type taskRunConfig struct {
	exec             *agent.Executor
	listID           string // the list this snapshot belongs to (for token accounting etc.)
	providerName     string
	model            string
	effortLevel      string
	projectPath      string          // resolved once from the list's agent chat
	triedProviders   map[string]bool // providers self-heal has already ruled out
	transientRetries int             // consecutive 5xx/timeout retries on the current provider
	// providerRoaming: when false (default), self-heal never switches this task
	// to a different provider — a dead provider parks + notifies instead of
	// silently walking data/providers.json. Set from the list's Task.md
	// "# sağlayıcı:" header / config default (see resolveProviderPolicy).
	providerRoaming bool
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

	exec := agent.NewTaskExecutor(a.agentExecutor, taskRouter)
	// A running Self-Driving task is unattended by definition — starting it is
	// the consent. The worker turn goes through this private executor, so the
	// bypass must live here, not only on the engine's ref-counted global
	// setBypass callback (which now only covers the fallback-to-global path).
	exec.SetBypassPermissions(true)

	return &taskRunConfig{
		exec:           exec,
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
		built.projectPath = a.taskListProjectPath(listID)
		built.listID = listID

		// Apply the list's provider policy (Task.md "# sağlayıcı:" header /
		// config default). "pinned" re-snapshots onto a specific enabled
		// provider; "roaming" lets self-heal switch providers on failure.
		pol := a.resolveProviderPolicy(listID)
		built.providerRoaming = pol.roaming
		if pol.pinned != "" && !strings.EqualFold(pol.pinned, built.providerName) {
			if r, m, eff, perr := a.agentRouterFromProviderName(pol.pinned, ""); perr == nil {
				built.exec = agent.NewTaskExecutor(a.agentExecutor, r)
				built.exec.SetBypassPermissions(true)
				built.providerName = pol.pinned
				built.model = m
				built.effortLevel = eff
				built.triedProviders = map[string]bool{pol.pinned: true}
			} else {
				logx.Printf("taskloop: list %s pinned provider %q unresolved: %v (keeping %s)",
					listID, pol.pinned, perr, built.providerName)
			}
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

// taskListProjectPath resolves a list's agent chat to its on-disk project
// root ("" if unknown).
func (a *App) taskListProjectPath(listID string) string {
	if a.taskloopStore == nil {
		return ""
	}
	tl, err := a.taskloopStore.Get(listID)
	if err != nil {
		return ""
	}
	if sm := a.getSessionManager(); sm != nil {
		return sm.GetProjectPath(tl.ChatID)
	}
	return ""
}

// clearTaskRunConfig drops a finished/paused list's provider snapshot so a
// later resume re-snapshots from the then-current global provider.
func (a *App) clearTaskRunConfig(listID string) {
	a.taskRunMu.Lock()
	delete(a.taskRunCfgs, listID)
	a.taskRunMu.Unlock()
}

// taskModelLabel is the engine's WithModelLabel hook: "provider/model" for the
// list's current worker/coder turn, shown in the in-chat activity log. Falls
// back to the planexec coder role or the global agent provider when the list
// has no snapshot yet.
func (a *App) taskModelLabel(listID string) string {
	a.taskRunMu.RLock()
	trc := a.taskRunCfgs[listID]
	a.taskRunMu.RUnlock()
	if trc != nil && trc.providerName != "" {
		if trc.model != "" {
			return trc.providerName + "/" + trc.model
		}
		return trc.providerName
	}
	if _, m, _, err := a.planexecRouting(listID, "coder"); err == nil && m != "" {
		return m
	}
	name := a.currentProviderLabel()
	if m := a.activeProviderModel(name); m != "" {
		return name + "/" + m
	}
	return name
}

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"memo/internal/api"
	"memo/internal/provider"
)

// planTaskConfig is the engine's WithPlanConfig callback. Once per task list,
// before any item runs, it lets the model pick this task's provider / model
// (and optionally toggle Orchestra) based on the work and the repo rules. It
// is fully autonomous — it applies the choice and emits a notification, it
// never asks. Any failure is swallowed: planning-config must not block a task.
func (a *App) planTaskConfig(ctx context.Context, listID, chatID string, items []string) error {
	if a.cfg == nil || !a.cfg.TaskLoop.PlanningSelfConfig {
		return nil
	}
	// Provider lock (default): the task runs on whatever provider is active
	// right now — planning must not pick a different one. Only "# sağlayıcı:
	// otomatik" re-enables autonomous provider/model selection here.
	if !a.resolveProviderPolicy(listID).roaming {
		return nil
	}

	// Make sure the task has a provider snapshot to mutate.
	a.taskRunMu.RLock()
	trc := a.taskRunCfgs[listID]
	a.taskRunMu.RUnlock()
	if trc == nil {
		built, err := a.buildTaskRunConfig()
		if err != nil {
			return fmt.Errorf("no provider snapshot: %w", err)
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

	enabled := a.enabledProviderConfigs()
	if len(enabled) < 1 {
		return nil
	}

	choice, err := a.askPlanConfigLLM(ctx, trc, enabled, items)
	if err != nil {
		return err
	}
	a.applyPlanConfig(listID, trc, enabled, choice)
	return nil
}

type planConfigChoice struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Effort    string `json:"effort"`
	Orchestra *bool  `json:"orchestra"`
}

func (a *App) askPlanConfigLLM(ctx context.Context, trc *taskRunConfig, enabled []provider.ProviderConfig, items []string) (planConfigChoice, error) {
	var names []string
	for _, p := range enabled {
		names = append(names, fmt.Sprintf("%s (model: %s)", p.Name, p.Model))
	}
	a.providerMu.RLock()
	orchOn := a.orchestraConductor != nil && a.orchestraConductor.Config().Enabled
	a.providerMu.RUnlock()

	itemList := strings.Join(items, "\n- ")
	sys := "You configure an autonomous coding task before it runs. Reply with ONLY a JSON object, no prose:\n" +
		`{"provider":"<one of the listed provider names, or empty to keep current>","model":"<model or empty>","effort":"<low|medium|high or empty>","orchestra":<true|false|null>}`
	usr := fmt.Sprintf(
		"Enabled providers:\n- %s\n\nCurrent: provider=%s model=%s. Orchestra multi-model mode is currently %v.\n\nTask items:\n- %s\n\nPick the best provider/model for this work. Only set orchestra if switching it clearly helps; null to leave it. %s",
		strings.Join(names, "\n- "), trc.providerName, trc.model, orchOn, itemList, a.memoSystemGuidanceShort())

	raw := a.callLLMForReview(ctx, []api.Message{
		api.NewTextMessage("system", sys),
		api.NewTextMessage("user", usr),
	}, categoryTaskPlan)
	if strings.HasPrefix(raw, "⚠") {
		return planConfigChoice{}, fmt.Errorf("plan-config LLM: %s", raw)
	}

	jsonPart := raw
	if i := strings.IndexByte(raw, '{'); i >= 0 {
		if j := strings.LastIndexByte(raw, '}'); j > i {
			jsonPart = raw[i : j+1]
		}
	}
	var choice planConfigChoice
	if err := json.Unmarshal([]byte(jsonPart), &choice); err != nil {
		return planConfigChoice{}, fmt.Errorf("plan-config parse: %w (raw: %s)", err, truncateStr(raw, 160))
	}
	return choice, nil
}

func (a *App) applyPlanConfig(listID string, trc *taskRunConfig, enabled []provider.ProviderConfig, choice planConfigChoice) {
	var changes []string

	if want := strings.TrimSpace(choice.Provider); want != "" && !strings.EqualFold(want, trc.providerName) {
		for _, p := range enabled {
			if strings.EqualFold(p.Name, want) {
				router := provider.NewRouter(enabled)
				router.SetActiveProvider(p.Name)
				a.taskRunMu.Lock()
				trc.exec.SyncRouter(router)
				trc.providerName = p.Name
				trc.model = p.Model
				trc.effortLevel = p.EffortLevel
				trc.triedProviders[p.Name] = true
				a.taskRunMu.Unlock()
				changes = append(changes, "provider="+p.Name)
				break
			}
		}
	}
	if m := strings.TrimSpace(choice.Model); m != "" && m != trc.model {
		a.taskRunMu.Lock()
		trc.model = m
		a.taskRunMu.Unlock()
		changes = append(changes, "model="+m)
	}
	if e := strings.ToLower(strings.TrimSpace(choice.Effort)); e == "low" || e == "medium" || e == "high" {
		a.taskRunMu.Lock()
		trc.effortLevel = e
		a.taskRunMu.Unlock()
		changes = append(changes, "effort="+e)
	}
	if choice.Orchestra != nil {
		a.providerMu.RLock()
		cond := a.orchestraConductor
		a.providerMu.RUnlock()
		if cond != nil {
			cur := cond.Config()
			if cur.Enabled != *choice.Orchestra {
				cur.Enabled = *choice.Orchestra
				cond.UpdateConfig(cur)
				changes = append(changes, fmt.Sprintf("orchestra=%v", *choice.Orchestra))
			}
		}
	}

	if len(changes) > 0 {
		a.emitEvent("taskloop:config_changed", listID+": "+strings.Join(changes, " "))
	}
}

// memoSystemGuidanceShort returns a trimmed slice of the memo-system skill so
// the plan-config prompt stays small.
func (a *App) memoSystemGuidanceShort() string {
	g := a.memoSystemGuidance()
	if len(g) > 1200 {
		return g[:1200]
	}
	return g
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

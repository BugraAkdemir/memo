package app

import (
	"context"
	"fmt"
	"strings"

	"memo/internal/agent"
	"memo/internal/orchestra"
	"memo/internal/provider"
	"memo/internal/taskloop"
)

// appSubAgentRunner implements taskloop.SubAgentRunner. Each Run is an
// ephemeral agent.Executor turn: no session, no RAG/memory/persona (a bare
// system+user message pair), permissions bypassed. The coder gets the full
// tool registry; every other role gets the read-only registry.
type appSubAgentRunner struct{ a *App }

func (r *appSubAgentRunner) Run(ctx context.Context, spec taskloop.SubAgentSpec, writeCapable bool) (string, error) {
	trc := taskRunConfigFromCtx(ctx)

	var router *provider.Router
	projectPath, listID := "", ""
	base := r.a.agentExecutor
	if trc != nil {
		router = trc.exec.ActiveRouter()
		projectPath = trc.projectPath
		listID = trc.listID
	}
	if router == nil {
		router = provider.NewRouter(r.a.enabledProviderConfigs())
		r.a.providerMu.RLock()
		name := r.a.activeProviderName
		r.a.providerMu.RUnlock()
		if name != "" {
			router.SetActiveProvider(name)
		}
	}

	registry := agent.NewReadOnlyRegistry()
	if writeCapable {
		registry = agent.NewRegistry()
	}
	exec := agent.NewSubAgentExecutor(base, registry, router, projectPath)
	exec.SetBypassPermissions(true)

	msgs := []provider.Message{
		provider.TextMessage("system", spec.SystemPrompt),
		provider.TextMessage("user", spec.Task),
	}
	// Mirror this sub-agent's tool calls into the live task card, the same
	// way a plain worker/coder turn does (emitStepToolActivity) — role-tagged
	// so four sub-agents interleaving in parallel read as four sub-agents,
	// not one. Before this, Spawn's four goroutines ran with onEvent a no-op:
	// no tokens counted, no activity logged, so SilentSec sat frozen at the
	// list's full elapsed time regardless of how much real work the
	// sub-agents were doing underneath it — live, a 6-minute run (three of
	// those on one sub-agent hitting a slow provider) looked, from the card,
	// indistinguishable from a hang.
	onEvent := func(ev agent.AgentEvent) {}
	if listID != "" {
		onEvent = func(ev agent.AgentEvent) { r.a.emitSubAgentActivity(listID, string(spec.Role), ev) }
	}
	streamCh, err := exec.RunStream(ctx, "", spec.Model, "", msgs, onEvent, projectPath)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for chunk := range streamCh {
		if chunk.Error != "" {
			return sb.String(), fmt.Errorf("sub-agent %s: %s", spec.Role, chunk.Error)
		}
		sb.WriteString(chunk.Content)
	}
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return "", fmt.Errorf("sub-agent %s produced no output", spec.Role)
	}
	return out, nil
}

// subRoleToOrchestra maps a sub-agent role to the closest Orchestra role for
// model/prompt selection (there is no "coder" Orchestra role).
func subRoleToOrchestra(role taskloop.SubRole) orchestra.RoleName {
	switch role {
	case taskloop.SubRoleCoder:
		return orchestra.RoleBackend
	case taskloop.SubRoleAnalyzer:
		return orchestra.RolePlanner
	case taskloop.SubRoleReviewer:
		return orchestra.RoleReviewer
	case taskloop.SubRoleTester:
		return orchestra.RoleDevOps
	default:
		return orchestra.RoleGeneral
	}
}

// resolveSubAgentSpecs turns one item into a writer + read-only roles. When
// Orchestra is enabled each role's model/prompt comes from its RoleConfig;
// otherwise every role runs on the task's own model with a role-specific
// system prompt.
func (a *App) resolveSubAgentSpecs(itemText, feedback string) []taskloop.SubAgentSpec {
	roles := []taskloop.SubRole{
		taskloop.SubRoleCoder,
		taskloop.SubRoleAnalyzer,
		taskloop.SubRoleReviewer,
		taskloop.SubRoleTester,
	}

	a.providerMu.RLock()
	cond := a.orchestraConductor
	a.providerMu.RUnlock()

	var orchCfg orchestra.OrchestraConfig
	orchOn := false
	if cond != nil {
		orchCfg = cond.Config()
		orchOn = orchCfg.Enabled
	}

	taskModel := ""
	if trc := a.currentAnyTaskModel(); trc != "" {
		taskModel = trc
	}

	coderTask := itemText
	if strings.TrimSpace(feedback) != "" {
		coderTask = itemText + "\n\nÖnceki turda eksik bulunanlar:\n" + feedback
	}

	specs := make([]taskloop.SubAgentSpec, 0, len(roles))
	for _, role := range roles {
		orole := subRoleToOrchestra(role)
		model := taskModel
		sysPrompt := orchestra.DefaultSystemPrompt(orole)
		if orchOn {
			if rc, ok := findOrchestraRole(orchCfg, orole); ok && rc.Enabled && rc.ModelName != "" {
				model = rc.ModelName
				if strings.TrimSpace(rc.SystemPrompt) != "" {
					sysPrompt = rc.SystemPrompt
				}
			} else if orchCfg.ChiefModel != "" {
				model = orchCfg.ChiefModel
			}
		}
		task := itemText
		switch role {
		case taskloop.SubRoleCoder:
			task = coderTask + "\n\nSADECE SEN dosya yazabilirsin. Değişiklikleri uygula."
		case taskloop.SubRoleAnalyzer:
			task = "Aşağıdaki maddeyi analiz et; ilgili kodu ve riskleri özetle (dosya YAZMA):\n" + itemText
		case taskloop.SubRoleReviewer:
			task = "Aşağıdaki madde için yapılan değişiklikleri gözden geçir, eksik/yanlışları listele (dosya YAZMA):\n" + itemText
		case taskloop.SubRoleTester:
			task = "Aşağıdaki madde ile ilgili testleri/derlemeyi çalıştır (run_command_readonly) ve sonucu raporla:\n" + itemText
		}
		specs = append(specs, taskloop.SubAgentSpec{
			Role:         role,
			Model:        model,
			SystemPrompt: sysPrompt,
			Task:         task,
		})
	}
	return specs
}

func findOrchestraRole(cfg orchestra.OrchestraConfig, name orchestra.RoleName) (orchestra.RoleConfig, bool) {
	for _, rc := range cfg.Roles {
		if rc.Role == name {
			return rc, true
		}
	}
	return orchestra.RoleConfig{}, false
}

// currentAnyTaskModel returns some running task's model as a fallback when a
// sub-agent spec is built outside a specific task ctx. Best-effort.
func (a *App) currentAnyTaskModel() string {
	a.taskRunMu.RLock()
	defer a.taskRunMu.RUnlock()
	for _, trc := range a.taskRunCfgs {
		if trc.model != "" {
			return trc.model
		}
	}
	return ""
}

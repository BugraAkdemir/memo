package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"memo/internal/agent"
	"memo/internal/logx"
	"memo/internal/provider"
	"memo/internal/taskloop"
)

// escalateStep is the engine's WithEscalator callback: a targeted re-plan of
// one step that the coder could not complete. Runs on the planner role's
// model (a strong/cloud model). A network error propagates so the engine can
// park the list offline and retry later.
func (a *App) escalateStep(ctx context.Context, listID string, step taskloop.PlanStep, failure taskloop.EscalationInput) ([]taskloop.PlanStep, error) {
	router, model, effort, err := a.planexecRouting(listID, "planner")
	if err != nil {
		return nil, err
	}

	projectPath := a.taskListProjectPath(listID)
	subExec := agent.NewSubAgentExecutor(a.agentExecutor, agent.NewReadOnlyRegistry(), router, projectPath)
	subExec.SetBypassPermissions(true)

	sys := `Sen bir kıdemli planlayıcısın. Bir uygulayıcı ajan, verilen TEK adımı tamamlayamadı.
Repoyu SALT-OKUNUR incele ve o adımı, uygulayıcının başarabileceği daha küçük/net adımlara böl.
SADECE şu JSON'u döndür: {"steps":[{"id":"","item_id":"...","text":"...","difficulty":"trivial|normal|hard","depends_on":[],"acceptance_checks":[{"kind":"command|grep|fuzzy","spec":"...","expect":""}]}]}`

	var user strings.Builder
	if rules, rerr := taskloop.ReadRules(projectPath); rerr == nil && strings.TrimSpace(rules) != "" {
		user.WriteString("# Repo kuralları\n\n" + rules + "\n\n")
	}
	fmt.Fprintf(&user, "# Takılan adım (item_id=%s)\n\n%s\n\n", step.ItemID, step.Text)
	if strings.TrimSpace(step.LiteralContent) != "" {
		user.WriteString("Hedeflenen içerik:\n```\n" + step.LiteralContent + "\n```\n\n")
	}
	fmt.Fprintf(&user, "# Hata\n\n%s\n", failure.Error)
	if strings.TrimSpace(failure.CheckOutput) != "" {
		fmt.Fprintf(&user, "\n# Kabul kontrolü çıktısı\n\n%s\n", failure.CheckOutput)
	}
	user.WriteString("\nBu adımı yeniden planla ve SADECE JSON döndür.")

	sctx, scancel := context.WithCancel(ctx)
	defer scancel()
	streamCh, err := subExec.RunStream(sctx, "", model, effort,
		[]provider.Message{
			provider.TextMessage("system", sys),
			provider.TextMessage("user", user.String()),
		}, func(ev agent.AgentEvent) { a.emitStepToolActivity(listID, ev) }, projectPath)
	if err != nil {
		return nil, err
	}
	out, derr := drainStreamIdle(streamCh, scancel, a.streamIdleTimeout())
	if derr != nil {
		// The raw string carries dial/DNS markers for isOfflineErr.
		return nil, fmt.Errorf("%w", derr)
	}
	raw := strings.TrimSpace(out)
	if raw == "" {
		return nil, fmt.Errorf("escalator produced no output")
	}

	// Lenient parse: ReplaceStep + the engine's plan.Normalize afterwards
	// validate the merged graph, so don't reject the replacement fragment
	// here for referencing the (about-to-be-removed) failed step.
	var wrap struct {
		Steps []taskloop.PlanStep `json:"steps"`
	}
	if err := json.Unmarshal([]byte(taskloop.ExtractJSONObject(raw)), &wrap); err != nil {
		logx.Printf("TASKLOOP: escalation JSON parse failed %s/%s: %v", listID, step.ID, err)
		return nil, err
	}
	if len(wrap.Steps) == 0 {
		return nil, fmt.Errorf("escalator returned no steps")
	}
	for i := range wrap.Steps {
		if wrap.Steps[i].ItemID == "" {
			wrap.Steps[i].ItemID = step.ItemID
		}
	}
	return wrap.Steps, nil
}

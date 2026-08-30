package app

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/logx"
	"memo/internal/provider"
	"memo/internal/taskloop"
)

// runPlanStep is the engine's WithStepRunner callback: one fresh, ephemeral
// coder turn per plan step. No session, no RAG/memory/persona — a bare
// system+user pair, permissions bypassed (starting the task was the consent).
func (a *App) runPlanStep(ctx context.Context, listID string, step taskloop.PlanStep, stateDoc string) (string, error) {
	rm := a.resolveRoleModels(listID)

	router, defModel, effort, err := a.resolveAgentProvider()
	if err != nil {
		return "", err
	}
	model := rm.Coder
	if model == "" {
		model = defModel
	}

	projectPath := a.taskListProjectPath(listID)
	subExec := agent.NewSubAgentExecutor(a.agentExecutor, agent.NewRegistry(), router, projectPath)
	subExec.SetBypassPermissions(true)

	msgs := []provider.Message{
		provider.TextMessage("system", coderStepSystemPrompt()),
		provider.TextMessage("user", coderStepUserPrompt(projectPath, step, stateDoc)),
	}

	sctx, scancel := context.WithCancel(ctx)
	defer scancel()
	streamCh, err := subExec.RunStream(sctx, "", model, effort, msgs, func(agent.AgentEvent) {}, projectPath)
	if err != nil {
		return "", err
	}
	out, derr := drainStreamIdle(streamCh, scancel, a.streamIdleTimeout())
	if derr != nil {
		return "", fmt.Errorf("coder: %w", derr)
	}
	return strings.TrimSpace(out), nil
}

func coderStepSystemPrompt() string {
	return `Sen bir uygulayıcı (coder) ajansın. Sana bir planın TEK bir adımı verilecek.
SADECE o adımı uygula — fazlasını yapma, sıradaki adımlara geçme.
Dosya oku/yaz/düzenle ve gerekiyorsa komut çalıştır. Bittiğinde kısa bir özet ver:
hangi dosyalara dokundun, ne değişti. Repo kurallarına uy.`
}

func coderStepUserPrompt(projectPath string, step taskloop.PlanStep, stateDoc string) string {
	var b strings.Builder
	if rules, err := taskloop.ReadRules(projectPath); err == nil && strings.TrimSpace(rules) != "" {
		b.WriteString("# Repo kuralları\n\n")
		b.WriteString(rules)
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(stateDoc) != "" {
		b.WriteString("# Şu ana kadarki ilerleme (bağlam)\n\n")
		b.WriteString(strings.TrimSpace(stateDoc))
		b.WriteString("\n\n")
	}
	b.WriteString("# Bu adım\n\n")
	b.WriteString(strings.TrimSpace(step.Text))
	b.WriteByte('\n')
	if strings.TrimSpace(step.LiteralContent) != "" {
		b.WriteString("\nBirebir uygulanacak içerik:\n```\n")
		b.WriteString(step.LiteralContent)
		b.WriteString("\n```\n")
	}
	if len(step.AcceptanceChecks) > 0 {
		b.WriteString("\nBu adım şunları sağlamalı:\n")
		for _, c := range step.AcceptanceChecks {
			fmt.Fprintf(&b, "- (%s) %s\n", c.Kind, c.Spec)
		}
	}
	return b.String()
}

// acceptancecheck is the engine's WithAcceptanceChecker callback: run a step's
// checks after the coder turn. Deterministic checks (command / grep) run
// directly in the project dir; fuzzy checks go to one verifier-model call.
func (a *App) acceptancecheck(ctx context.Context, listID string, step taskloop.PlanStep) (bool, string, error) {
	projectPath := a.taskListProjectPath(listID)
	for _, c := range step.AcceptanceChecks {
		switch strings.ToLower(c.Kind) {
		case "command":
			if ok, detail := runCheckCommand(ctx, projectPath, c.Spec, c.Expect, a.acceptanceCommandTimeout()); !ok {
				return false, fmt.Sprintf("command %q: %s", c.Spec, detail), nil
			}
		case "grep":
			if ok, detail := runCheckGrep(ctx, projectPath, c.Spec, c.Expect); !ok {
				return false, fmt.Sprintf("grep %q: %s", c.Spec, detail), nil
			}
		case "fuzzy":
			ok, detail, err := a.runFuzzyCheck(ctx, projectPath, c.Spec)
			if err != nil {
				logx.Printf("TASKLOOP: fuzzy check error (%s): %v — treating as pass", listID, err)
				continue
			}
			if !ok {
				return false, "fuzzy: " + detail, nil
			}
		}
	}
	return true, "", nil
}

func (a *App) acceptanceCommandTimeout() time.Duration {
	a.cfgMu.RLock()
	s := a.cfg.TaskLoop.AcceptanceCommandTimeoutSec
	a.cfgMu.RUnlock()
	if s <= 0 {
		s = 300
	}
	return time.Duration(s) * time.Second
}

func runCheckCommand(ctx context.Context, dir, spec, expect string, timeout time.Duration) (bool, string) {
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "bash", "-lc", spec)
	if dir != "" {
		cmd.Dir = dir
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()
	if err != nil {
		return false, fmt.Sprintf("exit error: %v\n%s", err, tail(out, 500))
	}
	if expect != "" && !strings.Contains(out, expect) {
		return false, fmt.Sprintf("output missing %q", expect)
	}
	return true, ""
}

func runCheckGrep(ctx context.Context, dir, pattern, expect string) (bool, string) {
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	target := dir
	if target == "" {
		target = "."
	}
	cmd := exec.CommandContext(cctx, "grep", "-rIl", "--", pattern, target)
	err := cmd.Run()
	found := err == nil
	wantAbsent := strings.EqualFold(strings.TrimSpace(expect), "absent")
	if wantAbsent {
		if found {
			return false, "pattern present but expected absent"
		}
		return true, ""
	}
	if !found {
		return false, "pattern not found"
	}
	return true, ""
}

func (a *App) runFuzzyCheck(ctx context.Context, projectPath, criterion string) (bool, string, error) {
	ctxInfo := ""
	if projectPath != "" {
		if out, err := exec.CommandContext(ctx, "git", "-C", projectPath, "diff", "--stat").Output(); err == nil {
			ctxInfo = string(out)
		}
	}
	msgs := []api.Message{
		api.NewTextMessage("system", `Sen bir denetleyicisin. Sana bir kabul ölçütü ve son değişikliklerin özeti verilecek.
SADECE şu JSON'u döndür: {"approved": true, "feedback": ""}. Ölçüt karşılanmıyorsa approved:false ve kısa bir feedback ver.`),
		api.NewTextMessage("user", fmt.Sprintf("Ölçüt: %s\n\nSon değişiklikler:\n%s", criterion, tail(ctxInfo, 2000))),
	}
	raw := a.callLLMForReview(ctx, msgs)
	if strings.HasPrefix(raw, "⚠") {
		return false, "", fmt.Errorf("verifier LLM: %s", raw)
	}
	pass, reason, err := taskloop.ExtractAndParseReview(raw)
	if err != nil {
		return false, "", err
	}
	return pass, reason, nil
}

// compactPlanState is the engine's WithStateCompactor callback.
func (a *App) compactPlanState(ctx context.Context, listID, current string) (string, error) {
	msgs := []api.Message{
		api.NewTextMessage("system", `Aşağıdaki ilerleme kaydını kısalt. Kararları, dokunulan dosyaları ve keşfedilen tuzakları KORU; tekrarları ve ayrıntıyı at. Markdown döndür.`),
		api.NewTextMessage("user", current),
	}
	raw := a.callLLMForReview(ctx, msgs)
	if strings.HasPrefix(raw, "⚠") {
		return "", fmt.Errorf("compactor LLM: %s", raw)
	}
	return strings.TrimSpace(raw), nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

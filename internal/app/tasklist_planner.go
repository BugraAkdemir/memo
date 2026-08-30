package app

import (
	"context"
	"fmt"
	"strings"

	"memo/internal/agent"
	"memo/internal/logx"
	"memo/internal/provider"
	"memo/internal/taskloop"
)

// planTask is the engine's WithPlanner callback for "planlayıcı" mode. It runs
// one bounded, read-only agent turn (the planner role's model) that inspects
// the repo and emits a structured Plan JSON, then parses it.
func (a *App) planTask(ctx context.Context, listID, chatID, projectRoot, preamble string, items []string, granularity string) (*taskloop.Plan, error) {
	router, model, effort, err := a.planexecRouting(listID, "planner")
	if err != nil {
		return nil, err
	}

	registry := agent.NewReadOnlyRegistry()
	exec := agent.NewSubAgentExecutor(a.agentExecutor, registry, router, projectRoot)
	exec.SetBypassPermissions(true)

	msgs := []provider.Message{
		provider.TextMessage("system", plannerSystemPrompt(granularity)),
		provider.TextMessage("user", plannerUserPrompt(preamble, items)),
	}

	sctx, scancel := context.WithCancel(ctx)
	defer scancel()
	streamCh, err := exec.RunStream(sctx, "", model, effort, msgs,
		func(ev agent.AgentEvent) { a.emitStepToolActivity(listID, ev) }, projectRoot)
	if err != nil {
		return nil, err
	}
	out, derr := drainStreamIdle(streamCh, scancel, a.streamIdleTimeout())
	if derr != nil {
		return nil, fmt.Errorf("planner: %w", derr)
	}
	raw := strings.TrimSpace(out)
	if raw == "" {
		return nil, fmt.Errorf("planner produced no output")
	}

	plan, err := taskloop.ParsePlannerJSON(raw)
	if err != nil {
		logx.Printf("TASKLOOP: planner JSON parse failed for %s: %v", listID, err)
		return nil, err
	}
	// The planner is told to tag steps with the 1-based item ordinal; drop any
	// step pointing at an item that doesn't exist rather than trusting it.
	valid := map[string]bool{}
	for i := range items {
		valid[fmt.Sprintf("%d", i+1)] = true
	}
	kept := plan.Steps[:0]
	for _, s := range plan.Steps {
		if s.ItemID == "" || valid[s.ItemID] {
			kept = append(kept, s)
		} else {
			logx.Printf("TASKLOOP: planner step %s targets unknown item %q, dropped", s.ID, s.ItemID)
		}
	}
	plan.Steps = kept
	if err := plan.Normalize(); err != nil {
		return nil, err
	}
	return plan, nil
}

func plannerSystemPrompt(granularity string) string {
	gran := "hybrid"
	switch granularity {
	case "intent", "literal", "hybrid":
		gran = granularity
	}
	detail := map[string]string{
		"intent":  "Her adım tek bir net niyet olsun; uygulayıcı ayrıntıyı kendi çözecek.",
		"literal": "Her adım tek bir dosya/fonksiyon düzeyinde, birebir uygulanabilir olsun; küçük dosyalar için literal_content'e tam içeriği koy.",
		"hybrid":  "Her adıma difficulty (trivial|normal|hard) ver; trivial/normal adımları birebir yaz, hard adımları niyet düzeyinde bırak.",
	}[gran]

	return `Sen bir kıdemli planlayıcısın. Sana bir görev listesinin (Task.md) maddeleri verilecek.
Repoyu SALT-OKUNUR araçlarla incele (dosya yazma, komut çalıştırma yok) ve işi küçük,
tek tek doğrulanabilir ADIMLARA böl.

` + detail + `

SADECE aşağıdaki şemada tek bir JSON nesnesi döndür — başka hiçbir metin yazma:

{
  "steps": [
    {
      "id": "S1",
      "item_id": "1",
      "text": "ne yapılacağının net ve kısa tarifi",
      "kind": "create|edit|command|check",
      "difficulty": "trivial|normal|hard",
      "literal_content": "(opsiyonel) küçük dosyanın tam içeriği",
      "depends_on": ["S0"],
      "acceptance_checks": [
        {"kind": "command", "spec": "go build ./..."},
        {"kind": "grep", "spec": "func Foo", "expect": "present"},
        {"kind": "fuzzy", "spec": "fonksiyon boş girdiyi düzgün ele alıyor"}
      ]
    }
  ]
}

Kurallar:
- id'ler S1, S2, … benzersiz olsun. item_id, verilen maddenin 1 tabanlı sırasıdır ("1".."N").
- depends_on yalnızca daha önce tanımlı id'lere işaret etsin; döngü olmasın.
- Bağımsız adımları paralel koşabilmek için gereksiz depends_on ekleme.
- Her adımda en az bir acceptance_checks olsun; mümkünse deterministik (command/grep).
- Repo kurallarına (aşağıda) uy.`
}

func plannerUserPrompt(preamble string, items []string) string {
	var b strings.Builder
	if strings.TrimSpace(preamble) != "" {
		b.WriteString(preamble)
		b.WriteString("\n\n")
	}
	b.WriteString("# Task.md maddeleri (item_id = sıra numarası)\n\n")
	for i, it := range items {
		fmt.Fprintf(&b, "%d. %s\n", i+1, it)
	}
	b.WriteString("\nBu maddeleri adımlara böl ve SADECE JSON döndür.")
	return b.String()
}

package app

import (
	"testing"

	"memo/internal/api"
	"memo/internal/cliproxy"
	"memo/internal/config"
	"memo/internal/memory"
)

func gaugeApp() *App {
	a := &App{cfg: &config.AppConfig{}}
	a.cfg.AgentMode.ConversationCompactEnabled = true
	a.cfg.AgentMode.CompactThresholdPct = 90
	a.cfg.Llama.MaxContextTokens = 10000 // pins the window of a provider-less test App
	return a
}

func TestContextReport_UsesTheProvidersCountAndScalesTheMakeup(t *testing.T) {
	a := gaugeApp()
	prov, model := a.activeModelIdentity()
	a.recordContextSnap("c1", contextSnap{window: 10000, system: 300, history: 600, tools: 100, current: 0, provider: prov, model: model})
	a.noteTurnUsage("c1", &usageMeta{Provider: prov, Model: model, PromptTokens: 2000, PromptReal: true}, 500)

	rep := a.ContextReport("c1")
	if !rep.UsedReal || rep.Used != 2500 {
		t.Fatalf("used = %d (real=%v), want the provider's 2000 prompt + 500 reply", rep.Used, rep.UsedReal)
	}
	if rep.Window != 10000 || rep.Percent != 25 {
		t.Errorf("window=%d percent=%d, want 10000 / 25", rep.Window, rep.Percent)
	}
	sum := 0
	for _, c := range rep.Categories {
		sum += c.Tokens
	}
	if sum < 1995 || sum > 2005 {
		t.Errorf("categories add up to %d, want them scaled to the provider's 2000 prompt tokens: %+v", sum, rep.Categories)
	}
	if !rep.AutoCompactEnabled || rep.AutoCompactPct != 90 {
		t.Errorf("auto compact = %v/%d, want on at 90", rep.AutoCompactEnabled, rep.AutoCompactPct)
	}
}

func TestContextReport_EstimateWhenTheProviderReportedNothing(t *testing.T) {
	a := gaugeApp()
	prov, model := a.activeModelIdentity()
	a.recordContextSnap("c1", contextSnap{window: 10000, system: 300, history: 600, provider: prov, model: model})
	a.noteTurnUsage("c1", &usageMeta{Provider: prov, Model: model, PromptTokens: 900, PromptReal: false}, 100)

	rep := a.ContextReport("c1")
	if rep.UsedReal {
		t.Fatal("an estimate must not be presented as the provider's count")
	}
	if rep.Used != 1000 { // 300 + 600 estimated prompt + the 100-token reply
		t.Errorf("used = %d, want 1000", rep.Used)
	}
}

func TestContextReport_AnotherModelsCountIsNotThisModels(t *testing.T) {
	a := gaugeApp()
	a.recordContextSnap("c1", contextSnap{window: 10000, history: 400})
	a.noteTurnUsage("c1", &usageMeta{Provider: "openai", Model: "gpt-x", PromptTokens: 9000, PromptReal: true}, 100)
	rep := a.ContextReport("c1")
	// 400 estimated prompt + the 100-token reply the chat now also holds; the other
	// model's 9000 prompt tokens (its own tokenizer, its own window) are not used.
	if rep.UsedReal || rep.Used != 500 {
		t.Errorf("used = %d real=%v, want the 500-token estimate: a count from another model must not be shown", rep.Used, rep.UsedReal)
	}
}

func TestContextReport_UnknownChatIsEmptyNotAnError(t *testing.T) {
	rep := gaugeApp().ContextReport("nope")
	if rep.Used != 0 || rep.Percent != 0 || rep.Categories == nil || rep.Limits == nil {
		t.Errorf("got %+v, want zero use and non-nil slices (the client decodes them as lists)", rep)
	}
}

func TestNoteContextSnap_SeparatesSummaryMemoryAndSkills(t *testing.T) {
	a := gaugeApp()
	hist := []api.Message{
		api.NewTextMessage("system", compactSummaryHeader+"- did X"),
		api.NewTextMessage("user", "hello there my friend"),
	}
	a.convSummaries = map[string]*convSummary{"c1": {coveredCount: 12}}
	mems := []memory.MemoryResult{{Content: string(make([]byte, 300))}} // 100 tokens
	a.noteContextSnap("c1", 10000, hist, mems, 500, 50, 70, 20)

	s := a.ctxState.snaps["c1"]
	if s.summary == 0 || s.summarized != 12 {
		t.Errorf("summary=%d summarized=%d, want a summary counted apart from history, covering 12 messages", s.summary, s.summarized)
	}
	if s.memory != 100 || s.skills != 50 || s.system != 350 {
		t.Errorf("memory=%d skills=%d system=%d, want 100/50/350 (system = 500 − memory − skills)", s.memory, s.skills, s.system)
	}
	if s.tools != 70 || s.current != 20 {
		t.Errorf("tools=%d current=%d", s.tools, s.current)
	}
}

func TestQuotaMeters_OnePerWindowOrTheSingleFigure(t *testing.T) {
	got := quotaMeters(cliproxy.Quota{Remaining: 0.2, Window: "7d", Windows: []cliproxy.QuotaWindow{
		{Label: "5h", Remaining: 0.69, ResetAt: "2026-10-08T01:00:00Z"}, {Label: "7d", Remaining: 0.2},
	}})
	if len(got) != 2 || got[0].Label != "5h" || got[0].RemainingPercent != 69 || got[1].RemainingPercent != 20 {
		t.Errorf("windows: %+v", got)
	}
	one := quotaMeters(cliproxy.Quota{Remaining: 0.456, ResetAt: "x"})
	if len(one) != 1 || one[0].RemainingPercent != 46 || one[0].Label != "" {
		t.Errorf("single figure: %+v", one)
	}
}

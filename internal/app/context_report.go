// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"memo/internal/api"
	"memo/internal/cliproxy"
	"memo/internal/memory"
	"memo/internal/models"
	"memo/internal/truncate"
)

// The context gauge: what the ring at the bottom of the chat and the popover
// behind it show — how full the model's window is, what it is made of, and for
// a Subscriptions model how much of the account's allowance is left.
//
// Two things feed it, both in memory only (nothing here is persisted, and
// nothing is recorded for another chat than the one the turn belonged to):
//
//   - a contextSnap, written by buildMessagesForSession every time it assembles
//     a prompt: the window it budgeted against and an estimate of each part;
//   - a turnUsage, written when a turn finishes: the provider's own prompt and
//     completion token counts when it reported them. Those are the number the
//     vendor will actually count against the window, so they win for the total.

// contextSnap is the make-up of the last prompt built for one chat. Every size
// is a len/3 estimate (truncate.EstimateTokens).
type contextSnap struct {
	window                                                   int
	system, memory, skills, tools, summary, history, current int
	// summarized is how many of the chat's oldest messages the summary stands in
	// for (0 = history went in verbatim).
	summarized int
	provider   string
	model      string
	at         time.Time
}

func (s contextSnap) total() int {
	return s.system + s.memory + s.skills + s.tools + s.summary + s.history + s.current
}

// turnUsage is what the provider reported for the last completed turn of a chat.
type turnUsage struct {
	prompt     int
	completion int
	// real is true when prompt came from the provider's own usage report rather
	// than a word-count estimate.
	real     bool
	provider string
	model    string
	window   int
	at       time.Time
}

// contextState holds the two per-chat maps. A struct of its own so App only
// grows by one field.
type contextState struct {
	mu    sync.Mutex
	snaps map[string]contextSnap
	usage map[string]turnUsage
}

// recordContextSnap remembers how the prompt just built for chatID was made up.
func (a *App) recordContextSnap(chatID string, s contextSnap) {
	if chatID == "" {
		return
	}
	s.at = time.Now()
	a.ctxState.mu.Lock()
	defer a.ctxState.mu.Unlock()
	if a.ctxState.snaps == nil {
		a.ctxState.snaps = map[string]contextSnap{}
	}
	a.ctxState.snaps[chatID] = s
}

// noteTurnUsage remembers what a finished turn cost in context. completion is
// the reply's token count; meta.PromptTokens is the provider's figure when
// meta.PromptReal says so.
func (a *App) noteTurnUsage(chatID string, meta *usageMeta, completion int) {
	if chatID == "" || meta == nil || meta.PromptTokens <= 0 {
		return
	}
	a.ctxState.mu.Lock()
	defer a.ctxState.mu.Unlock()
	if a.ctxState.usage == nil {
		a.ctxState.usage = map[string]turnUsage{}
	}
	snap := a.ctxState.snaps[chatID]
	a.ctxState.usage[chatID] = turnUsage{
		prompt:     meta.PromptTokens,
		completion: completion,
		real:       meta.PromptReal,
		provider:   meta.Provider,
		model:      meta.Model,
		window:     snap.window,
		at:         time.Now(),
	}
}

// forgetContext drops what is remembered for a chat (it was deleted or cleared).
func (a *App) forgetContext(chatID string) {
	a.ctxState.mu.Lock()
	delete(a.ctxState.snaps, chatID)
	delete(a.ctxState.usage, chatID)
	a.ctxState.mu.Unlock()
}

// realContextUsed is the size the conversation reached at the end of the last
// turn — the provider's prompt tokens plus the reply it then wrote, which is
// part of the next prompt — when that turn ran on the same model and window as
// now and the provider reported a real figure. ok is false otherwise: a number
// from another model's tokenizer says nothing about this one.
func (a *App) realContextUsed(chatID, provider, model string, window int) (int, bool) {
	a.ctxState.mu.Lock()
	u, ok := a.ctxState.usage[chatID]
	a.ctxState.mu.Unlock()
	if !ok || !u.real || u.provider != provider || u.model != model || (window > 0 && u.window != window) {
		return 0, false
	}
	return u.prompt + u.completion, true
}

// agentToolTokens estimates the tool schema an agent turn sends alongside the
// messages. It is real context the model's chat template folds into the prompt,
// yet it travels in a separate "tools" field, so no message-based count sees it.
func (a *App) agentToolTokens(chatID string) int {
	if !a.GetAgentEnabled() || a.agentExecutor == nil {
		return 0
	}
	defs := a.agentExecutor.Registry().ToOpenAITools(a.activeSkillSet(chatID))
	if len(defs) == 0 {
		return 0
	}
	raw, err := json.Marshal(defs)
	if err != nil {
		return 0
	}
	return truncate.EstimateTokens(string(raw))
}

// currentContextWindow is the context window of whatever model a turn would run
// on right now: the running local server's real window, else the active API
// provider's budget.
func (a *App) currentContextWindow() int {
	if a.llamaServer != nil && a.llamaServer.IsRunning() {
		return a.localContextWindow()
	}
	return a.apiContextBudget()
}

// localContextWindow is the running llama-server's real window — what
// buildMessagesForSession budgets a local turn against, before it keeps its own
// safety margin below that wall.
func (a *App) localContextWindow() int {
	a.cfgMu.RLock()
	maxLocal := a.cfg.Llama.CtxSize
	maxContextTokens := a.cfg.Llama.MaxContextTokens
	a.cfgMu.RUnlock()
	if maxLocal <= 0 {
		maxLocal = 8192
	}
	// The server was launched with clampContextSize(cfg.CtxSize) — reduced to the
	// model's trained max whenever the configured value was too large. Budget
	// against THAT real window, not the config: with --no-context-shift an
	// over-window request is a hard error, not a silent truncation, and
	// cfg.CtxSize can legitimately exceed it (a global setting kept while a
	// smaller model is loaded).
	if a.llamaServer != nil {
		if realCtx := a.llamaServer.CtxSize(); realCtx > 0 && realCtx < maxLocal {
			maxLocal = realCtx
		}
	}
	if maxContextTokens > 0 && maxContextTokens < maxLocal {
		return maxContextTokens
	}
	return maxLocal
}

// activeModelIdentity names the provider and model a turn would run on now, the
// same pair usageMeta carries.
func (a *App) activeModelIdentity() (providerName, model string) {
	providerName = a.currentProviderLabel()
	if providerName == "local" {
		return providerName, a.localModelName()
	}
	return providerName, a.activeProviderModel(providerName)
}

// compactionSettings reads the auto-compact switch and threshold the way
// maybeCompactHistory applies them, so the gauge and the compaction agree.
func (a *App) compactionSettings() (enabled bool, pct int) {
	a.cfgMu.RLock()
	enabled = a.cfg.AgentMode.ConversationCompactEnabled
	pct = a.cfg.AgentMode.CompactThresholdPct
	a.cfgMu.RUnlock()
	if pct <= 0 || pct > 95 {
		pct = 90
	}
	return enabled, pct
}

// ContextReport builds the gauge for one chat. It never calls a model and never
// waits on the network: the allowance figures come from the quota cache.
func (a *App) ContextReport(chatID string) models.ContextReport {
	providerName, model := a.activeModelIdentity()
	window := a.currentContextWindow()
	enabled, pct := a.compactionSettings()
	rep := models.ContextReport{
		Provider:           providerName,
		Model:              model,
		Window:             window,
		AutoCompactEnabled: enabled,
		AutoCompactPct:     pct,
		Categories:         []models.ContextCategory{},
		Limits:             []models.QuotaMeter{},
	}

	a.ctxState.mu.Lock()
	snap, haveSnap := a.ctxState.snaps[chatID]
	use, haveUse := a.ctxState.usage[chatID]
	a.ctxState.mu.Unlock()

	// A snapshot or usage figure from a different model describes a different
	// window and tokenizer; keep the make-up (it is still what the chat holds)
	// but do not present its real count as this model's.
	sameModel := haveUse && use.provider == providerName && use.model == model
	if haveUse && sameModel && use.real {
		rep.Used = use.prompt + use.completion
		rep.UsedReal = true
	}

	switch {
	case haveSnap:
		est := snap.total()
		rep.SummarizedMessages = snap.summarized
		scale := 1.0
		if rep.UsedReal && est > 0 {
			scale = float64(use.prompt) / float64(est)
			if scale < 0.3 {
				scale = 0.3
			} else if scale > 3 {
				scale = 3
			}
		}
		add := func(key string, tokens int) {
			if tokens > 0 {
				rep.Categories = append(rep.Categories, models.ContextCategory{Key: key, Tokens: int(float64(tokens)*scale + 0.5)})
			}
		}
		add("messages", snap.history)
		add("summary", snap.summary)
		add("system", snap.system)
		add("memory", snap.memory)
		add("skills", snap.skills)
		add("tools", snap.tools)
		add("current", snap.current)
		if !rep.UsedReal {
			rep.Used = est
			if haveUse && use.completion > 0 {
				rep.Used += use.completion
			}
		}
	default:
		// Nothing assembled since Memo started: all there is to go on is what the
		// chat has stored.
		if sm := a.getSessionManager(); sm != nil {
			total := 0
			for _, m := range sm.GetActiveMessagesForSession(chatID) {
				total += truncate.EstimateTokens(m.Content)
			}
			if total > 0 {
				rep.Categories = append(rep.Categories, models.ContextCategory{Key: "messages", Tokens: total})
				rep.Used = total
			}
		}
	}

	if window > 0 {
		rep.Percent = rep.Used * 100 / window
		if rep.Percent > 100 {
			rep.Percent = 100
		}
	}
	rep.Limits, rep.LimitsVendor = a.contextLimits(providerName, model)
	return rep
}

// contextLimits returns the allowance meters of the Subscriptions account behind
// model — every window the vendor reports, or the model's own single figure —
// from the quota cache. Empty for any other provider.
func (a *App) contextLimits(providerName, model string) ([]models.QuotaMeter, string) {
	if providerName != subsProviderName || model == "" {
		return []models.QuotaMeter{}, ""
	}
	owner := a.subsOwnerOf(model)
	q, ok := a.subsQuotas(a.lifecycle(), true, 0).For(model, owner)
	if !ok {
		return []models.QuotaMeter{}, owner
	}
	return quotaMeters(q), owner
}

// subsOwnerOf returns the vendor key (owned_by) of a Subscriptions model, from
// the cached model list.
func (a *App) subsOwnerOf(model string) string {
	for _, md := range a.subsManager().CachedModels() {
		if md.ID == model {
			return md.OwnedBy
		}
	}
	return ""
}

// quotaMeters turns a Quota into the meters the popover lists: one per window
// when the vendor reports several, else the single figure.
func quotaMeters(q cliproxy.Quota) []models.QuotaMeter {
	pct := func(f float64) int {
		p := int(f*100 + 0.5)
		if p < 0 {
			return 0
		}
		if p > 100 {
			return 100
		}
		return p
	}
	if len(q.Windows) > 0 {
		out := make([]models.QuotaMeter, 0, len(q.Windows))
		for _, w := range q.Windows {
			out = append(out, models.QuotaMeter{Label: w.Label, RemainingPercent: pct(w.Remaining), ResetAt: w.ResetAt})
		}
		return out
	}
	return []models.QuotaMeter{{Label: q.Window, RemainingPercent: pct(q.Remaining), ResetAt: q.ResetAt}}
}

// estimateHistoryTokens is the len/3 size of a message list.
func estimateHistoryTokens(msgs []api.Message) int {
	n := 0
	for _, m := range msgs {
		n += truncate.EstimateTokens(m.GetTextContent())
	}
	return n
}

// isCompactSummary reports whether m is the "earlier conversation" summary
// maybeCompactHistory puts at the front of a compacted history.
func isCompactSummary(m api.Message) bool {
	return m.Role == "system" && strings.HasPrefix(m.GetTextContent(), compactSummaryHeader)
}

// noteContextSnap records the make-up of the prompt buildMessagesForSession just
// assembled for chatID. history is what went into the prompt (a compaction
// summary, when there is one, is its first message); systemTokens covers the
// whole system prompt, of which the retrieved memories and the active-skill
// block are shown apart.
func (a *App) noteContextSnap(chatID string, window int, history []api.Message, memories []memory.MemoryResult, systemTokens, skillTokens, toolTokens, userTokens int) {
	s := contextSnap{window: window, tools: toolTokens, current: userTokens, skills: skillTokens}
	s.provider, s.model = a.activeModelIdentity()
	if len(history) > 0 && isCompactSummary(history[0]) {
		s.summary = truncate.EstimateTokens(history[0].GetTextContent())
		s.history = estimateHistoryTokens(history[1:])
		a.convSummaryMu.Lock()
		if cs := a.convSummaries[chatID]; cs != nil {
			s.summarized = cs.coveredCount
		}
		a.convSummaryMu.Unlock()
	} else {
		s.history = estimateHistoryTokens(history)
	}
	for _, m := range memories {
		s.memory += truncate.EstimateTokens(m.Content)
	}
	// The memory block is part of the system prompt, never more than it.
	if s.memory+s.skills > systemTokens {
		s.memory = max(systemTokens-s.skills, 0)
	}
	s.system = max(systemTokens-s.memory-s.skills, 0)
	a.recordContextSnap(chatID, s)
}

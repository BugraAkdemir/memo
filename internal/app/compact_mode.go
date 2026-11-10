package app

import (
	"context"
	"sort"
	"sync"

	"memo/internal/agent"
	"memo/internal/provider"
	"memo/internal/truncate"
)

// compactWindowLimit is the context window (tokens) below which a local model
// runs in the compact profile: a short system prompt with nothing personal in
// it, a reserved share for the reply, smaller memory and skill shares, no
// per-turn calendar or sub-agent blocks, and no extra LLM calls of Memo's own
// (title, fact extraction, history summary). Measured on 2026-11-10: the full
// system prompt alone is ~1.1k tokens and the agent tool schema ~10k, so a
// 4k–8k window had no room left for the conversation or the answer.
//
// Windows at or above the limit, and every cloud model, are untouched.
const compactWindowLimit = 10000

// isCompactWindow reports whether a model with this real context window gets
// the compact profile. 0 means "unknown" and never counts as small.
func isCompactWindow(window int) bool {
	return window > 0 && window < compactWindowLimit
}

// compactReplyReserve is the part of a compact window kept free for the reply.
// llama.max_tokens defaults to 0 (no limit) and the budget's own safety margin
// is only ~5%, so without this a filled 4k prompt left ~256 tokens to answer in.
func compactReplyReserve(window int) int {
	return window / 4
}

// compactMemoryShare and compactSkillShare cap the memory block and the
// active-skill instructions on a compact window (the regular local budget
// gives memory up to 2/5 of the prompt and skills 1/5).
func compactMemoryShare(window int) int { return window / 10 }
func compactSkillShare(window int) int  { return window * 7 / 100 }

// compactWorkingSetTokens caps the agent working-set digest on a compact
// window (the regular cap is agent_mode.working_set_max_tokens, default 600).
const compactWorkingSetTokens = 200

// clipToTokens shortens s to roughly maxTokens (EstimateTokens is len/3).
func clipToTokens(s string, maxTokens int) string {
	if truncate.EstimateTokens(s) <= maxTokens {
		return s
	}
	return truncate.Text(s, maxTokens*3)
}

// localCompactActive reports whether turns right now go to a running local
// model with a compact window and no cloud provider in front of it. Used by
// the work that happens outside buildMessagesForSession — the chat title and
// fact extraction — which would otherwise queue an extra request on the one
// local inference slot, minutes long on a CPU.
func (a *App) localCompactActive() bool {
	if a.llamaServer == nil || !a.llamaServer.IsRunning() || a.GetActiveProvider() != "" {
		return false
	}
	return isCompactWindow(a.localContextWindow())
}

// compactToolShare is the most the tool definitions may take on a compact
// window (load_tools plus the groups it loaded).
func compactToolShare(window int) int { return window / 5 }

// compactTurnWindow is the window of a turn that runs in the compact profile,
// or 0 when the turn is not one: the auto router's local route with a small
// window, or no route at all and a small running local model with no cloud
// provider in front of it (localCompactActive).
func (a *App) compactTurnWindow(ctx context.Context) int {
	if r := subscriptionTextRouteFromCtx(ctx); r != nil {
		if r.auto && routeProvider(r) == "local" && isCompactWindow(routeWindow(r)) {
			return routeWindow(r)
		}
		return 0
	}
	if a.localCompactActive() {
		return a.localContextWindow()
	}
	return 0
}

// compactToolsFor is the on-demand tool state for chatID's agent turn, or nil
// when the turn is not compact. Groups loaded in earlier turns stay loaded.
func (a *App) compactToolsFor(ctx context.Context, chatID string) *agent.CompactTools {
	window := a.compactTurnWindow(ctx)
	if window == 0 {
		return nil
	}
	return agent.NewCompactTools(compactToolShare(window), a.compactGroups.get(chatID), func(g string) {
		a.compactGroups.add(chatID, g)
	})
}

// compactGroupStore is chatID -> loaded tool groups. Lost on restart, which
// costs one load_tools call, nothing more.
type compactGroupStore struct {
	mu     sync.Mutex
	byChat map[string]map[string]bool
}

func (s *compactGroupStore) get(chatID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.byChat[chatID]))
	for g := range s.byChat[chatID] {
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

func (s *compactGroupStore) add(chatID, group string) {
	if chatID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byChat == nil {
		s.byChat = map[string]map[string]bool{}
	}
	if s.byChat[chatID] == nil {
		s.byChat[chatID] = map[string]bool{}
	}
	s.byChat[chatID][group] = true
}

// compactAgentToolTokens is what the tool definitions of chatID's next compact
// agent turn cost: load_tools plus the loaded groups (0 with agent mode off).
// Mirrors agentToolTokens and Pipeline.turnTools so the budget and the gauge
// count what is really sent.
func (a *App) compactAgentToolTokens(ctx context.Context, chatID string) int {
	if !a.GetAgentEnabled() || a.agentExecutor == nil {
		return 0
	}
	c := a.compactToolsFor(ctx, chatID)
	if c == nil {
		return a.agentToolTokens(chatID)
	}
	reg := a.agentExecutor.Registry()
	defs := reg.ToOpenAITools(a.activeSkillSet(chatID))
	if a.resolveCodeMode(chatID) {
		defs = agent.ForCodeMode(defs)
	}
	return agent.DefinitionTokens(c.Filter(defs, func(d provider.ToolDefinition) bool {
		td, ok := reg.Get(d.Function.Name)
		return ok && td.SkillOwner != ""
	}))
}

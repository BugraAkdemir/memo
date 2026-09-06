package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/logx"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memo/internal/api"
	"memo/internal/memory"
	"memo/internal/truncate"
)

// apiContextBudget returns the context-window token budget for the active API
// provider. It follows the model, not the provider type: the user-configured
// ContextTokens for the active provider's model wins; an explicit global
// MaxContextTokens override comes next; otherwise a conservative default.
func (a *App) apiContextBudget() int {
	a.providerMu.RLock()
	active := a.activeProviderName
	a.providerMu.RUnlock()
	return a.contextBudgetFor(active)
}

// contextBudgetFor returns the context-window token budget for a named
// provider, following the model not the type: an explicit global
// MaxContextTokens override wins, then the user-configured per-model
// ContextTokens, then a conservative per-provider fallback. Used by the
// planner/executor mode's context gauge as well as apiContextBudget.
func (a *App) contextBudgetFor(providerName string) int {
	a.cfgMu.RLock()
	maxContextTokens := a.cfg.Llama.MaxContextTokens
	a.cfgMu.RUnlock()
	if maxContextTokens > 0 {
		return maxContextTokens
	}

	if a.providerCfgMgr != nil && providerName != "" {
		for _, p := range a.providerCfgMgr.GetEnabled() {
			if p.Name == providerName && p.ContextTokens > 0 {
				return p.ContextTokens
			}
		}
	}

	switch providerName {
	case "gemini":
		return 1024 * 1024
	case "claude":
		return 200 * 1024
	default:
		return 128 * 1024
	}
}

// buildMemoryQuery enriches the retrieval query with recent conversation
// context (Memory.QueryHistoryTurns prior user turns, default 1) so that
// follow-up questions like "buna ne demiştik?" can still find relevant
// memories when the current message alone is too vague. The old hardcoded 3
// blended four turns into one near-static vector, so consecutive turns
// retrieved almost the same set regardless of what was asked.
func (a *App) buildMemoryQuery(userMsg string) string {
	a.cfgMu.RLock()
	window := a.cfg.Memory.QueryHistoryTurns
	a.cfgMu.RUnlock()
	if window <= 0 {
		return userMsg
	}
	history := a.getSessionHistory()
	var recent []string
	count := 0
	for i := len(history) - 1; i >= 0 && count < window; i-- {
		if history[i].Role != "user" {
			continue
		}
		text := history[i].GetTextContent()
		if len(strings.Fields(text)) > 3 {
			recent = append([]string{text}, recent...)
			count++
		}
	}
	if len(recent) == 0 {
		return userMsg
	}
	return strings.Join(recent, " | ") + " | " + userMsg
}

// buildMessages builds the prompt for whatever chat is active *right now*.
// Thin wrapper around buildMessagesForSession — see PLAN_chatid_refactor.md
// Phase 2. Kept for callers that are intentionally still tied to the global
// active chat (the WhatsApp bridge's own pipeline; non-streaming
// SendMessage/SendMessageWithImage/SendMessageWithFile, out of this phase's
// scope). Streaming entry points in chat.go capture a chatID once up front
// and call buildMessagesForSession directly instead.
func (a *App) buildMessages(ctx context.Context, userMsg string, extraImageB64 []string) []api.Message {
	sm := a.getSessionManager()
	var chatID string
	if sm != nil {
		chatID = sm.GetActiveID()
	}
	return a.buildMessagesForSession(ctx, chatID, userMsg, extraImageB64, nil)
}

// buildMessagesForSession is buildMessages anchored to an explicit chatID —
// history is read from exactly that chat, not whatever the global "active"
// chat happens to be at call time. This is what lets a stream keep reading
// and (via the sessionID already threaded through routeStream/callLLMStream/
// finishStream) writing to the *same* chat throughout, even if the user
// switches the active chat mid-stream (BUG-H1).
//
// retrievedCountOut, if non-nil, is set to how many memories were actually
// retrieved for this turn — lets a caller that cares (sendMessageStreamCore,
// for the "N anı kullanıldı" badge) learn the count without this function's
// return type changing for its other seven call sites, which have no use
// for it.
func (a *App) buildMessagesForSession(ctx context.Context, chatID, userMsg string, extraImageB64 []string, retrievedCountOut *int) []api.Message {
	var memories []memory.MemoryResult
	// A Self-Driving task turn with "task memory" off gets no RAG/memory block
	// at all — the task loop carries its own state, and personal memories are
	// noise (and a privacy surface) for autonomous code work.
	if a.GetMemoryEnabled() && !taskMemoryDisabled(ctx) {
		memories = a.retrieveMemory(ctx, a.buildMemoryQuery(userMsg))
	}
	if retrievedCountOut != nil {
		*retrievedCountOut = len(memories)
	}
	// Read once and reuse below rather than calling GetWebSearchEnabled()/
	// GetAgentEnabled() twice each — a toggle landing between reads could
	// otherwise tell the model one thing via buildCapabilitiesBlock (which
	// only describes what's OFF) than what routeStream actually does with it
	// moments later. Web search itself no longer happens in this function at
	// all — see App.routeStream/callWebSearchAgentStream (chat.go/llm.go):
	// plain (non-agent) chat now gets a scoped web_search tool via native
	// tool-calling in the same completion request, exactly like agent mode's
	// full toolset, instead of this function blindly running a search on
	// every message and injecting the results into the system prompt
	// regardless of whether the message needed one.
	webSearchEnabled := a.GetWebSearchEnabled()
	agentEnabled := a.GetAgentEnabled()
	// Memory formatting is independent of mood — the mood engine must have ZERO
	// influence when disabled.
	// stripAssistant=true: the memory block carries only the user's own past
	// words, not Memo's replies to them. The reply half roughly doubled each
	// conversational memory's token cost and was the thing the "never a
	// template to copy" guardrail existed to police; the durable content is
	// what the user said, which is kept.
	// MinimalMode's promise is "pure model, zero Memo injection". Read through
	// a.identity (the field BuildSystemPrompt just checked), not
	// a.cfg.Identity.MinimalMode — SetMinimalMode's two non-atomic writes
	// could otherwise disagree with this read for the duration of a toggle
	// and produce a half-applied prompt.
	minimal := a.identity.GetMinimalMode()

	systemPrompt := a.identity.BuildSystemPrompt(memories, true, agentEnabled, webSearchEnabled, a.whatsappReachable(), a.telegramReachable())
	if !minimal {
		// Mood is fully opt-in. When the engine is disabled the model is driven
		// solely by the configured system prompt: no directive, no neutral block,
		// no self-interest text is injected.
		if a.mood != nil && a.mood.Enabled() {
			systemPrompt += a.mood.BuildDirective()
			systemPrompt += a.mood.BuildSelfInterestDirective()
		}
		// An active skill is something the user explicitly turned on — but it
		// is still Memo injecting text the bare model wouldn't see, so Minimal
		// Mode strips it too (previously it did not).
		if skillPrompt := a.buildActiveSkillPrompt(); skillPrompt != "" {
			systemPrompt += skillPrompt
		}
	}

	// Volatile per-turn grounding — current time, and the agent working-set
	// digest — kept OUT of systemPrompt. Both change every turn; folded into
	// the front of the prompt (which is what happened when they were part of
	// systemPrompt, since the local-model branch merges systemPrompt into the
	// first history message) they broke the local llama-server's KV-cache
	// prefix match and forced a full re-prefill of the whole conversation on
	// every message — the single biggest reason a long Memo chat crawls
	// compared to raw llama.cpp. They now ride on the *current* user message
	// instead, so everything before it stays byte-identical turn to turn.
	// Skipped entirely under Minimal Mode.
	volatileCtx := ""
	if !minimal {
		volatileCtx = a.timeContextBlockForChat(chatID) + a.renderWorkingSet(chatID)
	}
	effectiveUserMsg := userMsg
	if volatileCtx != "" {
		effectiveUserMsg = userMsg + volatileCtx
	}

	var tokenBudget int
	if a.llamaServer != nil && a.llamaServer.IsRunning() {
		a.cfgMu.RLock()
		maxLocal := a.cfg.Llama.CtxSize
		maxContextTokens := a.cfg.Llama.MaxContextTokens
		a.cfgMu.RUnlock()
		if maxLocal <= 0 {
			maxLocal = 8192
		}
		if maxContextTokens > 0 && maxContextTokens < maxLocal {
			tokenBudget = maxContextTokens
		} else {
			tokenBudget = maxLocal
		}
		// Agent mode sends the tool schema (agent.ToOpenAITools) alongside
		// every request as a separate "tools" field, which the model's chat
		// template folds into the actual prompt it sees — real context-window
		// tokens that this budget must leave room for. Without this, a small
		// local ctx-size (this codebase's own default was 4096 before the
		// bug below) could be exceeded by tool-schema overhead alone even for
		// a one-word message, since nothing here ever accounted for it. Found
		// live: "selam" with agent mode on and a 32B local model produced a
		// ~4800-token request against a 4096 ctx-size — the visible message
		// content was a handful of tokens, the rest was this unbudgeted gap.
		if a.GetAgentEnabled() && a.agentExecutor != nil {
			if toolDefs := a.agentExecutor.Registry().ToOpenAITools(); len(toolDefs) > 0 {
				if raw, err := json.Marshal(toolDefs); err == nil {
					tokenBudget -= truncate.EstimateTokens(string(raw))
				}
			}
		}
		if tokenBudget < 512 {
			tokenBudget = 512
		}
	} else {
		tokenBudget = a.apiContextBudget()
	}

	systemTokens := truncate.EstimateTokens(systemPrompt)
	userTokens := truncate.EstimateTokens(effectiveUserMsg)
	historyBudget := tokenBudget - systemTokens - userTokens
	if historyBudget < 512 {
		historyBudget = 512
	}

	history := a.getSessionHistoryTokenAwareForSession(chatID, historyBudget)
	// Once history fills most of the budget, condense its oldest stretch into
	// one summary message instead of letting drop-oldest quietly lose early
	// turns. No-op below the threshold, and the summary is cached so the
	// extra LLM call only runs when the condensed region actually changes.
	// Skipped under Minimal Mode — it is a Memo-initiated LLM call, exactly
	// the kind of extra work Minimal Mode promises not to do.
	if !minimal {
		history = a.maybeCompactHistory(ctx, chatID, history, tokenBudget)
	}
	history = append([]api.Message{}, history...)
	var msgs []api.Message

	// appendUser adds the current turn's user message (with any images).
	appendUser := func() {
		if len(extraImageB64) > 0 {
			msgs = append(msgs, api.NewMultimodalMessage("user", effectiveUserMsg, extraImageB64...))
		} else {
			msgs = append(msgs, api.NewTextMessage("user", effectiveUserMsg))
		}
	}

	if a.llamaServer != nil && a.llamaServer.IsRunning() {
		if len(history) == 0 {
			combined := effectiveUserMsg
			if systemPrompt != "" {
				combined = systemPrompt + "\n\n" + effectiveUserMsg
			}
			if len(extraImageB64) > 0 {
				msgs = append(msgs, api.NewMultimodalMessage("user", combined, extraImageB64...))
			} else {
				msgs = append(msgs, api.NewTextMessage("user", combined))
			}
		} else {
			// Only rewrite the first history message when there is actually a
			// system prompt to fold in — under Minimal Mode (systemPrompt
			// empty) history stays byte-identical to the previous turn, so the
			// local server reuses its KV cache and only prefills the new text.
			if systemPrompt != "" {
				for i, h := range history {
					if h.Role == "user" {
						history[i] = api.NewTextMessage("user", systemPrompt+"\n\n"+h.GetTextContent())
						break
					}
				}
			}
			msgs = append(msgs, history...)
			appendUser()
		}
	} else {
		if systemPrompt != "" {
			msgs = append(msgs, api.NewTextMessage("system", systemPrompt))
		}
		msgs = append(msgs, history...)
		appendUser()
	}

	histUsed := 0
	for _, h := range history {
		histUsed += truncate.EstimateTokens(h.GetTextContent())
	}
	logx.Printf("CONTEXT: minimal=%v budget=%d system=%d user=%d hist_budget=%d hist_used=%d history_msgs=%d total_msgs=%d",
		minimal, tokenBudget, systemTokens, userTokens, historyBudget, histUsed, len(history), len(msgs))
	return msgs
}

// timeGapMentionThreshold is the minimum conversation silence before the
// time-context block mentions the gap at all. Below this, "last message was
// 4 minutes ago" is pure noise — ordinary same-session back-and-forth, and
// every WhatsApp/Telegram self-chat turn (whose user message is persisted
// seconds before buildMessagesForSession runs on some paths), would all read
// as "just now" and teach the model nothing while spending tokens.
const timeGapMentionThreshold = 15 * time.Minute

// timeContextBlock renders the temporal-grounding block injected into every
// system prompt: the current local time always, plus — only when the chat
// has actually been silent for a while — how long ago its last activity was.
// A pure function of its two arguments so threshold/format behavior is
// unit-testable without an App; timeContextBlockForChat is the thin wiring.
func timeContextBlock(now, lastActivity time.Time) string {
	block := "\n\n[Time context] Current local time: " + now.Format("Monday, 2 January 2006, 15:04") + "."
	if !lastActivity.IsZero() {
		if gap := now.Sub(lastActivity); gap >= timeGapMentionThreshold {
			block += " Last message in this conversation was " + humanizeGap(gap) + " ago."
		}
	}
	// A recalled memory may contain a time/date value from a past turn — it
	// is history, not the current time. This line is always authoritative.
	block += " This is the current time now — if a memory or earlier message mentions a different time or date, that one is stale; use this."
	return block
}

// humanizeGap renders a silence duration roughly the way a person would say
// it. Callers guarantee d >= timeGapMentionThreshold, so minutes is the
// smallest unit it ever emits.
func humanizeGap(d time.Duration) string {
	switch {
	case d >= 48*time.Hour:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	case d >= 24*time.Hour:
		return "a day"
	case d >= 2*time.Hour:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	case d >= time.Hour:
		return "an hour"
	default:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
}

// timeContextBlockForChat is timeContextBlock wired to chatID's real last
// activity. A missing session manager or unknown chatID degrades gracefully
// to current-time-only — a first message in a brand-new chat has no gap to
// mention anyway.
func (a *App) timeContextBlockForChat(chatID string) string {
	var last time.Time
	if sm := a.getSessionManager(); sm != nil && chatID != "" {
		last = sm.LastActivity(chatID)
	}
	return timeContextBlock(time.Now(), last)
}

// buildConversationContext extracts conversation history from messages for orchestra chief.
func buildConversationContext(messages []api.Message, userPrompt string) string {
	var sb strings.Builder

	var prevMsgs []api.Message
	for i := len(messages) - 2; i >= 0; i-- {
		if messages[i].Role == "system" {
			continue
		}
		prevMsgs = append(prevMsgs, messages[i])
	}

	for i := len(prevMsgs) - 1; i >= 0; i-- {
		msg := prevMsgs[i]
		roleLabel := "Kullanıcı"
		if msg.Role == "assistant" {
			roleLabel = "Asistan"
		}
		if text, ok := msg.Content.(string); ok {
			cleanText := stripOrchestraLines(text)
			sb.WriteString(fmt.Sprintf("%s: %s\n", roleLabel, cleanText))
		}
	}

	if sb.Len() > 0 {
		ctx := fmt.Sprintf("Önceki konuşma:\n%s\n---\nYeni mesaj:\nKullanıcı: %s", sb.String(), userPrompt)
		return ctx
	}

	return fmt.Sprintf("Kullanıcı: %s", userPrompt)
}

var orchestraPrefixes = []string{"🎵", "🧙", "🧠", "✅", "❌", "📝", "📋", "🎯", "🤖"}

// stripOrchestraLines removes lines that start with orchestra debug prefixes.
func stripOrchestraLines(text string) string {
	lines := strings.Split(text, "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		skip := false
		for _, prefix := range orchestraPrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				skip = true
				break
			}
		}
		if !skip && strings.Contains(trimmed, "**: ") {
			for _, role := range []string{"planner", "frontend", "backend", "bug_fixer", "reviewer", "security", "devops", "general"} {
				if strings.HasPrefix(trimmed, "**"+role+"**:") {
					skip = true
					break
				}
			}
		}
		if !skip && strings.HasPrefix(trimmed, "Sistem talimatları:") {
			skip = true
		}
		if !skip {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

func (a *App) getSessionHistory() []api.Message {
	return a.getSessionHistoryTokenAware(0)
}

// getSessionHistoryTokenAware returns the active chat's history. Thin
// wrapper around getSessionHistoryTokenAwareForSession — see
// PLAN_chatid_refactor.md Phase 2.
func (a *App) getSessionHistoryTokenAware(tokenBudget int) []api.Message {
	sm := a.getSessionManager()
	if sm == nil {
		return nil
	}
	return a.getSessionHistoryTokenAwareForSession(sm.GetActiveID(), tokenBudget)
}

// getSessionHistoryTokenAwareForSession returns chatID's history, not
// whatever chat happens to be globally active at call time.
func (a *App) getSessionHistoryTokenAwareForSession(chatID string, tokenBudget int) []api.Message {
	sm := a.getSessionManager()
	if sm == nil {
		return nil
	}

	var history []map[string]string
	if tokenBudget > 0 {
		history = sm.GetHistoryForAPITokenAwareForSession(chatID, tokenBudget)
	} else {
		a.cfgMu.RLock()
		maxHistory := a.cfg.Llama.MaxHistory
		a.cfgMu.RUnlock()
		history = sm.GetHistoryForAPIForSession(chatID, maxHistory)
	}

	var msgs []api.Message
	for _, h := range history {
		msgs = append(msgs, api.NewTextMessage(h["role"], h["content"]))
	}
	return msgs
}

func detectMime(path string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	}
	mime := http.DetectContentType(data)
	if mime == "application/octet-stream" {
		return "image/jpeg"
	}
	return mime
}

// downloadFile downloads a GGUF model from HuggingFace synchronously.
func (a *App) downloadFile(repoID, filename, destPath string) error {
	downloadURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repoID, filename)

	req, err := http.NewRequestWithContext(a.lifecycleCtx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	dlClient := &http.Client{Timeout: 10 * time.Minute}
	resp, err := dlClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download status %d: %s", resp.StatusCode, string(body))
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	tmpPath := destPath + ".downloading"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("download write: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpPath, destPath); err != nil {
		if copyErr := copyFile(tmpPath, destPath); copyErr != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("rename and copy fallback both failed: rename: %w, copy: %v", err, copyErr)
		}
		os.Remove(tmpPath)
	}

	logx.Printf("Downloaded %s (%d bytes)", destPath, written)
	return nil
}

// copyFile copies a file from src to dst (cross-device safe).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

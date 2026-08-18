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
	// Explicit global override (Llama/GPU settings) applies to API too.
	a.cfgMu.RLock()
	maxContextTokens := a.cfg.Llama.MaxContextTokens
	a.cfgMu.RUnlock()
	if maxContextTokens > 0 {
		return maxContextTokens
	}

	a.providerMu.RLock()
	active := a.activeProviderName
	a.providerMu.RUnlock()

	// Per-model context window, set by the user when configuring the provider.
	if a.providerCfgMgr != nil {
		for _, p := range a.providerCfgMgr.GetEnabled() {
			if p.Name == active && p.ContextTokens > 0 {
				return p.ContextTokens
			}
		}
	}

	// Fallback when the model's context window hasn't been set yet.
	switch active {
	case "gemini":
		return 1024 * 1024
	case "claude":
		return 200 * 1024
	default:
		return 128 * 1024
	}
}

// buildMemoryQuery enriches the retrieval query with recent conversation context
// (up to 3 prior user turns) so that follow-up questions like "buna ne demiştik?"
// can find relevant memories even when the current message alone is too vague.
func (a *App) buildMemoryQuery(userMsg string) string {
	history := a.getSessionHistory()
	var recent []string
	count := 0
	for i := len(history) - 1; i >= 0 && count < 3; i-- {
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
	if a.GetMemoryEnabled() {
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
	systemPrompt := a.identity.BuildSystemPrompt(memories, false, agentEnabled, webSearchEnabled)
	// MinimalMode means zero injection beyond memory — mood and web search
	// context are both prompt injection just like identity/persona is, so
	// they're skipped here too rather than only in BuildSystemPrompt. Read
	// through a.identity (the same field BuildSystemPrompt just checked
	// above), not a.cfg.Identity.MinimalMode — a separate copy kept in sync
	// by SetMinimalMode's two non-atomic writes, which could disagree with
	// this one for the duration of a toggle and produce a half-applied
	// prompt (identity block skipped but mood/web-search still injected, or
	// vice versa).
	if !a.identity.GetMinimalMode() {
		// Mood is fully opt-in. When the engine is disabled the model is driven
		// solely by the configured system prompt: no directive, no neutral block,
		// no self-interest text is injected.
		if a.mood != nil && a.mood.Enabled() {
			systemPrompt += a.mood.BuildDirective()
			systemPrompt += a.mood.BuildSelfInterestDirective()
		}
	}

	// Deliberately outside the MinimalMode check above: mood/web-search are
	// ambient enhancements Minimal Mode is meant to strip, but an active
	// skill is something the user explicitly turned on, not incidental
	// prompt bloat.
	//
	// Injected here — into systemPrompt itself, before the local/external
	// branch below decides how to fold it into the outgoing messages —
	// rather than by the caller appending it onto a `role: "system"`
	// message afterward (the previous approach, in routeStream/
	// callLLMStream's Orchestra branch). That approach silently dropped
	// every active skill's instructions whenever a.llamaServer was running:
	// the local-model branch just below never emits a `role: "system"`
	// message at all (it merges systemPrompt straight into a user-role
	// message instead, apparently for chat-template compatibility), so a
	// search for `msg.Role == "system"` after the fact found nothing to
	// attach to. Baking it into systemPrompt up front means it rides along
	// no matter which branch below actually uses it.
	if skillPrompt := a.buildActiveSkillPrompt(); skillPrompt != "" {
		systemPrompt += skillPrompt
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
	userTokens := truncate.EstimateTokens(userMsg)
	historyBudget := tokenBudget - systemTokens - userTokens
	if historyBudget < 512 {
		historyBudget = 512
	}

	history := a.getSessionHistoryTokenAwareForSession(chatID, historyBudget)
	history = append([]api.Message{}, history...)
	var msgs []api.Message

	if a.llamaServer != nil && a.llamaServer.IsRunning() {
		if len(history) == 0 {
			combinedMsg := systemPrompt + "\n\n" + userMsg
			if len(extraImageB64) > 0 {
				msgs = append(msgs, api.NewMultimodalMessage("user", combinedMsg, extraImageB64...))
			} else {
				msgs = append(msgs, api.NewTextMessage("user", combinedMsg))
			}
		} else {
			injected := false
			for i, h := range history {
				if !injected && h.Role == "user" {
					content := systemPrompt + "\n\n" + h.GetTextContent()
					history[i] = api.NewTextMessage("user", content)
					injected = true
				}
			}
			msgs = append(msgs, history...)
			if len(extraImageB64) > 0 {
				msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, extraImageB64...))
			} else {
				msgs = append(msgs, api.NewTextMessage("user", userMsg))
			}
		}
	} else {
		msgs = append(msgs, api.NewTextMessage("system", systemPrompt))
		msgs = append(msgs, history...)
		if len(extraImageB64) > 0 {
			msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, extraImageB64...))
		} else {
			msgs = append(msgs, api.NewTextMessage("user", userMsg))
		}
	}

	logx.Printf("CONTEXT: budget=%d system=%d user=%d history=%d history_msgs=%d total_msgs=%d",
		tokenBudget, systemTokens, userTokens, historyBudget, len(history), len(msgs))
	return msgs
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

package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"memo/internal/logx"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"memo/internal/api"
	moodpkg "memo/internal/mood"
)

// forwardStream drains inner into out, preferring delivery of each chunk
// over honoring ctx cancellation. A plain
// `select { case out <- chunk: case <-ctx.Done(): return }` lets Go's
// random tie-breaking between simultaneously-ready cases silently drop a
// chunk — including the final Done:true one — if ctx happens to become
// Done at the exact moment out's reader is also ready to receive it. The
// HTTP handler reading the outer channel then just sees it close with no
// final chunk ever written to the SSE response, so the client never learns
// the stream finished and its "sending" UI state is stuck forever (see the
// matching fix and full explanation on trySend in llm.go). out is always
// created with a generous buffer (128) by every caller of this function, so
// the non-blocking attempt below succeeds immediately in the overwhelming
// majority of real cases, sidestepping the race entirely.
func forwardStream(ctx context.Context, inner <-chan api.StreamChunk, out chan<- api.StreamChunk) {
	for chunk := range inner {
		select {
		case out <- chunk:
			continue
		default:
		}
		select {
		case out <- chunk:
		case <-ctx.Done():
			return
		}
	}
}

// GetIncognito reports whether incognito mode is currently active.
func (a *App) GetIncognito() bool {
	a.incognitoMu.RLock()
	defer a.incognitoMu.RUnlock()
	return a.isIncognito
}

// ToggleIncognito enables or disables incognito mode.
func (a *App) ToggleIncognito(enabled bool) {
	a.incognitoMu.Lock()
	a.isIncognito = enabled
	a.incognitoMessages = nil
	a.incognitoMu.Unlock()
	if enabled {
		logx.Info("Entered Incognito Mode")
	} else {
		logx.Info("Exited Incognito Mode")
	}
}

func (a *App) handleIncognito(userMsg string, b64 string) string {
	a.incognitoMu.Lock()
	if b64 != "" {
		a.incognitoMessages = append(a.incognitoMessages, api.NewMultimodalMessage("user", userMsg, b64))
	} else {
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("user", userMsg))
	}
	msgs := []api.Message{api.NewTextMessage("system", a.cfg.Identity.IncognitoPrompt)}
	msgs = append(msgs, a.incognitoMessages...)
	a.incognitoMu.Unlock()

	reply := a.callLLM(context.Background(), msgs, categoryChat)

	a.incognitoMu.Lock()
	a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("assistant", reply))
	a.incognitoMu.Unlock()
	return reply
}

// drainToReply collapses a stream started via a.streamMu-guarded routing
// (sendMessageStreamCore/routeStream) into a single display-ready reply
// string, for the non-streaming Send* API surface (SendMessage,
// SendMessageWithImage, SendMessageWithFile, WhatsApp self-chat — used by
// POST /api/send and friends).
//
// Only a chunk with an empty FinishReason carries real reply text — every
// other FinishReason ("agent_event", "status", "activity", "usage",
// "memory_used", "stop", and any future one) is a status/metadata marker
// whose Content an SSE consumer renders separately (a badge, a memory-used
// pill, a "searching the web" line — see chat_provider.dart) and is never
// meant to land inside the message body itself.
//
// BUG (found live): this used to allowlist only "agent_event" instead of
// denylisting every non-content marker, so anything added afterwards —
// "status" (web-search indicator, Content: "web_search") and "memory_used"
// (Content: the memory count as a bare number, e.g. "9") in particular —
// silently leaked straight into the reply text. Invisible in the normal
// Flutter chat UI (it reads each chunk's FinishReason itself over SSE and
// never concatenates raw chunks into one string), but the WhatsApp
// self-chat assistant drains through exactly this function — every one of
// its replies was arriving with a stray leading "9", "10", or
// "web_searchweb_search" glued onto the front, reported live.
//
// The first Error chunk wins and is returned as-is (already a
// display-ready "⚠️ ..." string, matching every other error path in this
// file) without draining further, mirroring how callLLM/callLLMStream
// treat a stream error as terminal.
func drainToReply(ch <-chan api.StreamChunk) string {
	var reply strings.Builder
	for chunk := range ch {
		if chunk.Error != "" {
			return chunk.Error
		}
		if chunk.FinishReason != "" {
			continue
		}
		reply.WriteString(chunk.Content)
	}
	return reply.String()
}

// SendMessage sends a plain-text user message and returns the reply
// (non-streaming, used by POST /api/send).
//
// Routes through the same sendMessageStreamCore/routeStream path
// SendMessageStream uses instead of calling callLLM directly, so agent mode
// applies here too. Before this, SendMessage bypassed routeStream entirely —
// no tool definitions, no agent system prompt — so a tool-requiring message
// sent through this endpoint always got a plain, tool-less reply (the model
// just claiming it had no tools) regardless of whether agent mode was on,
// with no error or indication anything was skipped. This is the same bug
// class routeStream's own doc comment already describes being fixed for the
// image/file *streaming* variants; SendMessage/SendMessageWithImage/
// SendMessageWithFile (their non-streaming counterparts) were the
// remaining unfixed instances. sendMessageStreamCore/finishStream now own
// session recording, memory saving, mood updates, observer/intent
// recording, and title generation as side effects of the drain below — the
// manual duplicates of all of those that used to live in this function are
// gone, not just moved.
func (a *App) SendMessage(userMsg string) string {
	logx.Printf(">> SendMessage: %q", userMsg)
	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognito(userMsg, "")
	}

	if !a.streamMu.TryLock() {
		return a.t("⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", "⏳ Please wait until the previous response finishes.")
	}
	defer a.streamMu.Unlock()

	sm := a.getSessionManager()
	var chatID string
	if sm != nil {
		chatID = sm.GetActiveID()
	}

	ch := a.sendMessageStreamCore(context.Background(), chatID, userMsg, false)
	return drainToReply(ch)
}

// SendMessageStream sends a user message and streams the reply token by token.
func (a *App) SendMessageStream(ctx context.Context, userMsg string) <-chan api.StreamChunk {
	logx.Printf(">> SendMessageStream: %q", userMsg)

	// Handle skill commands
	if ch := a.handleSkillCommand(ctx, userMsg); ch != nil {
		return ch
	}

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		if !a.streamMu.TryLock() {
			errCh := make(chan api.StreamChunk, 1)
			errCh <- api.StreamChunk{Error: a.t("⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", "⏳ Please wait until the previous response finishes."), Done: true}
			close(errCh)
			return errCh
		}
		innerCh := a.handleIncognitoStream(ctx, userMsg, "")
		out := make(chan api.StreamChunk, 128)
		go func() {
			defer close(out)
			defer a.streamMu.Unlock()
			defer recoverPanic("forwardStream")
			forwardStream(ctx, innerCh, out)
		}()
		return out
	}

	// The "searching the web" status (FinishReason:"status", Content:
	// "web_search") used to fire here unconditionally whenever the web-search
	// toggle was on, for every message regardless of whether a search would
	// actually happen — the same "looks like it's searching indiscriminately"
	// problem the routeStream/callWebSearchAgentStream redesign (see that
	// function's doc comment) was built to fix. It now fires precisely, from
	// inside callWebSearchAgentStream's tool-execution callback, only when
	// the model actually decides to call web_search — so it's gone from here.
	return a.sendMessageStreamInner(ctx, userMsg)
}

// routeStream applies skill/agent system-prompt injection to messages and
// dispatches to the agent pipeline, agent+orchestra, or a plain LLM stream,
// based on the current agent-mode/provider/local-model state.
//
// Shared by sendMessageStreamInnerTo, SendMessageWithImageStream and
// SendMessageWithFileStream so an image- or file-attached message gets
// identical agent routing to a plain text one — the image/file streams used
// to build their own message list and call callLLMStream directly, so
// agent tools (file read/write, command execution) silently did not run
// for those message types even with agent mode on.
//
// forceAgent activates tool execution for this call regardless of the
// global agent-mode toggle — set when sessionID is itself an agent chat
// (see SendMessageStreamTo's doc comment).
func (a *App) routeStream(ctx context.Context, messages []api.Message, userMsg, imagePath, filePath, sessionID string, forceAgent bool) <-chan api.StreamChunk {
	// Active-skill instructions are baked into systemPrompt by
	// buildMessagesForSession (helpers.go) itself now, before messages was
	// even built — see that call site's comment for why re-injecting here
	// after the fact (the previous approach) silently missed local-model
	// chats. Re-appending here too would double the instructions for every
	// caller that already goes through buildMessagesForSession (all three
	// real callers of routeStream do).

	// Ambient proactive nudging is skipped entirely in Incognito Mode — same
	// "secure session, nothing persisted, nothing recalled" contract that
	// already keeps saveMemoryAsync/updateMoodAsync out of finishStream's
	// incognito branch. A nudge references a learned habit pulled from
	// persisted state, and checkAmbientNudgeOutcome would persist a
	// confidence adjustment back to it — both are exactly the kind of
	// cross-session leakage Incognito Mode promises not to have.
	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()

	// Independent of memory/RAG (see proactive_ambient.go's package doc
	// comment) — fired before the reply so a stale pending suggestion from
	// several turns ago doesn't linger past this one, but backgrounded since
	// it makes its own LLM call and must not add latency to this turn's
	// actual reply. Gated here (not just inside checkAmbientNudgeOutcome)
	// so a disabled/off/MinimalMode/incognito setup doesn't pay for a
	// goroutine spawn and a pending.json read on every single message.
	if !incog && a.ambientNudgingActive() {
		goRecover("checkAmbientNudgeOutcome", func() { a.checkAmbientNudgeOutcome(userMsg) })
	}

	var nudge string
	if !incog {
		nudge = a.buildProactiveNudgeBlock(time.Now())
	}
	if nudge != "" {
		for i, msg := range messages {
			if msg.Role == "system" {
				if content, ok := msg.Content.(string); ok {
					messages[i].Content = content + nudge
				}
				break
			}
		}
	}

	a.agentMu.RLock()
	agentActive := a.agentEnabled
	a.agentMu.RUnlock()
	if forceAgent {
		agentActive = true
	}

	if agentActive {
		for i, msg := range messages {
			if msg.Role == "system" {
				if content, ok := msg.Content.(string); ok {
					messages[i].Content = content + buildAgentSystemPrompt()
				}
				break
			}
		}
	}

	orchestraEnabled := a.orchestraConductor != nil && a.orchestraConductor.Config().Enabled
	localModelRunning := a.llamaServer != nil && a.llamaServer.IsRunning()

	a.providerMu.RLock()
	hasProvider := a.activeProviderName != ""
	a.providerMu.RUnlock()

	if agentActive && (hasProvider || localModelRunning) {
		if orchestraEnabled {
			a.observerRecorder.RecordOrchestraRun(userMsg)
			return a.callAgentWithOrchestra(ctx, messages, userMsg, sessionID)
		}
		a.observerRecorder.RecordAgentRun(userMsg)
		return a.callAgentStream(ctx, messages, userMsg, sessionID)
	}

	// Non-agent "web search mode": route through the same native
	// tool-calling machinery as agent mode, but scoped to just the
	// web_search tool (see callWebSearchAgentStream's doc comment) instead
	// of blindly injecting search results into every message. Excluded when
	// orchestraEnabled — the conductor's own single-completion RunSingle
	// path (callLLMStream's orchestra branch) has no tool-calling support to
	// plug this into; that combination simply gets no web search, same as
	// it would with agent mode off and web search off. Also excluded under
	// MinimalMode: buildMessagesForSession used to gate the old blind
	// injection behind `!a.identity.GetMinimalMode()` (still does, for
	// mood) precisely because Minimal Mode's whole promise is "zero
	// injection beyond memory" — a tool definition riding along in the
	// request is the same category of overhead as the old injected results
	// text, so it must be gated the same way here now that the decision
	// lives in routeStream instead of buildMessagesForSession.
	if a.GetWebSearchEnabled() && !orchestraEnabled && !a.identity.GetMinimalMode() && (hasProvider || localModelRunning) {
		return a.callWebSearchAgentStream(ctx, messages, userMsg, sessionID)
	}
	return a.callLLMStream(ctx, messages, userMsg, imagePath, filePath, sessionID)
}

// sendMessageStreamInner is the core of SendMessageStream: it records the
// message, builds the prompt (including any web-search context), and
// dispatches to the agent, orchestra, or plain LLM stream — always against
// whatever chat is currently active.
func (a *App) sendMessageStreamInner(ctx context.Context, userMsg string) <-chan api.StreamChunk {
	sm := a.getSessionManager()
	var chatID string
	if sm != nil {
		chatID = sm.GetActiveID()
	}
	return a.sendMessageStreamInnerTo(ctx, chatID, userMsg, false)
}

// SendMessageStreamTo is the explicit-chatID counterpart to
// SendMessageStream: it sends userMsg into chatID's own history and streams
// the reply, without reading or touching which chat is globally "active".
//
// This is the public API docs/plans/PLAN_chatid_refactor.md's Faz 3 needed
// so a caller like the task loop (internal/app/tasklist.go) no longer has
// to SwitchChat + force the global agent-mode flag on and back off around
// every call — a pattern that raced a concurrent user-driven chat switch or
// manual agent-mode toggle (a.agentMu is a different lock than
// taskloopRunMu). Tool execution is active for this one call if chatID
// itself is an agent chat (sm.IsAgentChat — true for every task-loop and
// CLI-created chat), regardless of the global agent-mode toggle's current
// state, so no shared flag needs to move at all.
func (a *App) SendMessageStreamTo(ctx context.Context, chatID, userMsg string) <-chan api.StreamChunk {
	logx.Printf(">> SendMessageStreamTo(%s): %q", chatID, userMsg)
	sm := a.getSessionManager()
	if sm == nil || !sm.SessionExists(chatID) {
		errCh := make(chan api.StreamChunk, 1)
		errCh <- api.StreamChunk{Error: fmt.Sprintf(a.t("sohbet bulunamadı: %s", "chat not found: %s"), chatID), Done: true}
		close(errCh)
		return errCh
	}
	return a.sendMessageStreamInnerTo(ctx, chatID, userMsg, sm.IsAgentChat(chatID))
}

// SendMessageStreamToAsAgent mirrors SendMessageStreamTo but always forces
// agent-tool access for this one call, independent of chatID's own
// IsAgentChat status and the global agent-mode toggle (a.agentEnabled).
//
// Used by WhatsApp/Telegram self-chat (handleWhatsAppSelfChatMessage/
// handleTelegramMessage) — those sessions are created via
// sessions.Manager.NewBackgroundChat specifically so they never hijack the
// user's actual active chat (see that method's own doc comment), but
// NewBackgroundChat never sets ProjectPath, so IsAgentChat is always false
// for them and plain SendMessageStreamTo falls back to the *global*
// agentEnabled flag — off by default, and self-chat has no UI toggle for it
// at all short of the /agent on command. Found live: a user asked their
// WhatsApp self-chat to create a recurring routine and got a generic "I
// can't do that yet" reply — the create_routine/list_routines/
// cancel_routine/whatsapp_send/web_search tools all genuinely were not on
// the table for that turn, since agentActive was false in routeStream. The
// intended safety boundary for self-chat's tool access is /auto-perm's
// permission-asking flow (see selfchat_permission.go), not a second,
// easy-to-forget "is agent mode nominally on" gate on top of it — matching
// runAgentRoutine's own forceAgent=true call into sendMessageStreamCore for
// the same underlying reason.
func (a *App) SendMessageStreamToAsAgent(ctx context.Context, chatID, userMsg string) <-chan api.StreamChunk {
	logx.Printf(">> SendMessageStreamToAsAgent(%s): %q", chatID, userMsg)
	sm := a.getSessionManager()
	if sm == nil || !sm.SessionExists(chatID) {
		errCh := make(chan api.StreamChunk, 1)
		errCh <- api.StreamChunk{Error: fmt.Sprintf(a.t("sohbet bulunamadı: %s", "chat not found: %s"), chatID), Done: true}
		close(errCh)
		return errCh
	}
	return a.sendMessageStreamInnerTo(ctx, chatID, userMsg, true)
}

// sendMessageStreamInnerTo is the shared core behind sendMessageStreamInner
// and SendMessageStreamTo. forceAgent activates tool execution for this
// call regardless of the global agent-mode toggle — see SendMessageStreamTo's
// doc comment for why.
func (a *App) sendMessageStreamInnerTo(ctx context.Context, chatID, userMsg string, forceAgent bool) <-chan api.StreamChunk {
	// Prevent concurrent stream goroutines. If a stream is already running,
	// return an error immediately instead of racing two parallel streams that
	// would interleave user/user/assistant/assistant into the session history.
	if !a.streamMu.TryLock() {
		errCh := make(chan api.StreamChunk, 1)
		errCh <- api.StreamChunk{Error: a.t("⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", "⏳ Please wait until the previous response finishes."), Done: true}
		close(errCh)
		return errCh
	}

	innerCh := a.sendMessageStreamCore(ctx, chatID, userMsg, forceAgent)

	// Wrap the inner channel so streamMu is released when the stream completes.
	out := make(chan api.StreamChunk, 128)
	go func() {
		defer close(out)
		defer a.streamMu.Unlock()
		defer recoverPanic("forwardStream")
		forwardStream(ctx, innerCh, out)
	}()
	return out
}

// sendMessageStreamCore does the actual routing/streaming work for chatID.
// Callers must already hold a.streamMu and are responsible for releasing it
// once the returned channel is fully drained (or abandoned) — the two
// existing ways that happens are sendMessageStreamInnerTo's async forwarding
// goroutine above, and runAgentRoutine's synchronous drain (internal/app/
// routine.go), which needs to hold the lock for its whole call so an
// unattended routine's auto-permission scoping can't leak into a concurrent
// interactive stream — see runAgentRoutine's doc comment.
func (a *App) sendMessageStreamCore(ctx context.Context, chatID, userMsg string, forceAgent bool) <-chan api.StreamChunk {
	a.observerRecorder.RecordMessage(userMsg)
	goRecover("processMessageIntent", func() { a.processMessageIntent(userMsg, "chat", "", time.Now()) })

	sm := a.getSessionManager()
	var memUsed int
	messages := a.buildMessagesForSession(ctx, chatID, userMsg, nil, &memUsed)
	if sm != nil {
		sm.AddMessageToSession(chatID, "user", userMsg, "", "")
	}
	// Carried via ctx (rather than adding a parameter to routeStream/
	// callLLMStream/callAgentStream, which ctx already threads through
	// unchanged) so finishStream can pick it up and attach it to the
	// saved assistant reply without every intermediate function needing
	// to know or forward it — see memoryUsedCtxKey's doc comment.
	if memUsed > 0 {
		ctx = context.WithValue(ctx, memoryUsedCtxKey{}, memUsed)
	}

	inner := a.routeStream(ctx, messages, userMsg, "", "", chatID, forceAgent)
	if memUsed <= 0 {
		return inner
	}
	// A live counterpart to the ctx-carried value above: that one only
	// reaches the *persisted* session (read back on the next chat load),
	// same as how a tool-call's agent_event chunks are what let its badge
	// render immediately in the still-open chat rather than only after a
	// reload — same "status chunk ahead of the real content" shape as the
	// web_search indicator in SendMessageStream, just with
	// finishReason=="memory_used" as the discriminator instead.
	out := make(chan api.StreamChunk, 128)
	go func() {
		defer close(out)
		defer recoverPanic("sendMessageStreamCore/memory_used")
		trySend(ctx, out, api.StreamChunk{FinishReason: "memory_used", Content: strconv.Itoa(memUsed)})
		forwardStream(ctx, inner, out)
	}()
	return out
}

// SendMessageWithImageStream sends a user message together with an image file.
func (a *App) SendMessageWithImageStream(ctx context.Context, userMsg string, imagePath string) <-chan api.StreamChunk {
	logx.Printf(">> VisionStream: %q with image %s", userMsg, imagePath)

	if !a.streamMu.TryLock() {
		errCh := make(chan api.StreamChunk, 1)
		errCh <- api.StreamChunk{Error: a.t("⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", "⏳ Please wait until the previous response finishes."), Done: true}
		close(errCh)
		return errCh
	}

	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		a.streamMu.Unlock()
		ch := make(chan api.StreamChunk, 1)
		ch <- api.StreamChunk{Error: "⚠️ Cannot read image: " + err.Error(), Done: true}
		close(ch)
		return ch
	}
	mime := detectMime(imagePath, imgData)
	b64 := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imgData)

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		innerCh := a.handleIncognitoStream(ctx, userMsg, b64)
		out := make(chan api.StreamChunk, 128)
		go func() {
			defer close(out)
			defer a.streamMu.Unlock()
			defer recoverPanic("forwardStream")
			forwardStream(ctx, innerCh, out)
		}()
		return out
	}

	// Captured once, up front — see the matching comment in
	// sendMessageStreamInner (BUG-H1).
	sm := a.getSessionManager()
	var chatID string
	if sm != nil {
		chatID = sm.GetActiveID()
	}

	// buildMessagesForSession (not a hand-rolled system+history+user list) so
	// image messages get the same mood directive, web search context, and
	// token-aware history truncation as plain text ones — the manual
	// construction this replaced skipped all three (BUG-QL5).
	msgs := a.buildMessagesForSession(ctx, chatID, userMsg, []string{b64}, nil)
	if sm != nil {
		sm.AddMessageToSession(chatID, "user", userMsg, imagePath, "")
	}

	innerCh := a.routeStream(ctx, msgs, userMsg, imagePath, "", chatID, false)

	out := make(chan api.StreamChunk, 128)
	go func() {
		defer close(out)
		defer a.streamMu.Unlock()
		defer recoverPanic("forwardStream")
		forwardStream(ctx, innerCh, out)
	}()
	return out
}

// SendMessageWithFileStream attaches a file's content to the message.
func (a *App) SendMessageWithFileStream(ctx context.Context, userMsg string, filePath string) <-chan api.StreamChunk {
	logx.Printf(">> FileStream: %q with %s", userMsg, filePath)

	if !a.streamMu.TryLock() {
		errCh := make(chan api.StreamChunk, 1)
		errCh <- api.StreamChunk{Error: a.t("⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", "⏳ Please wait until the previous response finishes."), Done: true}
		close(errCh)
		return errCh
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		a.streamMu.Unlock()
		ch := make(chan api.StreamChunk, 1)
		ch <- api.StreamChunk{Error: "⚠️ Cannot read file: " + err.Error(), Done: true}
		close(ch)
		return ch
	}

	fileName := filepath.Base(filePath)
	fileContent := string(content)
	if len(fileContent) > 10000 {
		fileContent = fileContent[:10000] + "\n\n... (truncated, file too large)"
	}

	combined := fmt.Sprintf("%s\n\n--- File: %s ---\n%s", userMsg, fileName, fileContent)

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		innerCh := a.handleIncognitoStream(ctx, combined, "")
		out := make(chan api.StreamChunk, 128)
		go func() {
			defer close(out)
			defer a.streamMu.Unlock()
			defer recoverPanic("forwardStream")
			forwardStream(ctx, innerCh, out)
		}()
		return out
	}

	// Captured once, up front — see the matching comment in
	// sendMessageStreamInner (BUG-H1).
	sm := a.getSessionManager()
	var chatID string
	if sm != nil {
		chatID = sm.GetActiveID()
	}

	messages := a.buildMessagesForSession(ctx, chatID, combined, nil, nil)
	if sm != nil {
		sm.AddMessageToSession(chatID, "user", userMsg, "", filePath)
	}

	innerCh := a.routeStream(ctx, messages, userMsg, "", filePath, chatID, false)

	out := make(chan api.StreamChunk, 128)
	go func() {
		defer close(out)
		defer a.streamMu.Unlock()
		defer recoverPanic("forwardStream")
		forwardStream(ctx, innerCh, out)
	}()
	return out
}

func (a *App) handleIncognitoStream(ctx context.Context, userMsg string, b64 string) <-chan api.StreamChunk {
	a.incognitoMu.Lock()
	if b64 != "" {
		a.incognitoMessages = append(a.incognitoMessages, api.NewMultimodalMessage("user", userMsg, b64))
	} else {
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("user", userMsg))
	}
	msgs := []api.Message{api.NewTextMessage("system", a.cfg.Identity.IncognitoPrompt)}
	msgs = append(msgs, a.incognitoMessages...)
	a.incognitoMu.Unlock()

	return a.callLLMStream(ctx, msgs, userMsg, "", "", "")
}

// SendMessageWithImage sends a vision message (non-streaming). Same
// SendMessageStream/routeStream routing as SendMessage — see its doc
// comment for why this replaced a direct callLLM call (this function had
// the identical agent-skipping bug).
func (a *App) SendMessageWithImage(userMsg string, imagePath string) string {
	logx.Printf(">> Vision: %q with image %s", userMsg, imagePath)

	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "⚠️ Cannot read image: " + err.Error()
	}
	mime := detectMime(imagePath, imgData)
	b64 := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imgData)

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognito(userMsg, b64)
	}

	if !a.streamMu.TryLock() {
		return a.t("⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", "⏳ Please wait until the previous response finishes.")
	}
	defer a.streamMu.Unlock()

	sm := a.getSessionManager()
	var chatID string
	if sm != nil {
		chatID = sm.GetActiveID()
	}

	msgs := a.buildMessagesForSession(context.Background(), chatID, userMsg, []string{b64}, nil)
	if sm != nil {
		sm.AddMessageToSession(chatID, "user", userMsg, imagePath, "")
	}

	ch := a.routeStream(context.Background(), msgs, userMsg, imagePath, "", chatID, false)
	reply := drainToReply(ch)

	// Cosmetic only: finishStream (inside the drain above) already recorded
	// the raw reply to session history/memory before this substitution runs,
	// same trade-off the streaming vision path (SendMessageWithImageStream)
	// already has — it never had this substitution at all.
	if strings.Contains(reply, "image input is not supported") || strings.Contains(reply, "mmproj") {
		reply = a.t("⚠️ Bu model görsel/resim desteklemiyor. Resim gönderebilmek için vision destekli bir model kullanmalısınız (örn: LLaVA, BakLLaVA, Llama Vision gibi).", "⚠️ This model does not support image input. Use a vision-capable model to send images (e.g. LLaVA, BakLLaVA, Llama Vision).")
	}
	return reply
}

// SendMessageWithFile sends a file-attached message (non-streaming). Same
// SendMessageStream/routeStream routing as SendMessage — see its doc
// comment for why this replaced a direct callLLM call (this function had
// the identical agent-skipping bug).
func (a *App) SendMessageWithFile(userMsg string, filePath string) string {
	logx.Printf(">> File: %q with %s", userMsg, filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "⚠️ Cannot read file: " + err.Error()
	}

	fileName := filepath.Base(filePath)
	fileContent := string(content)

	if len(fileContent) > 10000 {
		fileContent = fileContent[:10000] + "\n\n... (truncated, file too large)"
	}

	combined := fmt.Sprintf("%s\n\n--- File: %s ---\n%s", userMsg, fileName, fileContent)

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognito(combined, "")
	}

	if !a.streamMu.TryLock() {
		return a.t("⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", "⏳ Please wait until the previous response finishes.")
	}
	defer a.streamMu.Unlock()

	sm := a.getSessionManager()
	var chatID string
	if sm != nil {
		chatID = sm.GetActiveID()
	}

	messages := a.buildMessagesForSession(context.Background(), chatID, combined, nil, nil)
	if sm != nil {
		sm.AddMessageToSession(chatID, "user", userMsg, "", filePath)
	}

	ch := a.routeStream(context.Background(), messages, userMsg, "", filePath, chatID, false)
	return drainToReply(ch)
}

// updateMoodAsync duygu skorunu arka planda asenkron günceller.
func (a *App) updateMoodAsync(userMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	scorer := moodpkg.NewScorer(func(ctx context.Context, sys, user string) (string, error) {
		msgs := []api.Message{
			api.NewTextMessage("system", sys),
			api.NewTextMessage("user", user),
		}
		return a.callLLM(ctx, msgs, categoryMood), nil
	})

	iAnlik := scorer.Score(ctx, userMsg)
	if err := a.mood.Update(ctx, iAnlik); err != nil {
		logx.Printf("mood.Update: %v", err)
	}
}

// buildAgentSystemPrompt returns agent-specific instructions that guide the
// model to behave as an efficient coding agent with proper tool usage.
func buildAgentSystemPrompt() string {
	return `

## Çalışma Modu: Kodlama Ajanı

Şu anda dosya okuma/yazma/düzenleme ve terminal komutu çalıştırma yetkisine sahip bir
kodlama ajanı olarak çalışıyorsun. Aşağıdaki kurallara uy:

### Çalışma Sırası
1. **Önce oku, sonra yaz.** Mevcut dosyaları ve proje yapısını anlamadan kod yazma.
2. **Plan yap.** Neyi nasıl yapacağını adım adım düşün, sonra uygula.
3. **Kullanıcı ne dediyse onu kullan.** "Go ile yap" dediyse Go, "Python" dediyse Python.
   Kendi kafana göre dil/framework değiştirme.
4. **Her adımda test et.** Kod yazdıktan sonra derle/çalıştır, hata varsa düzelt.
5. **İşin bitince özet geç.** Ne yaptığını, nereye yazdığını kısaca söyle.

### Verimlilik
- Gereksiz araç çağrısı yapma. Bir kere okumak yeterliyse 5 kere okuma.
- Aynı dosyayı art arda okumak yerine içeriği aklında tut.
- Uzun işleri parçalara böl, her parçayı tamamla, sonra devam et.
- Bütün araçları aynı anda kullanman gerekmiyor — sadece ihtiyacın olanı kullan.

### Araç Çağırma Biçimi
- Bir araç çağırmak istediğinde SADECE sana verilen gerçek tool-calling mekanizmasını kullan.
  Asla "<function_calls>", "<invoke>" gibi metin/XML etiketleri kendi cevabına yazma — bunlar
  gerçek bir çağrı OLUŞTURMAZ, sadece kullanıcıya anlamsız, çalışmayan metin olarak görünür.
  Gerçek bir araç kullanmak istiyorsan platformun sana sağladığı yapılandırılmış çağrı
  mekanizmasını kullan; emin değilsen araç çağırma, sadece düz metinle cevap ver.

### Hafıza Hakkında
- Hafızan zaten bu sistem promptunun içinde, kullanıcıyla ilgili geçmiş konuşmalardan
  derlenmiş düz metin olarak SANA VERİLMİŞ durumda ("relevant memories" başlıklı bölüm).
  Kullanıcı hakkında bir şey (isim, doğum günü, tercih, vs.) hatırlayıp hatırlamadığını
  kontrol etmek için ASLA read_file/list_directory/search gibi bir dosya aracı çağırma —
  "memory.json" veya benzeri bir hafıza dosyası diskte YOK (gerçek hafıza SQLite veritabanında
  tutuluyor, senin erişimin olmayan bir yerde). O bölümde bir bilgi geçmiyorsa hatırlamıyorsundur,
  bunu kontrol etmek için dosya sistemine bakmana gerek yok.
- Aynı şekilde, hatırlaman gerektiğinde HİÇBİR ZAMAN kendi başına bir dosyaya (ör. "memory.json"
  gibi uydurma bir dosya) yazarak "hatırlamaya" çalışma. Hafıza kaydı tamamen otomatik ve arka
  planda hallediliyor — konuşmadan sonra sistem, kalıcı bilgileri kendisi tespit edip kaydediyor.
  Senin bu konuda hiçbir şey yapmana gerek yok, dosya okuma/yazma araçlarını hafıza amacıyla
  asla kullanma.

### Takvim Hakkında
- Kullanıcı "takvimimde ne var", "bu hafta ne yapacağım" gibi bir şey sorduğunda hafızandan/RAG'dan
  TAHMİN ETME — gerçek, kaydedilmiş etkinlikleri okumak için get_calendar_events aracını çağır.
  Bu, gerçekten kaydedilmiş bir etkinlik ile sadece sohbette bahsedilmiş ama hiç kaydedilmemiş bir
  şeyi birbirine karıştırmanı engeller.`
}

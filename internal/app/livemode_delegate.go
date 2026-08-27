package app

import (
	"context"
	"fmt"
	"os"

	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/logx"
	"memo/internal/truncate"
)

// getOrCreateLiveModeChat returns the dedicated background chat Live Mode
// delegation runs in — created once (via sessions.Manager.NewBackgroundChat,
// the same mechanism WhatsApp/Telegram self-chat already use) and reused
// for the process's lifetime. Never the user's interactive chat: a
// delegated task's history must not interleave with whatever the user is
// actively looking at.
//
// A single scalar (not a map keyed by session/device) is enough for v1 —
// this assumes at most one Live Mode session is meaningfully active at a
// time, reasonable for a single-user desktop app. See
// docs/plans/PLAN_live_mode_v2.md §8's open-question note: if Memo's
// remote-access surface ever needs concurrent independent Live Mode
// sessions from different devices, this needs to become a map before that
// lands, not after.
func (a *App) getOrCreateLiveModeChat() (string, error) {
	a.liveModeChatMu.Lock()
	defer a.liveModeChatMu.Unlock()

	sm := a.getSessionManager()
	if sm == nil {
		return "", fmt.Errorf("sessions not initialized")
	}
	if a.liveModeChatID != "" && sm.SessionExists(a.liveModeChatID) {
		return a.liveModeChatID, nil
	}
	a.liveModeChatID = sm.NewBackgroundChat("Live Mode")
	if a.liveModeChatID == "" {
		return "", fmt.Errorf("could not create Live Mode background chat")
	}
	// Give the Live Mode chat a real working directory: the user's home.
	// Without this the agent (delegated or standalone) starts in the
	// backend's own cwd (the repo when run from source), so a hands-free
	// voice request like "list the files on my Desktop" or "create a note
	// on my Desktop" either lands in the wrong place or forces a
	// change_directory dance mid-conversation. Rooted at home so anything
	// under it — ~/Desktop, ~/Documents, … — is reachable with a plain
	// relative path and no change_directory at all; change_directory still
	// works to step outside it. A normal agent chat gets its ProjectPath
	// from the folder the user explicitly picked; Live Mode has no such
	// picker, and home is the least-surprising default.
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if err := sm.SetProjectPath(a.liveModeChatID, home); err != nil {
			logx.Printf("livemode: could not set Live Mode chat working dir to %q: %v", home, err)
		}
	}
	return a.liveModeChatID, nil
}

// startLiveJob/finishLiveJob mirror startCLIJob/finishCLIJob
// (internal/app/cli_stream.go) exactly: per-chat exclusivity via a map of
// cancel funcs, never the global a.streamMu — a live-voice delegation task
// must not block (or be blocked by) anything else the user is doing in a
// normal chat tab. Since Live Mode currently has one dedicated chat (see
// getOrCreateLiveModeChat), this map in practice only ever has zero or one
// entry, but is keyed by chatID anyway for the same reason CLI jobs are:
// it becomes correct automatically if getOrCreateLiveModeChat ever needs
// to become session-keyed.
func (a *App) startLiveJob(chatID string, cancel context.CancelFunc) bool {
	a.liveJobsMu.Lock()
	defer a.liveJobsMu.Unlock()
	if a.liveJobs == nil {
		a.liveJobs = make(map[string]context.CancelFunc)
	}
	if _, running := a.liveJobs[chatID]; running {
		return false
	}
	a.liveJobs[chatID] = cancel
	return true
}

func (a *App) finishLiveJob(chatID string) {
	a.liveJobsMu.Lock()
	defer a.liveJobsMu.Unlock()
	delete(a.liveJobs, chatID)
}

// SendLiveDelegatedMessageStream sends a Live Mode delegation instruction
// (from the live model — Google Live/OpenAI Realtime's own reasoning,
// deciding a delegate_to_main_model call is warranted) to Memo's ana model
// and streams the reply back. See docs/plans/PLAN_live_mode_v2.md §4.
//
// Combines SendMessageStreamToAsAgent's tool-execution behavior
// (forceAgent=true, the exact same routeStream/agent.Executor pipeline
// every normal agent-mode chat uses — calling a.sendMessageStreamCore
// directly, the same shared core sendMessageStreamInnerTo/
// SendCLIMessageStream/runAgentRoutine all route through) with
// SendCLIMessageStream's concurrency model: per-job exclusivity via
// a.liveJobsMu/a.liveJobs, deliberately NOT a.streamMu. Two reasons that
// lock's purpose doesn't apply here: (a) this always targets its own
// dedicated background chat (getOrCreateLiveModeChat), never the chat the
// interactive user is looking at, so there is no session-history
// interleaving risk a.streamMu protects against; (b) callers resolve
// permission_request events per-request-ID via HandleAgentPermission (see
// drainLiveDelegatedReply) rather than touching a.agentExecutor's shared
// bypassPermissions/autoPermission fields the way runAgentRoutine's
// AutoApproveTools path does — which is exactly why runAgentRoutine still
// needs to hold a.streamMu for its whole call (its own doc comment,
// internal/app/routine.go, spells this out) and this does not.
//
// sessionCtx should be the live realtime session's own context (created
// when its WebSocket connects, cancelled when it closes) — NOT
// a.lifecycleCtx the way SendCLIMessageStream's CLI jobs deliberately are.
// A CLI task is meant to keep running after its triggering request ends,
// surviving the user closing the tab; a delegated task should outlive the
// one HTTP/WS read that triggered it but still die with the live session
// itself, not the whole app process. Do not "fix" this back to
// a.lifecycleCtx by analogy with the CLI precedent — it is a deliberate
// difference, not an oversight.
func (a *App) SendLiveDelegatedMessageStream(sessionCtx context.Context, instruction string) <-chan api.StreamChunk {
	chatID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		return errStreamChunk(err.Error())
	}

	jobCtx, cancel := context.WithCancel(sessionCtx)
	if !a.startLiveJob(chatID, cancel) {
		cancel()
		return errStreamChunk(a.t("⏳ Live Mode için zaten bir görev çalışıyor.", "⏳ A Live Mode task is already running."))
	}

	logx.Printf("livemode delegate: START chat=%s instruction=%q", chatID, truncate.Text(instruction, 200))
	outCh := make(chan api.StreamChunk, 128)
	go func() {
		defer close(outCh)
		defer a.finishLiveJob(chatID)
		defer cancel()
		defer recoverStreamPanic(jobCtx, outCh, "SendLiveDelegatedMessageStream")

		inner := a.sendMessageStreamCore(jobCtx, chatID, instruction, true /* forceAgent */)
		forwardStream(jobCtx, inner, outCh)
		logx.Printf("livemode delegate: DONE chat=%s", chatID)
	}()
	return outCh
}

// drainLiveDelegatedReply drains a delegated task's stream down to its
// final reply text, resolving any permission_request events along the way
// — the Live-Mode-facing name for exactly the same drain loop
// drainSelfChatReply (selfchat_permission.go) already implements. That
// function's signature (channel, autoApprove, buildQuestion/sendQuestion/
// awaitAnswer callbacks) is already fully provider-agnostic despite its
// name — WhatsApp/Telegram self-chat is just its first caller — so this is
// a thin, honestly-named wrapper rather than a duplicated copy of the same
// loop. Callers (Phase 10's client tool-call handlers) supply
// autoApprove=true when LiveModeConfig.AgentPermissionPolicy is
// "auto_allow_once"; the real "ask out loud through the open realtime
// session" buildQuestion/sendQuestion/awaitAnswer implementations for
// "voice_prompt" land in Phase 12 — until then, a caller not yet wired for
// voice prompting should pass autoApprove=false with callbacks that deny
// safely (matching drainSelfChatReply's own "sendQuestion failure ->
// DenyOnce, no hang" behavior when sendQuestion itself errors).
func (a *App) drainLiveDelegatedReply(
	ch <-chan api.StreamChunk,
	autoApprove bool,
	buildQuestion func(ev agent.AgentEvent) string,
	sendQuestion func(text string) error,
	awaitAnswer func(ctx context.Context) (string, bool),
) string {
	return a.drainSelfChatReply(ch, autoApprove, buildQuestion, sendQuestion, awaitAnswer)
}

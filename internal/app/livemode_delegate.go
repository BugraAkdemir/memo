package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/logx"
	"memo/internal/truncate"
)

const (
	// liveDelegateTimeout is how long runDelegate blocks the realtime voice
	// turn on a delegated agent turn before it stops waiting and hands the
	// live model a stall ("still working…"). It does NOT cancel the agent
	// turn — that keeps running and its result is injected into the session
	// when it lands (see runDelegate's background goroutine). 20s clears a
	// normal web_search + a fetch or two + synthesis (~14-18s observed) so
	// the common case never even hits the stall, while still capping how
	// long the user waits in silence if a turn genuinely drags (a
	// fetch_page falling through to a headless browser render of a
	// scraper-hostile page — reddit was measured at ~23s+).
	liveDelegateTimeout = 20 * time.Second

	// liveDelegateTimeoutMarker is a sentinel chunk SendLiveDelegatedMessage
	// Stream forwards into the stream when liveDelegateTimeout elapses while
	// the agent turn is still running. runDelegate (livemode_session.go)
	// treats it as "stop blocking on me, pick the finished result up in the
	// background". NUL-wrapped so it can never collide with real reply text,
	// and so it is obviously wrong (not speakable) if it ever leaks.
	liveDelegateTimeoutMarker = "\x00livemode-delegate-timeout\x00"

	// liveDelegateNudgeInterval is how often runDelegate re-reminds the
	// realtime model, mid-delegation, to make a brief natural sound instead
	// of going silent (see nudgeLiveModeCompany). The single instruction
	// injected at delegation start fades from the model's attention over a
	// 10-20s wait. Kept sparse on purpose so the model doesn't start
	// launching full replies over the top of itself.
	liveDelegateNudgeInterval = 7 * time.Second
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
//
// timeout is how long the caller (runDelegate) will block a realtime voice
// turn on this before it gets a "still working" signal. When it elapses
// while the agent turn is still running, a single liveDelegateTimeoutMarker
// chunk is forwarded into the stream and forwarding then CONTINUES to
// completion — the agent turn is never cancelled here, so its real result
// still arrives on this channel for runDelegate's background goroutine to
// pick up and inject into the session. A non-positive timeout never emits
// the marker (used by tests that drive the inner stream directly). The
// deferred cancel still fires when the turn finishes on its own or the
// session context dies.
func (a *App) SendLiveDelegatedMessageStream(sessionCtx context.Context, instruction string, timeout time.Duration) <-chan api.StreamChunk {
	chatID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		return errStreamChunk(err.Error())
	}

	jobCtx, cancel := context.WithCancel(sessionCtx)
	if !a.startLiveJob(chatID, cancel) {
		cancel()
		// A delegation is already in flight for the Live Mode chat — the
		// realtime model (Gemini Live especially) fires delegate_to_main_model
		// again while the first call is still running. Do NOT surface this as
		// an error the model reads out loud: it was paraphrasing the old
		// "⏳ bir görev çalışıyor" string to the user as "there's a background
		// process, please wait / I'm trying to stop it", which is confusing
		// (the user started nothing) and makes the feature feel stuck. Return
		// a plain instruction instead — the in-flight call will deliver the
		// real answer (directly or via runDelegate's background inject).
		ch := make(chan api.StreamChunk, 1)
		ch <- api.StreamChunk{Content: a.t(
			"Bu istek için ana modele zaten bir devir işlemi sürüyor ve birazdan yanıtlayacak. Yeni bir devir BAŞLATMA. Kullanıcıya en fazla kısacık \"hallediyorum\" de ve sonucu bekle. \"arka planda bir işlem/görev var\", \"başka bir şey çalışıyor\", \"beklemedeyim\", \"durdurmaya çalışıyorum\" gibi ifadeleri ASLA kullanma — bunlar iç detay, kullanıcının başlattığı bir işlem yok.",
			"A delegation to the main model for this is already in progress and will answer shortly. Do NOT start another one. Tell the user at most a tiny \"on it\" and wait for the result. NEVER say things like \"a background task/process is running\", \"something else is working\", \"please hold\", or \"trying to stop it\" — those are internal details; the user started no such process.",
		)}
		close(ch)
		return ch
	}

	logx.Printf("livemode delegate: START chat=%s timeout=%s instruction=%q", chatID, timeout, truncate.Text(instruction, 200))
	outCh := make(chan api.StreamChunk, 128)
	go func() {
		defer close(outCh)
		defer a.finishLiveJob(chatID)
		defer cancel()
		defer recoverStreamPanic(jobCtx, outCh, "SendLiveDelegatedMessageStream")

		inner := a.sendMessageStreamCore(jobCtx, chatID, instruction, true /* forceAgent */)
		slow := forwardWithSlowMarker(jobCtx, inner, outCh, timeout)
		if slow {
			logx.Printf("livemode delegate: DONE (ran past %s, result went to background inject) chat=%s", timeout, chatID)
		} else {
			logx.Printf("livemode delegate: DONE chat=%s", chatID)
		}
	}()
	return outCh
}

// forwardWithSlowMarker forwards inner→out to completion. If timeout elapses
// before inner closes, it forwards one liveDelegateTimeoutMarker chunk into
// out and keeps forwarding the rest — it does not stop or cancel anything.
// Returns whether the marker was emitted. ctx cancellation ends it early.
// The forwarding runs on its own goroutine writing to an internal relay so
// out has exactly one writer (this function), letting the caller close out
// safely; the relay forwarder unwinds once ctx is cancelled or inner ends.
// A non-positive timeout never emits the marker.
func forwardWithSlowMarker(ctx context.Context, inner <-chan api.StreamChunk, out chan<- api.StreamChunk, timeout time.Duration) (markerSent bool) {
	relay := make(chan api.StreamChunk, 128)
	go func() {
		forwardStream(ctx, inner, relay)
		close(relay)
	}()

	var timer <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		timer = t.C
	}

	for {
		select {
		case <-ctx.Done():
			return markerSent
		case chunk, ok := <-relay:
			if !ok {
				return markerSent
			}
			select {
			case out <- chunk:
			case <-ctx.Done():
				return markerSent
			}
		case <-timer:
			timer = nil // one-shot
			select {
			case out <- api.StreamChunk{Content: liveDelegateTimeoutMarker}:
				markerSent = true
			case <-ctx.Done():
				return markerSent
			}
		}
	}
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

// drainLiveDelegatedReplyUntilMarker drains ch like drainLiveDelegatedReply
// but stops the moment a liveDelegateTimeoutMarker chunk arrives, returning
// markerHit=true with whatever text preceded it and leaving the rest of ch
// unread — runDelegate then hands the live model a stall and finishes the
// drain (for the real result) on a background goroutine. Text and
// permission events that arrive before the marker are handled exactly as in
// the full drain. An Error chunk still short-circuits (markerHit=false).
func (a *App) drainLiveDelegatedReplyUntilMarker(
	ch <-chan api.StreamChunk,
	autoApprove bool,
	buildQuestion func(ev agent.AgentEvent) string,
	sendQuestion func(text string) error,
	awaitAnswer func(ctx context.Context) (string, bool),
) (reply string, markerHit bool) {
	var b strings.Builder
	for chunk := range ch {
		if chunk.Error != "" {
			return chunk.Error, false
		}
		if chunk.FinishReason == "agent_event" {
			var ev agent.AgentEvent
			if err := json.Unmarshal([]byte(chunk.Content), &ev); err == nil && ev.Type == agent.EventPermissionRequest {
				a.resolveSelfChatPermission(ev, autoApprove, buildQuestion, sendQuestion, awaitAnswer)
			}
			continue
		}
		if chunk.FinishReason != "" {
			continue
		}
		if i := strings.Index(chunk.Content, liveDelegateTimeoutMarker); i >= 0 {
			b.WriteString(chunk.Content[:i])
			return b.String(), true
		}
		b.WriteString(chunk.Content)
	}
	return b.String(), false
}

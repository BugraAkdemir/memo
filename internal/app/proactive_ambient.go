// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"memo/internal/api"
	"memo/internal/logx"
	"memo/internal/observer"
	"memo/internal/proactive"
)

// Ambient proactive nudges: instead of only the background engine's separate
// timer tick + banner (proactive.Engine, internal/proactive/engine.go), the
// main chat model itself can be told — inline, in its own system prompt —
// that a learned habit might be worth mentioning right now, and left to
// decide whether it fits naturally into its own reply. No extra LLM call is
// spent deciding whether to nudge (that part is pure local math, reusing
// proactive.Match against the same patterns the background engine reads);
// the only LLM calls this adds are two narrow, single-purpose checks
// (checkAmbientNudgeSurfaced, checkAmbientNudgeOutcome below) that only ever
// fire when a pattern actually matched "now", not on every message — the
// same cost shape as extractAndPinFacts (internal/app/memory.go).
//
// Deliberately independent of internal/memory/*: this does not hook into
// saveMemorySync/memorySaveWorker, which are gated behind
// cfg.Memory.MemoryEnabled — proactive learning has its own on/off switch
// and must keep working (or not) regardless of whether RAG memory is on.

const (
	ambientNudgeMinConfidence = 0.3
	ambientNudgeMinScore      = 0.15
)

// ambientNudgingActive is the single shared gate all three entry points
// below must pass — MinimalMode's contract ("zero extra tokens/prompt
// injection reach the model") applies to *offering* a nudge AND to
// *resolving* one, not just the system-prompt addition, so
// checkAmbientNudgeSurfaced/checkAmbientNudgeOutcome check this too, not
// only buildProactiveNudgeBlock. A single shared function (rather than each
// site repeating its own subset of these conditions) is what keeps that
// true as the checks evolve.
func (a *App) ambientNudgingActive() bool {
	if a.cfg == nil || !a.cfg.Proactive.Enabled {
		return false
	}
	if a.identity == nil || a.identity.GetMinimalMode() {
		return false
	}
	return a.proactiveLevel() != proactive.LevelOff
}

// buildProactiveNudgeBlock returns a system-prompt addendum inviting the
// model to casually bring up a matching learned habit, or "" if nothing
// currently matches (or ambient nudging isn't applicable right now). It does
// no I/O beyond the already-loaded pattern file — safe to call synchronously
// while assembling a turn's messages.
//
// On a match, it also records the matched pattern (proactiveNudgeMu-
// guarded) so finishStream can later hand it to checkAmbientNudgeSurfaced to
// confirm whether the model actually used it; on no match, it clears that
// state, so a later turn's check never mistakes a stale value for "this
// turn offered a nudge". The pattern's full value is stored, not just its
// ID — checkAmbientNudgeSurfaced used to re-Load() the pattern file and
// scan for the ID, which was both wasted I/O and a genuine bug: if the
// pattern got rewritten (e.g. by the analyzer's periodic re-save) between
// this call and that later lookup, it silently found nothing and dropped
// the surfaced-check with no log. Passing the value itself makes that
// impossible.
func (a *App) buildProactiveNudgeBlock(now time.Time) string {
	if !a.ambientNudgingActive() {
		return ""
	}
	if a.observerPatterns == nil || a.proactivePending == nil {
		return ""
	}
	// One outstanding suggestion (banner or ambient) at a time, exactly like
	// the background engine's own tick — see Engine.tickAt's doc comment.
	if a.proactivePending.HasPending() {
		a.clearNudgedPattern()
		return ""
	}

	actives, err := a.observerPatterns.LoadActive(now, ambientNudgeMinConfidence)
	if err != nil || len(actives) == 0 {
		a.clearNudgedPattern()
		return ""
	}

	var best observer.TimePattern
	bestScore := 0.0
	for _, p := range actives {
		if score := proactive.Match(p, now); score > bestScore {
			bestScore = score
			best = p
		}
	}
	if bestScore <= ambientNudgeMinScore {
		a.clearNudgedPattern()
		return ""
	}

	a.proactiveNudgeMu.Lock()
	a.lastNudgedPattern = &best
	a.proactiveNudgeMu.Unlock()

	return "\n\nThe user has a learned habit around this time of day: \"" + best.ActivityType +
		"\" (confidence " + formatConfidence(best.Confidence) + "). If — and only if — it fits naturally " +
		"into your reply, you may casually mention or ask about it once, the way a friend would " +
		"(\"hey, isn't it about that time — want to get started?\"). Don't force it, don't repeat it if " +
		"you already brought it up recently, and don't let it derail what the user is actually asking about."
}

func (a *App) clearNudgedPattern() {
	a.proactiveNudgeMu.Lock()
	a.lastNudgedPattern = nil
	a.proactiveNudgeMu.Unlock()
}

// takeNudgedPattern atomically reads and clears the pattern (if any) offered
// this turn. Called synchronously from finishStream, before it spawns
// checkAmbientNudgeSurfaced as a goroutine — the capture must happen on
// finishStream's own call path, not inside the backgrounded goroutine,
// because the app's single active-chat/streamMu serialization only
// guarantees ordering between one turn's finishStream and the *next* turn's
// routeStream, not between a spawned-but-not-yet-scheduled goroutine and
// that next turn. Reading late (inside the goroutine) let a fast-enough next
// turn's buildProactiveNudgeBlock overwrite or clear the field first,
// misattributing one turn's reply to a different turn's pattern.
func (a *App) takeNudgedPattern() *observer.TimePattern {
	a.proactiveNudgeMu.Lock()
	defer a.proactiveNudgeMu.Unlock()
	p := a.lastNudgedPattern
	a.lastNudgedPattern = nil
	return p
}

func formatConfidence(c float64) string {
	pct := min(max(int(c*100), 0), 100)
	return strconv.Itoa(pct) + "%"
}

const ambientSurfacedSystemPrompt = `You are checking whether an AI assistant's reply casually brought up a specific suggested topic to the user — even briefly, as a passing question or remark, not necessarily the main subject of the reply.

Answer with exactly one word: YES or NO. Nothing else.`

const ambientOutcomeSystemPrompt = `You are classifying how a user responded to a suggestion an assistant made in a chat message. The user's reply may be in any language.

Answer with exactly one word: ACCEPT, DECLINE, or UNCLEAR. UNCLEAR means the reply doesn't clearly address the suggestion at all (e.g. it changes the subject). Nothing else.`

// checkAmbientNudgeSurfaced runs after a turn's reply is ready — called from
// finishStream with whatever takeNudgedPattern captured synchronously there
// (independently of memory — see this file's package doc comment), if
// buildProactiveNudgeBlock offered a nudge this same turn. It asks the model
// itself whether the reply actually used it — necessary because the nudge
// block only ever gives the model *permission* to bring the habit up, never
// a command to, so most matching turns won't actually mention it. Only when
// the answer is yes does this arm the pending-suggestion slot that
// checkAmbientNudgeOutcome later resolves.
//
// Builds its own bounded context rather than taking one from the caller —
// this is a fire-and-forget background job (started via `go`) that must
// keep running after the request that triggered it has already returned,
// same as updateMoodAsync right above it in chat.go.
func (a *App) checkAmbientNudgeSurfaced(pattern *observer.TimePattern, reply string) {
	if pattern == nil || reply == "" || isLLMErrorReply(reply) || a.proactivePending == nil {
		return
	}
	// Re-checked here, not just at the offering site: MinimalMode/Proactive
	// could have been toggled off in the time between buildProactiveNudgeBlock
	// (start of the turn) and this call (after the reply finished streaming).
	if !a.ambientNudgingActive() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	msgs := []api.Message{
		api.NewTextMessage("system", ambientSurfacedSystemPrompt),
		api.NewTextMessage("user", "Suggested topic: "+pattern.ActivityType+"\n\nAssistant's reply:\n"+reply),
	}
	answer := a.callLLM(ctx, msgs)
	if isLLMErrorReply(answer) || !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(answer)), "YES") {
		return
	}

	if err := a.proactivePending.Set(proactive.PendingSuggestion{
		ID:        uuid.NewString(),
		Message:   pattern.ActivityType,
		PatternID: pattern.ID,
		Action:    proactive.ActionAmbient,
		CreatedAt: time.Now(),
	}); err != nil {
		logx.Printf("PROACTIVE: ambient set pending: %v", err)
	}
}

// checkAmbientNudgeOutcome runs at the start of a turn (fired independently,
// before/alongside generating that turn's reply — see routeStream) if there
// is a pending *ambient* suggestion awaiting a response. It classifies
// userMsg — free-form chat text in whatever language the user writes — via
// the model itself rather than a keyword list (OutcomeFromResponse's
// rejectWords/acceptWords only cover Turkish/English, appropriate for a
// frontend button's fixed label, not open-ended user text). An UNCLEAR
// verdict leaves the pending suggestion untouched — it will either be
// resolved by a later, clearer message or expire naturally via PendingTTL —
// rather than consuming it on a guess.
//
// Only ever acts on Action == ActionAmbient. A pending suggestion can also
// come from the background Engine's own tick (a formal banner/notification
// the frontend shows with explicit Accept/Decline controls, resolved via
// HandleResponse) — without this check, an ordinary chat message sent while
// that banner is still up would get silently classified as an answer to it
// and clear it out from under the user before they ever tapped a button.
//
// resolvingID guards against double-processing: two user messages sent in
// quick succession, both landing before the first's ~30s-bounded LLM call
// returns, would otherwise both fetch the same still-pending suggestion and
// could both resolve it (double confidence adjustment, or the second racing
// a suggestion that the first has already cleared and a newer one replaced).
//
// Builds its own bounded context rather than taking one from the caller —
// see checkAmbientNudgeSurfaced's doc comment.
func (a *App) checkAmbientNudgeOutcome(userMsg string) {
	if a.proactivePending == nil || a.proactiveEngine == nil || userMsg == "" {
		return
	}
	if !a.ambientNudgingActive() {
		return
	}
	pending, err := a.proactivePending.Get()
	if err != nil || pending == nil || pending.Action != proactive.ActionAmbient {
		return
	}

	a.proactiveNudgeMu.Lock()
	if a.resolvingSuggestionID == pending.ID {
		a.proactiveNudgeMu.Unlock()
		return
	}
	a.resolvingSuggestionID = pending.ID
	a.proactiveNudgeMu.Unlock()
	defer func() {
		a.proactiveNudgeMu.Lock()
		if a.resolvingSuggestionID == pending.ID {
			a.resolvingSuggestionID = ""
		}
		a.proactiveNudgeMu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	msgs := []api.Message{
		api.NewTextMessage("system", ambientOutcomeSystemPrompt),
		api.NewTextMessage("user", "The assistant suggested: "+pending.Message+"\n\nThe user's reply:\n"+userMsg),
	}
	answer := strings.ToUpper(strings.TrimSpace(a.callLLM(ctx, msgs)))
	if isLLMErrorReply(answer) {
		return
	}

	var outcome proactive.Outcome
	switch {
	case strings.HasPrefix(answer, "ACCEPT"):
		outcome = proactive.OutcomeAccepted
	case strings.HasPrefix(answer, "DECLINE"):
		outcome = proactive.OutcomeRejected
	default:
		return // UNCLEAR (or an unparseable answer) — leave it pending.
	}

	// Re-fetch rather than reuse the pending value captured above: if a
	// concurrent call already resolved (and cleared) this exact suggestion
	// while this LLM call was in flight, Get() now returns nil (or a
	// different, newer suggestion) and HandleOutcome must not run against
	// the stale copy.
	current, err := a.proactivePending.Get()
	if err != nil || current == nil || current.ID != pending.ID {
		return
	}
	if _, err := a.proactiveEngine.HandleOutcome(*current, outcome); err != nil {
		logx.Printf("PROACTIVE: ambient handle outcome: %v", err)
	}
}

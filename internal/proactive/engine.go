// SPDX-License-Identifier: AGPL-3.0-or-later

package proactive

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"memo/internal/observer"
)

// Tunables with sensible defaults (filled in by NewEngine when zero).
const (
	defaultInterval      = 30 * time.Second
	defaultMinScore      = 0.1
	defaultMinConfidence = 0.3
	defaultCooldown      = 30 * time.Minute
	historyLimit         = 5
)

// Decider asks the Chief model for a decision. It is injected so the engine
// stays decoupled from the provider/orchestra layer and is easy to fake in
// tests. It returns the raw model text (expected to contain a JSON object).
type Decider func(ctx context.Context, systemPrompt, userPrompt string) (string, error)

// Emitter delivers a suggestion to the user-facing layer (chat event / mobile
// notification). The pending suggestion is also persisted independently for
// polling, so Emitter may be a no-op.
type Emitter func(p PendingSuggestion)

// AutoRunner executes an "auto" action (the agent pipeline). Optional; when nil,
// auto decisions are delivered as suggestions instead.
type AutoRunner func(ctx context.Context, p PendingSuggestion)

// Config tunes the engine loop.
type Config struct {
	Interval      time.Duration
	MinScore      float64
	MinConfidence float64
	Cooldown      time.Duration
}

// Engine is the proactive heartbeat (README §6). It periodically matches the
// current moment against learned patterns and asks the Chief what to do.
type Engine struct {
	cfg      Config
	patterns *observer.PatternStore
	pending  *PendingStore
	decide   Decider
	emit     Emitter
	auto     AutoRunner
	levelFn  func() Level

	mu            sync.Mutex
	history       []FeedbackEntry
	lastConsultAt time.Time // throttles Chief consultations, regardless of outcome
}

// NewEngine builds an engine. levelFn supplies the live proactivity level (so
// the user can toggle it without restarting); a nil levelFn means Off.
func NewEngine(cfg Config, patterns *observer.PatternStore, pending *PendingStore,
	decide Decider, emit Emitter, levelFn func() Level) *Engine {

	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}
	if cfg.MinScore <= 0 {
		cfg.MinScore = defaultMinScore
	}
	if cfg.MinConfidence <= 0 {
		cfg.MinConfidence = defaultMinConfidence
	}
	if cfg.Cooldown <= 0 {
		cfg.Cooldown = defaultCooldown
	}
	if levelFn == nil {
		levelFn = func() Level { return LevelOff }
	}
	if emit == nil {
		emit = func(PendingSuggestion) {}
	}
	return &Engine{
		cfg:      cfg,
		patterns: patterns,
		pending:  pending,
		decide:   decide,
		emit:     emit,
		levelFn:  levelFn,
	}
}

// SetAutoRunner installs the optional agent-pipeline runner for "auto" actions.
func (e *Engine) SetAutoRunner(r AutoRunner) { e.auto = r }

// Start runs the loop until ctx is cancelled. Intended to be called as a
// goroutine.
func (e *Engine) Start(ctx context.Context) {
	ticker := time.NewTicker(e.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.tick(ctx)
		}
	}
}

// Tick triggers a single heartbeat manually (used by tests and the CLI demo).
// Uses the real wall clock.
func (e *Engine) Tick(ctx context.Context) {
	e.tickAt(ctx, time.Now())
}

// TickAt triggers a single heartbeat at the given time (used by the demo).
func (e *Engine) TickAt(ctx context.Context, now time.Time) {
	e.tickAt(ctx, now)
}

// tick is one heartbeat: reap expired prompts, find matches, and (if clear) ask
// the Chief and act.
func (e *Engine) tick(ctx context.Context) {
	e.tickAt(ctx, time.Now())
}

func (e *Engine) tickAt(ctx context.Context, now time.Time) {
	if e.levelFn() == LevelOff {
		return
	}
	if e.patterns == nil || e.pending == nil || e.decide == nil {
		return
	}

	e.reapExpired()

	actives, err := e.patterns.LoadActive(now, e.cfg.MinConfidence)
	if err != nil {
		log.Printf("PROACTIVE: load patterns: %v", err)
		return
	}
	if len(actives) == 0 {
		return
	}

	matches := make([]MatchResult, 0, len(actives))
	for _, p := range actives {
		if score := Match(p, now); score > e.cfg.MinScore {
			matches = append(matches, MatchResult{Pattern: p, Score: score})
		}
	}
	if len(matches) == 0 {
		return
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })

	// Don't pile up: one in-flight suggestion at a time, and don't re-consult the
	// Chief more than once per cooldown — even when it keeps saying "none" — so a
	// long matching window doesn't trigger an LLM call every tick.
	if e.pending.HasPending() {
		return
	}
	e.mu.Lock()
	cooling := !e.lastConsultAt.IsZero() && time.Since(e.lastConsultAt) < e.cfg.Cooldown
	if !cooling {
		e.lastConsultAt = time.Now()
	}
	history := append([]FeedbackEntry(nil), e.history...)
	e.mu.Unlock()
	if cooling {
		return
	}

	decision := e.askChief(ctx, matches, now, history)
	e.execute(ctx, decision)
}

// askChief performs a single Chief consultation and parses its decision. The
// self-correcting retry is driven across ticks by feedback history rather than
// by blocking the loop.
func (e *Engine) askChief(ctx context.Context, matches []MatchResult, now time.Time, history []FeedbackEntry) Decision {
	user := BuildContextPrompt(matches, now, history, e.levelFn(), len(history)+1)
	raw, err := e.decide(ctx, ChiefSystemPrompt, user)
	if err != nil {
		log.Printf("PROACTIVE: chief decide: %v", err)
		return Decision{Action: ActionNone}
	}
	d, err := ParseDecision(raw)
	if err != nil {
		log.Printf("PROACTIVE: parse decision: %v (raw=%q)", err, truncate(raw, 200))
		return Decision{Action: ActionNone}
	}
	// The Chief may omit the pattern id; default to the strongest match.
	if d.PatternID == "" && len(matches) > 0 {
		d.PatternID = matches[0].Pattern.ID
	}
	return d
}

// execute carries out a decision.
func (e *Engine) execute(ctx context.Context, d Decision) {
	if d.Action == ActionNone || d.Message == "" {
		return
	}

	ps := PendingSuggestion{
		ID:        uuid.NewString(),
		Message:   d.Message,
		PatternID: d.PatternID,
		Action:    d.Action,
		CreatedAt: time.Now(),
	}

	if err := e.pending.Set(ps); err != nil {
		log.Printf("PROACTIVE: set pending: %v", err)
		return
	}

	e.emit(ps)

	if d.Action == ActionAuto && e.auto != nil {
		go e.auto(ctx, ps)
	}
	log.Printf("PROACTIVE: %s for pattern %s: %q", d.Action, d.PatternID, truncate(d.Message, 80))
}

// reapExpired records an Outcome of "ignored" for a prompt that timed out and
// clears it, so the engine can act again and the Chief learns from the silence.
func (e *Engine) reapExpired() {
	p, err := e.pending.GetRaw()
	if err != nil || p == nil || p.Responded {
		return
	}
	if time.Since(p.CreatedAt) <= PendingTTL {
		return
	}
	e.recordOutcome(*p, OutcomeIgnored)
	if err := e.pending.Clear(); err != nil {
		log.Printf("PROACTIVE: clear expired: %v", err)
	}
}

// HandleResponse applies the user's answer to the pending suggestion: it records
// feedback, adjusts the pattern, and clears the prompt. It reports whether the
// user accepted (so callers can kick off the corresponding action).
func (e *Engine) HandleResponse(id, response string) (accepted bool, err error) {
	p, err := e.pending.Respond(id, response)
	if err != nil {
		return false, err
	}
	if p == nil {
		return false, nil
	}
	outcome := OutcomeFromResponse(response)
	e.recordOutcome(*p, outcome)
	if cerr := e.pending.Clear(); cerr != nil {
		log.Printf("PROACTIVE: clear after response: %v", cerr)
	}
	return outcome == OutcomeAccepted, nil
}

// recordOutcome updates the pattern's confidence (or deletes it) and appends to
// the rolling feedback history.
func (e *Engine) recordOutcome(p PendingSuggestion, outcome Outcome) {
	switch outcome {
	case OutcomeStopped:
		// Retire permanently so re-analysis does not resurrect it.
		if _, err := e.patterns.Suppress(p.PatternID); err != nil {
			log.Printf("PROACTIVE: suppress pattern %s: %v", p.PatternID, err)
		}
	default:
		if delta := ConfidenceDelta(outcome); delta != 0 {
			if _, err := e.patterns.AdjustConfidence(p.PatternID, delta); err != nil {
				log.Printf("PROACTIVE: adjust pattern %s: %v", p.PatternID, err)
			}
		}
	}

	e.mu.Lock()
	e.history = append(e.history, FeedbackEntry{
		PatternID: p.PatternID,
		Action:    p.Action,
		Message:   p.Message,
		Outcome:   outcome,
	})
	if len(e.history) > historyLimit {
		e.history = e.history[len(e.history)-historyLimit:]
	}
	e.mu.Unlock()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

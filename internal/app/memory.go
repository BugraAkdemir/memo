package app

import (
	"context"
	"fmt"
	"memo/internal/logx"
	"strings"
	"time"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/memory"
	"memo/internal/models"
	"memo/internal/truncate"
)

// isEmbeddingBackendDown reports whether err means the embedding endpoint is
// simply unreachable (no local embedding server running). This is expected when
// the user runs an API/Orchestra-only setup, so we log it but don't surface a
// scary "memory error" toast on every single message.
func isEmbeddingBackendDown(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connect: connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "EOF")
}

// isLLMErrorReply reports whether reply is one of callLLM's synthetic
// "⚠️ ..." error strings (model not loaded, provider error, empty response,
// unreadable attachment, etc.) rather than a genuine model response. Every
// error path in llm.go and chat.go consistently uses this prefix — the same
// convention the streaming path's recordStreamError relies on — so it's a
// reliable signal that reply must not be indexed into RAG memory.
func isLLMErrorReply(reply string) bool {
	return strings.HasPrefix(reply, "⚠️")
}

func (a *App) saveMemoryAsync(userMsg, reply string) {
	if reply == "" || isLLMErrorReply(reply) || !a.GetMemoryEnabled() {
		return
	}
	// Shutdown() closes memorySaveCh once webSrv.Stop() returns — but that
	// only waits on each HTTP handler's own call stack, not on a detached
	// background goroutine a handler already started (a streaming reply's
	// own goroutine can still be finishing finishStream/saveMemoryAsync
	// after the request itself has ended). A send racing that close panics
	// ("send on closed channel"); recovering it right here — instead of
	// letting it propagate into whatever unrelated streaming goroutine
	// happens to be running this call, where recoverStreamPanic would
	// report it as a generic "internal error" mid-turn — keeps the failure
	// correctly attributed and makes the loss observable instead of
	// silent (BUG_REPORT.md RC-7). Narrow shutdown-timing window; doesn't
	// happen on a normal, running backend.
	defer func() {
		if r := recover(); r != nil {
			logx.Printf("WARN: memory save skipped (shutting down): %v", r)
		}
	}()
	select {
	case a.memorySaveCh <- saveTask{userMsg: userMsg, reply: reply}:
	case <-time.After(2 * time.Second):
		logx.Info("WARN: memory save channel full, dropping")
		a.emitEvent("memory:error", "Hafıza kaydetme kuyruğu dolu; bu mesaj hatırlanmayabilir")
	}
}

func (a *App) memorySaveWorker() {
	for task := range a.memorySaveCh {
		// Recovered per-task, not once around the whole loop: a panic while
		// saving one turn must not permanently kill every future save for
		// the rest of the process's life (memorySaveCh keeps filling until
		// full, then silently drops — see saveMemoryAsync). See recoverPanic
		// (app.go) for why this matters even inside an already-`go`-started
		// worker.
		func() {
			defer recoverPanic("memorySaveWorker/saveMemorySync")
			a.saveMemorySync(a.lifecycleCtx, task.userMsg, task.reply)
		}()
	}
}

func (a *App) saveMemorySync(ctx context.Context, userMsg, reply string) {
	if !a.GetMemoryEnabled() {
		return
	}
	start := time.Now()

	// BUG-L1: pure acks/greetings ("tamam", "ok", "selam") with a short
	// reply add only RAG noise. Explicit /remember (SaveExplicit) never
	// reaches this path. Near-duplicate cosine skip in store.SaveInteraction
	// still applies to everything that does get saved.
	if memory.IsLowValueTurn(userMsg, reply) {
		logx.Printf("MEMORY SAVE SKIPPED: low-value interaction %q", truncate.Text(userMsg, 40))
		return
	}

	// Hold the lock only long enough to grab the store reference so that the
	// heavy embedding I/O below does not block concurrent memory reads/writes.
	a.storeMu.RLock()
	store := a.store
	a.storeMu.RUnlock()
	if store == nil {
		logx.Info("MEMORY SAVE SKIPPED: store not initialized")
		a.emitEvent("memory:error", "Hafıza kaydedilemedi: depo başlatılmamış")
		return
	}

	mctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := store.SaveInteraction(mctx, userMsg, reply); err != nil {
		logx.Printf("LATENCY app.memory_save_sync total_ms=%d status=error", time.Since(start).Milliseconds())
		logx.Printf("MEMORY SAVE FAILED: %v", err)
		if !isEmbeddingBackendDown(err) {
			a.emitEvent("memory:error", fmt.Sprintf("Hafıza kaydedilemedi: %v", err))
		}
	} else {
		logx.Printf("LATENCY app.memory_save_sync total_ms=%d status=ok", time.Since(start).Milliseconds())
		logx.Printf("Memory saved: %q → %d chars reply", truncate.Text(userMsg, 60), len(reply))
		// Only signal here, once the async save has actually completed — lets
		// clients (e.g. the terminal REPL) show a "memory saved" confirmation
		// that's tied to a real write instead of just assuming it happened.
		a.emitEvent("memory:saved", "")
		a.syncMu.RLock()
		sm := a.syncManager
		a.syncMu.RUnlock()
		if sm != nil {
			sm.Increment()
		}
		// Fired off, not awaited: extraction can take as long as callLLM's own
		// budget (up to 300s for an external provider/orchestra call), and
		// memorySaveWorker is a single goroutine draining every queued save —
		// blocking it here would back up every other chat's save behind this
		// one turn's extraction call.
		goRecover("extractAndPinFacts", func() { a.extractAndPinFacts(ctx, userMsg) })
	}
}

// factExtractionSystemPrompt drives a narrow, single-purpose LLM call: pull
// zero or more durable personal facts out of one chat turn. Deliberately not
// asked of the same call that generates the user-facing reply (e.g. via an
// inline tag) — that would tie memory's reliability to the same "does the
// model reliably follow a formatting instruction" fragility that makes agent
// mode not yet fully stable. This runs as its own request, and NONE is the
// expected, common-case output (most turns — greetings, small talk — have
// nothing worth pinning), which is why the format is plain text with an
// explicit sentinel rather than JSON: it must degrade gracefully across every
// provider/local model this app supports, including ones with no reliable
// JSON/structured-output mode.
const factExtractionSystemPrompt = `You extract durable personal facts about the user from a single chat turn, for long-term memory.

Extract a fact only if it will still be true weeks or months from now:
identity, relationships, pets, ongoing preferences, recurring habits,
biographical details (birthday, job, location, etc).

Do NOT extract: greetings, small talk, one-off requests, questions,
opinions about this conversation, or anything temporary.

Rules:
- One fact per line, nothing else. No numbering, no markdown, no explanation.
- Each fact must be a short, self-contained, third-person statement
  (e.g. "User's dog is named Zeytin", not "my dog is named Zeytin").
- If nothing is worth remembering long-term, output exactly: NONE`

const (
	maxExtractedFactsPerTurn = 5
	maxExtractedFactLength   = 300
)

// extractAndPinFacts is fired off in its own goroutine by its caller (see
// saveMemorySync) after a turn is already saved and shown to the user, so a
// slow, wrong, or failed extraction can never affect the user-visible
// response, and never blocks the single memorySaveWorker goroutine behind
// it — it only ever adds to what tomorrow's system prompts know. Extracted
// facts are pinned via SaveExplicitMemory, which makes them show up in every
// future prompt unconditionally through GetPinnedFacts/retrieveMemory,
// bypassing RAG ranking entirely. See AGENTS.md's Known Pitfalls, Memory /
// Vector Store section (2026-07-15 entries) for the bug class this exists to
// close: every chat turn saves with equal importance by default, so a fact
// stated casually has no way to stand out from routine chit-chat under pure
// retrieval ranking.
//
// Routes through callLLM (Orchestra → external provider → local model, same
// priority chain as regular chat) rather than reaching into a.providerRouter
// directly — this exact shortcut was already found and fixed once in this
// package (ImportMemoryFromText, memory_import.go, 2026-07-13): bypassing
// callLLM means a local-only setup (no external provider configured — this
// app's core "local-first" use case) gets a nil providerRouter and the
// feature silently never runs at all. callLLM also applies its own
// appropriate per-branch timeout (120–300s) — this must not wrap that in a
// shorter timeout of its own, or it would truncate callLLM's budget instead.
// extractAndPinFacts only feeds the user's own message to the extraction
// call, never the assistant's reply. BUG-H4: the reply can carry third-party
// information the assistant surfaced via a tool call (e.g. it read out a
// WhatsApp group's contents) — feeding that alongside "User: ...\nAssistant:
// ..." let the extraction model confuse a fact about someone else entirely
// with a durable fact about the user, permanently pinning it into every
// future prompt (pinned facts bypass RAG ranking entirely). The user's own
// words are always genuinely about the user, which the assistant's reply is
// not guaranteed to be.
func (a *App) extractAndPinFacts(ctx context.Context, userMsg string) {
	if !a.cfg.Memory.AutoFactExtraction {
		return
	}

	// Registered so a subsequent real chat message can preempt this call
	// before it reaches the local model — see preemptBackgroundLLM (llm.go)
	// and BUG_REPORT TD-2.
	bgCtx, done := a.beginBackgroundLLMCall(ctx)
	defer done()

	msgs := []api.Message{
		api.NewTextMessage("system", factExtractionSystemPrompt),
		api.NewTextMessage("user", fmt.Sprintf("User: %s", userMsg)),
	}
	reply2 := a.callLLM(bgCtx, msgs, categoryFactExtraction)
	if isLLMErrorReply(reply2) {
		// Best-effort enhancement, not a core save path — log only, never
		// surface as memory:error (that event means "a message may not be
		// remembered at all", which isn't true here: SaveInteraction already
		// succeeded before this ever runs).
		logx.Printf("MEMORY: fact extraction skipped: %s", reply2)
		return
	}

	facts := parseExtractedFacts(reply2)
	if len(facts) == 0 {
		return
	}

	// Dedup against what's already pinned (see BUG-M6): the same durable
	// fact gets re-extracted on every turn it's still relevant to (a name,
	// city, job), and since pinned facts bypass RAG ranking entirely and
	// have a fixed cap, unchecked duplicates crowd out real one-off facts.
	// Refetched once per call (not cached) so it also catches duplicates
	// created within this same batch of facts.
	pinned := a.pinnedFactTexts(ctx)
	for _, fact := range facts {
		key := normalizeFactText(fact)
		if _, dup := pinned[key]; dup {
			logx.Printf("MEMORY: skipping duplicate extracted fact: %q", truncate.Text(fact, 60))
			continue
		}
		if err := a.SaveExplicitMemory(fact, "auto-extracted"); err != nil {
			logx.Printf("MEMORY: failed to pin extracted fact: %v", err)
			continue
		}
		pinned[key] = struct{}{}
	}
}

// pinnedFactTexts returns the normalized text of every currently pinned
// fact, for extractAndPinFacts' dedup check. Returns an empty (non-nil) set
// on any failure to read the store, so callers never skip pinning due to a
// transient read error.
func (a *App) pinnedFactTexts(ctx context.Context) map[string]struct{} {
	a.storeMu.RLock()
	store := a.store
	a.storeMu.RUnlock()
	set := make(map[string]struct{})
	if store == nil {
		return set
	}
	pinned, err := store.GetPinnedFacts(ctx)
	if err != nil {
		logx.Printf("MEMORY: dedup check GetPinnedFacts: %v", err)
		return set
	}
	for _, p := range pinned {
		set[normalizeFactText(p.Content)] = struct{}{}
	}
	return set
}

// normalizeFactText makes two differently-cased/punctuated but otherwise
// identical fact strings compare equal for dedup purposes.
func normalizeFactText(s string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(s)), ".!? ")
}

// factListMarkerPrefixes are stripped from the front of an extracted line
// before it's checked for emptiness/NONE — a defense against a model that
// ignores the "no numbering, no markdown" instruction and emits "1. fact" or
// "* fact" instead of a bare line. Longest-first within each style so e.g.
// "10." doesn't get half-stripped into "0.".
var factListMarkerPrefixes = []string{"- ", "* ", "• ", "-", "*", "•"}

// parseExtractedFacts turns the extraction call's raw text output into a
// bounded, sanitized list of facts. Nothing here trusts the model's output
// format blindly — a misbehaving or hallucinating model must not be able to
// flood the pinned-facts list (capped per turn), inject arbitrarily long
// garbage into a system prompt (capped per fact), or leave stray list-marker
// artifacts that would then sit in every future system prompt forever, since
// pinned facts bypass the RAG relevance decay a normal memory would age out
// under.
func parseExtractedFacts(raw string) []string {
	var facts []string
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		line = stripFactListMarker(line)
		if line == "" || strings.EqualFold(line, "none") {
			continue
		}
		if len(line) > maxExtractedFactLength {
			line = line[:maxExtractedFactLength]
		}
		facts = append(facts, line)
		if len(facts) >= maxExtractedFactsPerTurn {
			break
		}
	}
	return facts
}

// stripFactListMarker removes one leading list-style marker: a dash/asterisk/
// bullet ("-", "*", "•"), or a numbered prefix ("1.", "12)", etc).
func stripFactListMarker(line string) string {
	for _, p := range factListMarkerPrefixes {
		if strings.HasPrefix(line, p) {
			return strings.TrimSpace(line[len(p):])
		}
	}
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i > 0 && i < len(line) && (line[i] == '.' || line[i] == ')') {
		return strings.TrimSpace(line[i+1:])
	}
	return line
}

func (a *App) retrieveMemory(ctx context.Context, query string) []memory.MemoryResult {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		logx.Info("Memory: store not initialized, skipping retrieve")
		return nil
	}
	start := time.Now()
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	m, err := a.store.RetrieveContext(rctx, query, a.cfg.Memory.TopK, a.cfg.Memory.MinSimilarity)
	if err != nil {
		logx.Printf("LATENCY app.retrieve_memory total_ms=%d status=error", time.Since(start).Milliseconds())
		logx.Printf("MEMORY RETRIEVE FAILED: %v", err)
		// Embedding backend unreachable is expected in API/Orchestra-only mode —
		// degrade silently instead of flooding the UI with error toasts.
		if !isEmbeddingBackendDown(err) {
			a.emitEvent("memory:error", fmt.Sprintf("Hafıza okunamadı: %v", err))
		}
		return nil
	}

	// Pinned facts (explicit saves, importance=5) are merged in unconditionally
	// — never subject to RetrieveContext's topK/similarity ranking — so a core
	// personal fact can't be crowded out by routine conversational noise. Uses
	// its own timeout rather than reusing rctx: RetrieveContext can itself
	// spend most of a shared budget on multi-embed compound-query
	// decomposition, which would otherwise leave GetPinnedFacts almost no
	// time and silently break the "unconditional" guarantee under load.
	pinCtx, pinCancel := context.WithTimeout(ctx, 5*time.Second)
	pinned, pinErr := a.store.GetPinnedFactsRanked(pinCtx, query, a.cfg.Memory.PinnedFactsPerTurn)
	pinCancel()
	if pinErr != nil {
		logx.Printf("MEMORY: GetPinnedFactsRanked: %v", pinErr)
	} else if len(pinned) > 0 {
		// Pinned facts go FIRST, not appended at the end: BuildSystemPrompt
		// (internal/identity) truncates the formatted memory block from the
		// tail once it exceeds its token budget — appending here would make
		// pinned facts the first thing dropped for exactly the heavy users
		// (large RAG history + many pinned facts) who most need them protected.
		seen := make(map[string]struct{}, len(pinned))
		merged := make([]memory.MemoryResult, 0, len(pinned)+len(m))
		for _, p := range pinned {
			merged = append(merged, p)
			seen[p.ID] = struct{}{}
		}
		for _, r := range m {
			if _, dup := seen[r.ID]; dup {
				continue
			}
			// Drop a RAG hit that only re-states a pinned fact in different
			// words — exact-ID was the only dedup here before, so "kedimin
			// adı Zeytin" pinned and the conversational turn it came from
			// both landed in the prompt.
			nearDup := false
			for _, p := range pinned {
				if memory.NearDuplicateContent(p.Content, r.Content) {
					nearDup = true
					break
				}
			}
			if !nearDup {
				merged = append(merged, r)
			}
		}
		m = merged
	}

	logx.Printf("LATENCY app.retrieve_memory total_ms=%d returned=%d", time.Since(start).Milliseconds(), len(m))
	if len(m) > 0 {
		best := m[0].Similarity
		for _, r := range m[1:] {
			if r.Similarity > best {
				best = r.Similarity
			}
		}
		logx.Printf("Memory: found %d relevant memories (best=%.0f%%)", len(m), best*100)
	}
	return m
}

func (a *App) reinitMemoryStore(client *api.Client, model string) {
	// Remember the old store, then set a.store = nil so read-side guards
	// (retrieveMemory etc.) return early while we build the replacement.
	// If NewStore fails we restore the old store; on success we close the
	// old one under lock and swap in the new one — no permanent nil window.
	a.storeMu.Lock()
	oldStore := a.store
	a.store = nil
	a.storeMu.Unlock()

	embeddingFunc := memory.NewEmbeddingFunc(client, model)
	newStore, err := memory.NewStore(memory.StoreConfig{
		Dir:                 a.cfg.Memory.PersistDir,
		Dimension:           a.cfg.Memory.EmbeddingDimension,
		EmbeddingFunc:       embeddingFunc,
		DreamSettings:       a.dreamSettings,
		RecencyHalfLifeDays: a.cfg.Memory.RecencyHalfLifeDays,
	})
	if err != nil {
		logx.Printf("WARN: memory re-init: %v (restoring old store)", err)
		a.emitEvent("memory_store_error", err.Error())
		a.storeMu.Lock()
		a.store = oldStore
		a.storeMu.Unlock()
		return
	}

	newStore.SetConsolidationFunc(a.mergeMemoriesLLM)
	newStore.SetDreamFunc(a.dreamPinnedFactsLLM)

	a.storeMu.Lock()
	if oldStore != nil {
		if err := oldStore.Close(); err != nil {
			logx.Printf("WARN: memory store close during re-init: %v", err)
		}
	}
	a.store = newStore
	a.storeMu.Unlock()
	logx.Info("Memory store re-initialized")
}

// DebugMemorySearch searches memory WITHOUT similarity filter — for
// debugging. Also merges in pinned facts (source='explicit'/importance=5,
// see GetPinnedFacts) the same way retrieveMemory does for the real chat
// path — otherwise this panel mislabels a pinned fact as a plain "Vektör"/
// "FTS" match whenever it also happens to score well on the hybrid search
// below, giving no visibility into which results are actually guaranteed to
// be injected into every prompt versus merely similarity-matched.
func (a *App) DebugMemorySearch(query string) []memory.MemoryResult {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	results := a.store.DebugSearch(ctx, query, 10)

	pinned, err := a.store.GetPinnedFacts(ctx)
	if err != nil {
		logx.Printf("MEMORY: DebugMemorySearch GetPinnedFacts: %v", err)
		return results
	}
	seen := make(map[string]struct{}, len(pinned))
	merged := make([]memory.MemoryResult, 0, len(pinned)+len(results))
	for _, p := range pinned {
		merged = append(merged, p)
		seen[p.ID] = struct{}{}
	}
	for _, r := range results {
		if _, dup := seen[r.ID]; !dup {
			merged = append(merged, r)
		}
	}
	return merged
}

// GetMemoryCount returns the number of stored memory entries.
func (a *App) GetMemoryCount() int {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return 0
	}
	return a.store.Count()
}

// ClearAllMemory removes all stored memory entries.
func (a *App) ClearAllMemory() error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		return fmt.Errorf("no memory store")
	}
	logx.Info("Clearing all memory...")
	return a.store.ClearAll()
}

// ListMemoryFiles lists the on-disk gob files in the memory store.
func (a *App) ListMemoryFiles() []memory.GobFileInfo {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return nil
	}
	return a.store.ListGobFiles()
}

// DeleteMemoryFile deletes a specific gob file from the memory store.
func (a *App) DeleteMemoryFile(relPath string) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		return fmt.Errorf("no memory store")
	}
	logx.Printf("Deleting memory file: %s", relPath)
	return a.store.DeleteGobFile(relPath)
}

// GetMemorySettings returns memory configuration.
func (a *App) GetMemorySettings() config.MemoryConfig {
	return a.cfg.Memory
}

// UpdateMemorySettings updates topK and minSimilarity in the memory config.
func (a *App) UpdateMemorySettings(topK int, minSimilarity float32) error {
	if topK < 1 || topK > 50 {
		return fmt.Errorf("top_k must be between 1 and 50")
	}
	if minSimilarity <= 0 || minSimilarity > 1 {
		return fmt.Errorf("min_similarity must be between 0.01 and 1")
	}

	a.cfg.Memory.TopK = topK
	a.cfg.Memory.MinSimilarity = minSimilarity
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	logx.Printf("Memory settings updated: top_k=%d min_similarity=%.2f", topK, minSimilarity)
	return nil
}

// GetMemoryEnabled reports whether memory is enabled.
func (a *App) GetMemoryEnabled() bool {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.Memory.MemoryEnabled
}

// dreamSettings is memory.DreamSettingsFunc's App-side implementation —
// wired into memory.StoreConfig by reinitMemoryStore. Reads under cfgMu
// since Settings can change these values concurrently with
// runDreamScheduler reading them on its own goroutine.
func (a *App) dreamSettings() (initialDelay, interval time.Duration, enabled bool) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return time.Duration(a.cfg.Memory.DreamInitialDelayMinutes) * time.Minute,
		time.Duration(a.cfg.Memory.DreamIntervalHours) * time.Hour,
		a.cfg.Memory.DreamEnabled
}

// GetMemoryDreamSettings returns Dream's current Settings-tab state.
func (a *App) GetMemoryDreamSettings() (enabled bool, initialDelayMinutes, intervalHours int) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.Memory.DreamEnabled, a.cfg.Memory.DreamInitialDelayMinutes, a.cfg.Memory.DreamIntervalHours
}

// SetMemoryDreamSettings updates Dream's schedule. Takes effect from the
// scheduler's next check onward (see runDreamScheduler's doc comment) — it
// does not restart an in-progress wait. For "apply right now" use
// RunDreamNow instead.
func (a *App) SetMemoryDreamSettings(enabled bool, initialDelayMinutes, intervalHours int) error {
	if initialDelayMinutes < 1 || initialDelayMinutes > 1440 {
		return fmt.Errorf("initial delay must be between 1 and 1440 minutes")
	}
	if intervalHours < 1 || intervalHours > 720 {
		return fmt.Errorf("interval must be between 1 and 720 hours")
	}

	a.cfgMu.Lock()
	a.cfg.Memory.DreamEnabled = enabled
	a.cfg.Memory.DreamInitialDelayMinutes = initialDelayMinutes
	a.cfg.Memory.DreamIntervalHours = intervalHours
	a.cfgMu.Unlock()

	if err := config.Save(a.cfg); err != nil {
		return err
	}
	logx.Printf("Dream settings updated: enabled=%v initial_delay=%dm interval=%dh", enabled, initialDelayMinutes, intervalHours)
	return nil
}

// RunDreamNow triggers an immediate Dream pass (Settings tab's manual "run
// now" button) — bypasses the scheduler's dreamThreshold, see
// memory.Store.RunDreamNow's doc comment. Independent of DreamEnabled: a
// disabled schedule only means "don't run automatically," not "the user can
// never trigger it by hand."
func (a *App) RunDreamNow(ctx context.Context) (before, after int, ran bool, err error) {
	a.storeMu.RLock()
	store := a.store
	a.storeMu.RUnlock()
	if store == nil {
		return 0, 0, false, fmt.Errorf("memory store not initialized")
	}
	return store.RunDreamNow(ctx)
}

// mergeMemoriesLLM sends two memory contents to the active provider and returns
// a single merged memory. Used as the Store's ConsolidationFunc.
func (a *App) mergeMemoriesLLM(ctx context.Context, content1, content2 string) (string, error) {
	msgs := []api.Message{
		api.NewTextMessage("system",
			"You are a memory consolidation assistant. Given two similar memory entries, "+
				"create ONE concise memory that preserves all important information from both. "+
				"Output ONLY the merged memory text — no explanations, no labels, no prefixes."),
		api.NewTextMessage("user",
			fmt.Sprintf("Memory 1: %s\n\nMemory 2: %s\n\nMerge into one memory:", content1, content2)),
	}
	reply := a.callLLM(ctx, msgs, categoryConsolidation)
	if isLLMErrorReply(reply) {
		return "", fmt.Errorf("merge LLM call: %s", reply)
	}
	return strings.TrimSpace(reply), nil
}

const dreamSystemPrompt = `You compress a list of durable personal facts about a user into a shorter list, for long-term memory storage.

Rules:
- Merge facts that are about the same specific topic (e.g. several facts about the same pet, the same job, the same relationship) into ONE denser fact that preserves every piece of information from all of them.
- Leave facts about unrelated topics unchanged, each on its own line.
- Never drop, guess, or invent information. Every detail present in the input must still be present in the output somewhere.
- Each output line must be a short, self-contained, third-person statement (e.g. "User's dog is named Zeytin", not "my dog is named Zeytin").
- Output ONLY the resulting facts, one per line. No numbering, no markdown, no explanation, no headers.`

// dreamPinnedFactsLLM sends the entire current pinned-facts set to the
// active model in one batch and asks it to rewrite the set as a whole,
// compressing facts about the same topic together — see runDream's doc
// comment (internal/memory/store.go) for why this needs a batch rewrite
// rather than mergeMemoriesLLM's pairwise shape. Used as the Store's
// DreamFunc.
//
// Routes through a.callLLM (Orchestra → external provider → local model),
// not a.providerRouter directly — a local-only setup with no external
// provider configured gets a nil router, so bypassing callLLM would mean
// this silently never runs (see extractAndPinFacts' doc comment and
// AGENTS.md's Memory / Vector Store notes for the same anti-pattern found
// and fixed twice before). mergeMemoriesLLM above used to have this exact
// bug too — fixed the same way.
func (a *App) dreamPinnedFactsLLM(ctx context.Context, facts []string) ([]string, error) {
	var sb strings.Builder
	for _, f := range facts {
		sb.WriteString("- ")
		sb.WriteString(f)
		sb.WriteString("\n")
	}

	msgs := []api.Message{
		api.NewTextMessage("system", dreamSystemPrompt),
		api.NewTextMessage("user", sb.String()),
	}
	reply := a.callLLM(ctx, msgs, categoryDream)
	if isLLMErrorReply(reply) {
		return nil, fmt.Errorf("dream LLM call: %s", reply)
	}

	var out []string
	for _, line := range strings.Split(reply, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out, nil
}

// SetMemoryEnabled toggles the memory feature.
// When enabling, EmbeddingAutoStart is persisted so the embedding model
// auto-starts on the next restart as well. If the embedding server is not
// already running and the model repo/file are configured, the download-and-
// start sequence runs immediately in the background.
func (a *App) SetMemoryEnabled(enabled bool) error {
	a.cfgMu.Lock()
	a.cfg.Memory.MemoryEnabled = enabled
	if enabled {
		a.cfg.Memory.EmbeddingAutoStart = true
	}
	a.cfgMu.Unlock()
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	if enabled &&
		a.cfg.Memory.EmbeddingModelRepo != "" &&
		a.cfg.Memory.EmbeddingModelFile != "" &&
		!a.llamaEmbedServer.IsRunning() {
		goRecover("startupEmbeddingModel", a.startupEmbeddingModel)
	}
	return nil
}

// SaveExplicitMemory saves a user-provided memory entry.
func (a *App) SaveExplicitMemory(content, tags string) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		return fmt.Errorf("memory store not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return a.store.SaveExplicit(ctx, content, tags)
}

// DeleteExplicitMemory deletes memories matching the given pattern.
func (a *App) DeleteExplicitMemory(pattern string) (int, error) {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		return 0, fmt.Errorf("memory store not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.store.DeleteByContent(ctx, pattern)
}

// ExportMemories exports all memories as JSON bytes.
func (a *App) ExportMemories() ([]byte, error) {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return nil, fmt.Errorf("memory store not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return a.store.Export(ctx)
}

// ImportMemories imports memories from JSON bytes.
func (a *App) ImportMemories(data []byte) (int, error) {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return 0, fmt.Errorf("memory store not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	return a.store.Import(ctx, data)
}

func (a *App) GetMemoryStats() models.MemoryStats {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return models.MemoryStats{}
	}
	return a.store.Stats()
}

func (a *App) FilteredMemorySearch(query string, topK int, since string, tag string) []memory.MemoryResult {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var sinceTime time.Time
	if since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			sinceTime = t
		}
	}
	results, err := a.store.FilteredSearch(ctx, query, topK, a.cfg.Memory.MinSimilarity, sinceTime, tag)
	if err != nil {
		logx.Printf("MEMORY: filtered search: %v", err)
		return nil
	}
	return results
}

// CheckEmbeddingHealth tests if the embedding API is reachable and working.
func (a *App) CheckEmbeddingHealth(ctx context.Context) map[string]interface{} {
	result := map[string]interface{}{
		"ok":    false,
		"error": "",
		"count": 0,
	}

	a.storeMu.RLock()
	defer a.storeMu.RUnlock()

	if a.store == nil {
		result["error"] = "memory store not initialized"
		return result
	}

	result["count"] = a.store.Count()

	a.clientMu.RLock()
	client := a.client
	if a.embeddingClient != nil {
		client = a.embeddingClient
	}
	a.clientMu.RUnlock()

	ectx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := client.CreateEmbedding(ectx, a.cfg.API.EmbeddingModel, "test")
	if err != nil {
		result["error"] = err.Error()
		logx.Printf("EMBEDDING HEALTH CHECK FAILED: %v", err)
		return result
	}

	result["ok"] = true
	logx.Printf("Embedding health: OK (model=%s, memories=%d)", a.cfg.API.EmbeddingModel, a.store.Count())
	return result
}

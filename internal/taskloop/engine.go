package taskloop

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/logx"
	"memo/internal/truncate"
	"strings"
	"sync"
	"time"
)

type RunWorker func(ctx context.Context, chatID, prompt string) (string, error)
type ReviewChief func(ctx context.Context, itemText, workerOutput string) (approved bool, feedback string, err error)
type BypassSetter func(bool)

type Engine struct {
	store       *Store
	runWorker   RunWorker
	reviewChief ReviewChief
	setBypass   BypassSetter
	onEvent     func(name, data string)
	mu          sync.Mutex
	activeCount int
	active      map[string]context.CancelFunc

	// Optional hooks wired via EngineOption. All nil-safe.
	ruleReader       func(projectRoot string) (string, error)
	systemGuidance   func() string
	projectPathFn    func(chatID string) string
	workerConfigHook func(ctx context.Context, listID string) context.Context
	selfHeal         func(ctx context.Context, listID string, workerErr error) bool
	planConfig       func(ctx context.Context, listID, chatID string, items []string) error
	retry            *RetryScheduler
	subOrch          *SubAgentOrchestrator
	subSpecs         func(itemText, feedback string) []SubAgentSpec

	rtMu     sync.RWMutex
	runtimes map[string]*listRuntime

	skipReq map[string]bool // list IDs whose current item the user asked to skip
}

// EngineOption configures optional Engine behaviour without breaking the
// positional constructor.
type EngineOption func(*Engine)

// WithRuleReader supplies the function the planning phase uses to read a
// project's own rule files (AGENTS.md / CLAUDE.md / …). Without it the loop
// simply skips repo-rule injection.
func WithRuleReader(fn func(projectRoot string) (string, error)) EngineOption {
	return func(e *Engine) { e.ruleReader = fn }
}

// WithSystemGuidance supplies the built-in memo-system skill text prepended to
// every worker item during planning.
func WithSystemGuidance(fn func() string) EngineOption {
	return func(e *Engine) { e.systemGuidance = fn }
}

// WithProjectPathFn resolves a task list's agent chat ID to the on-disk
// project root, so the planning phase knows where to look for rule files.
func WithProjectPathFn(fn func(chatID string) string) EngineOption {
	return func(e *Engine) { e.projectPathFn = fn }
}

// WithWorkerConfigHook lets the host wrap the context handed to each worker
// turn — used to attach a task-local provider/model snapshot so the loop runs
// against a fixed provider independent of the user's global setting. Nil-safe;
// without it the worker turn uses whatever the host's default path resolves.
func WithWorkerConfigHook(fn func(ctx context.Context, listID string) context.Context) EngineOption {
	return func(e *Engine) { e.workerConfigHook = fn }
}

// WithSelfHeal supplies a callback invoked when a worker turn errors. If it
// returns true it has repaired the task's config (e.g. switched to another
// provider) and the item is retried; false means the error stands and the
// item is marked stuck. Nil-safe.
func WithSelfHeal(fn func(ctx context.Context, listID string, workerErr error) bool) EngineOption {
	return func(e *Engine) { e.selfHeal = fn }
}

// WithPlanConfig supplies a callback run once during the planning phase,
// before any item executes, letting the host choose this task's provider /
// model / Orchestra setup autonomously. Its error is logged and ignored — a
// planning-config failure must never block the task. Nil-safe.
func WithPlanConfig(fn func(ctx context.Context, listID, chatID string, items []string) error) EngineOption {
	return func(e *Engine) { e.planConfig = fn }
}

// WithRetryScheduler enables rate-limit handling: on a 429/quota worker error
// the list is parked in "waiting-limit" and a one-shot timer (interval, or
// DefaultRetryInterval when <= 0) later re-enters the loop from the same item.
func WithRetryScheduler(interval time.Duration) EngineOption {
	return func(e *Engine) {
		e.retry = NewRetryScheduler(interval, func(listID string) {
			if err := e.Start(context.Background(), listID); err != nil {
				logx.Printf("TASKLOOP: retry resume %s: %v", listID, err)
			}
		})
	}
}

// WithSubAgents enables fan-out: for an item shouldSpawn() considers large,
// the loop runs specFn(itemText, lastFeedback) sub-agents through orch instead
// of a single worker turn, then hands the aggregated output to the chief
// review. Both must be non-nil to take effect.
func WithSubAgents(orch *SubAgentOrchestrator, specFn func(itemText, feedback string) []SubAgentSpec) EngineOption {
	return func(e *Engine) {
		e.subOrch = orch
		e.subSpecs = specFn
	}
}

// ArmRetry parks listID's wait timer without a preceding worker error — used
// on startup to re-arm lists found persisted in "waiting-limit". No-op if no
// retry scheduler is configured.
func (e *Engine) ArmRetry(listID string) {
	e.retry.Arm(listID)
}

func NewEngine(store *Store, runWorker RunWorker, reviewChief ReviewChief, setBypass BypassSetter, onEvent func(name, data string), opts ...EngineOption) *Engine {
	e := &Engine{
		store:       store,
		runWorker:   runWorker,
		reviewChief: reviewChief,
		setBypass:   setBypass,
		onEvent:     onEvent,
		active:      make(map[string]context.CancelFunc),
		skipReq:     make(map[string]bool),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Engine) Start(ctx context.Context, listID string) error {
	e.mu.Lock()
	if _, running := e.active[listID]; running {
		e.mu.Unlock()
		return fmt.Errorf("tasklist %s zaten çalışıyor", listID)
	}
	listCtx, cancel := context.WithCancel(ctx)
	e.active[listID] = cancel
	e.activeCount++
	shouldBypass := e.activeCount == 1
	e.mu.Unlock()

	if shouldBypass {
		e.setBypass(true)
		if e.onEvent != nil {
			e.onEvent("taskloop:bypass_enabled", "araç izinleri otomatik onaylanıyor")
		}
	}

	go e.run(listCtx, listID)
	return nil
}

func (e *Engine) Stop(listID string) {
	// Cancel a pending rate-limit retry even if the list isn't actively
	// running (a suspended list is not in e.active).
	e.retry.Cancel(listID)

	e.mu.Lock()
	cancel, ok := e.active[listID]
	if !ok {
		e.mu.Unlock()
		// Not running, but it may be parked in waiting-limit/waiting-user —
		// still move it to paused so the UI shows a stopped list.
		if tl, err := e.store.Get(listID); err == nil {
			switch tl.Status {
			case taskListWaitingLimit, taskListWaitingUser:
				e.store.SetStatus(listID, taskListPaused)
				if e.onEvent != nil {
					e.onEvent("taskloop:paused", listID)
				}
			}
		}
		return
	}
	cancel()
	delete(e.active, listID)
	e.mu.Unlock()

	e.store.SetStatus(listID, "paused")
	if e.onEvent != nil {
		e.onEvent("taskloop:paused", listID)
	}
}

func (e *Engine) IsRunning(listID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.active[listID]
	return ok
}

// SkipCurrent abandons the item a running list is on and continues with the
// rest. If the list isn't running, the first pending item is marked stuck.
func (e *Engine) SkipCurrent(listID string) error {
	e.mu.Lock()
	cancel, running := e.active[listID]
	if running {
		e.skipReq[listID] = true
		cancel()
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()

	tl, err := e.store.Get(listID)
	if err != nil {
		return err
	}
	for _, it := range tl.Items {
		if it.Status == "pending" || it.Status == "running" {
			return e.store.SetItemStuck(listID, it.ID, "kullanıcı tarafından atlandı")
		}
	}
	return nil
}

func (e *Engine) run(ctx context.Context, listID string) {
	defer func() {
		e.mu.Lock()
		delete(e.active, listID)
		e.activeCount--
		shouldRestore := e.activeCount == 0
		e.mu.Unlock()

		if shouldRestore {
			e.setBypass(false)
			if e.onEvent != nil {
				e.onEvent("taskloop:bypass_disabled", "araç izinleri normale döndü")
			}
		}
	}()

	// A panic anywhere in the injected runWorker/reviewChief callbacks (or in
	// store/logic code below) must not take down the whole app — just this
	// one list.
	defer func() {
		if r := recover(); r != nil {
			logx.Printf("TASKLOOP: panic in list %s: %v", listID, r)
			if err := e.store.SetStatus(listID, "paused"); err != nil {
				logx.Printf("TASKLOOP: set list paused after panic %s: %v", listID, err)
			}
		}
	}()

	tl, err := e.store.Get(listID)
	if err != nil {
		logx.Printf("TASKLOOP: list %s not found: %v", listID, err)
		return
	}

	e.rtStart(listID)
	defer e.rtEnd(listID)

	// Planning phase: gather the repo's own rules and the memo-system
	// guidance into a preamble prepended to every worker item.
	if err := e.store.SetStatus(listID, taskListPlanning); err != nil {
		logx.Printf("TASKLOOP: set list planning %s: %v", listID, err)
	}
	if e.onEvent != nil {
		e.onEvent("taskloop:planning", listID)
	}
	preamble := e.buildPlanningPreamble(tl)

	if e.planConfig != nil {
		itemTexts := make([]string, len(tl.Items))
		for i, it := range tl.Items {
			itemTexts[i] = it.Text
		}
		if err := e.planConfig(ctx, listID, tl.ChatID, itemTexts); err != nil {
			logx.Printf("TASKLOOP: plan-config for %s: %v (continuing)", listID, err)
		}
	}

	select {
	case <-ctx.Done():
		if err := e.store.SetStatus(listID, taskListPaused); err != nil {
			logx.Printf("TASKLOOP: set list paused %s: %v", listID, err)
		}
		return
	default:
	}

	if err := e.store.SetStatus(listID, taskListExecuting); err != nil {
		logx.Printf("TASKLOOP: set list executing %s: %v", listID, err)
	}

	for i := range tl.Items {
		select {
		case <-ctx.Done():
			if err := e.store.SetStatus(listID, "paused"); err != nil {
				logx.Printf("TASKLOOP: set list paused %s: %v", listID, err)
			}
			return
		default:
		}

		item := &tl.Items[i]
		if item.Status == "done" || item.Status == "stuck" {
			continue
		}

		e.rtSetItem(listID, item.Text)
		if e.onEvent != nil {
			e.onEvent("tasklist:item_started", fmt.Sprintf("%s:%s", listID, item.ID))
		}

		if err := e.store.SetItemRunning(listID, item.ID); err != nil {
			logx.Printf("TASKLOOP: set item running %s/%s: %v", listID, item.ID, err)
		}

		itemCtx := ctx
		if e.workerConfigHook != nil {
			itemCtx = e.workerConfigHook(ctx, listID)
		}
		ok, cancelled, suspended := e.processItem(itemCtx, listID, item, tl.ChatID, preamble)

		if suspended {
			// Rate-limited: processItem parked the list in waiting-limit,
			// reset the item to pending, and armed the retry timer. Unwind
			// this run; the timer will re-enter from the same item.
			return
		}

		if cancelled {
			e.mu.Lock()
			skip := e.skipReq[listID]
			delete(e.skipReq, listID)
			e.mu.Unlock()
			if skip {
				// User asked to skip this item: mark it stuck and re-enter to
				// continue with the rest (a fresh Start, since our ctx is now
				// cancelled).
				if err := e.store.SetItemStuck(listID, item.ID, "kullanıcı tarafından atlandı"); err != nil {
					logx.Printf("TASKLOOP: skip mark stuck %s/%s: %v", listID, item.ID, err)
				}
				if e.onEvent != nil {
					e.onEvent("tasklist:item_stuck", fmt.Sprintf("%s:%s", listID, item.ID))
				}
				go func() {
					time.Sleep(150 * time.Millisecond) // let this run's deferred cleanup finish
					if err := e.Start(context.Background(), listID); err != nil {
						logx.Printf("TASKLOOP: restart after skip %s: %v", listID, err)
					}
				}()
				return
			}
			// Interrupted (Stop() or shutdown), not actually failed — put the
			// item back so a resumed run retries it instead of skipping it
			// forever.
			if err := e.store.ResetItemPending(listID, item.ID); err != nil {
				logx.Printf("TASKLOOP: reset item pending %s/%s: %v", listID, item.ID, err)
			}
			if err := e.store.SetStatus(listID, "paused"); err != nil {
				logx.Printf("TASKLOOP: set list paused %s: %v", listID, err)
			}
			return
		}

		if ok {
			if err := e.store.SetItemDone(listID, item.ID); err != nil {
				logx.Printf("TASKLOOP: set item done %s/%s: %v", listID, item.ID, err)
			}
			// Mirror the completion back into the source Task.md ("[ ]" -> "[x]").
			// Best-effort: a file that moved or is read-only must not fail the task.
			if tl.TaskMdPath != "" && item.Line > 0 {
				if err := MarkItemDone(tl.TaskMdPath, item.Line); err != nil {
					logx.Printf("TASKLOOP: mirror %s line %d: %v", tl.TaskMdPath, item.Line, err)
				}
			}
			if e.onEvent != nil {
				e.onEvent("tasklist:item_done", fmt.Sprintf("%s:%s", listID, item.ID))
			}
		} else {
			if err := e.store.SetItemStuck(listID, item.ID, item.Note); err != nil {
				logx.Printf("TASKLOOP: set item stuck %s/%s: %v", listID, item.ID, err)
			}
			if e.onEvent != nil {
				e.onEvent("tasklist:item_stuck", fmt.Sprintf("%s:%s", listID, item.ID))
			}
		}
	}

	// Terminal state: "done" if at least one item completed, "failed" if every
	// attempted item is stuck. An empty list is trivially "done".
	final := taskListDone
	if fresh, err := e.store.Get(listID); err == nil {
		anyDone, anyStuck := false, false
		for _, it := range fresh.Items {
			switch it.Status {
			case "done":
				anyDone = true
			case "stuck":
				anyStuck = true
			}
		}
		if !anyDone && anyStuck {
			final = taskListFailed
		}
	}
	if err := e.store.SetStatus(listID, final); err != nil {
		logx.Printf("TASKLOOP: set list %s %s: %v", final, listID, err)
	}
	if e.onEvent != nil {
		e.onEvent("tasklist:finished", fmt.Sprintf("%s:%s", listID, final))
	}
}

// buildPlanningPreamble assembles the standing instructions prepended to the
// first worker turn of every item: the repo's own rules, then the built-in
// memo-system guidance, then a one-line orientation.
func (e *Engine) buildPlanningPreamble(tl *TaskList) string {
	var b strings.Builder
	if e.projectPathFn != nil && e.ruleReader != nil {
		if root := e.projectPathFn(tl.ChatID); root != "" {
			if rules, err := e.ruleReader(root); err != nil {
				logx.Printf("TASKLOOP: read rules for %s: %v", tl.ID, err)
			} else if rules != "" {
				b.WriteString("# Repo kuralları (bu görevde bunlara uy)\n\n")
				b.WriteString(rules)
				b.WriteString("\n\n")
			}
		}
	}
	if e.systemGuidance != nil {
		if g := strings.TrimSpace(e.systemGuidance()); g != "" {
			b.WriteString("# Memo öz-yönetim rehberi\n\n")
			b.WriteString(g)
			b.WriteString("\n\n")
		}
	}
	if b.Len() > 0 {
		fmt.Fprintf(&b, "# Görev\n\nBu bir Self-Driving görev listesi (%d madde). Aşağıdaki maddeyi eksiksiz tamamla; commit davranışı için repo kurallarına uy.\n\n",
			len(tl.Items))
	}
	return b.String()
}

// processItem drives one task item through worker/CEO rounds.
//   - ok:        the item was approved
//   - cancelled: ctx was cancelled mid-item (Stop()/shutdown) — an
//     interruption, not a failure; the caller leaves the item resumable
//   - suspended: a rate-limit error parked the list in waiting-limit and armed
//     the retry timer; the caller must unwind the run without a terminal state
func (e *Engine) processItem(ctx context.Context, listID string, item *TaskItem, chatID, preamble string) (ok, cancelled, suspended bool) {
	withPreamble := func(s string) string {
		if preamble == "" {
			return s
		}
		return preamble + s
	}
	workerPrompt := withPreamble(item.Text)
	useSubAgents := e.subOrch != nil && e.subSpecs != nil && shouldSpawn(item.Text)
	lastFeedback := ""

	for round := 1; round <= maxRoundsPerItem; round++ {
		select {
		case <-ctx.Done():
			return false, true, false
		default:
		}

		logx.Printf("TASKLOOP: item %s round %d/%d (subagents=%v)", item.ID, round, maxRoundsPerItem, useSubAgents)

		var workerOutput string
		var err error
		if useSubAgents {
			specs := e.subSpecs(item.Text, lastFeedback)
			roles := make([]string, len(specs))
			for i, s := range specs {
				roles[i] = string(s.Role)
			}
			e.rtAddSubAgents(listID, roles...)
			if e.onEvent != nil {
				e.onEvent("tasklist:subagent_spawned", fmt.Sprintf("%s:%s:%d", listID, item.ID, len(specs)))
			}
			results, serr := e.subOrch.Spawn(ctx, item.Text, specs)
			if serr != nil {
				err = serr
			} else {
				workerOutput = AggregateResults(results)
				allFailed := len(results) > 0
				for _, r := range results {
					if r.Err == nil {
						allFailed = false
						break
					}
				}
				if allFailed {
					err = fmt.Errorf("tüm alt-agent'lar başarısız oldu")
				}
			}
		} else {
			workerOutput, err = e.runWorker(ctx, chatID, workerPrompt)
		}
		if err != nil {
			if ctx.Err() != nil {
				return false, true, false
			}
			logx.Printf("TASKLOOP: worker error on item %s round %d: %v", item.ID, round, err)
			if e.retry != nil && IsRateLimitErr(err) {
				if suspErr := e.suspendForRateLimit(listID, item, err); suspErr != nil {
					logx.Printf("TASKLOOP: suspend %s: %v", listID, suspErr)
				}
				return false, false, true
			}
			if e.selfHeal != nil && e.selfHeal(ctx, listID, err) {
				logx.Printf("TASKLOOP: self-heal recovered list %s, retrying item %s", listID, item.ID)
				if err := e.store.IncrementRounds(listID, item.ID); err != nil {
					logx.Printf("TASKLOOP: increment rounds %s/%s: %v", listID, item.ID, err)
				}
				continue
			}
			// Self-heal declined an auth failure -> every configured provider is
			// dead. Park the list in waiting-user (not stuck/failed): the item
			// goes back to pending and the run unwinds without a terminal state,
			// so a task_resume retries it once the user fixes a key.
			if IsAuthErr(err) {
				logx.Printf("TASKLOOP: providers exhausted for %s, parking in waiting-user", listID)
				e.parkWaitingUser(listID, item, err)
				return false, false, true
			}
			item.Note = fmt.Sprintf("İşçi hatası (tur %d): %v", round, err)
			return false, false, false
		}

		if workerOutput == "" {
			item.Note = fmt.Sprintf("İşçi boş çıktı döndü (tur %d)", round)
			return false, false, false
		}

		approved, feedback, err := e.reviewChief(ctx, item.Text, workerOutput)
		if err != nil {
			if ctx.Err() != nil {
				return false, true, false
			}
			logx.Printf("TASKLOOP: CEO review error on item %s round %d: %v", item.ID, round, err)
			if round < maxRoundsPerItem {
				workerPrompt = withPreamble(item.Text + "\n\nÖnceki çıktı:\n" + truncate.Text(workerOutput, 2000) + "\n\nEksik/yanlış: CEO yanıtı anlaşılamadı. Lütfen görevi eksiksiz tamamlayıp tekrar dene.")
				if err := e.store.IncrementRounds(listID, item.ID); err != nil {
					logx.Printf("TASKLOOP: increment rounds %s/%s: %v", listID, item.ID, err)
				}
				continue
			}
			item.Note = fmt.Sprintf("CEO inceleme hatası (tur %d): %v", round, err)
			return false, false, false
		}

		if approved {
			return true, false, false
		}

		if err := e.store.IncrementRounds(listID, item.ID); err != nil {
			logx.Printf("TASKLOOP: increment rounds %s/%s: %v", listID, item.ID, err)
		}

		if round >= maxRoundsPerItem {
			item.Note = fmt.Sprintf("5 tur sonunda onaylanmadı: %s", feedback)
			return false, false, false
		}

		lastFeedback = feedback
		workerPrompt = withPreamble(fmt.Sprintf(
			"Madde: %s\n\nÖnceki çıktı:\n%s\n\nCEO'nun eksik/yanlış buldukları:\n%s\n\nBu eksikleri gider, hataları düzelt ve görevi eksiksiz tamamla.",
			item.Text,
			truncate.Text(workerOutput, 2000),
			feedback,
		))
	}

	item.Note = "maksimum tur sayısına ulaşıldı"
	return false, false, false
}

// parkWaitingUser puts a list into waiting-user when self-heal has exhausted
// every provider: the current item goes back to pending and a waiting_user
// event fires. The user resumes it (task_resume) after fixing a key.
func (e *Engine) parkWaitingUser(listID string, item *TaskItem, cause error) {
	if err := e.store.ResetItemPending(listID, item.ID); err != nil {
		logx.Printf("TASKLOOP: park reset item %s/%s: %v", listID, item.ID, err)
	}
	if err := e.store.SetStatus(listID, taskListWaitingUser); err != nil {
		logx.Printf("TASKLOOP: park set status %s: %v", listID, err)
	}
	if e.onEvent != nil {
		e.onEvent("taskloop:waiting_user", fmt.Sprintf("%s:%v", listID, cause))
	}
}

// suspendForRateLimit parks a rate-limited list: item back to pending, list
// status waiting-limit, a waiting_limit event carrying any "try again in Ns"
// hint, and the retry timer armed to re-enter the loop later.
func (e *Engine) suspendForRateLimit(listID string, item *TaskItem, workerErr error) error {
	if err := e.store.ResetItemPending(listID, item.ID); err != nil {
		return err
	}
	if err := e.store.SetStatus(listID, taskListWaitingLimit); err != nil {
		return err
	}
	if e.onEvent != nil {
		detail := ""
		if s := RetryAfterSeconds(workerErr); s > 0 {
			detail = fmt.Sprintf("~%ds", s)
		}
		e.onEvent("taskloop:waiting_limit", fmt.Sprintf("%s:%s", listID, detail))
	}
	e.retry.Arm(listID)
	return nil
}

func ChiefReviewSystemPrompt() string {
	return `Sen bağımsız bir görev denetleyicisisin. Sana bir işçi ajanın yaptığı işin sonucu gösterilecek.
Görevin: İşçinin çıktısını ORİJİNAL görev maddesine göre incele, eksiksiz ve doğru olup olmadığına karar ver.

Kararını şu JSON formatında ver (sadece JSON, başka bir şey yazma):
{"approved": true, "feedback": ""}

Eğer onaylıyorsan feedback boş olabilir. Eğer onaylamıyorsan feedback'te EKSİK ve YANLIŞ olanları somut olarak belirt (işçiye geri bildirilecek, düzeltebilmesi için net ol). Kısa ve öz ol.`
}

func ChiefReviewPrompt(itemText, workerOutput string) string {
	return fmt.Sprintf(
		"Orijinal görev maddesi:\n%s\n\nİşçinin ürettiği çıktı:\n%s\n\nİncele ve JSON olarak kararını ver.",
		itemText,
		truncate.Text(workerOutput, 8000),
	)
}

func ExtractAndParseReview(raw string) (approved bool, feedback string, err error) {
	cleaned := extractJSON(raw)

	var result struct {
		Approved bool   `json:"approved"`
		Feedback string `json:"feedback"`
	}
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return false, "", fmt.Errorf("JSON ayrıştırılamadı: %w (ham: %s)", err, truncate.Text(cleaned, 200))
	}
	return result.Approved, result.Feedback, nil
}

func extractJSON(text string) string {
	if idx := strings.Index(text, "```json"); idx >= 0 {
		start := idx + 7
		if end := strings.Index(text[start:], "```"); end >= 0 {
			return strings.TrimSpace(text[start : start+end])
		}
	}
	if idx := strings.Index(text, "```"); idx >= 0 {
		start := idx + 3
		if nl := strings.Index(text[start:], "\n"); nl >= 0 {
			start += nl + 1
		}
		if end := strings.Index(text[start:], "```"); end >= 0 {
			extracted := strings.TrimSpace(text[start : start+end])
			if strings.HasPrefix(extracted, "{") || strings.HasPrefix(extracted, "[") {
				return extracted
			}
		}
	}
	braceIdx := strings.Index(text, "{")
	bracketIdx := strings.Index(text, "[")
	if braceIdx >= 0 && (bracketIdx < 0 || braceIdx < bracketIdx) {
		if extracted, ok := scanBalanced(text, braceIdx, '{', '}'); ok {
			return extracted
		}
	}
	if bracketIdx >= 0 && (braceIdx < 0 || bracketIdx < braceIdx) {
		if extracted, ok := scanBalanced(text, bracketIdx, '[', ']'); ok {
			return extracted
		}
	}
	return text
}

// scanBalanced returns the substring of text starting at start (the opening
// byte) through its matching closing byte, treating JSON string literals as
// opaque. A naive depth counter miscounts when a quoted string value (e.g. CEO
// feedback like `"Kapanış } eksik"`) contains a literal brace/bracket.
func scanBalanced(text string, start int, open, close byte) (string, bool) {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(text); i++ {
		c := text[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return text[start : i+1], true
			}
		}
	}
	return "", false
}

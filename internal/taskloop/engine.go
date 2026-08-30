package taskloop

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/logx"
	"memo/internal/truncate"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RunWorker func(ctx context.Context, chatID, prompt string) (string, error)
type ReviewChief func(ctx context.Context, itemText, workerOutput string) (approved bool, feedback string, err error)
type BypassSetter func(bool)

// Planner turns a task list's items into a reviewed Plan for "planlayıcı"
// mode. projectRoot / preamble carry the repo rules already assembled by
// buildPlanningPreamble; granularity is "intent" | "literal" | "hybrid".
type Planner func(ctx context.Context, listID, chatID, projectRoot, preamble string, items []string, granularity string) (*Plan, error)

// StepRunner executes one plan step (a fresh ephemeral coder turn). stateDoc
// is the running project-state handoff (empty until Phase B wires it).
type StepRunner func(ctx context.Context, listID string, step PlanStep, stateDoc string) (output string, err error)

// AcceptanceChecker verifies a step's acceptance checks after the coder ran.
type AcceptanceChecker func(ctx context.Context, listID string, step PlanStep) (pass bool, detail string, err error)

// Escalator re-plans one failed step (a targeted cloud call). It returns the
// replacement steps, or an error. A network error signals "offline" — the
// engine then parks the list in waiting-escalation and retries on resume.
type Escalator func(ctx context.Context, listID string, step PlanStep, failure EscalationInput) ([]PlanStep, error)

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

	// v4.5.0 planner/executor mode (all nil/zero-safe; only used when a list's
	// Mode == ModePlanner and a Planner is configured).
	planner        Planner
	stepRunner     StepRunner
	acceptChecker  AcceptanceChecker
	stateCompactor func(ctx context.Context, listID, current string) (string, error)
	escalator      Escalator
	granularity    string
	autoApprove    bool
	maxPar         int
	maxAttempts    int
	stateMaxTokens int

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

// WithPlanner enables "planlayıcı" mode: for a list whose Mode == ModePlanner
// the loop asks fn for a Plan, writes Plan.md, waits for approval (unless
// WithAutoApprovePlan), then runs the plan's steps via the StepRunner.
func WithPlanner(fn Planner) EngineOption { return func(e *Engine) { e.planner = fn } }

// WithStepRunner supplies the per-step coder turn for planner mode.
func WithStepRunner(fn StepRunner) EngineOption { return func(e *Engine) { e.stepRunner = fn } }

// WithAcceptanceChecker supplies the post-step verification for planner mode.
func WithAcceptanceChecker(fn AcceptanceChecker) EngineOption {
	return func(e *Engine) { e.acceptChecker = fn }
}

// WithGranularity sets the planner's step granularity ("intent"|"literal"|"hybrid").
func WithGranularity(g string) EngineOption { return func(e *Engine) { e.granularity = g } }

// WithAutoApprovePlan skips the plan approval gate.
func WithAutoApprovePlan(v bool) EngineOption { return func(e *Engine) { e.autoApprove = v } }

// WithMaxParallelSteps caps how many ready steps run at once (default 1).
func WithMaxParallelSteps(n int) EngineOption {
	return func(e *Engine) {
		if n > 0 {
			e.maxPar = n
		}
	}
}

// WithStateCompactor supplies a summariser for the running state doc, called
// when it grows past WithStateMaxTokens. Without it an oversized state doc is
// just truncated.
func WithStateCompactor(fn func(ctx context.Context, listID, current string) (string, error)) EngineOption {
	return func(e *Engine) { e.stateCompactor = fn }
}

// WithStateMaxTokens sets the state-doc size (approx tokens) that triggers
// compaction. 0 disables it.
func WithStateMaxTokens(n int) EngineOption { return func(e *Engine) { e.stateMaxTokens = n } }

// WithEscalator enables the escalation valve: after a step has failed
// WithMaxExecutorAttempts times it is re-planned by fn instead of just going
// stuck. Requires a RetryScheduler for the offline-park path.
func WithEscalator(fn Escalator) EngineOption { return func(e *Engine) { e.escalator = fn } }

// WithMaxExecutorAttempts sets how many times a step's coder turn may fail
// before it escalates (default 3).
func WithMaxExecutorAttempts(n int) EngineOption {
	return func(e *Engine) {
		if n > 0 {
			e.maxAttempts = n
		}
	}
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

	// v4.5.0: a planner/executor list takes a completely separate path. The
	// worker-mode body below is byte-for-byte unchanged.
	if tl.Mode == ModePlanner && e.planner != nil {
		e.runPlanExec(ctx, listID, tl)
		return
	}

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

// ---- v4.5.0 planner/executor mode -------------------------------------------

// runPlanExec drives a Mode==ModePlanner list: plan (once) -> approval gate ->
// execute the plan's steps. Re-entrant: on a resume after approval a plan
// already exists, so it skips straight to execution.
func (e *Engine) runPlanExec(ctx context.Context, listID string, tl *TaskList) {
	if !e.store.HasPlan(listID) {
		if err := e.store.SetStatus(listID, taskListPlanning); err != nil {
			logx.Printf("TASKLOOP: set planning %s: %v", listID, err)
		}
		e.emitEvent("taskloop:planning", listID)

		preamble := e.buildPlanningPreamble(tl)
		root := ""
		if e.projectPathFn != nil {
			root = e.projectPathFn(tl.ChatID)
		}
		plan, err := e.planner(ctx, listID, tl.ChatID, root, preamble, itemTexts(tl), e.granularity)
		if err != nil {
			e.failPlan(listID, "planlama başarısız: "+err.Error())
			return
		}
		if err := plan.Normalize(); err != nil {
			e.failPlan(listID, "geçersiz plan: "+err.Error())
			return
		}
		plan.ListID = listID
		if err := e.store.SavePlan(listID, *plan); err != nil {
			e.failPlan(listID, err.Error())
			return
		}
		if tl.TaskMdPath != "" {
			if err := WritePlanMd(PlanMdPath(tl.TaskMdPath), *plan, itemTextMap(tl)); err != nil {
				logx.Printf("TASKLOOP: write Plan.md %s: %v", listID, err)
			}
		}
		if !e.autoApprove {
			if err := e.store.SetStatus(listID, taskListAwaitingPlan); err != nil {
				logx.Printf("TASKLOOP: set awaiting-plan %s: %v", listID, err)
			}
			e.emitEvent("taskloop:awaiting_plan", listID)
			return
		}
	}

	select {
	case <-ctx.Done():
		_ = e.store.SetStatus(listID, taskListPaused)
		return
	default:
	}
	e.executePlan(ctx, listID, tl)
}

// ApprovePlan moves an awaiting-plan-approval list into execution. It re-reads
// Plan.md first (the user may have edited it) and starts the loop.
func (e *Engine) ApprovePlan(ctx context.Context, listID string) error {
	tl, err := e.store.Get(listID)
	if err != nil {
		return err
	}
	if tl.Status != taskListAwaitingPlan {
		return fmt.Errorf("tasklist %s onay bekleyen durumda değil (%s)", listID, tl.Status)
	}
	if tl.TaskMdPath != "" {
		if p, perr := ParsePlanMd(PlanMdPath(tl.TaskMdPath)); perr == nil {
			p.ListID = listID
			if serr := e.store.SavePlan(listID, *p); serr != nil {
				return serr
			}
		} else {
			logx.Printf("TASKLOOP: ApprovePlan re-read Plan.md %s: %v (keeping stored plan)", listID, perr)
		}
	}
	if err := e.store.SetStatus(listID, taskListExecuting); err != nil {
		return err
	}
	return e.Start(ctx, listID)
}

// executePlan runs the plan's steps in dependency order: each wave takes the
// ready set (pending steps whose deps are done) and runs it, up to maxPar at
// once. A running state doc is threaded to every coder turn and compacted when
// it grows too large.
func (e *Engine) executePlan(ctx context.Context, listID string, tl *TaskList) {
	if err := e.store.SetStatus(listID, taskListExecuting); err != nil {
		logx.Printf("TASKLOOP: set executing %s: %v", listID, err)
	}
	e.emitEvent("taskloop:executing", listID)

	if e.stepRunner == nil {
		e.failPlan(listID, "executor yapılandırılmamış (WithStepRunner)")
		return
	}
	plan, err := e.store.GetPlan(listID)
	if err != nil {
		e.failPlan(listID, err.Error())
		return
	}

	par := e.maxPar
	if par < 1 {
		par = 1
	}
	state := e.store.GetState(listID)

	// Resume path: a step failed offline last time and the list was parked.
	if plan.PendingEscalation != nil {
		if !e.resumePendingEscalation(ctx, listID, tl, plan) {
			return // still offline — re-armed and parked
		}
		reloaded, gerr := e.store.GetPlan(listID)
		if gerr != nil {
			e.failPlan(listID, gerr.Error())
			return
		}
		plan = reloaded
	}

	for {
		select {
		case <-ctx.Done():
			_ = e.store.SetStatus(listID, taskListPaused)
			return
		default:
		}
		ready := plan.ReadySteps()
		if len(ready) == 0 {
			// Nothing runnable. Before giving up, try the escalation valve on
			// any stuck step that has exhausted its coder attempts.
			if e.escalator != nil {
				esc, parked := e.escalateStuckSteps(ctx, listID, tl, plan)
				if parked {
					return // offline — parked in waiting-escalation, retry armed
				}
				if esc {
					if r2, gerr := e.store.GetPlan(listID); gerr == nil {
						plan = r2
						continue
					}
				}
			}
			break
		}

		if e.stateMaxTokens > 0 && approxTokens(state) > e.stateMaxTokens && e.stateCompactor != nil {
			if c, cerr := e.stateCompactor(ctx, listID, state); cerr == nil && strings.TrimSpace(c) != "" {
				state = strings.TrimSpace(c)
				_ = e.store.SaveState(listID, state)
			}
		}

		type stepRes struct {
			id      string
			err     error
			summary string
		}
		results := make(chan stepRes, len(ready))
		sem := make(chan struct{}, par)
		var wg sync.WaitGroup
		for _, step := range ready {
			wg.Add(1)
			go func(step PlanStep) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				if ctx.Err() != nil {
					results <- stepRes{step.ID, ctx.Err(), ""}
					return
				}
				e.rtSetItem(listID, step.Text)
				out, rerr := e.runOneStep(ctx, listID, step, state)
				results <- stepRes{step.ID, rerr, summariseStep(step, out, rerr)}
			}(step)
		}
		wg.Wait()
		close(results)

		if ctx.Err() != nil {
			_ = e.store.SetStatus(listID, taskListPaused)
			return
		}

		progressed := false
		for r := range results {
			if r.err != nil {
				logx.Printf("TASKLOOP: step %s/%s stuck: %v", listID, r.id, r.err)
				_ = e.store.SetStepStatus(listID, r.id, "stuck", r.err.Error())
			} else {
				_ = e.store.SetStepStatus(listID, r.id, "done", "")
				e.emitEvent("taskloop:step_done", fmt.Sprintf("%s:%s", listID, r.id))
				progressed = true
			}
			if r.summary != "" {
				state = appendState(state, r.summary)
			}
		}
		_ = e.store.SaveState(listID, state)

		reloaded, gerr := e.store.GetPlan(listID)
		if gerr != nil {
			e.failPlan(listID, gerr.Error())
			return
		}
		plan = reloaded
		if !progressed {
			// No step advanced this wave; the top-of-loop empty-ready branch
			// (with its escalation attempt) decides whether to stop.
			continue
		}
	}

	e.finishPlanItems(listID, tl, plan)
}

// escalateStuckSteps re-plans the first stuck step that has run out of coder
// attempts. Returns (escalated, parkedOffline).
func (e *Engine) escalateStuckSteps(ctx context.Context, listID string, tl *TaskList, plan *Plan) (bool, bool) {
	for _, s := range plan.Steps {
		if s.Status != "stuck" || s.Attempts < e.maxAtt() {
			continue
		}
		// Depth guard: a step already re-planned twice ("S2.1.3") is not
		// escalated a third time — it just stays stuck.
		if strings.Count(s.ID, ".") >= 2 {
			continue
		}
		input := EscalationInput{StepID: s.ID, Error: s.Note}
		e.emitEvent("taskloop:escalating", fmt.Sprintf("%s:%s", listID, s.ID))
		repl, err := e.escalator(ctx, listID, s, input)
		if err != nil {
			if isOfflineErr(err) {
				plan.PendingEscalation = &input
				_ = e.store.SavePlan(listID, *plan)
				_ = e.store.SetStatus(listID, taskListWaitingEscalation)
				e.emitEvent("taskloop:waiting_escalation", listID)
				if e.retry != nil {
					e.retry.Arm(listID)
				}
				return false, true
			}
			logx.Printf("TASKLOOP: escalator failed %s/%s: %v (leaving stuck)", listID, s.ID, err)
			continue
		}
		if len(repl) == 0 {
			continue
		}
		plan.ReplaceStep(s.ID, repl)
		if err := plan.Normalize(); err != nil {
			logx.Printf("TASKLOOP: escalated plan invalid %s: %v", listID, err)
			continue
		}
		_ = e.store.SavePlan(listID, *plan)
		if tl.TaskMdPath != "" {
			if werr := WritePlanMd(PlanMdPath(tl.TaskMdPath), *plan, itemTextMap(tl)); werr != nil {
				logx.Printf("TASKLOOP: rewrite Plan.md %s: %v", listID, werr)
			}
		}
		e.emitEvent("taskloop:escalated", fmt.Sprintf("%s:%s", listID, s.ID))
		return true, false
	}
	return false, false
}

// resumePendingEscalation retries a parked offline escalation. Returns true if
// it resolved (plan updated / given up cleanly), false if still offline (the
// list is re-armed and left parked).
func (e *Engine) resumePendingEscalation(ctx context.Context, listID string, tl *TaskList, plan *Plan) bool {
	pe := *plan.PendingEscalation
	var step PlanStep
	for _, s := range plan.Steps {
		if s.ID == pe.StepID {
			step = s
			break
		}
	}
	repl, err := e.escalator(ctx, listID, step, pe)
	if err != nil {
		if isOfflineErr(err) {
			if e.retry != nil {
				e.retry.Arm(listID)
			}
			_ = e.store.SetStatus(listID, taskListWaitingEscalation)
			return false
		}
		logx.Printf("TASKLOOP: resume escalation %s/%s gave up: %v", listID, pe.StepID, err)
		plan.PendingEscalation = nil
		_ = e.store.SavePlan(listID, *plan)
		return true
	}
	plan.ReplaceStep(pe.StepID, repl)
	plan.PendingEscalation = nil
	if nerr := plan.Normalize(); nerr != nil {
		logx.Printf("TASKLOOP: resumed escalation plan invalid %s: %v", listID, nerr)
	}
	_ = e.store.SavePlan(listID, *plan)
	if tl.TaskMdPath != "" {
		_ = WritePlanMd(PlanMdPath(tl.TaskMdPath), *plan, itemTextMap(tl))
	}
	e.emitEvent("taskloop:escalated", fmt.Sprintf("%s:%s", listID, pe.StepID))
	return true
}

func (e *Engine) maxAtt() int {
	if e.maxAttempts > 0 {
		return e.maxAttempts
	}
	return 3
}

// isOfflineErr reports whether err looks like a lost network connection (vs. a
// real API/logic failure).
func isOfflineErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, m := range []string{
		"no such host", "dial tcp", "connection refused", "network is unreachable",
		"no route to host", "i/o timeout", "tls handshake timeout", "temporary failure in name resolution",
		"connection reset by peer", "eof",
	} {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// runOneStep runs the coder turn for one step and applies its acceptance
// checks. Returns nil only when the step both ran and passed.
func (e *Engine) runOneStep(ctx context.Context, listID string, step PlanStep, stateDoc string) (string, error) {
	_ = e.store.SetStepStatus(listID, step.ID, "running", "")
	if _, err := e.store.IncrementStepAttempts(listID, step.ID); err != nil {
		logx.Printf("TASKLOOP: bump attempts %s/%s: %v", listID, step.ID, err)
	}
	out, err := e.stepRunner(ctx, listID, step, stateDoc)
	if err != nil {
		return "", err
	}
	if e.acceptChecker != nil {
		pass, detail, cerr := e.acceptChecker(ctx, listID, step)
		if cerr != nil {
			return "", fmt.Errorf("kabul kontrolü hatası: %w", cerr)
		}
		if !pass {
			return "", fmt.Errorf("kabul kontrolü geçmedi: %s", detail)
		}
	}
	return out, nil
}

func approxTokens(s string) int { return len(s) / 4 }

func summariseStep(step PlanStep, out string, err error) string {
	if err != nil {
		return fmt.Sprintf("- %s (%s): TAKILDI — %s", step.ID, step.Text, truncate.Text(err.Error(), 200))
	}
	line := fmt.Sprintf("- %s (%s): tamam", step.ID, step.Text)
	if o := strings.TrimSpace(out); o != "" {
		line += " — " + truncate.Text(o, 300)
	}
	return line
}

func appendState(state, line string) string {
	if strings.TrimSpace(state) == "" {
		return "## İlerleme\n" + line
	}
	return state + "\n" + line
}

// finishPlanItems mirrors step completion up to the Task.md items: an item is
// done when every step serving it is done, otherwise stuck. The list ends
// "done" if any item completed, else "failed".
func (e *Engine) finishPlanItems(listID string, tl *TaskList, plan *Plan) {
	anyDone, anyStuck := false, false
	for idx := range tl.Items {
		it := &tl.Items[idx]
		steps := plan.StepsForItem(strconv.Itoa(idx + 1))
		if len(steps) == 0 {
			continue
		}
		allDone := true
		for _, s := range steps {
			if s.Status != "done" {
				allDone = false
				break
			}
		}
		if allDone {
			if err := e.store.SetItemDone(listID, it.ID); err != nil {
				logx.Printf("TASKLOOP: plan item done %s/%s: %v", listID, it.ID, err)
			}
			if tl.TaskMdPath != "" && it.Line > 0 {
				if err := MarkItemDone(tl.TaskMdPath, it.Line); err != nil {
					logx.Printf("TASKLOOP: mirror %s line %d: %v", tl.TaskMdPath, it.Line, err)
				}
			}
			e.emitEvent("tasklist:item_done", fmt.Sprintf("%s:%s", listID, it.ID))
			anyDone = true
		} else {
			if err := e.store.SetItemStuck(listID, it.ID, "bazı adımlar tamamlanamadı"); err != nil {
				logx.Printf("TASKLOOP: plan item stuck %s/%s: %v", listID, it.ID, err)
			}
			e.emitEvent("tasklist:item_stuck", fmt.Sprintf("%s:%s", listID, it.ID))
			anyStuck = true
		}
	}
	final := taskListDone
	if !anyDone && anyStuck {
		final = taskListFailed
	}
	if err := e.store.SetStatus(listID, final); err != nil {
		logx.Printf("TASKLOOP: set list %s %s: %v", final, listID, err)
	}
	e.emitEvent("tasklist:finished", fmt.Sprintf("%s:%s", listID, final))
}

func (e *Engine) failPlan(listID, note string) {
	logx.Printf("TASKLOOP: plan-exec %s failed: %s", listID, note)
	if err := e.store.SetStatus(listID, taskListFailed); err != nil {
		logx.Printf("TASKLOOP: set failed %s: %v", listID, err)
	}
	e.emitEvent("tasklist:finished", listID+":"+taskListFailed)
}

func (e *Engine) emitEvent(name, data string) {
	if e.onEvent != nil {
		e.onEvent(name, data)
	}
}

func itemTexts(tl *TaskList) []string {
	out := make([]string, len(tl.Items))
	for i, it := range tl.Items {
		out[i] = it.Text
	}
	return out
}

func itemTextMap(tl *TaskList) map[string]string {
	m := make(map[string]string, len(tl.Items))
	for i, it := range tl.Items {
		m[strconv.Itoa(i+1)] = it.Text
	}
	return m
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

package taskloop

import "time"

// RunningTaskInfo is a live view of an executing task list, for the "task_list"
// command and GET /api/tasks/running. It is assembled from the store plus the
// engine's in-memory run state.
type RunningTaskInfo struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	ChatID      string   `json:"chat_id"`
	Phase       string   `json:"phase"`
	DoneCount   int      `json:"done_count"`
	ItemCount   int      `json:"item_count"`
	CurrentItem string   `json:"current_item"`
	ElapsedSec  int      `json:"elapsed_sec"`
	SubAgents   []string `json:"sub_agents"`
	NotifyLevel string   `json:"notify_level"`
	// SilentSec is how many seconds since the last activity/step signal — a
	// client uses it to tell "coder is working on a hard step" from "hung".
	SilentSec int `json:"silent_sec"`
	// Tokens is a running, approximate count of tokens processed by this list's
	// planner/coder turns (prompt context churn + generated output, len/4
	// estimate — not a billing figure). It only ever grows while the list runs
	// and is the chat block's "still doing something" signal alongside SilentSec.
	Tokens int `json:"tokens,omitempty"`
	// planner/executor mode only (zero otherwise):
	Mode           string `json:"mode,omitempty"`
	PlanSteps      int    `json:"plan_steps,omitempty"`
	PlanStepsDone  int    `json:"plan_steps_done,omitempty"`
	StateDocTokens int    `json:"state_doc_tokens,omitempty"`
	StateDocBudget int    `json:"state_doc_budget,omitempty"`
}

type listRuntime struct {
	startedAt      time.Time
	lastActivityAt time.Time
	currentItem    string
	subAgents      []string
	tokens         int
}

func (e *Engine) rtStart(listID string) {
	e.rtMu.Lock()
	if e.runtimes == nil {
		e.runtimes = make(map[string]*listRuntime)
	}
	now := time.Now()
	e.runtimes[listID] = &listRuntime{startedAt: now, lastActivityAt: now}
	e.rtMu.Unlock()
}

// rtTouch refreshes the last-activity timestamp — called on every activity/step
// signal so SilentSec measures "time since we last did something visible".
func (e *Engine) rtTouch(listID string) {
	e.rtMu.Lock()
	if rt := e.runtimes[listID]; rt != nil {
		rt.lastActivityAt = time.Now()
	}
	e.rtMu.Unlock()
}

// rtAddTokens adds n to a list's running token estimate (no-op for n <= 0 or a
// list that isn't currently executing in this process). Also refreshes the
// last-activity clock — tokens moving means the list is doing something.
func (e *Engine) rtAddTokens(listID string, n int) {
	if n <= 0 {
		return
	}
	e.rtMu.Lock()
	if rt := e.runtimes[listID]; rt != nil {
		rt.tokens += n
		rt.lastActivityAt = time.Now()
	}
	e.rtMu.Unlock()
}

func (e *Engine) rtSetItem(listID, itemText string) {
	e.rtMu.Lock()
	if rt := e.runtimes[listID]; rt != nil {
		rt.currentItem = itemText
		rt.lastActivityAt = time.Now()
	}
	e.rtMu.Unlock()
}

func (e *Engine) rtAddSubAgents(listID string, roles ...string) {
	e.rtMu.Lock()
	if rt := e.runtimes[listID]; rt != nil {
		rt.subAgents = append(rt.subAgents, roles...)
	}
	e.rtMu.Unlock()
}

func (e *Engine) rtEnd(listID string) {
	e.rtMu.Lock()
	delete(e.runtimes, listID)
	e.rtMu.Unlock()
}

// Runtime returns the live view of listID, or ok=false if it isn't currently
// executing in this process.
func (e *Engine) Runtime(listID string) (RunningTaskInfo, bool) {
	e.rtMu.RLock()
	rt := e.runtimes[listID]
	e.rtMu.RUnlock()
	if rt == nil {
		return RunningTaskInfo{}, false
	}
	tl, err := e.store.Get(listID)
	if err != nil {
		return RunningTaskInfo{}, false
	}
	done := 0
	for _, it := range tl.Items {
		if it.Status == "done" {
			done++
		}
	}
	subs := make([]string, len(rt.subAgents))
	copy(subs, rt.subAgents)
	info := RunningTaskInfo{
		ID:          tl.ID,
		Title:       tl.Title,
		ChatID:      tl.ChatID,
		Phase:       tl.Status,
		DoneCount:   done,
		ItemCount:   len(tl.Items),
		CurrentItem: rt.currentItem,
		ElapsedSec:  int(time.Since(rt.startedAt).Seconds()),
		SilentSec:   int(time.Since(rt.lastActivityAt).Seconds()),
		Tokens:      rt.tokens,
		SubAgents:   subs,
		NotifyLevel: string(tl.NotifyLevel),
		Mode:        tl.Mode,
	}
	if tl.Mode == ModePlanner {
		if p, err := e.store.GetPlan(listID); err == nil {
			info.PlanSteps = len(p.Steps)
			for _, s := range p.Steps {
				if s.Status == "done" {
					info.PlanStepsDone++
				}
			}
		}
		info.StateDocTokens = approxTokens(e.store.GetState(listID))
		_, _, info.StateDocBudget = e.execCfg()
	}
	return info, true
}

// RunningTasks returns a live view of every list currently executing.
func (e *Engine) RunningTasks() []RunningTaskInfo {
	e.rtMu.RLock()
	ids := make([]string, 0, len(e.runtimes))
	for id := range e.runtimes {
		ids = append(ids, id)
	}
	e.rtMu.RUnlock()
	out := make([]RunningTaskInfo, 0, len(ids))
	for _, id := range ids {
		if info, ok := e.Runtime(id); ok {
			out = append(out, info)
		}
	}
	return out
}

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
	// planner/executor mode only (zero otherwise):
	Mode           string `json:"mode,omitempty"`
	PlanSteps      int    `json:"plan_steps,omitempty"`
	PlanStepsDone  int    `json:"plan_steps_done,omitempty"`
	StateDocTokens int    `json:"state_doc_tokens,omitempty"`
	StateDocBudget int    `json:"state_doc_budget,omitempty"`
}

type listRuntime struct {
	startedAt   time.Time
	currentItem string
	subAgents   []string
}

func (e *Engine) rtStart(listID string) {
	e.rtMu.Lock()
	if e.runtimes == nil {
		e.runtimes = make(map[string]*listRuntime)
	}
	e.runtimes[listID] = &listRuntime{startedAt: time.Now()}
	e.rtMu.Unlock()
}

func (e *Engine) rtSetItem(listID, itemText string) {
	e.rtMu.Lock()
	if rt := e.runtimes[listID]; rt != nil {
		rt.currentItem = itemText
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

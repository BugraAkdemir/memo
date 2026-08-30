package app

import (
	"encoding/json"
	"strings"
)

// taskChatEvent is one Self-Driving event, enriched with the current progress
// snapshot, streamed to chat clients over GET /api/tasks/events so a task's
// live state shows up in the chat that started it (v4.6.0 Faz D).
type taskChatEvent struct {
	Event      string `json:"event"` // raw engine name minus the "taskloop:"/"tasklist:" prefix
	ListID     string `json:"list_id"`
	ChatID     string `json:"chat_id"`
	Detail     string `json:"detail,omitempty"`
	Phase      string `json:"phase,omitempty"`
	Mode       string `json:"mode,omitempty"`
	StepDone   int    `json:"step_done"`
	StepTotal  int    `json:"step_total"`
	ItemDone   int    `json:"item_done"`
	ItemTotal  int    `json:"item_total"`
	Current    string `json:"current,omitempty"`
	ElapsedSec int    `json:"elapsed_sec"`
}

// SubscribeTaskEvents registers a subscriber for enriched task-loop event
// JSON lines. The returned func unsubscribes. Channels are buffered and never
// closed by publishTaskEvent — a slow subscriber drops events rather than
// blocking the engine.
func (a *App) SubscribeTaskEvents() (<-chan string, func()) {
	ch := make(chan string, 64)
	a.taskEventMu.Lock()
	a.taskEventSubs = append(a.taskEventSubs, ch)
	a.taskEventMu.Unlock()
	return ch, func() {
		a.taskEventMu.Lock()
		for i, c := range a.taskEventSubs {
			if c == ch {
				a.taskEventSubs = append(a.taskEventSubs[:i], a.taskEventSubs[i+1:]...)
				break
			}
		}
		a.taskEventMu.Unlock()
	}
}

// publishTaskEvent fans one raw engine event (name, "listID" or "listID:extra")
// out to every task-event subscriber as an enriched JSON line. Best-effort:
// non-taskloop events are ignored, and a full subscriber channel is skipped.
func (a *App) publishTaskEvent(name, data string) {
	if !strings.HasPrefix(name, "taskloop:") && !strings.HasPrefix(name, "tasklist:") {
		return
	}
	a.taskEventMu.RLock()
	subs := append([]chan string(nil), a.taskEventSubs...)
	a.taskEventMu.RUnlock()
	if len(subs) == 0 {
		return
	}

	listID, extra := data, ""
	if i := strings.IndexByte(data, ':'); i >= 0 {
		listID, extra = data[:i], data[i+1:]
	}
	ev := taskChatEvent{
		Event:  strings.TrimPrefix(strings.TrimPrefix(name, "taskloop:"), "tasklist:"),
		ListID: listID,
		Detail: extra,
	}
	a.fillTaskEventSnapshot(&ev, listID)

	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	line := string(payload)
	for _, ch := range subs {
		select {
		case ch <- line:
		default:
		}
	}
}

// RunningTaskEventSnapshot returns one "snapshot" event line per currently
// running list, so a client that connects mid-run isn't blind until the next
// real event fires.
func (a *App) RunningTaskEventSnapshot() []string {
	if a.taskloopEngine == nil {
		return nil
	}
	var out []string
	for _, r := range a.taskloopEngine.RunningTasks() {
		ev := taskChatEvent{
			Event: "snapshot", ListID: r.ID, ChatID: r.ChatID, Phase: r.Phase, Mode: r.Mode,
			StepDone: r.PlanStepsDone, StepTotal: r.PlanSteps,
			ItemDone: r.DoneCount, ItemTotal: r.ItemCount,
			Current: r.CurrentItem, ElapsedSec: r.ElapsedSec,
		}
		if b, err := json.Marshal(ev); err == nil {
			out = append(out, string(b))
		}
	}
	return out
}

func (a *App) fillTaskEventSnapshot(ev *taskChatEvent, listID string) {
	if a.taskloopStore != nil {
		if tl, err := a.taskloopStore.Get(listID); err == nil {
			ev.ChatID = tl.ChatID
			ev.Phase = tl.Status
			ev.Mode = tl.Mode
		}
	}
	if a.taskloopEngine != nil {
		if rt, ok := a.taskloopEngine.Runtime(listID); ok {
			ev.Phase = rt.Phase
			ev.Mode = rt.Mode
			ev.StepDone, ev.StepTotal = rt.PlanStepsDone, rt.PlanSteps
			ev.ItemDone, ev.ItemTotal = rt.DoneCount, rt.ItemCount
			ev.Current = rt.CurrentItem
			ev.ElapsedSec = rt.ElapsedSec
			if ev.ChatID == "" {
				ev.ChatID = rt.ChatID
			}
		}
	}
}

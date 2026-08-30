package app

import (
	"encoding/json"
	"fmt"
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
	Kind       string `json:"kind,omitempty"` // for "activity" events: plan_start|plan_done|step_start|step_done|step_retry|step_stuck|escalate|tool|paused|resumed
	Text       string `json:"text,omitempty"` // for "activity" events: the human-readable line
	Phase      string `json:"phase,omitempty"`
	Mode       string `json:"mode,omitempty"`
	StepDone   int    `json:"step_done"`
	StepTotal  int    `json:"step_total"`
	ItemDone   int    `json:"item_done"`
	ItemTotal  int    `json:"item_total"`
	Current    string `json:"current,omitempty"`
	ElapsedSec int    `json:"elapsed_sec"`
	SilentSec  int    `json:"silent_sec"`
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
// taskEventSkip lists engine events whose data payload is a human-readable
// message, not a list ID — they carry no per-chat meaning and must not reach
// the chat card.
var taskEventSkip = map[string]bool{
	"taskloop:bypass_enabled":  true,
	"taskloop:bypass_disabled": true,
	"taskloop:notify":          true, // already delivered via the NotifyBus
}

func (a *App) publishTaskEvent(name, data string) {
	if !strings.HasPrefix(name, "taskloop:") && !strings.HasPrefix(name, "tasklist:") {
		return
	}
	if taskEventSkip[name] {
		return
	}
	a.taskEventMu.RLock()
	subs := append([]chan string(nil), a.taskEventSubs...)
	a.taskEventMu.RUnlock()
	if len(subs) == 0 {
		return
	}

	// "taskloop:activity" carries "listID\x1fkind\x1ftext" — a live "here's what
	// I'm doing" line for the chat activity block. Parsed before the generic
	// "listID:extra" split (the text may contain ':').
	if name == "taskloop:activity" {
		parts := strings.SplitN(data, "\x1f", 3)
		if len(parts) != 3 {
			return
		}
		listID := parts[0]
		if a.taskloopStore != nil {
			if _, err := a.taskloopStore.Get(listID); err != nil {
				return
			}
		}
		ev := taskChatEvent{Event: "activity", ListID: listID, Kind: parts[1], Text: parts[2]}
		a.fillTaskEventSnapshot(&ev, listID)
		if payload, err := json.Marshal(ev); err == nil {
			line := string(payload)
			for _, ch := range subs {
				select {
				case ch <- line:
				default:
				}
			}
		}
		return
	}

	listID, extra := data, ""
	if i := strings.IndexByte(data, ':'); i >= 0 {
		listID, extra = data[:i], data[i+1:]
	}
	// Guard against any other event whose data isn't a real list ID.
	if a.taskloopStore != nil {
		if _, err := a.taskloopStore.Get(listID); err != nil {
			return
		}
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
			Current: r.CurrentItem, ElapsedSec: r.ElapsedSec, SilentSec: r.SilentSec,
		}
		if b, err := json.Marshal(ev); err == nil {
			out = append(out, string(b))
		}
	}
	return out
}

// postTaskFinishMessage drops a single, plain (no LLM) assistant line into the
// task's chat when the loop ends, so there is a persisted record in the
// transcript even though the live activity block is ephemeral. data is
// "listID:done" / "listID:failed".
func (a *App) postTaskFinishMessage(data string) {
	id, final := data, ""
	if i := strings.IndexByte(data, ':'); i >= 0 {
		id, final = data[:i], data[i+1:]
	}
	if a.taskloopStore == nil {
		return
	}
	tl, err := a.taskloopStore.Get(id)
	if err != nil || tl.ChatID == "" {
		return
	}
	sm := a.getSessionManager()
	if sm == nil {
		return
	}
	done, stuck := 0, 0
	for _, it := range tl.Items {
		switch it.Status {
		case "done":
			done++
		case "stuck":
			stuck++
		}
	}
	total := len(tl.Items)
	var msg string
	if final == "failed" || stuck > 0 {
		msg = fmt.Sprintf(a.t("⚠️ Görev kısmen bitti — %d/%d madde, %d takıldı.", "⚠️ Task partly finished — %d/%d items, %d stuck."), done, total, stuck)
	} else {
		msg = fmt.Sprintf(a.t("✅ Görev bitti — %d/%d madde tamamlandı.", "✅ Task done — %d/%d items completed."), done, total)
	}
	sm.AddMessageToSession(tl.ChatID, "assistant", msg, "", "")
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
			ev.SilentSec = rt.SilentSec
			if ev.ChatID == "" {
				ev.ChatID = rt.ChatID
			}
		}
	}
}

package models

import "time"

// ActivityState is a coarse, app-wide "what is Memo doing right now"
// signal — for a consumer with no chat context of its own (the desktop
// mascot window is the first one), not a replacement for any per-chat
// event stream.
type ActivityState string

const (
	ActivityIdle       ActivityState = "idle"
	ActivityThinking   ActivityState = "thinking"
	ActivityTool       ActivityState = "tool"
	ActivityWriting    ActivityState = "writing"
	ActivityGenerating ActivityState = "generating"
	// ActivityDone is a brief, explicit "just finished" pulse — a turn that
	// ended successfully (plain chat or agent) reports this once, then
	// activityDoneTimeout reverts it to idle on its own shortly after, so
	// the desktop mascot gets a "completed" beat instead of either
	// lingering in "generating" or silently going straight to idle.
	ActivityDone ActivityState = "done"
)

// ActivityStatus is what GET /api/mascot/activity returns.
type ActivityStatus struct {
	State    ActivityState `json:"state"`
	ToolName string        `json:"tool_name,omitempty"`
	Since    time.Time     `json:"since"`
}

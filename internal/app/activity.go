package app

import (
	"sync"
	"sync/atomic"
	"time"

	"memo/internal/agent"
	"memo/internal/livemode"
	"memo/internal/models"
)

// codeModeWriteTools mirrors the file-editing tool set Code Mode's "auto"
// sub-mode auto-approves (see agent.Pipeline's codeSubMode /
// codeModeToolAutoApproveSet) — the same intuition applies here: these read
// as "writing" to a human watching, everything else as a more generic
// "tool".
var codeModeWriteTools = map[string]bool{
	"write_file":   true,
	"edit_file":    true,
	"insert_line":  true,
	"delete_lines": true,
}

// activityIdleTimeout: no new agent event within this window and the
// tracker reports idle again on its own — a crashed/abandoned turn
// (context cancelled, process killed mid-tool) would otherwise leave the
// tracker stuck reporting stale activity forever, since nothing else ever
// tells it the turn ended.
var activityIdleTimeout = 12 * time.Second // var, not const: shortened in tests

// activityDoneTimeout: how long models.ActivityDone's "completed" pulse
// stays up before falling back to idle on its own — short, since it's a
// one-off beat meant to be noticed and then get out of the way, not a
// lingering status the way "generating"/"tool" are.
var activityDoneTimeout = 2500 * time.Millisecond // var, not const: shortened in tests

// activitySpeakingTimeout: how long models.ActivitySpeaking stays up after
// the last audio_out chunk before falling back to idle — audio_out fires
// repeatedly throughout playback (re-arming this on every chunk, see
// wireLiveModeActivityHook), so in practice this only matters for the tail
// end once speech actually stops. Shorter than activityIdleTimeout since a
// gap this size in a real-time audio stream reliably means playback ended,
// not just a slow chunk.
var activitySpeakingTimeout = 4 * time.Second // var, not const: shortened in tests

type activityTracker struct {
	mu       sync.Mutex
	state    models.ActivityState
	toolName string
	since    time.Time
	gen      uint64 // invalidates a stale auto-idle timer race
}

// globalActivity is intentionally package-level, not a field on *App: the
// only thing that writes to it is agent.GlobalActivityHook (set once in
// NewApp, below), which every RunStream caller across the whole app goes
// through regardless of which chat, task list, or channel (WhatsApp/
// Telegram) it belongs to — see agent/executor.go's wrappedOnEvent. A
// per-App field would need the hook closure to capture *App, which is
// fine too, but this keeps the tracker's own concurrency story (a single
// mutex, no relation to App's own locks) fully self-contained.
var globalActivity = &activityTracker{state: models.ActivityIdle, since: time.Now()}

func setActivity(state models.ActivityState, toolName string) {
	globalActivity.mu.Lock()
	globalActivity.state = state
	globalActivity.toolName = toolName
	globalActivity.since = time.Now()
	globalActivity.gen++
	gen := globalActivity.gen
	globalActivity.mu.Unlock()

	if state == models.ActivityIdle {
		return
	}
	timeout := activityIdleTimeout
	switch state {
	case models.ActivityDone:
		timeout = activityDoneTimeout
	case models.ActivitySpeaking:
		timeout = activitySpeakingTimeout
	}
	time.AfterFunc(timeout, func() {
		globalActivity.mu.Lock()
		defer globalActivity.mu.Unlock()
		if globalActivity.gen == gen {
			globalActivity.state = models.ActivityIdle
			globalActivity.toolName = ""
			globalActivity.since = time.Now()
		}
	})
}

// wireGlobalActivityHook sets agent.GlobalActivityHook once at App
// construction. Deliberately coarse and best-effort (see
// models.ActivityStatus's doc comment): EventPermissionRequest/Denied
// aren't distinguished from an executing tool, and two concurrent agent
// turns (the task loop now allows this) just overwrite each other's
// state — good enough for a decorative desktop companion, not a queue or
// per-session record. Live Mode's activity signal is wired separately, see
// wireLiveModeActivityHook.
func wireGlobalActivityHook() {
	agent.GlobalActivityHook = func(ev agent.AgentEvent) {
		switch ev.Type {
		case agent.EventToolExecuting:
			if codeModeWriteTools[ev.ToolName] {
				setActivity(models.ActivityWriting, ev.ToolName)
			} else {
				setActivity(models.ActivityTool, ev.ToolName)
			}
		case agent.EventToolResult:
			setActivity(models.ActivityGenerating, ev.ToolName)
		case agent.EventToolError:
			setActivity(models.ActivityIdle, "")
		case agent.EventFinalResponse:
			setActivity(models.ActivityDone, "")
		}
	}
}

// lastLiveModeSpeaking throttles wireLiveModeActivityHook's setActivity
// calls to at most once per 2s. A compare-and-swap is required here because
// livemode.GlobalActivityHook can be invoked concurrently from more than
// one pumpLiveModeSessionEvents goroutine if more than one Live Mode
// session is open at once. A Load followed by Store would let two
// concurrent callers both pass the window and defeat the throttle.
var lastLiveModeSpeaking atomic.Int64

func shouldReportLiveModeSpeaking(now time.Time) bool {
	stamp := now.UnixNano()
	for {
		last := lastLiveModeSpeaking.Load()
		if last != 0 && now.Sub(time.Unix(0, last)) < 2*time.Second {
			return false
		}
		if lastLiveModeSpeaking.CompareAndSwap(last, stamp) {
			return true
		}
	}
}

// wireLiveModeActivityHook sets livemode.GlobalActivityHook once at App
// construction, the Live Mode half of wireGlobalActivityHook. See
// models.ActivitySpeaking's doc comment for what this does and doesn't
// cover.
func wireLiveModeActivityHook() {
	livemode.GlobalActivityHook = func() {
		if !shouldReportLiveModeSpeaking(time.Now()) {
			return
		}
		setActivity(models.ActivitySpeaking, "")
	}
}

// GetActivityStatus reports Memo's current app-wide activity. Read by the
// desktop mascot's own polling loop (a separate window/process with no chat
// context) via GET /api/mascot/activity.
func (a *App) GetActivityStatus() models.ActivityStatus {
	globalActivity.mu.Lock()
	defer globalActivity.mu.Unlock()
	return models.ActivityStatus{
		State:    globalActivity.state,
		ToolName: globalActivity.toolName,
		Since:    globalActivity.since,
	}
}

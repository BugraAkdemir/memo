package app

import (
	"context"
	"encoding/json"
	"time"

	"memo/internal/agent"
	"memo/internal/agent/tools"
	"memo/internal/api"
)

// BrowserFrame is the payload of a "browser_frame" SSE chunk: a live
// screenshot pushed to the client whenever browser_screenshot succeeds, so
// a Flutter browser pane can show it without polling. Reuses the exact same
// StreamChunk{Content, FinishReason} envelope every other "kind" of push
// already does (agent_event, status, usage, ...) — see writeSSEChunk in
// internal/webserver/handlers_flutter.go. Deliberately carries no URL: the
// client derives that from browser_navigate's own tool_executing/
// tool_result AgentEvent (Args.url), already flowing through the same
// agent_event stream — no need for a second source of truth here.
type BrowserFrame struct {
	Screenshot string `json:"screenshot_base64"`
	Timestamp  int64  `json:"ts"`
}

// emitBrowserFrame sends a browser_frame SSE chunk when ev is a successful
// browser_screenshot tool result — a no-op for every other event. Called
// from the onEvent closure in llm.go, the same place agent_event chunks are
// already built, so it shares that closure's ctx/outCh instead of needing
// separate wiring.
//
// Pulls the image from tools.LastBrowserFrame(), NOT from ev.Result — an
// earlier version parsed the base64 out of the tool's own text result, which
// required BrowserScreenshot to put the full base64 PNG in that text in the
// first place. That measurably bloated conversation history (every
// subsequent LLM call in the turn resends it) and cost real money in a live
// session — see tools.BrowserScreenshot's doc comment for the actual
// numbers. This side-channel lets the tool's text result stay a short
// confirmation while the live pane still gets every frame.
func emitBrowserFrame(ctx context.Context, outCh chan<- api.StreamChunk, ev agent.AgentEvent) {
	if ev.Type != agent.EventToolResult || ev.ToolName != "browser_screenshot" {
		return
	}
	b64, ok := tools.LastBrowserFrame()
	if !ok {
		return
	}
	frameData, err := json.Marshal(BrowserFrame{Screenshot: b64, Timestamp: time.Now().UnixMilli()})
	if err != nil {
		return
	}
	trySend(ctx, outCh, api.StreamChunk{Content: string(frameData), FinishReason: "browser_frame"})
}

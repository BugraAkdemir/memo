package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/agent/tools"
	"memo/internal/logx"
	"memo/internal/provider"
	"memo/internal/truncate"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
)

// AgentEventType defines the types of events emitted during pipeline execution.
type AgentEventType string

const (
	EventToolExecuting     AgentEventType = "tool_executing"
	EventToolResult        AgentEventType = "tool_result"
	EventToolError         AgentEventType = "tool_error"
	EventPermissionRequest AgentEventType = "permission_request"
	EventPermissionDenied  AgentEventType = "permission_denied"
	EventFinalResponse     AgentEventType = "final_response"
)

// AgentEvent represents an event in the agent execution pipeline.
type AgentEvent struct {
	Type        AgentEventType  `json:"type"`
	RequestID   string          `json:"request_id,omitempty"`
	ToolName    string          `json:"tool,omitempty"`
	Args        json.RawMessage `json:"args,omitempty"`
	Result      string          `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
	DangerLevel DangerLevel     `json:"danger_level,omitempty"`
	DurationMs  int64           `json:"duration_ms,omitempty"`
	Content     string          `json:"content,omitempty"`
	Preview     string          `json:"preview,omitempty"`
}

// AgentProvider is an interface that wraps ChatCompletion.
type AgentProvider interface {
	ChatCompletion(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error)
}

// Pipeline orchestrates the interaction between the LLM and tools.
type Pipeline struct {
	registry           *ToolRegistry
	permissions        *PermissionManager
	sandbox            *Sandbox
	prov               AgentProvider
	maxIters           int
	backup             *BackupManager
	maxTokens          int           // context window token budget for this turn (0 = unlimited)
	toolTimeout        time.Duration // max time per tool execution (0 = no limit)
	bypassPermissions  bool          // sistem yönetimi açıkken tüm izinleri otomatik onayla
	autoPermission     bool          // kullanıcı Shift+Tab ile açtığında tüm izinleri otomatik onayla
	// autoPermissionFn, when set, is consulted live at every permission check
	// so a toggle made DURING a long agentic loop takes effect on the next
	// tool call instead of only on the next turn (the struct copy above is
	// snapshotted once at RunStream construction). Executor.RunStream wires
	// this to e.GetAutoPermission.
	autoPermissionFn   func() bool

	// effortLevel is the active provider's resolved EffortLevel for this
	// run (see provider.ChatRequest.EffortLevel's doc comment) — set by the
	// Executor after construction, same pattern as bypassPermissions/
	// autoPermission above, since NewPipeline/NewPipelineWithBudget's
	// signatures are already exercised directly by pipeline_test.go and
	// don't need a new constructor param for this.
	effortLevel string
}

// NewPipeline creates a new agent execution pipeline.
func NewPipeline(registry *ToolRegistry, permissions *PermissionManager, sandbox *Sandbox, prov AgentProvider, backup *BackupManager) *Pipeline {
	return &Pipeline{
		registry:    registry,
		permissions: permissions,
		sandbox:     sandbox,
		prov:        prov,
		maxIters:    40,
		backup:      backup,
		toolTimeout: 120 * time.Second,
	}
}

// NewPipelineWithBudget creates a pipeline with a context token budget.
// When maxTokens > 0, the pipeline truncates accumulated tool-call history
// to stay within budget, dropping oldest tool result messages first.
func NewPipelineWithBudget(registry *ToolRegistry, permissions *PermissionManager, sandbox *Sandbox, prov AgentProvider, backup *BackupManager, maxTokens int) *Pipeline {
	return &Pipeline{
		registry:    registry,
		permissions: permissions,
		sandbox:     sandbox,
		prov:        prov,
		maxIters:    40,
		backup:      backup,
		maxTokens:   maxTokens,
		toolTimeout: 120 * time.Second,
	}
}

// RunStream executes the agent pipeline and streams the results.
// It returns a channel that emits StreamChunks.
// permissionWaitFn blocks until the user approves or denies a tool call.
func (p *Pipeline) RunStream(ctx context.Context, messages []provider.Message, modelName string, onEvent func(AgentEvent), permissionWaitFn func(requestID string, event AgentEvent) (PermissionPolicy, error)) (<-chan provider.StreamChunk, error) {
	outCh := make(chan provider.StreamChunk, 128)

	// Fresh per-run fetch_page domain budget — shared by every tool call
	// this run makes (toolCtx below derives from this ctx), reset each time
	// RunStream is called (once per agent turn). See tools.WithFetchBudget.
	ctx = tools.WithFetchBudget(ctx)

	// Let change_directory (internal/agent/tools/changedir.go) widen this
	// turn's sandbox root — Pipeline.sandbox is the per-call sandbox
	// Executor.RunStream just built, so this is safe to mutate mid-turn: no
	// other concurrent RunStream call shares it.
	ctx = tools.WithSandboxSetter(ctx, p.sandbox)

	go func() {
		defer close(outCh)
		defer recoverStreamPanic(ctx, outCh, "Pipeline.RunStream")

		currentMessages := make([]provider.Message, len(messages))
		copy(currentMessages, messages)

		// Real token accounting for this turn, summed across every
		// non-streaming ChatCompletion iteration below (each is a separately
		// billed call, so both prompt and completion accumulate). Attached to
		// whichever terminal chunk ends the turn via termUsage() so
		// callAgentStream/drainAgentStream can record the true cost instead of
		// a word-count estimate of the seed messages alone.
		var totalUsage provider.Usage
		sawUsage := false
		firstPromptTok := 0 // iteration-0 prefill: system + agent block + tool schema + history + user
		iters := 0
		// Tool names whose results have been dropped by intra-turn truncation
		// so far this run — surfaced to the model as one marker message.
		var evictedTools []string
		evictedSeen := map[string]bool{}
		termUsage := func() *provider.Usage {
			if !sawUsage {
				return nil
			}
			u := totalUsage
			return &u
		}
		// logContext emits the one honest per-turn accounting line for agent
		// mode. The app-side CONTEXT log is written before routeStream appends
		// the agent instruction block and before the tool schema is attached,
		// so it undercounts the real prefill by thousands of tokens; this one
		// is measured from what the provider actually billed.
		logContext := func(end string) {
			logx.Printf("AGENT-CONTEXT: end=%s iters=%d msgs=%d first_prompt_tok=%d total_prompt_tok=%d completion_tok=%d",
				end, iters, len(currentMessages), firstPromptTok, totalUsage.PromptTokens, totalUsage.CompletionTokens)
		}

		for iteration := 0; iteration < p.maxIters; iteration++ {
		iters = iteration + 1
		// Check context cancellation
		select {
		case <-ctx.Done():
			logContext("cancelled")
			trySend(ctx, outCh, provider.StreamChunk{Error: "Agent execution cancelled", Done: true, Usage: termUsage()})
			return
		default:
		}

		// Define request
		req := provider.ChatRequest{
			Model:       modelName,
			Messages:    currentMessages,
			Temperature: 0.2,
			Tools:       p.registry.ToOpenAITools(),
			Stream:      false,
			EffortLevel: p.effortLevel,
		}

		// Call LLM
		resp, err := p.prov.ChatCompletion(ctx, req)
		if err != nil {
			logx.Printf("AGENT: ChatCompletion error: %v", err)
			logContext("error")
			trySend(ctx, outCh, provider.StreamChunk{Error: fmt.Sprintf("LLM Error: %v", err), Done: true, Usage: termUsage()})
			return
		}

		if resp.Usage != nil {
			totalUsage.PromptTokens += resp.Usage.PromptTokens
			totalUsage.CompletionTokens += resp.Usage.CompletionTokens
			totalUsage.TotalTokens += resp.Usage.TotalTokens
			if resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0 {
				sawUsage = true
			}
			if firstPromptTok == 0 && resp.Usage.PromptTokens > 0 {
				firstPromptTok = resp.Usage.PromptTokens
			}
		}

		// If no tool calls, we are done
		if len(resp.ToolCalls) == 0 {
			content := stripHallucinatedToolSyntax(resp.Content)
			if content != "" {
				trySend(ctx, outCh, provider.StreamChunk{Content: content})
			}
			onEvent(AgentEvent{Type: EventFinalResponse, Content: content})
			logContext("stop")
			trySend(ctx, outCh, provider.StreamChunk{Done: true, FinishReason: "stop", Usage: termUsage()})
			return
		}

		// Add assistant message with tool_calls to history
		assistantMsg := provider.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		currentMessages = append(currentMessages, assistantMsg)

		// Snapshot basePath once per iteration to avoid repeated mutex acquisitions.
		basePath := p.sandbox.GetBasePath()

		// Enforce context token budget: if currentMessages exceeds maxTokens,
		// drop oldest assistant+tool message pairs, keeping system prompt and
		// the most recent turn. This prevents unbounded message growth when
		// many tool calls are made across iterations.
		if p.maxTokens > 0 {
			truncMsgs := make([]truncate.Message, len(currentMessages))
			for i, m := range currentMessages {
				var content string
				if s, ok := m.Content.(string); ok {
					content = s
				}
				truncMsgs[i] = truncate.Message{Role: m.Role, Content: content, Index: i}
			}
			truncMsgs = truncate.TruncateMessages(truncMsgs, p.maxTokens)
			// Recover original messages by position (Index), not content equality.
			// Rebuilding from truncate.Message would strip ToolCalls, causing
			// orphaned tool-role messages and LLM API rejections. Matching on
			// content (the old approach) is unsafe because distinct tool
			// results can share identical text (e.g. two calls both returning
			// "OK"), which could truncate the wrong message.
			kept := make(map[int]struct{}, len(truncMsgs))
			for _, tm := range truncMsgs {
				kept[tm.Index] = struct{}{}
			}
			// Note which tools' outputs are being dropped so the model gets a
			// one-line marker instead of silently losing them and blindly
			// re-running the same reads a few iterations later.
			for i, m := range currentMessages {
				if _, ok := kept[i]; ok {
					continue
				}
				for _, tc := range m.ToolCalls {
					if n := tc.Function.Name; n != "" && !evictedSeen[n] {
						evictedSeen[n] = true
						evictedTools = append(evictedTools, n)
					}
				}
			}
			filtered := make([]provider.Message, 0, len(truncMsgs)+1)
			stubInserted := false
			for _, tm := range truncMsgs {
				if tm.Index < 0 || tm.Index >= len(currentMessages) {
					continue
				}
				m := currentMessages[tm.Index]
				// Drop any earlier trim-marker; a single fresh one covering the
				// whole evicted set is re-inserted below.
				if s, ok := m.Content.(string); ok && strings.HasPrefix(s, contextTrimMarker) {
					continue
				}
				filtered = append(filtered, m)
				if !stubInserted && m.Role == "system" && len(evictedTools) > 0 {
					filtered = append(filtered, provider.Message{Role: "assistant", Content: evictionStub(evictedTools)})
					stubInserted = true
				}
			}
			if !stubInserted && len(evictedTools) > 0 {
				filtered = append([]provider.Message{{Role: "assistant", Content: evictionStub(evictedTools)}}, filtered...)
			}
			currentMessages = filtered
		}

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			// Some local models omit the tool_call_id field. Generate a fallback so
			// the assistant + tool message pair is always well-formed.
			if tc.ID == "" {
				tc.ID = generateID()
			}

			toolName := tc.Function.Name
			args := tc.Function.Arguments

			// Fix double-encoded JSON args (some LLMs return string inside string)
			args = fixJSONArgs(args)

			// Ensure tool exists
			toolDef, ok := p.registry.Get(toolName)
			if !ok {
				errMsg := fmt.Sprintf("Unknown tool: %s", toolName)
				onEvent(AgentEvent{Type: EventToolError, ToolName: toolName, Error: errMsg})
				currentMessages = append(currentMessages, provider.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("Error: %s", errMsg),
				})
				continue
			}

			// Check Rate Limit
			if err := p.sandbox.RateLimit(toolName, hashArgs(args)); err != nil {
				onEvent(AgentEvent{Type: EventToolError, ToolName: toolName, Error: err.Error()})
				currentMessages = append(currentMessages, provider.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("Error: %s", err.Error()),
				})
				continue
			}

			// Check Permission
			permRes := p.permissions.Check(toolName, args, toolDef.DangerLevel)

			// Sistem yönetimi modunda izin ekranı çıkmaz — tüm tool'lar otomatik onaylanır.
			if p.bypassPermissions {
				logx.Printf("AGENT: [BYPASS] auto-approving %q (system management mode; prompt_required=%v)", toolName, permRes.NeedPrompt)
				permRes.NeedPrompt = false
				permRes.Allowed = true
			}

			// Shift+Tab auto-permission modunda da izin ekranı çıkmaz. Read it
			// live (autoPermissionFn) as well as from the construction-time
			// snapshot, so enabling it mid-loop applies to the very next tool
			// call rather than only the next turn.
			autoNow := p.autoPermission
			if !autoNow && p.autoPermissionFn != nil {
				autoNow = p.autoPermissionFn()
			}
			if autoNow {
				logx.Printf("AGENT: [AUTO] auto-approving %q (auto-permission mode; prompt_required=%v)", toolName, permRes.NeedPrompt)
				permRes.NeedPrompt = false
				permRes.Allowed = true
			}

			if permRes.NeedPrompt {
				var preview string
				if toolDef.PreviewFn != nil {
					preview, _ = toolDef.PreviewFn(args, basePath)
				}

				reqID := generateID()
				ev := AgentEvent{
					Type:        EventPermissionRequest,
					RequestID:   reqID,
					ToolName:    toolName,
					Args:        args,
					DangerLevel: toolDef.DangerLevel,
					Preview:     preview,
				}
				onEvent(ev)

				policy, err := permissionWaitFn(reqID, ev)
				if err != nil {
					onEvent(AgentEvent{Type: EventToolError, ToolName: toolName, Error: "Permission wait cancelled"})
					// Append a stub tool response for this call so the assistant
					// message's ToolCalls list stays well-formed. Then stub out any
					// remaining calls in this batch before returning.
					currentMessages = append(currentMessages, provider.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    "Error: permission request cancelled",
					})
					for _, remaining := range resp.ToolCalls {
						if remaining.ID == tc.ID {
							continue
						}
						id := remaining.ID
						if id == "" {
							id = generateID()
						}
						currentMessages = append(currentMessages, provider.Message{
							Role:       "tool",
							ToolCallID: id,
							Content:    "Error: earlier permission request cancelled",
						})
					}
					trySend(ctx, outCh, provider.StreamChunk{Error: "Agent execution cancelled (permission timeout)", Done: true})
					return
				}

				p.permissions.HandleResponse(toolName, args, policy)

				if policy == DenyOnce || policy == DenyForever {
					permRes.Allowed = false
				} else {
					permRes.Allowed = true
				}
			}

			if !permRes.Allowed {
				// Clear DenyOnce so the user is prompted again on the next identical call.
				p.permissions.ClearOnce(toolName, args)
				onEvent(AgentEvent{Type: EventPermissionDenied, ToolName: toolName, Args: args})
				currentMessages = append(currentMessages, provider.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    "Error: User denied permission to execute this tool.",
				})
				continue
			}

			// Execute! Args ride along like they do on every other event here —
			// the task card renders this one as the "starting" line for slow
			// tools and printed a bare "Komut …" while it was empty. (This is
			// the emission the agent loop actually uses; ExecuteToolCall has
			// its own, fixed the same way.)
			onEvent(AgentEvent{Type: EventToolExecuting, ToolName: toolName, Args: args, DangerLevel: toolDef.DangerLevel})

			start := time.Now()
			toolCtx := ctx
			var toolCancel context.CancelFunc
			if p.toolTimeout > 0 {
				toolCtx, toolCancel = context.WithTimeout(ctx, p.toolTimeout)
			}
			result, err := p.registry.Execute(toolCtx, toolName, args, basePath, p.backup.CreateBackup)
			if toolCancel != nil {
				toolCancel()
			}
			duration := time.Since(start).Milliseconds()

			p.permissions.ClearOnce(toolName, args)

			if err != nil {
				onEvent(AgentEvent{Type: EventToolError, ToolName: toolName, Args: args, Error: err.Error(), DurationMs: duration})
				currentMessages = append(currentMessages, provider.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("Execution Error: %v", err),
				})
			} else {
				onEvent(AgentEvent{Type: EventToolResult, ToolName: toolName, Args: args, Result: result, DurationMs: duration})
				currentMessages = append(currentMessages, provider.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    result,
				})
			}
		}
		}

		logContext("max_iters")
		trySend(ctx, outCh, provider.StreamChunk{Content: "\n\n⚠️ Agent reached the maximum number of tool calls (40). The task may be incomplete.", Done: true, Usage: termUsage()})
	}()

	return outCh, nil
}

// contextTrimMarker prefixes the synthetic assistant message that stands in
// for tool results dropped by intra-turn truncation. Recognisable so a later
// truncation pass can replace it with a fresh combined one rather than
// stacking markers.
const contextTrimMarker = "[context-trim]"

// evictionStub is that message's body: it tells the model which tools'
// outputs from earlier in this same turn were trimmed to fit the context
// window, so it re-runs one deliberately instead of assuming it never ran.
func evictionStub(tools []string) string {
	return fmt.Sprintf("%s Earlier in this turn some tool outputs were trimmed to fit the context window (tools used: %s). If you need any of that information again, call the tool again rather than assuming it was never run.", contextTrimMarker, strings.Join(tools, ", "))
}

// trySend delivers chunk to outCh, preferring the send over ctx cancellation.
// A plain `select { case outCh <- chunk: case <-ctx.Done(): }` lets Go's
// random tie-breaking between simultaneously-ready cases silently drop the
// chunk — including the final Done:true one — if ctx becomes Done at the
// exact moment outCh also has buffer room. Mirrors trySend in
// internal/provider/provider.go and internal/app/llm.go (BUG-H1).
func trySend(ctx context.Context, outCh chan<- provider.StreamChunk, chunk provider.StreamChunk) {
	select {
	case outCh <- chunk:
		return
	default:
	}
	select {
	case outCh <- chunk:
	case <-ctx.Done():
	}
}

// recoverStreamPanic must be deferred *after* `defer close(outCh)` (defers
// run LIFO, so this one fires first) — a panic anywhere in the tool-call
// loop (a malformed tool response, a nil dereference in a tool handler)
// must not take down the whole backend along with every other active chat,
// the same reasoning taskloop/engine.go's run() already applies to
// task-list goroutines.
func recoverStreamPanic(ctx context.Context, outCh chan<- provider.StreamChunk, label string) {
	if r := recover(); r != nil {
		logx.Printf("PANIC in %s: %v\n%s", label, r, string(debug.Stack()))
		trySend(ctx, outCh, provider.StreamChunk{Error: "⚠️ internal error", Done: true})
	}
}

// fixJSONArgs handles LLMs that return double-encoded JSON
// (e.g., args is "\"{\\\"path\\\": ...}\"" instead of {"path": ...})
func fixJSONArgs(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	// Try to unmarshal as-is into a generic map
	var v interface{}
	if err := json.Unmarshal(raw, &v); err == nil {
		// If it's a string, it might be double-encoded JSON
		if s, ok := v.(string); ok {
			// Try parsing the inner string as JSON
			var inner interface{}
			if json.Unmarshal([]byte(s), &inner) == nil {
				// It was double-encoded — re-marshal as proper JSON
				fixed, _ := json.Marshal(inner)
				return fixed
			}
		}
		// Valid JSON, return as-is
		return raw
	}
	return raw
}

// hallucinatedToolCallPatterns match a model imitating some tool-invocation
// syntax as literal reply text, instead of using the real structured
// tool_calls field this pipeline actually reads (see the len(resp.ToolCalls)
// == 0 branch above). Weaker/free models — or a passthrough provider like
// OpenCode Zen that doesn't wire native function calling — fall back to
// whatever tool syntax they saw most in pretraining. Since resp.ToolCalls
// is empty, this text was never a real call; it's dead prose that would
// otherwise leak straight into the user-visible reply.
//
// Two shapes seen in the wild:
//  1. Claude's <function_calls><invoke name="...">...</invoke></function_calls> XML.
//  2. OpenCode / opencode-zen: <tool_calls:HEXID> ... <tool_call:HEXID>Bash
//     command`> ... description`> ... </tool_calls:HEXID>  (per-call hex id,
//     both singular and plural tags, often several openers and one closer).
//
// Each matches from the opening tag through its matching closing tag, or —
// as a fallback for a truncated response — a bare unclosed one through
// end-of-string. Bounded, no nested quantifiers.
var hallucinatedToolCallPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<function_calls>.*?(</function_calls>|$)`),
	regexp.MustCompile(`(?is)<tool_calls?(:[0-9a-z]+)?>.*?(</tool_calls?(:[0-9a-z]+)?>|$)`),
}

// stripHallucinatedToolSyntax removes hallucinated pseudo-tool-call syntax
// from content the model produced as its final answer (no real tool call
// was parsed for this turn). Only touches content matching the specific
// patterns above — ordinary text mentioning unrelated tags or code samples
// is left untouched.
func stripHallucinatedToolSyntax(content string) string {
	if !strings.Contains(content, "<function_calls>") && !strings.Contains(content, "<tool_call") {
		return content
	}
	cleaned := content
	for _, re := range hallucinatedToolCallPatterns {
		cleaned = re.ReplaceAllString(cleaned, "")
	}
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		logx.Printf("AGENT: model emitted only hallucinated tool-call syntax with no real content, nothing to show")
	}
	return cleaned
}

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"memo/internal/agent/tools"
)

// ExecuteToolCall runs a single tool call in isolation — permission-check
// then execute, no ChatCompletion loop around it — for callers whose "loop"
// is external to this package. The concrete motivating case is Live Mode's
// "standalone" WorkMode (docs/plans/PLAN_live_mode_v2.md §4b): a native
// realtime engine (Google Live/OpenAI Realtime) is given this executor's
// full tool registry directly and decides for itself, via its own
// reasoning, when to call one — there is no local iterate-until-no-more-
// tool-calls loop the way Pipeline.RunStream has, because the engine's own
// session already is that loop.
//
// Mirrors Pipeline.RunStream's per-tool-call section (pipeline.go) as a
// single synchronous call: rate limit, permission check (bypassPermissions/
// autoPermission still apply, same as RunStream), and — the one place this
// can't just inline RunStream's logic — an async permission wait built the
// exact same way RunStream's own waitFn is (e.mu/pendingPerms, 60s
// auto-deny), so a caller resolves a pending request exactly the same way
// as any other agent-mode permission prompt: Executor.HandlePermissionResponse
// (App.HandleAgentPermission). onEvent receives the same AgentEvent types
// RunStream's onEvent would (tool_executing/tool_result/tool_error/
// permission_request/permission_denied) and every call is written to the
// same audit log via logEvent — standalone mode is not a quieter or less
// audited path than normal agent mode, only a differently-driven one.
func (e *Executor) ExecuteToolCall(ctx context.Context, sessionID, toolName string, args json.RawMessage, onEvent func(AgentEvent), projectPath ...string) (string, error) {
	toolDef, ok := e.registry.Get(toolName)
	if !ok {
		err := fmt.Errorf("unknown tool: %s", toolName)
		e.emitToolCallEvent(sessionID, onEvent, AgentEvent{Type: EventToolError, ToolName: toolName, Error: err.Error()})
		return "", err
	}

	effectiveBase := e.basePath
	if len(projectPath) > 0 && projectPath[0] != "" {
		effectiveBase = projectPath[0]
	}
	// A per-call sandbox, same reasoning as RunStream's sessionSandbox:
	// concurrent calls (e.g. this one racing a normal agent-mode RunStream)
	// must not overwrite each other's basePath on a shared sandbox.
	sandbox := NewSandbox(DefaultSandboxConfig(effectiveBase))

	// Mirrors RunStream's own context wiring (executor.go) — without this,
	// change_directory has nothing to widen (SandboxSetterFromContext
	// returns ok=false, it errors "internal error: no sandbox available")
	// and nothing to persist a switch into for the caller's next call
	// (ProjectPathSetterFromContext likewise finds nothing). Found live:
	// this was simply never wired up, so change_directory has never worked
	// through this entry point at all — the tool was offered to standalone-
	// mode Live Mode sessions but silently couldn't do the one thing it's
	// for. The turn-local half (WithSandboxSetter) only matters within
	// this one call; the persist-for-next-call half
	// (WithProjectPathSetter) is what the caller (buildLiveModeToolCallHandler)
	// must also read back via sessionManager.GetProjectPath before its next
	// ExecuteToolCall, the same way llm.go already does for RunStream —
	// this alone only makes the switch stick for later tool calls *within*
	// this call's own registry.Execute, which is a no-op for change_directory
	// itself (SetBasePath) but is what makes SetProjectPath actually get
	// called at all.
	ctx = tools.WithSandboxSetter(ctx, sandbox)
	if e.sessionManager != nil {
		ctx = tools.WithProjectPathSetter(ctx, e.sessionManager, sessionID)
	}

	if err := sandbox.RateLimit(toolName, hashArgs(args)); err != nil {
		e.emitToolCallEvent(sessionID, onEvent, AgentEvent{Type: EventToolError, ToolName: toolName, Error: err.Error()})
		return "", err
	}

	permRes := e.permissions.Check(toolName, args, toolDef.DangerLevel)
	if e.GetBypassPermissions() || e.GetAutoPermission() {
		permRes.NeedPrompt = false
		permRes.Allowed = true
	}

	if permRes.NeedPrompt {
		reqID := generateID()
		var preview string
		if toolDef.PreviewFn != nil {
			preview, _ = toolDef.PreviewFn(args, effectiveBase)
		}
		ev := AgentEvent{
			Type:        EventPermissionRequest,
			RequestID:   reqID,
			ToolName:    toolName,
			Args:        args,
			DangerLevel: toolDef.DangerLevel,
			Preview:     preview,
		}

		// Register the pending request BEFORE emitting the event: a caller's
		// onEvent handler can answer synchronously (HandleAgentPermission /
		// HandlePermissionResponse), and that answer must find reqID already
		// in pendingPerms. Emitting first left a window where a fast
		// responder got "permission request not found" — flaky under -race
		// in CI (TestExecuteToolCall_WaitsForHandlePermissionResponse).
		resCh := e.registerPending(reqID, ev)
		defer e.unregisterPending(reqID)

		e.emitToolCallEvent(sessionID, onEvent, ev)

		policy, err := e.awaitPermission(ctx, resCh)
		if err != nil {
			e.emitToolCallEvent(sessionID, onEvent, AgentEvent{Type: EventToolError, ToolName: toolName, Error: err.Error()})
			return "", err
		}
		e.permissions.HandleResponse(toolName, args, policy)
		permRes.Allowed = policy != DenyOnce && policy != DenyForever
	}

	if !permRes.Allowed {
		e.permissions.ClearOnce(toolName, args)
		e.emitToolCallEvent(sessionID, onEvent, AgentEvent{Type: EventPermissionDenied, ToolName: toolName, Args: args})
		return "", fmt.Errorf("permission denied")
	}

	e.emitToolCallEvent(sessionID, onEvent, AgentEvent{Type: EventToolExecuting, ToolName: toolName, DangerLevel: toolDef.DangerLevel})

	start := time.Now()
	result, err := e.registry.Execute(ctx, toolName, args, effectiveBase, e.backup.CreateBackup)
	duration := time.Since(start).Milliseconds()
	e.permissions.ClearOnce(toolName, args)

	if err != nil {
		e.emitToolCallEvent(sessionID, onEvent, AgentEvent{Type: EventToolError, ToolName: toolName, Args: args, Error: err.Error(), DurationMs: duration})
		return "", err
	}
	e.emitToolCallEvent(sessionID, onEvent, AgentEvent{Type: EventToolResult, ToolName: toolName, Args: args, Result: result, DurationMs: duration})
	return result, nil
}

// emitToolCallEvent logs ev to the audit trail (same as RunStream's
// wrappedOnEvent) and forwards it to the caller-supplied onEvent, if any —
// onEvent is optional (a caller that only wants the final result/error, not
// a live progress feed, may pass nil).
func (e *Executor) emitToolCallEvent(sessionID string, onEvent func(AgentEvent), ev AgentEvent) {
	e.logEvent(sessionID, ev)
	if onEvent != nil {
		onEvent(ev)
	}
}

// registerPending records a pending permission request and returns the
// channel HandlePermissionResponse will deliver the answer on. Must be
// called (and its unregisterPending deferred) before the caller emits the
// permission_request event — see the call site in ExecuteToolCall.
func (e *Executor) registerPending(requestID string, ev AgentEvent) <-chan PermissionPolicy {
	resCh := make(chan PermissionPolicy, 1)
	e.mu.Lock()
	e.pendingPerms[requestID] = &PermissionRequest{ID: requestID, Event: ev, ResCh: resCh}
	e.mu.Unlock()
	return resCh
}

func (e *Executor) unregisterPending(requestID string) {
	e.mu.Lock()
	delete(e.pendingPerms, requestID)
	e.mu.Unlock()
}

// awaitPermission blocks until resCh (from registerPending) receives an
// answer, or auto-denies after 60s.
func (e *Executor) awaitPermission(ctx context.Context, resCh <-chan PermissionPolicy) (PermissionPolicy, error) {
	permTimer := time.NewTimer(60 * time.Second)
	defer permTimer.Stop()

	select {
	case <-ctx.Done():
		return DenyOnce, ctx.Err()
	case <-permTimer.C:
		return DenyOnce, fmt.Errorf("permission timed out")
	case policy := <-resCh:
		return policy, nil
	}
}

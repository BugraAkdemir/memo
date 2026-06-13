package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"memo/internal/provider"
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
	registry    *ToolRegistry
	permissions *PermissionManager
	sandbox     *Sandbox
	prov        AgentProvider
	maxIters    int
	backup      *BackupManager
}

// NewPipeline creates a new agent execution pipeline.
func NewPipeline(registry *ToolRegistry, permissions *PermissionManager, sandbox *Sandbox, prov AgentProvider, backup *BackupManager) *Pipeline {
	return &Pipeline{
		registry:    registry,
		permissions: permissions,
		sandbox:     sandbox,
		prov:        prov,
		maxIters:    20, // Max 20 tool calls per turn to prevent infinite loops
		backup:      backup,
	}
}

// RunStream executes the agent pipeline and streams the results.
// It returns a channel that emits StreamChunks.
// permissionWaitFn blocks until the user approves or denies a tool call.
func (p *Pipeline) RunStream(ctx context.Context, messages []provider.Message, modelName string, onEvent func(AgentEvent), permissionWaitFn func(requestID string, event AgentEvent) (PermissionPolicy, error)) (<-chan provider.StreamChunk, error) {
	outCh := make(chan provider.StreamChunk, 128)

	go func() {
		defer close(outCh)

		currentMessages := make([]provider.Message, len(messages))
		copy(currentMessages, messages)

		for iteration := 0; iteration < p.maxIters; iteration++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			trySend(ctx, outCh, provider.StreamChunk{Error: "Agent execution cancelled", Done: true})
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
		}

		// Call LLM
		resp, err := p.prov.ChatCompletion(ctx, req)
		if err != nil {
			log.Printf("AGENT: ChatCompletion error: %v", err)
			trySend(ctx, outCh, provider.StreamChunk{Error: fmt.Sprintf("LLM Error: %v", err), Done: true})
			return
		}

		// If no tool calls, we are done
		if len(resp.ToolCalls) == 0 {
			if resp.Content != "" {
				trySend(ctx, outCh, provider.StreamChunk{Content: resp.Content})
			}
			onEvent(AgentEvent{Type: EventFinalResponse, Content: resp.Content})
			trySend(ctx, outCh, provider.StreamChunk{Done: true, FinishReason: "stop"})
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

			// Execute!
			onEvent(AgentEvent{Type: EventToolExecuting, ToolName: toolName, Args: args, DangerLevel: toolDef.DangerLevel})

			start := time.Now()
			result, err := p.registry.Execute(toolName, args, basePath, p.backup.CreateBackup)
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

		trySend(ctx, outCh, provider.StreamChunk{Content: "\n\n⚠️ Agent reached the maximum number of tool calls (20). The task may be incomplete.", Done: true})
	}()

	return outCh, nil
}

func trySend(ctx context.Context, outCh chan<- provider.StreamChunk, chunk provider.StreamChunk) {
	select {
	case outCh <- chunk:
	case <-ctx.Done():
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

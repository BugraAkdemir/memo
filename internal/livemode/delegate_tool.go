package livemode

import (
	"context"
	"encoding/json"
)

// ToolSpec is a provider-agnostic tool declaration. internal/livemode/google
// and internal/livemode/openai_realtime each translate a []ToolSpec into
// their own native function-calling wire format (Google's
// functionDeclarations, OpenAI's tools array) — this is the one seam that
// needs to know both, kept out of internal/app so the two provider
// subpackages stay ignorant of anything agent/delegation-specific. See
// docs/plans/PLAN_live_mode_v2.md §4/§4b.
type ToolSpec struct {
	Name        string
	Description string
	// Parameters is a JSON-schema object — both Google's and OpenAI's
	// current function-calling contracts accept the same
	// {"type":"object","properties":{...},"required":[...]} shape, so this
	// is passed through unchanged into each provider's own field name
	// rather than needing a per-provider schema translation.
	Parameters json.RawMessage
}

// ToolCallHandler resolves one tool call to a result string (or an error,
// which the caller formats into a result the model can react to — "Error:
// ..." — same convention agent.Pipeline.RunStream already uses for a
// failed tool call). Supplied by internal/app when constructing a session:
// for WorkMode "delegate" it routes through
// App.SendLiveDelegatedMessageStream/drainLiveDelegatedReply; for
// "standalone" it routes through agent.Executor.ExecuteToolCall. Neither
// google.Client nor openai_realtime.Client know or care which — they only
// know "call this, get a string back, send it as the tool's result".
type ToolCallHandler func(ctx context.Context, name string, args json.RawMessage) (result string, err error)

// DelegateToolName/DelegateToolSpec define the one tool a "delegate"
// WorkMode session gets — see the plan's §4.
const DelegateToolName = "delegate_to_main_model"

func DelegateToolSpec() ToolSpec {
	return ToolSpec{
		Name: DelegateToolName,
		Description: "Hand off a real work request (coding, file/command access, research) " +
			"to Memo's main model. Use this whenever the user asks you to actually do " +
			"something, not just chat. You will receive a result to narrate back to the " +
			"user naturally — you do not do the work yourself.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"instruction":{"type":"string","description":"The task, phrased as if speaking to a capable engineer."}},"required":["instruction"]}`),
	}
}

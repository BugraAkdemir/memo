package agent

import "context"

// TurnOverrides carries per-turn agent-loop settings that override the
// executor's defaults for one RunStream call — set by the app when a turn
// runs in Code Mode. A zero value overrides nothing.
type TurnOverrides struct {
	// MaxIters / MaxContinuations replace the pipeline's ceilings for this
	// turn when > 0.
	MaxIters         int
	MaxContinuations int
	// AutoApproveMedium auto-approves Medium-danger tools (file edits/writes)
	// without a permission prompt. Dangerous tools still prompt.
	AutoApproveMedium bool
}

type turnOverridesCtxKey struct{}

// WithTurnOverrides attaches o to ctx for the next Executor.RunStream to
// apply. A zero-valued o is still attached but changes nothing.
func WithTurnOverrides(ctx context.Context, o TurnOverrides) context.Context {
	return context.WithValue(ctx, turnOverridesCtxKey{}, o)
}

func turnOverridesFromCtx(ctx context.Context) TurnOverrides {
	o, _ := ctx.Value(turnOverridesCtxKey{}).(TurnOverrides)
	return o
}

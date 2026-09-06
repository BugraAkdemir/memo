package agent

import (
	"context"
	"testing"
)

func TestWithTurnOverrides_RoundTrips(t *testing.T) {
	base := context.Background()
	if got := turnOverridesFromCtx(base); got != (TurnOverrides{}) {
		t.Fatalf("no overrides on a bare ctx, got %+v", got)
	}
	o := TurnOverrides{MaxIters: 80, MaxContinuations: 3, AutoApproveMedium: true}
	ctx := WithTurnOverrides(base, o)
	if got := turnOverridesFromCtx(ctx); got != o {
		t.Fatalf("turnOverridesFromCtx = %+v, want %+v", got, o)
	}
}

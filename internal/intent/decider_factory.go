// SPDX-License-Identifier: AGPL-3.0-or-later

package intent

import (
	"context"

	"memo/internal/config"
)

// OrchestraFn is the signature of the app's Orchestra-based LLM call.
type OrchestraFn func(ctx context.Context, prompt string) (string, error)

// SingleCallFn is the signature of a direct single-model LLM call.
type SingleCallFn func(ctx context.Context, modelID, systemPrompt, userPrompt string) (string, error)

// BuildDecider returns a Decider that routes through Orchestra or a single
// model depending on cfg. Both the proactive engine and the intent extractor
// share this factory so the user's "single model mode" preference applies
// uniformly to the whole learning system.
func BuildDecider(cfg config.LearningConfig, orchestra OrchestraFn, single SingleCallFn) Decider {
	if cfg.SingleModelEnabled && cfg.ModelID != "" && single != nil {
		return func(ctx context.Context, system, user string) (string, error) {
			return single(ctx, cfg.ModelID, system, user)
		}
	}
	return func(ctx context.Context, system, user string) (string, error) {
		combined := system + "\n\n---\n\n" + user
		return orchestra(ctx, combined)
	}
}

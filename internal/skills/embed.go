// Package skills embeds Memo's built-in skills into the binary. They are
// written to the on-disk skills directory at startup (see
// skill.MaterializeEmbedded) so they are discovered and inspected like any
// user skill, and reappear if deleted.
package skills

import "embed"

// MemoSystemFS holds the built-in "memo-system" skill: engine-only guidance
// for how a Self-Driving task manages its own provider, model, Orchestra,
// sub-agents, rate-limit waits, and notifications.
//
//go:embed memo-system
var MemoSystemFS embed.FS

package agent

import "context"

type delegatedPermissionsKey struct{}

// WithDelegatedPermissions authorizes the selected task tools for unattended
// execution but preserves explicit user denies. Tools outside the coding set
// and explicitly selected skills fail instead of opening an invisible prompt.
func WithDelegatedPermissions(ctx context.Context) context.Context {
	return context.WithValue(ctx, delegatedPermissionsKey{}, true)
}

func chatSubAgentCoordinationTool(name string) bool {
	switch name {
	case "spawn_agent", "send_agent", "list_agents", "close_agent", "cancel_agent", "merge_agent":
		return true
	}
	return false
}

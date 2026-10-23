package agent

import "context"

type subAgentSkillsKey struct{}

// WithSubAgentSkills carries the explicit child allowlist without enabling a
// skill in any ordinary chat. The caller has validated all selected names.
func WithSubAgentSkills(ctx context.Context, skills map[string]bool) context.Context {
	copySet := make(map[string]bool, len(skills))
	for k, v := range skills {
		copySet[k] = v
	}
	return context.WithValue(ctx, subAgentSkillsKey{}, copySet)
}

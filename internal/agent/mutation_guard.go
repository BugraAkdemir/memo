package agent

import "context"

type mutationGuardKey struct{}
type MutationGuard func(context.Context, string) (func(), error)

// WithMutationGuard lets a host serialize project mutations without preventing
// ordinary conversation or read-only tools while a delegated writer is active.
func WithMutationGuard(ctx context.Context, guard MutationGuard) context.Context {
	return context.WithValue(ctx, mutationGuardKey{}, guard)
}

func mutationTool(t ToolDef) bool {
	if t.SkillOwner != "" {
		return true
	}
	switch t.Name {
	case "write_file", "edit_file", "insert_line", "delete_lines", "delete_file", "run_command", "run_command_readonly", "change_directory", "create_task_md", "edit_task_md", "save_code_plan":
		return true
	}
	return false
}

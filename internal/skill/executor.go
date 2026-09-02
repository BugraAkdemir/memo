package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"memo/internal/agent/tools"
)

// ExecuteTool runs the configured shell command for an active skill's tool
// and returns its captured output. This is what makes a manifest's `tools:`
// entries — previously a purely declarative, never-executed list (TD-1,
// BUG_REPORT.md) — actually runnable by the agent.
//
// The LLM's call arguments are delivered on the child process's stdin as
// raw JSON (never interpolated into the command string, unlike run_command)
// and mirrored in the MEMO_SKILL_ARGS env var for scripts that prefer that.
// The command's working directory is the skill's own install directory, so
// a manifest can reference its own bundled scripts with a relative path
// (e.g. "python3 scripts/format.py"). basePath — the agent's current
// sandboxed project root, the same value run_command receives — is passed
// through as MEMO_PROJECT_DIR for tools that need to act on the user's
// project rather than the skill bundle itself.
func (m *Manager) ExecuteTool(ctx context.Context, skillName, toolName string, args json.RawMessage, basePath string) (string, error) {
	m.mu.RLock()
	def, ok := m.skills[skillName]
	m.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("skill %q not found", skillName)
	}

	var toolSpec *SkillTool
	for i := range def.Manifest.Tools {
		if def.Manifest.Tools[i].Name == toolName {
			toolSpec = &def.Manifest.Tools[i]
			break
		}
	}
	if toolSpec == nil {
		return "", fmt.Errorf("skill %q has no tool %q", skillName, toolName)
	}
	if toolSpec.Command == "" {
		return "", fmt.Errorf("skill tool %q/%q has no command configured", skillName, toolName)
	}

	if pattern, blocked := tools.CheckDestructivePatterns(toolSpec.Command); blocked {
		return "", fmt.Errorf("skill tool command is blacklisted for safety: %s", pattern)
	}

	if len(args) == 0 {
		args = json.RawMessage("{}")
	}

	cmd, execCtx, cancel := tools.PrepareCommand(ctx, toolSpec.Command, def.Path)
	defer cancel()
	cmd.Stdin = bytes.NewReader(args)
	cmd.Env = append(cmd.Env,
		"MEMO_SKILL_ARGS="+string(args),
		"MEMO_SKILL_NAME="+skillName,
		"MEMO_SKILL_DIR="+def.Path,
		"MEMO_PROJECT_DIR="+basePath,
	)

	// Same file-backed capture run_command uses — see RunCapturingOutput for
	// why a bytes.Buffer here would hang on any skill that backgrounds a
	// process.
	stdout, stderr, runErr := tools.RunCapturingOutput(cmd)
	timedOut := execCtx.Err() == context.DeadlineExceeded

	return tools.FormatCommandOutput(runErr, timedOut, stdout, stderr), nil
}

package app

import (
	"context"
	"encoding/json"
	"fmt"

	"memo/internal/agent"
	"memo/internal/logx"
	"memo/internal/skill"
)

// skillToolRegistrar bridges skill.Manager's activate/deactivate lifecycle
// to the real agent.ToolRegistry the agent pipeline executes tool calls
// against. Before this, skill.Manager.SetToolRegistrar was never called
// anywhere in prod code, so a skill's `tools:` manifest entries were
// entirely inert — declared but never reachable by the LLM (TD-1,
// BUG_REPORT.md).
type skillToolRegistrar struct {
	registry *agent.ToolRegistry
	skillMgr *skill.Manager
}

func newSkillToolRegistrar(registry *agent.ToolRegistry, skillMgr *skill.Manager) *skillToolRegistrar {
	return &skillToolRegistrar{registry: registry, skillMgr: skillMgr}
}

// RegisterTool implements skill.ToolRegistrar.
func (r *skillToolRegistrar) RegisterTool(name string, toolDef any) error {
	reg, ok := toolDef.(skill.SkillToolRegistration)
	if !ok {
		return fmt.Errorf("skill tool registrar: unexpected tool definition type %T", toolDef)
	}

	if reg.Tool.Command == "" {
		// Declarative-only entry (no execution mechanism) — a skill manifest
		// may list a tool purely as documentation for its own instructions.
		logx.Printf("SKILL: tool %q (skill %q) has no command, not registered as a callable agent tool", reg.Tool.Name, reg.SkillName)
		return nil
	}

	params := json.RawMessage(`{"type":"object"}`)
	if reg.Tool.Parameters != nil {
		if b, err := json.Marshal(reg.Tool.Parameters); err == nil {
			params = b
		}
	}

	skillName := reg.SkillName
	toolName := reg.Tool.Name

	r.registry.Register(agent.ToolDef{
		Name:        name,
		Description: fmt.Sprintf("[skill:%s] %s", skillName, reg.Tool.Description),
		Parameters:  params,
		DangerLevel: agent.FromString(string(reg.Tool.DangerLevel)),
		ExecuteFn: func(ctx context.Context, args json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
			return r.skillMgr.ExecuteTool(ctx, skillName, toolName, args, basePath)
		},
	})
	return nil
}

// UnregisterTool implements skill.ToolRegistrar.
func (r *skillToolRegistrar) UnregisterTool(name string) {
	r.registry.Unregister(name)
}

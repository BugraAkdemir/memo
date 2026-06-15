package app

import (
	"context"
	"fmt"
	"strings"

	"memo/internal/api"
	"memo/internal/skill"
)

// ListSkills returns all installed skill definitions.
func (a *App) ListSkills() []skill.SkillDefinition {
	if a.skillManager == nil {
		return nil
	}
	defs := a.skillManager.List()
	result := make([]skill.SkillDefinition, len(defs))
	for i, d := range defs {
		result[i] = *d
	}
	return result
}

// InstallSkill installs a skill from the given path.
func (a *App) InstallSkill(path string) (*skill.SkillDefinition, error) {
	if a.skillManager == nil {
		return nil, fmt.Errorf("skill manager not initialized")
	}
	return a.skillManager.Install(path)
}

// RemoveSkill uninstalls a skill by name.
func (a *App) RemoveSkill(name string) error {
	if a.skillManager == nil {
		return fmt.Errorf("skill manager not initialized")
	}
	return a.skillManager.Remove(name)
}

// GetSkill retrieves a skill definition by name.
func (a *App) GetSkill(name string) (*skill.SkillDefinition, error) {
	if a.skillManager == nil {
		return nil, fmt.Errorf("skill manager not initialized")
	}
	def, ok := a.skillManager.Get(name)
	if !ok {
		return nil, fmt.Errorf("skill %q not found", name)
	}
	return def, nil
}

// SetActiveSkills sets the list of active skill names.
func (a *App) SetActiveSkills(names []string) error {
	if a.skillManager == nil {
		return fmt.Errorf("skill manager not initialized")
	}
	return a.skillManager.SetActive(names)
}

// GetActiveSkills returns the names of currently active skills.
func (a *App) GetActiveSkills() []string {
	if a.skillManager == nil {
		return nil
	}
	return a.skillManager.GetActiveNames()
}

// handleSkillCommand intercepts /skill prefixed messages and handles them as commands.
func (a *App) handleSkillCommand(ctx context.Context, userMsg string) <-chan api.StreamChunk {
	if a.skillManager == nil {
		return nil
	}
	if !strings.HasPrefix(userMsg, "/skill") {
		return nil
	}

	ch := make(chan api.StreamChunk, 10)
	msg := strings.TrimSpace(strings.TrimPrefix(userMsg, "/skill"))

	if msg == "" {
		var b strings.Builder
		skills := a.skillManager.List()
		if len(skills) == 0 {
			b.WriteString("**📦 Installed Skills:**\n\nNo skills installed. Use `/skill install <path>` to add one.")
		} else {
			b.WriteString("**📦 Installed Skills:**\n\n")
			for _, s := range skills {
				active := ""
				if a.skillManager.IsActive(s.Manifest.Name) {
					active = " ✅ **active**"
				}
				b.WriteString(fmt.Sprintf("- **%s**%s — %s\n", s.Manifest.Name, active, s.Manifest.Description))
			}
			b.WriteString("\nUse `/skill:on <name>` to activate, `/skill:off <name>` to deactivate.")
		}
		ch <- api.StreamChunk{Content: b.String()}
		ch <- api.StreamChunk{Done: true}
		close(ch)
		return ch
	}

	parts := strings.Fields(msg)
	cmd := parts[0]

	switch {
	case cmd == "install" && len(parts) >= 2:
		path := parts[1]
		def, err := a.skillManager.Install(path)
		if err != nil {
			ch <- api.StreamChunk{Content: fmt.Sprintf("❌ Install failed: %s", err.Error())}
		} else {
			ch <- api.StreamChunk{Content: fmt.Sprintf("✅ Skill **%s** installed successfully!", def.Manifest.Name)}
		}
		ch <- api.StreamChunk{Done: true}
		close(ch)
		return ch

	case cmd == "remove" && len(parts) >= 2:
		name := parts[1]
		if err := a.skillManager.Remove(name); err != nil {
			ch <- api.StreamChunk{Content: fmt.Sprintf("❌ Remove failed: %s", err.Error())}
		} else {
			ch <- api.StreamChunk{Content: fmt.Sprintf("✅ Skill **%s** removed.", name)}
		}
		ch <- api.StreamChunk{Done: true}
		close(ch)
		return ch

	case strings.HasPrefix(cmd, ":on"):
		name := strings.TrimPrefix(cmd, ":on")
		if name == "" && len(parts) >= 2 {
			name = parts[1]
		}
		if name == "" {
			ch <- api.StreamChunk{Content: "❌ Usage: `/skill:on <skill-name>`"}
			ch <- api.StreamChunk{Done: true}
			close(ch)
			return ch
		}
		active := a.skillManager.GetActiveNames()
		alreadyActive := false
		for _, n := range active {
			if n == name {
				alreadyActive = true
				break
			}
		}
		if alreadyActive {
			ch <- api.StreamChunk{Content: fmt.Sprintf("ℹ️ Skill **%s** is already active.", name)}
		} else {
			if err := a.skillManager.SetActive(append(active, name)); err != nil {
				ch <- api.StreamChunk{Content: fmt.Sprintf("❌ Activation failed: %s", err.Error())}
			} else {
				ch <- api.StreamChunk{Content: fmt.Sprintf("✅ Skill **%s** activated.", name)}
			}
		}
		ch <- api.StreamChunk{Done: true}
		close(ch)
		return ch

	case strings.HasPrefix(cmd, ":off"):
		name := strings.TrimPrefix(cmd, ":off")
		if name == "" && len(parts) >= 2 {
			name = parts[1]
		}
		if name == "" {
			a.skillManager.SetActive(nil)
			ch <- api.StreamChunk{Content: "✅ All skills deactivated."}
		} else {
			active := a.skillManager.GetActiveNames()
			var remaining []string
			for _, n := range active {
				if n != name {
					remaining = append(remaining, n)
				}
			}
			if err := a.skillManager.SetActive(remaining); err != nil {
				ch <- api.StreamChunk{Content: fmt.Sprintf("❌ Deactivation failed: %s", err.Error())}
			} else {
				ch <- api.StreamChunk{Content: fmt.Sprintf("✅ Skill **%s** deactivated.", name)}
			}
		}
		ch <- api.StreamChunk{Done: true}
		close(ch)
		return ch

	default:
		ch <- api.StreamChunk{Content: fmt.Sprintf("❌ Unknown skill command: `%s`\n\nUsage:\n- `/skill` — list skills\n- `/skill install <path>` — install from path\n- `/skill remove <name>` — remove skill\n- `/skill:on <name>` — activate skill\n- `/skill:off <name>` — deactivate skill\n- `/skill:off` — deactivate all", cmd)}
		ch <- api.StreamChunk{Done: true}
		close(ch)
		return ch
	}
}

// buildActiveSkillPrompt returns formatted skill instructions block for active skills.
func (a *App) buildActiveSkillPrompt() string {
	if a.skillManager == nil {
		return ""
	}
	activations := a.skillManager.ActiveInstructions()
	if len(activations) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n## Active Skills\n\n")
	b.WriteString("The following skills are active. Follow their instructions carefully:\n\n")
	for _, act := range activations {
		b.WriteString(fmt.Sprintf("### Skill: %s\n", act.Name))
		if act.Description != "" {
			b.WriteString(fmt.Sprintf("_%s_\n\n", act.Description))
		}
		b.WriteString(act.Instructions)
		b.WriteString("\n\n---\n\n")
	}
	return b.String()
}

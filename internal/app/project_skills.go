// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"memo/internal/beta"
	"memo/internal/project"
	"memo/internal/skill"
)

// Project skills live in `<project>/.memo/skills/<name>/SKILL.md`.
//
// They are *instructions only*: whatever tools a project skill's manifest
// declares are dropped. A skill that arrives with a cloned repository is
// untrusted text; letting it register shell-command tools (whose danger level
// it writes itself) would let a repo run commands on the machine of whoever
// opens it. Its instructions are only ever injected into a chat whose person
// switches it on (/skill:on), exactly like a global skill.
//
// The feature is part of the Beta project-folder work: with Beta off project
// skills are invisible.

// parseProjectSkill adapts the real SKILL.md parser to what internal/project
// asks of one, so a project skill is accepted or refused exactly like any other.
func parseProjectSkill(data []byte, dir string) (project.SkillMeta, error) {
	def, err := skill.ParseSkill(data, dir)
	if err != nil {
		return project.SkillMeta{}, err
	}
	return project.SkillMeta{
		Name:        def.Manifest.Name,
		Description: def.Manifest.Description,
		HasTools:    len(def.Manifest.Tools) > 0,
	}, nil
}

// projectRootForChat returns the root of the set-up project chatID works in.
func (a *App) projectRootForChat(chatID string) (path, root string, ok bool) {
	if !beta.Enabled() || !a.isProjectChat(chatID) {
		return "", "", false
	}
	path = a.getSessionManager().GetProjectPath(chatID)
	res, err := a.projectManager().Open(path)
	if err != nil || !res.SetUp {
		return "", "", false
	}
	return path, res.Root, true
}

// projectSkillList lists the project skills available to chatID.
func (a *App) projectSkillList(chatID string) []project.SkillInfo {
	_, root, ok := a.projectRootForChat(chatID)
	if !ok {
		return nil
	}
	return project.ListSkills(root, parseProjectSkill)
}

// projectSkillDef loads project skill name for chatID, tools stripped.
func (a *App) projectSkillDef(chatID, name string) (*skill.SkillDefinition, bool) {
	_, root, ok := a.projectRootForChat(chatID)
	if !ok {
		return nil, false
	}
	data, err := project.ReadSkillFile(root, name)
	if err != nil {
		return nil, false
	}
	def, err := skill.ParseSkill(data, project.SkillsDir(root))
	if err != nil {
		return nil, false
	}
	def.Manifest.Tools = nil
	return def, true
}

// lookupSkill finds a skill by name for chatID: the chat's own project first,
// then the globally installed skills.
func (a *App) lookupSkill(chatID, name string) (*skill.SkillDefinition, bool) {
	if def, ok := a.projectSkillDef(chatID, name); ok {
		return def, true
	}
	if a.skillManager == nil {
		return nil, false
	}
	return a.skillManager.Get(name)
}

// InstallProjectSkill installs a skill from a GitHub source (see
// project.ParseSkillSource) into chatID's project.
func (a *App) InstallProjectSkill(ctx context.Context, chatID, source string) (*project.InstallResult, error) {
	if !beta.Enabled() {
		return nil, errors.New("project skills are a Beta feature; turn on Settings > Beta Features first")
	}
	path, _, ok := a.projectRootForChat(chatID)
	if !ok {
		if !a.isProjectChat(chatID) {
			return nil, errors.New("this is not a project chat; open a chat in a project folder first")
		}
		return nil, project.ErrNotSetUp
	}
	src, err := project.ParseSkillSource(source)
	if err != nil {
		return nil, err
	}
	return a.projectManager().InstallSkill(ctx, path, src, project.InstallOptions{Parse: parseProjectSkill, BaseURL: a.projectSkillBaseURL})
}

// RemoveProjectSkill deletes project skill name from chatID's project and
// switches it off in the chat.
func (a *App) RemoveProjectSkill(chatID, name string) error {
	path, _, ok := a.projectRootForChat(chatID)
	if !ok {
		return project.ErrSkillNotFound
	}
	if err := a.projectManager().RemoveSkill(path, name); err != nil {
		return err
	}
	if sm := a.getSessionManager(); sm != nil {
		var keep []string
		for _, n := range sm.GetActiveSkills(chatID) {
			if n != name {
				keep = append(keep, n)
			}
		}
		_ = sm.SetActiveSkills(chatID, keep)
	}
	return nil
}

// projectSkillToolAdapter backs the install_project_skill agent tool.
type projectSkillToolAdapter struct{ a *App }

func (ad projectSkillToolAdapter) InstallProjectSkill(ctx context.Context, source string) (string, error) {
	res, err := ad.a.InstallProjectSkill(ctx, currentChatIDFromContext(ctx), source)
	if err != nil {
		return "", err
	}
	return describeInstalledSkill(res), nil
}

func describeInstalledSkill(res *project.InstallResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Installed skill %q (%d files) into .memo/skills/%s from %s.", res.Name, res.Files, res.Dir, res.Source)
	if res.Description != "" {
		fmt.Fprintf(&sb, " %s.", strings.TrimRight(res.Description, "."))
	}
	fmt.Fprintf(&sb, " It is not active yet: the user turns it on with /skill:on %s.", res.Dir)
	if res.HasTools {
		sb.WriteString(" Tools declared by this skill are ignored; only its instructions are used.")
	}
	return sb.String()
}

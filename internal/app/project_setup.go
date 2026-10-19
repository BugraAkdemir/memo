// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"fmt"
	"path/filepath"

	"memo/internal/beta"
	"memo/internal/project"
)

// minimalModeOn reports Minimal Mode, read from the identity (the field the
// prompt builder itself checks), not the config copy.
func (a *App) minimalModeOn() bool {
	return a.identity != nil && a.identity.GetMinimalMode()
}

// ProjectStatus tells the UI whether path can be a Memo project, whether it is
// set up, and whether to offer setting it up. It writes nothing.
func (a *App) ProjectStatus(path string) project.UIStatus {
	st := project.UIStatus{Status: a.projectManager().Status(path), Enabled: beta.Enabled()}
	st.ShouldAsk = st.Enabled && !a.minimalModeOn() && st.Allowed && !st.SetUp && !st.Declined
	return st
}

// SetupProject creates the project's `.memo/` folder and starter files. It is
// an explicit user action (the UI asks first) and is part of the Beta feature.
func (a *App) SetupProject(path string) (*project.ScaffoldResult, error) {
	if !beta.Enabled() {
		return nil, fmt.Errorf("project folders are a Beta feature; turn on Settings > Beta Features first")
	}
	return a.projectManager().Scaffold(path)
}

// DeclineProject records "never set up Memo in this folder" (or undoes it).
func (a *App) DeclineProject(path string, declined bool) error {
	return a.projectManager().SetDeclined(path, declined)
}

// projectInstruction is the one line that starts every session in a set-up
// project: read the generic rules first. The files themselves are never put in
// the prompt (the model reads them with its tools when it needs them), which
// keeps the prompt small and its prefix byte-stable for prompt caches.
//
// It is empty under Minimal Mode (pure wrapper: Memo adds nothing), when the
// feature is off, for plain chats, and for a folder that is not set up.
func (a *App) projectInstruction(chatID string) string {
	if !beta.Enabled() || a.minimalModeOn() || !a.isProjectChat(chatID) {
		return ""
	}
	sm := a.getSessionManager()
	path := sm.GetProjectPath(chatID)
	res, err := a.projectManager().Open(path)
	if err != nil || !res.SetUp {
		return ""
	}
	rules := filepath.ToSlash(filepath.Join(project.DirName, "AGENTS.md"))
	if filepath.Clean(path) != res.Root {
		// The chat works in a sub-folder of the project: point at the real root.
		rules = filepath.ToSlash(filepath.Join(res.Root, rules))
	}
	return "\n\nThis is a Memo project. Before anything else, read `" + rules + "` and follow it."
}

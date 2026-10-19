// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"path/filepath"

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
	st := project.UIStatus{Status: a.projectManager().Status(path)}
	st.ShouldAsk = !a.minimalModeOn() && st.Allowed && !st.SetUp && !st.Declined
	return st
}

// SetupProject creates the project's `.memo/` folder and starter files. It is
// an explicit user action (the UI asks first).
func (a *App) SetupProject(path string) (*project.ScaffoldResult, error) {
	return a.projectManager().Scaffold(path)
}

// DeclineProject records "never set up Memo in this folder" (or undoes it).
func (a *App) DeclineProject(path string, declined bool) error {
	return a.projectManager().SetDeclined(path, declined)
}

// ProjectPermissionsStatus returns the existence and trust status of a project's permissions.json.
func (a *App) ProjectPermissionsStatus(path string) (project.PermissionsStatus, error) {
	return a.projectManager().CheckPermissionsStatus(path)
}

// GetProjectPermissions reads the project's permissions file and status.
func (a *App) GetProjectPermissions(path string) (*project.Permissions, project.PermissionsStatus, error) {
	st, err := a.projectManager().CheckPermissionsStatus(path)
	if err != nil {
		return nil, st, err
	}
	root, ok := a.projectManager().Guard().FindRoot(path)
	if !ok {
		var checkErr error
		if root, checkErr = a.projectManager().Guard().Check(path); checkErr != nil {
			return nil, st, checkErr
		}
	}
	perms, _, err := project.ReadPermissionsFile(root)
	return perms, st, err
}

// SaveProjectPermissions writes and automatically trusts permissions.json for a project.
func (a *App) SaveProjectPermissions(path string, perms *project.Permissions) error {
	return a.projectManager().SavePermissions(path, perms)
}

// TrustProjectPermissions marks an existing permissions.json as trusted by saving its hash.
func (a *App) TrustProjectPermissions(path string) (string, error) {
	return a.projectManager().TrustPermissions(path)
}

// projectInstruction is the one line that starts every session in a set-up
// project: read the generic rules first. The files themselves are never put in
// the prompt (the model reads them with its tools when it needs them), which
// keeps the prompt small and its prefix byte-stable for prompt caches.
//
// It is empty under Minimal Mode (pure wrapper: Memo adds nothing), for plain
// chats, and for a folder that is not set up.
func (a *App) projectInstruction(chatID string) string {
	if a.minimalModeOn() || !a.isProjectChat(chatID) {
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

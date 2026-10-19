// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"os"

	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/memory"
	"memo/internal/project"
)

// projectManager returns the per-device project registry, building it on first
// use. The guard it carries keeps a project from ever being rooted at the home
// folder or inside Memo's own install/data/config directories.
func (a *App) projectManager() *project.Manager {
	a.projectMgrMu.Lock()
	defer a.projectMgrMu.Unlock()
	if a.projectMgr == nil {
		home, _ := os.UserHomeDir()
		protected := []string{config.DataDir(), config.ConfigDir()}
		if memoHome, err := memoHomeDir(); err == nil {
			protected = append(protected, memoHome)
		}
		a.projectMgr = project.NewManager(
			project.Guard{Home: home, Protected: protected},
			config.DataPath("projects.json"),
		)
	}
	return a.projectMgr
}

// memoryScopeForChat decides which part of the memory pool a chat may read and
// write.
//
//   - A plain chat (no ProjectPath), and the Live Mode background chat, use
//     memory.AllProjects: they read every pool and write to the general pool.
//   - A project (agent) chat is confined to its own project's pool. The project
//     id comes from the folder's `.memo/project.json` (or the registry before
//     the folder is set up), is recorded on the session, and is re-resolved on
//     every call so change_directory into another project switches the scope.
//   - A project chat whose project cannot be identified (e.g. the folder was
//     deleted and no id was ever recorded) gets ok=false: no memory at all.
//     Failing closed here is the privacy guarantee — it must never fall back to
//     the general pool.
func (a *App) memoryScopeForChat(chatID string) (sc memory.Scope, ok bool) {
	sm := a.getSessionManager()
	if sm == nil || chatID == "" || chatID == a.liveModeChatID {
		return memory.AllProjects, true
	}
	path := sm.GetProjectPath(chatID)
	if path == "" {
		return memory.AllProjects, true
	}

	res, err := a.projectManager().Open(path)
	if err == nil {
		if res.ID != sm.GetProjectID(chatID) {
			if setErr := sm.SetProjectID(chatID, res.ID); setErr != nil {
				logx.Printf("PROJECT: record id on chat %s: %v", chatID, setErr)
			}
		}
		return memory.ProjectScope(res.ID), true
	}
	if id := sm.GetProjectID(chatID); id != "" {
		// The folder is gone or unreadable right now, but this chat already
		// belongs to a known project: keep it there. (A recorded id can never be
		// stale from a directory change: SetProjectPath clears it.)
		return memory.ProjectScope(id), true
	}
	logx.Printf("PROJECT: chat %s has no usable project (%v); memory off for it", chatID, err)
	return memory.Scope{}, false
}

// memoryAllowedFor reports whether a chat with scope sc may read/write memory.
//
// A project (agent) chat has a memory of its own and it does not depend on Code
// Mode: Code Mode is the default for project chats, and it only strips what is
// *personal* (persona, mood, time block, background LLM calls); the project's
// own pool is not personal. A plain chat is unchanged: memory unless the chat
// was pinned to Code Mode. Whether memory is on at all is the Memory switch
// (GetMemoryEnabled), checked by every caller.
func (a *App) memoryAllowedFor(sc memory.Scope, code bool) bool {
	return sc.InProject() || !code
}

// isProjectChat reports whether chatID is a project (agent) chat for which the
// user's personal identity must not reach the model.
func (a *App) isProjectChat(chatID string) bool {
	sm := a.getSessionManager()
	if sm == nil || chatID == "" || chatID == a.liveModeChatID {
		return false
	}
	return sm.GetProjectPath(chatID) != ""
}

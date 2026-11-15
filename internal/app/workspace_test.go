package app

import (
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/memory"
	"memo/internal/sessions"
)

func TestResolveDefaultWorkspaceDir_UsesHomeMemoWorkspaceForPersistentDataDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	nonTmpDataDir := "/var/memo-persistent-test/.memo/data"
	wantWorkspace := filepath.Join(fakeHome, "MemoWorkspace")
	if got := resolveDefaultWorkspaceDir(nonTmpDataDir); got != wantWorkspace {
		t.Fatalf("resolveDefaultWorkspaceDir(%q) = %q, want %q", nonTmpDataDir, got, wantWorkspace)
	}

	isolated := resolveDefaultWorkspaceDir(t.TempDir())
	if filepath.Base(isolated) != "workspace" {
		t.Fatalf("expected temp test data dir to isolate into .../workspace, got %q", isolated)
	}
}

func TestChangeDirectoryPreservesNonProjectChatAndMemoryScope(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager: %v", err)
	}
	a := &App{sessions: sm}

	tgChat := sm.NewBackgroundChat("Telegram Asistanı")
	newDir := t.TempDir()
	if err := sm.SetWorkingDir(tgChat, newDir); err != nil {
		t.Fatalf("SetWorkingDir: %v", err)
	}

	if sm.IsAgentChat(tgChat) {
		t.Fatal("SetWorkingDir must NOT turn a Telegram/background chat into a project chat")
	}
	if got := sm.GetProjectPath(tgChat); got != "" {
		t.Fatalf("GetProjectPath = %q, want empty", got)
	}
	if got := sm.GetWorkingDir(tgChat); got != newDir {
		t.Fatalf("GetWorkingDir = %q, want %q", got, newDir)
	}
	if scope, ok := a.memoryScopeForChat(tgChat); !ok || scope != memory.AllProjects {
		t.Fatalf("memoryScopeForChat = (%+v, %v), want (%+v, true)", scope, ok, memory.AllProjects)
	}
}

func TestDefaultWorkspaceNameIsNotMemoInstallDir(t *testing.T) {
	if defaultWorkspaceFolderName != "MemoWorkspace" {
		t.Fatalf("unexpected defaultWorkspaceFolderName %q", defaultWorkspaceFolderName)
	}
	a := &App{}
	dir := a.defaultWorkspaceDir()
	if dir == "" || strings.HasSuffix(dir, "/.memo") {
		t.Fatalf("defaultWorkspaceDir returned unsafe install dir %q", dir)
	}
}

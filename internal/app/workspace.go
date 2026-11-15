package app

import (
	"os"
	"path/filepath"
	"strings"

	"memo/internal/config"
	"memo/internal/logx"
)

const defaultWorkspaceFolderName = "MemoWorkspace"

// defaultWorkspaceDir resolves the safe default sandbox directory for
// non-project chats (desktop, web, Telegram, WhatsApp) when no WorkingDir or
// ProjectPath has been selected on the session.
//
// In normal operation it uses ~/MemoWorkspace so Memo never falls back to its
// own installation folder (~/.memo). When DataDir lives inside os.TempDir()
// (isolated Go unit/E2E tests), it uses <DataDir>/workspace instead so tests
// never touch the real user's home directory.
func (a *App) defaultWorkspaceDir() string {
	return resolveDefaultWorkspaceDir(config.DataDir())
}

func resolveDefaultWorkspaceDir(dataDir string) string {
	tmpClean := filepath.Clean(os.TempDir())
	dataClean := filepath.Clean(dataDir)
	var dir string
	if dataClean != "" && (dataClean == tmpClean || strings.HasPrefix(dataClean, tmpClean+string(filepath.Separator))) {
		dir = filepath.Join(dataClean, "workspace")
	} else if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		dir = filepath.Join(home, defaultWorkspaceFolderName)
	} else if dataClean != "" {
		dir = filepath.Join(dataClean, "workspace")
	} else {
		dir, _ = filepath.Abs(".")
	}
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			logx.Printf("workspace: could not ensure default workspace %q: %v", dir, err)
		}
	}
	return dir
}

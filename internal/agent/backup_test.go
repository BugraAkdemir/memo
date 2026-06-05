package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "backup_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	bm := NewBackupManager(tempDir)

	testFilePath := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFilePath, []byte("original content"), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	err = bm.CreateBackup(testFilePath)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// Modify file
	err = os.WriteFile(testFilePath, []byte("modified content"), 0644)
	if err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	// Undo
	err = bm.UndoLast()
	if err != nil {
		t.Fatalf("failed to undo: %v", err)
	}

	content, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}
	if string(content) != "original content" {
		t.Errorf("expected original content, got %s", string(content))
	}
}

func TestBackupManager_MaxBackups(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "backup_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	bm := NewBackupManager(tempDir)
	bm.maxEntries = 2

	testFilePath := filepath.Join(tempDir, "test.txt")
	
	for i := 0; i < 4; i++ {
		os.WriteFile(testFilePath, []byte("content"), 0644)
		bm.CreateBackup(testFilePath)
	}

	// Check history
	bm.mu.Lock()
	historyLength := len(bm.history)
	bm.mu.Unlock()

	if historyLength != 2 {
		t.Errorf("expected history length to be 2, got %d", historyLength)
	}
}

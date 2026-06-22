package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type BackupActionType string

const (
	ActionModify BackupActionType = "modify"
	ActionCreate BackupActionType = "create"
	ActionDelete BackupActionType = "delete"
)

type BackupEntry struct {
	ID         string           `json:"id"`
	Action     BackupActionType `json:"action"`
	FilePath   string           `json:"file_path"`   // Absolute path to the original file
	BackupPath string           `json:"backup_path"` // Absolute path to the backup file
	Timestamp  time.Time        `json:"timestamp"`
}

type BackupManager struct {
	mu         sync.Mutex
	backupDir  string
	history    []BackupEntry
	historyFile string
	maxEntries int
}

func NewBackupManager(dataDir string) *BackupManager {
	backupDir := filepath.Join(dataDir, "agent-backups")
	historyFile := filepath.Join(backupDir, "history.json")

	os.MkdirAll(backupDir, 0755)

	bm := &BackupManager{
		backupDir:  backupDir,
		historyFile: historyFile,
		maxEntries: 50,
	}
	bm.loadHistory()
	return bm
}

func (bm *BackupManager) loadHistory() {
	data, err := os.ReadFile(bm.historyFile)
	if err == nil {
		if err := json.Unmarshal(data, &bm.history); err != nil {
			log.Printf("AGENT: failed to parse backup history: %v", err)
		}
	}
}

func (bm *BackupManager) saveHistoryLocked() {
	// Keep only the last maxEntries
	if len(bm.history) > bm.maxEntries {
		// Clean up old backup files
		for i := 0; i < len(bm.history)-bm.maxEntries; i++ {
			if bm.history[i].BackupPath != "" {
				os.Remove(bm.history[i].BackupPath)
			}
		}
		bm.history = bm.history[len(bm.history)-bm.maxEntries:]
	}

	data, err := json.MarshalIndent(bm.history, "", "  ")
	if err == nil {
		os.WriteFile(bm.historyFile, data, 0600)
	}
}

// CreateBackup takes a snapshot of a file before modification or deletion.
// If the file does not exist, it's considered a "create" action.
func (bm *BackupManager) CreateBackup(filePath string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	
	// Check if file exists
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		// File doesn't exist, so this will be a Create action
		entry := BackupEntry{
			ID:        id,
			Action:    ActionCreate,
			FilePath:  filePath,
			Timestamp: time.Now(),
		}
		bm.history = append(bm.history, entry)
		bm.saveHistoryLocked()
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// File exists, take a backup
	backupName := fmt.Sprintf("%s_%s.bak", filepath.Base(filePath), id)
	backupPath := filepath.Join(bm.backupDir, backupName)

	if err := copyFile(filePath, backupPath); err != nil {
		return fmt.Errorf("failed to copy file to backup: %w", err)
	}

	entry := BackupEntry{
		ID:         id,
		Action:     ActionModify, // Default to modify, tools can explicitly change this to Delete later if needed
		FilePath:   filePath,
		BackupPath: backupPath,
		Timestamp:  time.Now(),
	}
	bm.history = append(bm.history, entry)
	bm.saveHistoryLocked()
	return nil
}

// RecordDelete marks the last backup entry for this file as a Delete action
func (bm *BackupManager) RecordDelete(filePath string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for i := len(bm.history) - 1; i >= 0; i-- {
		if bm.history[i].FilePath == filePath && bm.history[i].Action == ActionModify {
			bm.history[i].Action = ActionDelete
			bm.saveHistoryLocked()
			return
		}
	}
}

// UndoLast reverts the last action in the history stack.
func (bm *BackupManager) UndoLast() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if len(bm.history) == 0 {
		return fmt.Errorf("no actions to undo")
	}

	lastEntry := bm.history[len(bm.history)-1]

	var undoErr error
	switch lastEntry.Action {
	case ActionCreate:
		// File was created, so undo means deleting it
		if err := os.Remove(lastEntry.FilePath); err != nil && !os.IsNotExist(err) {
			undoErr = fmt.Errorf("failed to delete created file: %w", err)
		}
	case ActionModify, ActionDelete:
		// File was modified or deleted, so undo means restoring it from backup
		if lastEntry.BackupPath == "" {
			undoErr = fmt.Errorf("no backup path found for entry %s", lastEntry.ID)
			break
		}
		
		// Ensure destination directory exists
		os.MkdirAll(filepath.Dir(lastEntry.FilePath), 0755)
		
		if err := copyFile(lastEntry.BackupPath, lastEntry.FilePath); err != nil {
			undoErr = fmt.Errorf("failed to restore file from backup: %w", err)
		} else {
			// Cleanup backup file
			os.Remove(lastEntry.BackupPath)
		}
	}

	if undoErr != nil {
		return undoErr
	}

	// Remove the entry from history
	bm.history = bm.history[:len(bm.history)-1]
	bm.saveHistoryLocked()

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

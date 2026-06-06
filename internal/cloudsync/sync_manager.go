// Package cloudsync implements a background Google Drive backup pipeline for Memo.
//
// Every 50 persisted messages the manager:
//  1. Archives all .gob files from the memory persist directory into an in-memory ZIP.
//  2. Encrypts the ZIP with AES-256-GCM using a key derived from the user's passphrase
//     (or a hardware ID if no passphrase is configured).
//  3. Uploads the encrypted blob to Google Drive's hidden appDataFolder.
//  4. Prunes old backups, keeping only the 3 most recent.
//
// The entire pipeline runs in a goroutine so it never blocks the UI.
// Progress is reported to the Wails frontend via `sync:status` and `sync:error` events.
package cloudsync

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

const (
	// SyncInterval is the number of saved interactions that trigger a backup.
	SyncInterval = 50
	// RollingBackupCount is the maximum number of backups kept in Drive.
	RollingBackupCount = 3
)

// Manager counts saved interactions and kicks off the backup pipeline
// every SyncInterval interactions.
type Manager struct {
	ctx        context.Context
	persistDir string
	passphrase string
	drive      *driveClient
	interval   int64

	count      atomic.Int64
	mu         sync.Mutex // guards in-flight backup
	scheduleMu sync.Mutex // serializes Increment / TriggerNow scheduling race
	inFlight   bool
}

// AccountInfo describes the connected Google account.
type AccountInfo struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// New creates a Manager. It is safe to call when sync is disabled — in that
// case Increment() is a no-op.
//
//   - persistDir: directory containing .gob files (e.g. "./data/memory")
//   - passphrase: AES key source; empty = hardware-derived key
//   - clientID/clientSecret: Google OAuth2 Desktop App credentials
//   - tokenPath: where the OAuth token is stored on disk
func New(
	ctx context.Context,
	persistDir string,
	passphrase string,
	intervalMessages int,
	clientID, clientSecret, tokenPath string,
) *Manager {
	if intervalMessages <= 0 {
		intervalMessages = SyncInterval
	}
	if passphrase == "" {
		passphrase = loadOrCreateMachineID(persistDir)
	}
	dc, err := newDriveClient(clientID, clientSecret, tokenPath)
	if err != nil {
		log.Printf("WARN: cloudsync: %v", err)
	}
	return &Manager{
		ctx:        ctx,
		persistDir: persistDir,
		passphrase: passphrase,
		drive:      dc,
		interval:   int64(intervalMessages),
	}
}

// loadOrCreateMachineID reads a persistent machine identifier from a file,
// or generates one on first run and stores it.
func loadOrCreateMachineID(dir string) string {
	path := filepath.Join(dir, ".machine-id")
	if data, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id
		}
	}
	id := uuid.NewString()
	if err := os.WriteFile(path, []byte(id+"\n"), 0600); err != nil {
		log.Printf("cloudsync: failed to write machine-id: %v", err)
	}
	return id
}

// Increment records one saved interaction. When the count reaches a multiple
// of SyncInterval a background backup is launched (at most one at a time).
func (m *Manager) Increment() {
	m.scheduleMu.Lock()
	n := m.count.Add(1)
	if n%m.interval != 0 {
		m.scheduleMu.Unlock()
		return
	}
	m.mu.Lock()
	if m.inFlight {
		m.mu.Unlock()
		m.scheduleMu.Unlock()
		log.Println("cloudsync: backup already in flight, skipping")
		return
	}
	m.inFlight = true
	m.mu.Unlock()
	m.scheduleMu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.inFlight = false
			m.mu.Unlock()
		}()
		m.runPipeline()
	}()
}

// TriggerNow forces an immediate backup regardless of the message counter.
// It is non-blocking; it spawns a goroutine if no backup is already running.
func (m *Manager) TriggerNow() {
	m.scheduleMu.Lock()
	m.mu.Lock()
	if m.inFlight {
		m.mu.Unlock()
		m.scheduleMu.Unlock()
		m.emit("sync:status", "busy")
		return
	}
	m.inFlight = true
	m.mu.Unlock()
	m.scheduleMu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.inFlight = false
			m.mu.Unlock()
		}()
		m.runPipeline()
	}()
}

// TriggerPullNow downloads, decrypts and restores the latest backup from Drive.
func (m *Manager) TriggerPullNow() {
	m.mu.Lock()
	if m.inFlight {
		m.mu.Unlock()
		m.emit("sync:status", "busy")
		return
	}
	m.inFlight = true
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.inFlight = false
			m.mu.Unlock()
		}()
		m.runPullPipeline()
	}()
}

// TriggerFullSyncNow runs push (backup) then pull (restore latest) in one flow.
func (m *Manager) TriggerFullSyncNow() {
	m.mu.Lock()
	if m.inFlight {
		m.mu.Unlock()
		m.emit("sync:status", "busy")
		return
	}
	m.inFlight = true
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.inFlight = false
			m.mu.Unlock()
		}()
		m.emit("sync:status", "syncing")
		if !m.runPipeline() {
			return
		}
		m.runPullPipeline()
	}()
}

// IsAuthenticated returns true if Google Drive auth is ready.
func (m *Manager) IsAuthenticated() bool {
	return m.drive.IsAuthenticated()
}

// StartAuthFlow begins the OAuth2 loopback flow and returns the URL to open.
func (m *Manager) StartAuthFlow() (string, error) {
	return m.drive.StartAuthFlow()
}

// WaitForAuth blocks until the OAuth2 flow completes or ctx is cancelled.
func (m *Manager) WaitForAuth(ctx context.Context) error {
	return m.drive.WaitForAuth(ctx)
}

// GetAccountInfo returns name/email for the authenticated Google account.
func (m *Manager) GetAccountInfo(ctx context.Context) (AccountInfo, error) {
	name, email, err := m.drive.GetAccountInfo(ctx)
	if err != nil {
		return AccountInfo{}, err
	}
	return AccountInfo{Name: name, Email: email}, nil
}

// ─── Pipeline ─────────────────────────────────────────────────────────────────

func (m *Manager) runPipeline() bool {
	log.Println("cloudsync: starting backup pipeline")

	// 1. Archive
	m.emit("sync:status", "archiving")
	zipBuf, err := m.archive()
	if err != nil {
		m.emitError(fmt.Sprintf("Archive failed: %v", err))
		return false
	}
	log.Printf("cloudsync: archive ready (%d bytes)", len(zipBuf))

	// 2. Encrypt (local, before any network I/O)
	m.emit("sync:status", "encrypting")
	blob, err := encrypt(m.passphrase, zipBuf)
	if err != nil {
		m.emitError(fmt.Sprintf("Encryption failed: %v", err))
		return false
	}
	log.Printf("cloudsync: encrypted blob (%d bytes)", len(blob))

	// 3. Upload
	m.emit("sync:status", "uploading")
	if _, err := m.drive.UploadBackup(blob); err != nil {
		m.emitError(fmt.Sprintf("Upload failed: %v", err))
		return false
	}

	// 4. Prune old backups (rolling window)
	m.emit("sync:status", "pruning")
	if err := m.drive.PruneOldBackups(RollingBackupCount); err != nil {
		// Non-fatal: log and continue.
		log.Printf("cloudsync: prune warning: %v", err)
	}

	m.emit("sync:status", "done")
	log.Println("cloudsync: backup pipeline complete")
	return true
}

func (m *Manager) runPullPipeline() bool {
	log.Println("cloudsync: starting pull pipeline")

	m.emit("sync:status", "pulling")
	name, blob, err := m.drive.DownloadLatestBackup()
	if err != nil {
		m.emitError(fmt.Sprintf("Pull failed: %v", err))
		return false
	}
	log.Printf("cloudsync: pulled backup %s (%d bytes)", name, len(blob))

	m.emit("sync:status", "decrypting")
	zipBuf, err := decrypt(m.passphrase, blob)
	if err != nil {
		m.emitError(fmt.Sprintf("Decrypt failed: %v", err))
		return false
	}

	m.emit("sync:status", "restoring")
	if err := m.restoreZip(zipBuf); err != nil {
		m.emitError(fmt.Sprintf("Restore failed: %v", err))
		return false
	}

	m.emit("sync:status", "done")
	log.Println("cloudsync: pull pipeline complete")
	return true
}

// archive walks persistDir and adds all .gob files (including _chromem.gob)
// into an in-memory ZIP buffer.
func (m *Manager) archive() ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	added := 0

	err := filepath.Walk(m.persistDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(info.Name()), ".gob") {
			return nil
		}

		relPath, err := filepath.Rel(m.persistDir, path)
		if err != nil {
			return err
		}

		// Use zip.FileHeader to preserve timestamps for accurate restore.
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath
		header.Method = zip.Deflate

		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(w, f)
		if err == nil {
			added++
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("cloudsync: archive walk: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("cloudsync: archive close: %w", err)
	}
	if added == 0 {
		return nil, fmt.Errorf("cloudsync: no .gob files found under %s", m.persistDir)
	}
	return buf.Bytes(), nil
}

func (m *Manager) restoreZip(zipData []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("cloudsync: restore open zip: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "memo-sync-restore-*")
	if err != nil {
		return fmt.Errorf("cloudsync: restore temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	extracted := 0
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		cleanName := filepath.Clean(zf.Name)
		if cleanName == "." || strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			continue
		}
		if !strings.EqualFold(filepath.Ext(cleanName), ".gob") {
			continue
		}

		target := filepath.Join(tmpDir, cleanName)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("cloudsync: restore mkdir: %w", err)
		}

		rc, err := zf.Open()
		if err != nil {
			return fmt.Errorf("cloudsync: restore open entry: %w", err)
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return fmt.Errorf("cloudsync: restore create file: %w", err)
		}
		_, cpErr := io.Copy(out, rc)
		clErr := out.Close()
		rc.Close()
		if cpErr != nil {
			return fmt.Errorf("cloudsync: restore copy entry: %w", cpErr)
		}
		if clErr != nil {
			return fmt.Errorf("cloudsync: restore close file: %w", clErr)
		}
		extracted++
	}
	if extracted == 0 {
		return fmt.Errorf("cloudsync: restore zip has no .gob files")
	}

	if err := os.MkdirAll(m.persistDir, 0755); err != nil {
		return fmt.Errorf("cloudsync: restore ensure persist dir: %w", err)
	}
	if err := filepath.Walk(m.persistDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(info.Name()), ".gob") {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("cloudsync: restore clear old gob: %w", err)
	}

	if err := filepath.Walk(tmpDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(tmpDir, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(m.persistDir, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		dstFile, err := os.Create(dest)
		if err != nil {
			srcFile.Close()
			return err
		}
		_, cpErr := io.Copy(dstFile, srcFile)
		srcCloseErr := srcFile.Close()
		dstCloseErr := dstFile.Close()
		if cpErr != nil {
			return cpErr
		}
		if srcCloseErr != nil {
			return srcCloseErr
		}
		if dstCloseErr != nil {
			return dstCloseErr
		}
		return nil
	}); err != nil {
		return fmt.Errorf("cloudsync: restore copy to persist: %w", err)
	}

	return nil
}

// ─── Event helpers ────────────────────────────────────────────────────────────

func (m *Manager) emit(event string, payload any) {
	// Headless modunda (Flutter client) Wails events emit fonksiyonu panik yarattığı için kaldırıldı.
	log.Printf("SYNC EVENT: %s - %v", event, payload)
}

func (m *Manager) emitError(msg string) {
	log.Printf("cloudsync ERROR: %s", msg)
	m.emit("sync:error", msg)
}

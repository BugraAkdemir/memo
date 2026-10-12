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
	"database/sql"
	"fmt"
	"io"
	"memo/internal/backupdata"
	"memo/internal/logx"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
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
	persistDir string // e.g. data/memory  — SQLite + legacy .gob
	dataDir    string // e.g. data/         — sessions/, providers.json, etc.
	configFile string
	passphrase string
	drive      *driveClient
	interval   int64

	count      atomic.Int64
	stopped    atomic.Bool // set by Stop() to prevent new goroutine launches
	mu         sync.Mutex  // guards in-flight backup
	scheduleMu sync.Mutex  // serializes Increment / TriggerNow scheduling race
	inFlight   bool

	// BeforeRestore/AfterRestore let the owner release and re-open resources that
	// hold the files being overwritten by a pull (notably the live SQLite memory
	// store). Restoring memory.db while it is still open corrupts the database, so
	// the store must be closed before restoreZip and re-opened afterwards.
	BeforeRestore func()
	AfterRestore  func()

	// CheckpointMemoryDB/CheckpointMoodDB let the owner route archive()'s
	// pre-backup WAL checkpoint through the live store (memory.Store.Checkpoint/
	// mood.Engine.Checkpoint) instead of this package opening its own raw
	// sql.Open connection to the same file — see BUG_REPORT.md P1-4: a second,
	// unsynchronized connection racing the real store's own writes. Optional:
	// this package has no live store reference of its own (it's constructed
	// from file paths only, runs as a background job independent of the
	// app's object graph), so a nil callback falls back to the old
	// raw-connection behavior rather than skipping the checkpoint outright —
	// set by internal/app at construction time; tests that build a Manager
	// directly are unaffected if left unset.
	CheckpointMemoryDB func(ctx context.Context) error
	CheckpointMoodDB   func(ctx context.Context) error
}

// SetBackupPaths is called before scheduling starts, so a custom memory
// directory never changes the user-data root or the resolved config location.
func (m *Manager) SetBackupPaths(dataDir, configFile string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dataDir = dataDir
	m.configFile = configFile
}

// AccountInfo describes the connected Google account.
type AccountInfo struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// New creates a Manager. It is safe to call when sync is disabled — in that
// case Increment() is a no-op.
//
//   - persistDir: memory directory (e.g. "./data/memory")
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
	dataDir := filepath.Dir(persistDir) // data/memory → data/
	if passphrase == "" {
		// Migrate machine-id from old location (data/memory/.machine-id)
		// to new wipe-safe location (data/.machine-id). The old location
		// lived inside the memory directory which WipeAllData removes.
		oldPath := filepath.Join(persistDir, ".machine-id")
		newPath := filepath.Join(dataDir, ".machine-id")
		if _, err := os.Stat(oldPath); err == nil {
			if data, err := os.ReadFile(oldPath); err == nil {
				_ = os.WriteFile(newPath, data, 0600)
				_ = os.Remove(oldPath)
			}
		}
		passphrase = loadOrCreateMachineID(dataDir)
		logx.Printf("WARN: cloudsync: no passphrase configured — backups are encrypted with a "+
			"machine-specific key stored in %s. Backups cannot be restored on a different machine. "+
			"Set a passphrase in Settings → Cloud Sync for portable encryption.", dataDir)
	}
	dc, err := newDriveClient(clientID, clientSecret, tokenPath)
	if err != nil {
		logx.Printf("WARN: cloudsync: %v", err)
	}
	return &Manager{
		ctx:        ctx,
		persistDir: persistDir,
		dataDir:    dataDir,
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
		logx.Printf("cloudsync: failed to write machine-id: %v", err)
	}
	return id
}

// Stop marks the Manager as stopped so no new backup goroutines are launched.
// In-flight uploads (if any) run to completion.
func (m *Manager) Stop() {
	m.stopped.Store(true)
}

// Increment records one saved interaction. When the count reaches a multiple
// of SyncInterval a background backup is launched (at most one at a time).
func (m *Manager) Increment() {
	if m.stopped.Load() {
		return
	}
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
		logx.Info("cloudsync: backup already in flight, skipping")
		return
	}
	m.inFlight = true
	m.mu.Unlock()
	m.scheduleMu.Unlock()

	go func() {
		defer logx.Recover("cloudsync.Manager/runPipeline")
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
	if m == nil || m.drive == nil {
		return
	}
	if m.stopped.Load() {
		return
	}
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
		defer logx.Recover("cloudsync.Manager.TriggerNow/runPipeline")
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
	if m == nil || m.drive == nil {
		return
	}
	if m.stopped.Load() {
		return
	}
	m.mu.Lock()
	if m.inFlight {
		m.mu.Unlock()
		m.emit("sync:status", "busy")
		return
	}
	m.inFlight = true
	m.mu.Unlock()

	go func() {
		defer logx.Recover("cloudsync.Manager.TriggerPullNow/runPullPipeline")
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
	if m == nil || m.drive == nil {
		return
	}
	if m.stopped.Load() {
		return
	}
	m.mu.Lock()
	if m.inFlight {
		m.mu.Unlock()
		m.emit("sync:status", "busy")
		return
	}
	m.inFlight = true
	m.mu.Unlock()

	go func() {
		defer logx.Recover("cloudsync.Manager.TriggerFullSyncNow")
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
	if m == nil || m.drive == nil {
		return false
	}
	return m.drive.IsAuthenticated()
}

// StartAuthFlow begins the OAuth2 loopback flow and returns the URL to open.
func (m *Manager) StartAuthFlow() (string, error) {
	if m == nil || m.drive == nil {
		return "", fmt.Errorf("cloud sync drive client not initialized")
	}
	return m.drive.StartAuthFlow()
}

// WaitForAuth blocks until the OAuth2 flow completes or ctx is cancelled.
func (m *Manager) WaitForAuth(ctx context.Context) error {
	if m == nil || m.drive == nil {
		return fmt.Errorf("cloud sync drive client not initialized")
	}
	return m.drive.WaitForAuth(ctx)
}

// GetAccountInfo returns name/email for the authenticated Google account.
func (m *Manager) GetAccountInfo(ctx context.Context) (AccountInfo, error) {
	if m == nil || m.drive == nil {
		return AccountInfo{}, fmt.Errorf("cloud sync drive client not initialized")
	}
	name, email, err := m.drive.GetAccountInfo(ctx)
	if err != nil {
		return AccountInfo{}, err
	}
	return AccountInfo{Name: name, Email: email}, nil
}

// ─── Pipeline ─────────────────────────────────────────────────────────────────

func (m *Manager) runPipeline() bool {
	if m == nil || m.drive == nil {
		return false
	}
	logx.Info("cloudsync: starting backup pipeline")

	// 1. Archive
	m.emit("sync:status", "archiving")
	zipBuf, err := m.archive()
	if err != nil {
		m.emitError(fmt.Sprintf("Archive failed: %v", err))
		return false
	}
	logx.Printf("cloudsync: archive ready (%d bytes)", len(zipBuf))

	// 2. Encrypt (local, before any network I/O)
	m.emit("sync:status", "encrypting")
	blob, err := encrypt(m.passphrase, zipBuf)
	if err != nil {
		m.emitError(fmt.Sprintf("Encryption failed: %v", err))
		return false
	}
	logx.Printf("cloudsync: encrypted blob (%d bytes)", len(blob))

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
		logx.Printf("cloudsync: prune warning: %v", err)
	}

	m.emit("sync:status", "done")
	logx.Info("cloudsync: backup pipeline complete")
	return true
}

func (m *Manager) runPullPipeline() bool {
	if m == nil || m.drive == nil {
		return false
	}
	logx.Info("cloudsync: starting pull pipeline")

	m.emit("sync:status", "pulling")
	name, blob, err := m.drive.DownloadLatestBackup()
	if err != nil {
		m.emitError(fmt.Sprintf("Pull failed: %v", err))
		return false
	}
	logx.Printf("cloudsync: pulled backup %s (%d bytes)", name, len(blob))

	m.emit("sync:status", "decrypting")
	zipBuf, err := decrypt(m.passphrase, blob)
	if err != nil {
		m.emitError(fmt.Sprintf("Decrypt failed: %v", err))
		return false
	}

	m.emit("sync:status", "restoring")
	// Close any open handles to the files we're about to overwrite (the live
	// memory.db), then always re-open afterwards — even on failure — so the app
	// is never left without its memory store.
	if m.BeforeRestore != nil {
		m.BeforeRestore()
	}
	restoreErr := m.restoreZip(zipBuf)
	if m.AfterRestore != nil {
		m.AfterRestore()
	}
	if restoreErr != nil {
		m.emitError(fmt.Sprintf("Restore failed: %v", restoreErr))
		return false
	}

	m.emit("sync:status", "done")
	logx.Info("cloudsync: pull pipeline complete")
	return true
}

// archive creates an in-memory ZIP of all user data:
//
//	memory/memory.db          — SQLite vector+memory store
//	memory/<*.gob>            — legacy chromem-go files (kept for compat)
//	sessions/<id>.json        — chat history
//	data/providers.json       — API provider config
//	data/orchestra.json       — orchestra config
//	data/permissions.json     — agent permissions
//	data/profile/patterns.json — learned time patterns
func (m *Manager) archive() ([]byte, error) {
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	for _, item := range []struct {
		path       string
		checkpoint func(context.Context) error
	}{
		{filepath.Join(m.persistDir, "memory.db"), m.CheckpointMemoryDB},
		{filepath.Join(m.dataDir, "mood", "mood.db"), m.CheckpointMoodDB},
	} {
		_, err := os.Lstat(item.path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if item.checkpoint != nil {
			err = item.checkpoint(ctx)
		} else {
			var db *sql.DB
			db, err = sql.Open("sqlite3", item.path)
			if err == nil {
				_, err = db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
				db.Close()
			}
		}
		if err != nil {
			return nil, fmt.Errorf("cloudsync: checkpoint %s: %w", item.path, err)
		}
	}
	configFile := m.configFile
	if configFile == "" {
		configFile = filepath.Join(filepath.Dir(m.dataDir), "config", "config.yaml")
	}
	data, count, err := backupdata.Archive(backupdata.Sources(m.dataDir, m.persistDir, configFile, true, false))
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, fmt.Errorf("cloudsync: no data files found to archive under %s", m.dataDir)
	}
	logx.Printf("cloudsync: archive complete — %d files", count)
	return data, nil
}

// restoreZip extracts a backup ZIP and writes files back to their correct locations.
//
// ZIP path prefixes map to real directories:
//
//	memory/memory.db          → persistDir/memory.db
//	memory/<rel>.gob          → persistDir/<rel>.gob  (legacy)
//	sessions/<id>.json        → dataDir/sessions/<id>.json
//	data/<name>.json          → dataDir/<name>.json
//	data/profile/patterns.json → dataDir/profile/patterns.json
func (m *Manager) restoreZip(zipData []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("cloudsync: restore open zip: %w", err)
	}

	const maxTotalBytes = 500 << 20 // 500 MB guard
	const maxFileBytes = 200 << 20  // 200 MB per file

	// resolveEntry maps a zip entry name (forward-slash, no leading /)
	// to an absolute destination path. Returns "" to skip unknown entries.
	resolveEntry := func(name string) string {
		name = filepath.ToSlash(filepath.Clean(name))
		if name == "." || strings.HasPrefix(name, "..") {
			return ""
		}
		switch {
		case name == "config/config.yaml":
			if m.configFile != "" {
				return m.configFile
			}
			return filepath.Join(filepath.Dir(m.dataDir), "config", "config.yaml")
		case name == "memory/memory.db",
			name == "memory/memory.db-wal",
			name == "memory/memory.db-shm":
			return filepath.Join(m.persistDir, filepath.FromSlash(strings.TrimPrefix(name, "memory/")))
		case strings.HasPrefix(name, "memory/") && strings.HasSuffix(name, ".gob"):
			rel := strings.TrimPrefix(name, "memory/")
			return filepath.Join(m.persistDir, filepath.FromSlash(rel))
		case strings.HasPrefix(name, "sessions/") && strings.HasSuffix(name, ".json"):
			base := filepath.Base(filepath.FromSlash(name))
			return filepath.Join(m.dataDir, "sessions", base)
		case strings.HasPrefix(name, "data/") && backupdata.IsDataEntry(strings.TrimPrefix(name, "data/"), false):
			rel := strings.TrimPrefix(name, "data/")
			return filepath.Join(m.dataDir, filepath.FromSlash(rel))
		case name == "mood/mood.db",
			name == "mood/mood.db-wal",
			name == "mood/mood.db-shm":
			return filepath.Join(m.dataDir, "mood", filepath.FromSlash(strings.TrimPrefix(name, "mood/")))
		// Backwards compat: old zips stored .gob files without a prefix.
		case strings.HasSuffix(name, ".gob") && !strings.Contains(name, "/"):
			return filepath.Join(m.persistDir, name)
		case strings.HasSuffix(name, ".gob"):
			return filepath.Join(m.persistDir, filepath.FromSlash(name))
		}
		return ""
	}

	var totalUncompressed int64
	extracted := 0

	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		dest := resolveEntry(zf.Name)
		if dest == "" {
			logx.Printf("cloudsync: restore: skipping unknown entry %q", zf.Name)
			continue
		}

		uncompSize := int64(zf.UncompressedSize64)
		if uncompSize > maxFileBytes {
			return fmt.Errorf("cloudsync: restore entry %q too large (%d bytes)", zf.Name, uncompSize)
		}
		if totalUncompressed+uncompSize > maxTotalBytes {
			return fmt.Errorf("cloudsync: restore total size exceeds %d bytes", maxTotalBytes)
		}
		totalUncompressed += uncompSize

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return fmt.Errorf("cloudsync: restore mkdir for %s: %w", dest, err)
		}

		// Write to a temp file next to the destination, then rename atomically.
		tmpDest := dest + ".synctmp"
		rc, err := zf.Open()
		if err != nil {
			return fmt.Errorf("cloudsync: restore open zip entry %q: %w", zf.Name, err)
		}
		// Restored files (memory.db, mood.db, sessions/*.json, providers.json,
		// permissions.json, orchestra.json, patterns.json) all live under the
		// data directory and may contain sensitive data (e.g. encrypted API
		// keys in providers.json), so write them with owner-only permissions
		// rather than relying on os.Create's default (~0644).
		out, err := os.OpenFile(tmpDest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			rc.Close()
			return fmt.Errorf("cloudsync: restore create tmp %s: %w", tmpDest, err)
		}
		_, cpErr := io.Copy(out, io.LimitReader(rc, maxFileBytes))
		clErr := out.Close()
		rc.Close()
		if cpErr != nil {
			os.Remove(tmpDest)
			return fmt.Errorf("cloudsync: restore copy %q: %w", zf.Name, cpErr)
		}
		if clErr != nil {
			os.Remove(tmpDest)
			return fmt.Errorf("cloudsync: restore close tmp: %w", clErr)
		}
		if err := os.Rename(tmpDest, dest); err != nil {
			// Windows: destination may be briefly open by antivirus/indexer.
			// Fall back to copy-then-delete so restore doesn't fail mid-way.
			if copyErr := copyRestoreFile(tmpDest, dest); copyErr != nil {
				os.Remove(tmpDest)
				return fmt.Errorf("cloudsync: restore rename %s: %w", dest, err)
			}
			os.Remove(tmpDest)
		}
		extracted++
		logx.Printf("cloudsync: restored %s", dest)
	}

	if extracted == 0 {
		return fmt.Errorf("cloudsync: restore zip had no recognisable entries")
	}
	logx.Printf("cloudsync: restore complete — %d files", extracted)
	return nil
}

// ─── Event helpers ────────────────────────────────────────────────────────────

// copyRestoreFile copies src to dst byte-for-byte (cross-platform rename fallback).
func copyRestoreFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func (m *Manager) emit(event string, payload any) {
	// Headless modunda (Flutter client) Wails events emit fonksiyonu panik yarattığı için kaldırıldı.
	logx.Printf("SYNC EVENT: %s - %v", event, payload)
}

func (m *Manager) emitError(msg string) {
	logx.Printf("cloudsync ERROR: %s", msg)
	m.emit("sync:error", msg)
}

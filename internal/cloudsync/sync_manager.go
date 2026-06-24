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
	persistDir string // e.g. data/memory  — SQLite + legacy .gob
	dataDir    string // e.g. data/         — sessions/, providers.json, etc.
	passphrase string
	drive      *driveClient
	interval   int64

	count      atomic.Int64
	stopped    atomic.Bool   // set by Stop() to prevent new goroutine launches
	mu         sync.Mutex    // guards in-flight backup
	scheduleMu sync.Mutex    // serializes Increment / TriggerNow scheduling race
	inFlight   bool

	// BeforeRestore/AfterRestore let the owner release and re-open resources that
	// hold the files being overwritten by a pull (notably the live SQLite memory
	// store). Restoring memory.db while it is still open corrupts the database, so
	// the store must be closed before restoreZip and re-opened afterwards.
	BeforeRestore func()
	AfterRestore  func()
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
		dataDir:    filepath.Dir(persistDir), // data/memory → data/
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
	log.Println("cloudsync: pull pipeline complete")
	return true
}

// archive creates an in-memory ZIP of all user data:
//
//   memory/memory.db          — SQLite vector+memory store
//   memory/<*.gob>            — legacy chromem-go files (kept for compat)
//   sessions/<id>.json        — chat history
//   data/providers.json       — API provider config
//   data/orchestra.json       — orchestra config
//   data/permissions.json     — agent permissions
//   data/profile/patterns.json — learned time patterns
func (m *Manager) archive() ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	added := 0

	addFile := func(src, destPath string) error {
		info, err := os.Stat(src)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = destPath
		header.Method = zip.Deflate
		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	}

	// 1. memory.db (primary SQLite memory store)
	memDB := filepath.Join(m.persistDir, "memory.db")
	if err := addFile(memDB, "memory/memory.db"); err == nil {
		added++
		log.Printf("cloudsync: archived memory.db")
	} else if !os.IsNotExist(err) {
		log.Printf("cloudsync: WARN skipping memory.db: %v", err)
	}

	// 2. Legacy .gob files (chromem-go; may be absent on new installs)
	_ = filepath.Walk(m.persistDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(info.Name()), ".gob") {
			return nil
		}
		rel, err := filepath.Rel(m.persistDir, path)
		if err != nil {
			return nil
		}
		if addErr := addFile(path, "memory/"+filepath.ToSlash(rel)); addErr == nil {
			added++
		}
		return nil
	})

	// 3. Chat sessions
	sessDir := filepath.Join(m.dataDir, "sessions")
	_ = filepath.Walk(sessDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(info.Name()), ".json") {
			return nil
		}
		if addErr := addFile(path, "sessions/"+info.Name()); addErr == nil {
			added++
		}
		return nil
	})

	// 4. Root data JSON files
	for _, name := range []string{"providers.json", "orchestra.json", "permissions.json"} {
		p := filepath.Join(m.dataDir, name)
		if err := addFile(p, "data/"+name); err == nil {
			added++
		}
	}

	// 5. Mood DB
	moodDB := filepath.Join(m.dataDir, "mood", "mood.db")
	if err := addFile(moodDB, "mood/mood.db"); err == nil {
		added++
		log.Printf("cloudsync: archived mood.db")
	}

	// 6. Learned patterns
	patternsPath := filepath.Join(m.dataDir, "profile", "patterns.json")
	if err := addFile(patternsPath, "data/profile/patterns.json"); err == nil {
		added++
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("cloudsync: archive close: %w", err)
	}
	if added == 0 {
		return nil, fmt.Errorf("cloudsync: no data files found to archive under %s", m.dataDir)
	}
	log.Printf("cloudsync: archive complete — %d files", added)
	return buf.Bytes(), nil
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
		case name == "memory/memory.db":
			return filepath.Join(m.persistDir, "memory.db")
		case strings.HasPrefix(name, "memory/") && strings.HasSuffix(name, ".gob"):
			rel := strings.TrimPrefix(name, "memory/")
			return filepath.Join(m.persistDir, filepath.FromSlash(rel))
		case strings.HasPrefix(name, "sessions/") && strings.HasSuffix(name, ".json"):
			base := filepath.Base(filepath.FromSlash(name))
			return filepath.Join(m.dataDir, "sessions", base)
		case strings.HasPrefix(name, "data/") && strings.HasSuffix(name, ".json"):
			rel := strings.TrimPrefix(name, "data/")
			return filepath.Join(m.dataDir, filepath.FromSlash(rel))
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
			log.Printf("cloudsync: restore: skipping unknown entry %q", zf.Name)
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
		out, err := os.Create(tmpDest)
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
			os.Remove(tmpDest)
			return fmt.Errorf("cloudsync: restore rename %s: %w", dest, err)
		}
		extracted++
		log.Printf("cloudsync: restored %s", dest)
	}

	if extracted == 0 {
		return fmt.Errorf("cloudsync: restore zip had no recognisable entries")
	}
	log.Printf("cloudsync: restore complete — %d files", extracted)
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

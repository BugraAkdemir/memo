package app

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"memo/internal/logx"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"memo/internal/config"
	"memo/internal/fileutil"
	"memo/internal/sessions"
)

// backupsDir returns the directory where pre-import snapshots are stored.
func backupsDir() string { return config.DataPath("backups") }

// ExportData packages all user data (except models) into a .memo zip archive.
func (a *App) ExportData(includeModels bool) ([]byte, error) {
	// Force WAL checkpoint on memory.db so that committed-but-unmerged WAL
	// transactions are flushed to the main database file before archiving.
	// Without this, recent interactions may be missing from the export.
	checkpointMemoryDB(config.DataPath("memory", "memory.db"))

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	addFile := func(name, src string) error {
		fi, err := os.Stat(src)
		if err != nil {
			return nil // skip missing files
		}
		if fi.IsDir() {
			return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				// Skip atomic-write temp files — they are never valid data.
				if strings.HasSuffix(path, ".tmp") {
					return nil
				}
				rel, _ := filepath.Rel(src, path)
				w, err := zw.Create(filepath.Join(name, rel))
				if err != nil {
					return err
				}
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				_, err = io.Copy(w, f)
				f.Close()
				return err
			})
		}
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		f.Close()
		return err
	}

	addFile("sessions/", config.DataPath("sessions"))
	addFile("config/config.yaml", "config/config.yaml")
	addFile("data/providers.json", config.DataPath("providers.json"))
	addFile("data/orchestra.json", config.DataPath("orchestra.json"))
	addFile("data/permissions.json", config.DataPath("permissions.json"))
	// machine.key decrypts providers.json's encrypted API keys (see
	// provider.defaultMachineKey) — without it, restoring providers.json on a
	// different machine (or after a data wipe that regenerates the key)
	// silently leaves every API key as undecryptable garbage. This was
	// missing entirely before: providers.json was exported but its key never
	// was, so importing this backup anywhere but the exact machine that made
	// it already broke every configured provider.
	addFile("data/machine.key", config.DataPath("machine.key"))
	addFile("data/memory/", config.DataPath("memory"))
	addFile("data/whatsapp/", config.DataPath("whatsapp"))
	addFile("data/mood/", config.DataPath("mood"))
	addFile("data/calendar/", config.DataPath("calendar"))
	addFile("data/profile/", config.DataPath("profile"))
	addFile("data/routines/", config.DataPath("routines"))
	addFile("data/tasklists/", config.DataPath("tasklists"))
	addFile("data/stats/", config.DataPath("stats"))
	addFile("data/agent-backups/", config.DataPath("agent-backups"))
	addFile("data/skills/", config.DataPath("skills"))
	// Deliberately NOT exported — machine/account-specific, not portable user
	// data: sync_token.json (a Google Drive incremental-sync cursor tied to
	// this device; restoring it elsewhere could make cloud sync think it's
	// already up to date and skip real remote changes), tailscale/ (tsnet
	// node identity/keys — same reason WipeAllData's wipePreserve treats it
	// as machine state, not user content), and backups/ (this function's own
	// pre-import snapshot directory — including it would nest old .memo
	// archives inside the new one).
	if includeModels {
		addFile("data/models/", config.DataPath("models"))
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// resolveImportTarget maps a zip entry name (as written by ExportData) to its
// final on-disk destination, given the current data and config directories.
// ok is false for anything outside the three recognized top-level prefixes,
// or whose resolved target would escape its expected root — including via a
// crafted "../.." entry name in a foreign or tampered archive.
//
// dataDir/configDir are passed in (rather than read from the config package
// directly) so this stays a pure function: easy to unit test with fake
// roots, and immune to config.DataDir()'s process-wide sync.Once caching.
func resolveImportTarget(name, dataDir, configDir string) (target, root string, ok bool) {
	switch {
	case strings.HasPrefix(name, "sessions/"):
		root = dataDir
		target = filepath.Join(root, "sessions", strings.TrimPrefix(name, "sessions/"))
	case strings.HasPrefix(name, "data/"):
		root = dataDir
		target = filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(name, "data/")))
	case strings.HasPrefix(name, "config/"):
		// This case was missing entirely before: ExportData writes
		// "config/config.yaml" (see below), but neither the "sessions/" nor
		// "data/" case matched it, so it fell into the old "default: target
		// = clean" branch — a path relative to the *process's current
		// working directory*, not any controlled root. The old escape-check
		// (relative to DataDir/DataPath, both siblings of ConfigDir, not
		// ancestors of it) then always rejected it as "escaping the data
		// directory", so every .memo import silently restored everything
		// except config.yaml — all settings (llama, sync, identity, memory,
		// API, learning, calendar) stayed on their pre-import values with no
		// error surfaced anywhere.
		root = configDir
		target = filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(name, "config/")))
	default:
		return "", "", false
	}

	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", false
	}
	return target, root, true
}

// ImportData restores user data from a .memo zip archive.
func (a *App) ImportData(data []byte) error {
	// Validate the archive first — before touching anything on disk.
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("import: invalid zip: %w", err)
	}

	// Snapshot existing data into the dedicated backups subdirectory.
	// Non-fatal: a failed snapshot logs a warning but does not block the import.
	a.snapshotPreImport()

	// Two-phase import for atomicity. All archive entries are first extracted
	// into a private staging directory; only once EVERY entry has been staged
	// successfully do we move them into their live destinations. If staging
	// fails partway through, the live data directory is never touched — the
	// half-written staging dir is simply discarded, leaving the original files
	// (memory.db, providers.json, sessions, …) completely intact. This is a
	// stronger guarantee than relying on the pre-import snapshot for recovery.
	if err := os.MkdirAll(config.DataDir(), 0755); err != nil {
		return fmt.Errorf("import: mkdir data dir: %w", err)
	}
	// The staging dir lives inside the data dir so that the final moves are
	// same-filesystem renames (atomic, no cross-device copy).
	staging, err := os.MkdirTemp(config.DataDir(), ".import-staging-")
	if err != nil {
		return fmt.Errorf("import: create staging dir: %w", err)
	}
	// Always clean up: after a successful import the staging tree holds only
	// empty directories (files were renamed out); after a failure it still holds
	// partial data. Either way it must not linger under the data dir.
	defer os.RemoveAll(staging)

	// pending records staged-file → final-destination pairs, applied only after
	// the entire archive has been extracted without error.
	type pendingMove struct{ staged, target string }
	var pending []pendingMove

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		clean := filepath.Clean(filepath.FromSlash(f.Name))
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
			continue
		}

		target, _, ok := resolveImportTarget(f.Name, config.DataDir(), config.ConfigDir())
		if !ok {
			continue
		}

		staged := filepath.Join(staging, clean)

		if err := os.MkdirAll(filepath.Dir(staged), 0755); err != nil {
			return fmt.Errorf("import: mkdir staging: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("import: open %s: %w", f.Name, err)
		}

		out, err := os.Create(staged)
		if err != nil {
			rc.Close()
			return fmt.Errorf("import: create staged %s: %w", f.Name, err)
		}

		_, err = io.Copy(out, rc)
		rc.Close()
		cerr := out.Close()
		if err != nil {
			return fmt.Errorf("import: write staged %s: %w", f.Name, err)
		}
		if cerr != nil {
			return fmt.Errorf("import: close staged %s: %w", f.Name, cerr)
		}

		pending = append(pending, pendingMove{staged: staged, target: target})
	}

	// Phase 2: every entry is safely staged. Move each staged file into its live
	// destination. Renames are same-filesystem (staging lives inside the data
	// dir) so each is atomic. Should a rename fail here, the pre-import snapshot
	// taken above remains available for manual recovery.
	for _, m := range pending {
		if err := os.MkdirAll(filepath.Dir(m.target), 0755); err != nil {
			return fmt.Errorf("import: mkdir: %w", err)
		}
		if err := os.Rename(m.staged, m.target); err != nil {
			return fmt.Errorf("import: install %s: %w", m.target, err)
		}
	}

	// Re-initialize components after import
	a.sessionsMu.Lock()
	if a.sessions != nil {
		sm, err := sessions.NewManager(config.DataPath("sessions"))
		if err == nil {
			a.sessions = sm
		}
	}
	a.sessionsMu.Unlock()

	// Reload memory.db into a fresh *memory.Store. os.Rename replaces the
	// directory entry, but the live store still holds its original SQLite
	// file handle/inode open (Linux rename semantics don't invalidate an
	// already-open fd) — without this, the running app keeps reading and
	// writing the *old* memory.db until the next full restart, and the
	// freshly-imported memory.db just sits on disk unused. Mirrors the same
	// reinitMemoryStore call WipeAllData already makes after replacing the
	// store's backing file.
	a.clientMu.RLock()
	embClient := a.embeddingClient
	mainClient := a.client
	a.clientMu.RUnlock()
	client := embClient
	if client == nil {
		client = mainClient
	}
	if client != nil {
		a.reinitMemoryStore(client, a.cfg.API.EmbeddingModel)
	} else {
		logx.Printf("WARN: import: no embedding/main client available, memory store left on pre-import data until restart")
	}

	// providers.json/orchestra.json were just replaced on disk (both are
	// exported under "data/"), but without this the running app kept
	// serving chats through the pre-import in-memory router/conductor —
	// including a provider the import just removed still looking "active" —
	// until the next full restart.
	a.reinitProviderAndOrchestra()

	return nil
}

// snapshotPreImport creates a timestamped backup of the current data in the
// dedicated backups/ subdirectory, then prunes old backups keeping the most
// recent maxImportBackups copies.
func (a *App) snapshotPreImport() {
	const maxImportBackups = 3

	dir := backupsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		logx.Printf("import: snapshot: mkdir %s: %v", dir, err)
		return
	}

	existing, err := a.ExportData(false)
	if err != nil {
		logx.Printf("import: snapshot: export failed (proceeding anyway): %v", err)
		return
	}

	name := fmt.Sprintf("pre_import_%s.zip", time.Now().Format("20060102_150405"))
	path := filepath.Join(dir, name)
	if err := fileutil.AtomicWrite(path, existing, 0600); err != nil {
		logx.Printf("import: snapshot: write failed (proceeding anyway): %v", err)
		return
	}
	logx.Printf("import: snapshot saved → %s", path)

	pruneImportBackups(dir, maxImportBackups)
}

// pruneImportBackups removes the oldest pre_import_*.zip files, keeping only
// the most recent `keep` snapshots.
func pruneImportBackups(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var snapshots []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "pre_import_") && strings.HasSuffix(e.Name(), ".zip") {
			snapshots = append(snapshots, filepath.Join(dir, e.Name()))
		}
	}

	// ReadDir returns entries sorted by name; our timestamp format is lexicographically
	// monotone, so oldest entries come first.
	sort.Strings(snapshots)

	for len(snapshots) > keep {
		old := snapshots[0]
		snapshots = snapshots[1:]
		if err := os.Remove(old); err != nil {
			logx.Printf("import: prune backup %s: %v", old, err)
		} else {
			logx.Printf("import: pruned old snapshot %s", old)
		}
	}
}

// wipePreserve is the set of top-level data-dir entries that survive a full
// wipe: the user's downloaded models plus the llama.cpp/runtime binaries, so a
// reset does not force a multi-hundred-MB re-download. Everything else under
// the data dir (sessions, memory, whatsapp, providers, mood, calendar,
// profile, permissions, skills, datasets, backups…) is removed.
var wipePreserve = map[string]bool{
	"models":      true, // downloaded GGUF models
	"bin":         true, // llama.cpp binaries
	"binaries":    true, // downloaded runtime binaries
	"tmp_llama":   true, // llama.cpp build/extract scratch
	"tailscale":   true, // tsnet state
	".machine-id": true, // cloud sync machine identity (encryption key derivation)
}

// WipeAllData removes ALL user data under the data directory except the
// downloaded models and the llama/runtime binaries. This is a true factory
// reset of personal state, not just memory.
func (a *App) WipeAllData() error {
	root := config.DataDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("wipe: read data dir: %w", err)
	}
	for _, e := range entries {
		name := e.Name()
		if wipePreserve[name] || name == "providers.example.json" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
			return fmt.Errorf("wipe: %s: %w", name, err)
		}
	}

	a.sessionsMu.Lock()
	if a.sessions != nil {
		sm, err := sessions.NewManager(config.DataPath("sessions"))
		if err == nil {
			a.sessions = sm
		}
	}
	a.sessionsMu.Unlock()

	// Re-create the memory store so the freshly-deleted DB is rebuilt.
	// Without this, a.store stays nil and every subsequent save/retrieve
	// fails with "store not initialized" until the app is restarted.
	// Prefer the dedicated embedding client; fall back to the main client.
	a.clientMu.RLock()
	embClient := a.embeddingClient
	mainClient := a.client
	a.clientMu.RUnlock()
	client := embClient
	if client == nil {
		client = mainClient
	}
	if client != nil {
		a.reinitMemoryStore(client, a.cfg.API.EmbeddingModel)
	} else {
		a.storeMu.Lock()
		a.store = nil
		a.storeMu.Unlock()
		logx.Info("WARN: wipe: no embedding/main client available, memory store left uninitialized")
	}

	logx.Info("All user data wiped")
	return nil
}

// checkpointMemoryDB forces a WAL checkpoint on memory.db so committed
// transactions are flushed to the main database file. This ensures exports
// contain recent data that may still be in the WAL journal. Errors are
// non-fatal — a missing/unopened DB (e.g. first run) is silently skipped.
func checkpointMemoryDB(dbPath string) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		logx.Printf("export: WAL checkpoint on %s: %v", dbPath, err)
	}
}

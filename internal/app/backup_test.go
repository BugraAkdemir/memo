package app

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"memo/internal/config"
)

// TestResolveImportTarget is a regression test for BUG-QC1: ExportData
// writes "config/config.yaml" (backup.go's addFile calls), but
// ImportData's target-resolution switch previously had no case for the
// "config/" prefix, so it fell through to a "relative to CWD" default and
// was then rejected by the escape-check (config dir is a sibling of the
// data dir, not a subdirectory) — every .memo import silently restored
// everything except config.yaml, with no error.
func TestResolveImportTarget(t *testing.T) {
	dataDir := filepath.FromSlash("/data")
	configDir := filepath.FromSlash("/config")

	cases := []struct {
		name       string
		entry      string
		wantTarget string
		wantOK     bool
	}{
		{
			name:       "sessions entry",
			entry:      "sessions/abc123.json",
			wantTarget: filepath.Join(dataDir, "sessions", "abc123.json"),
			wantOK:     true,
		},
		{
			name:       "data entry",
			entry:      "data/providers.json",
			wantTarget: filepath.Join(dataDir, "providers.json"),
			wantOK:     true,
		},
		{
			name:       "data entry nested",
			entry:      "data/memory/memory.db",
			wantTarget: filepath.Join(dataDir, "memory", "memory.db"),
			wantOK:     true,
		},
		{
			name:       "config entry — the actual bug",
			entry:      "config/config.yaml",
			wantTarget: filepath.Join(configDir, "config.yaml"),
			wantOK:     true,
		},
		{
			name:   "unrecognized top-level entry rejected",
			entry:  "etc/passwd",
			wantOK: false,
		},
		{
			name:   "path traversal inside config prefix rejected",
			entry:  "config/../../etc/passwd",
			wantOK: false,
		},
		{
			name:   "path traversal inside data prefix rejected",
			entry:  "data/../../etc/passwd",
			wantOK: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			target, _, ok := resolveImportTarget(c.entry, dataDir, configDir)
			if ok != c.wantOK {
				t.Fatalf("resolveImportTarget(%q) ok = %v, want %v (target=%q)", c.entry, ok, c.wantOK, target)
			}
			if ok && target != c.wantTarget {
				t.Errorf("resolveImportTarget(%q) target = %q, want %q", c.entry, target, c.wantTarget)
			}
		})
	}
}

// writeAndRestore writes content to a config.DataPath()-relative file for the
// duration of the test, restoring whatever (if anything) was there before —
// config.DataDir()'s resolved value is cached process-wide via sync.Once
// (see internal/config/config.go), so a plain t.Setenv("MEMO_DATA_DIR", ...)
// here only works if no earlier test in this same `go test` binary already
// triggered that cache with a different directory. Operating against
// whatever config.DataPath() actually resolves to right now — rather than
// fighting the cache — keeps this test correct regardless of run order,
// without ever touching real data it didn't create itself.
func writeAndRestore(t *testing.T, rel, content string) {
	t.Helper()
	full := config.DataPath(filepath.FromSlash(rel))
	original, readErr := os.ReadFile(full)
	hadOriginal := readErr == nil

	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if hadOriginal {
			_ = os.WriteFile(full, original, 0o600)
		} else {
			_ = os.Remove(full)
		}
	})
}

// TestExportData_IncludesEveryDataStore is a regression test: ExportData used
// to only archive sessions/memory/whatsapp/mood/providers.json/orchestra.json
// — calendar, learned-pattern profiles, routines, task lists, usage stats,
// agent file-edit safety backups, installed skills, agent permissions, and
// (most importantly) machine.key were silently left out of every .memo
// backup. Without machine.key specifically, providers.json's encrypted API
// keys can never be decrypted again after restoring on a different machine
// or a fresh data directory — this asserts every one of those entries now
// makes it into the archive.
func TestExportData_IncludesEveryDataStore(t *testing.T) {
	writeAndRestore(t, "machine.key", "top-secret-machine-key")
	writeAndRestore(t, "permissions.json", `{"policy":"ask"}`)
	writeAndRestore(t, "calendar/events.db", "calendar-bytes")
	writeAndRestore(t, "profile/patterns.json", `{"patterns":[]}`)
	writeAndRestore(t, "routines/routines.json", `{"routines":[]}`)
	writeAndRestore(t, "tasklists/list.json", `{"tasks":[]}`)
	writeAndRestore(t, "stats/usage.db", "stats-bytes")
	writeAndRestore(t, "agent-backups/some-file.bak", "backup-bytes")
	writeAndRestore(t, "skills/my-skill/SKILL.md", "# my skill")

	a := &App{}
	zipData, err := a.ExportData(false)
	if err != nil {
		t.Fatalf("ExportData: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("open produced zip: %v", err)
	}
	contents := map[string]string{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read entry %s: %v", f.Name, err)
		}
		contents[f.Name] = string(data)
	}

	want := map[string]string{
		"data/machine.key":                 "top-secret-machine-key",
		"data/permissions.json":            `{"policy":"ask"}`,
		"data/calendar/events.db":          "calendar-bytes",
		"data/profile/patterns.json":       `{"patterns":[]}`,
		"data/routines/routines.json":      `{"routines":[]}`,
		"data/tasklists/list.json":         `{"tasks":[]}`,
		"data/stats/usage.db":              "stats-bytes",
		"data/agent-backups/some-file.bak": "backup-bytes",
		"data/skills/my-skill/SKILL.md":    "# my skill",
	}
	for name, wantContent := range want {
		got, ok := contents[name]
		if !ok {
			t.Errorf("archive missing entry %q (got entries: %v)", name, contents)
			continue
		}
		if got != wantContent {
			t.Errorf("entry %q content = %q, want %q", name, got, wantContent)
		}
	}
}

// TestExportData_ExcludesNonPortableMachineState asserts sync_token.json and
// tailscale/ are deliberately left out — both are machine/account-specific
// and would be actively wrong to replay onto a different install (see the
// comment in ExportData).
func TestExportData_ExcludesNonPortableMachineState(t *testing.T) {
	writeAndRestore(t, "sync_token.json", `{"token":"abc"}`)
	writeAndRestore(t, "tailscale/state.db", "tsnet-state")
	// install_id identifies THIS install (see install_id.go). Exporting it
	// would make a backup restored onto a fresh machine claim to be the
	// same install, so every client would keep its now-stale saved
	// sign-in instead of resetting it.
	writeAndRestore(t, installIDFile, "deadbeefdeadbeef")

	a := &App{}
	zipData, err := a.ExportData(false)
	if err != nil {
		t.Fatalf("ExportData: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("open produced zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == "data/sync_token.json" || filepath.Dir(f.Name) == "data/tailscale" ||
			f.Name == "data/"+installIDFile {
			t.Errorf("archive unexpectedly contains non-portable entry %q", f.Name)
		}
	}
}

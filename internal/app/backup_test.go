package app

import (
	"path/filepath"
	"testing"
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

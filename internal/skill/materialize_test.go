package skill

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
)

func embeddedFixture() fstest.MapFS {
	return fstest.MapFS{
		"memo-system/SKILL.md": &fstest.MapFile{Data: []byte(`---
name: memo-system
description: "Memo self-management guidance"
danger_level: safe
---

# memo-system

Engine-only guidance.
`)},
		"memo-system/reference/notes.md": &fstest.MapFile{Data: []byte("extra reference\n")},
	}
}

func TestMaterializeEmbedded_CreatesThenDiscovers(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	created, err := MaterializeEmbedded(m, "memo-system", embeddedFixture())
	if err != nil {
		t.Fatalf("MaterializeEmbedded: %v", err)
	}
	if !created {
		t.Fatal("created = false on first run, want true")
	}

	// The whole tree landed on disk.
	if _, err := os.Stat(filepath.Join(m.SkillsDir(), "memo-system", "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md not materialized: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.SkillsDir(), "memo-system", "reference", "notes.md")); err != nil {
		t.Fatalf("nested file not materialized: %v", err)
	}

	if err := m.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	def, ok := m.Get("memo-system")
	if !ok {
		t.Fatal("memo-system not discovered after materialize")
	}
	if def.Instructions == "" {
		t.Fatal("discovered skill has empty instructions")
	}
}

func TestMaterializeEmbedded_NoopWhenPresent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	if _, err := MaterializeEmbedded(m, "memo-system", embeddedFixture()); err != nil {
		t.Fatalf("first MaterializeEmbedded: %v", err)
	}

	// Hand-edit the on-disk copy, then re-run: it must be left untouched.
	skillMd := filepath.Join(m.SkillsDir(), "memo-system", "SKILL.md")
	edited := []byte("user edited this\n")
	if err := os.WriteFile(skillMd, edited, 0644); err != nil {
		t.Fatal(err)
	}

	created, err := MaterializeEmbedded(m, "memo-system", embeddedFixture())
	if err != nil {
		t.Fatalf("second MaterializeEmbedded: %v", err)
	}
	if created {
		t.Fatal("created = true on second run, want false (already present)")
	}
	got, _ := os.ReadFile(skillMd)
	if string(got) != string(edited) {
		t.Fatalf("on-disk SKILL.md was overwritten: %q", string(got))
	}
}

// TestMaterializeEmbedded_RefreshesUntouchedCopyWhenEmbeddedContentChanges
// proves a build that ships a fix to a built-in skill actually reaches an
// install that never touched its materialized copy — the presence-only
// check in the pre-fix code silently blocked this forever.
func TestMaterializeEmbedded_RefreshesUntouchedCopyWhenEmbeddedContentChanges(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	if _, err := MaterializeEmbedded(m, "memo-system", embeddedFixture()); err != nil {
		t.Fatalf("first MaterializeEmbedded: %v", err)
	}

	updated := fstest.MapFS{
		"memo-system/SKILL.md": &fstest.MapFile{Data: []byte(`---
name: memo-system
description: "Memo self-management guidance"
danger_level: safe
---

# memo-system

Updated engine-only guidance.
`)},
		"memo-system/reference/notes.md": &fstest.MapFile{Data: []byte("extra reference\n")},
	}

	created, err := MaterializeEmbedded(m, "memo-system", updated)
	if err != nil {
		t.Fatalf("second MaterializeEmbedded: %v", err)
	}
	if !created {
		t.Fatal("created = false on content change to an untouched copy, want true")
	}

	got, err := os.ReadFile(filepath.Join(m.SkillsDir(), "memo-system", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Updated engine-only guidance.") {
		t.Fatalf("SKILL.md was not refreshed with the new embedded content, got: %q", string(got))
	}
}

// TestMaterializeEmbedded_DoesNotRefreshUserEditedCopy proves a directory
// with no recorded materialization signature (an old install predating this
// check, or a genuinely user-provided skill) is never touched even when the
// embedded content changes — only a copy Memo can prove it wrote and no one
// has modified since is eligible for a refresh.
func TestMaterializeEmbedded_DoesNotRefreshUserEditedCopy(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	skillDir := filepath.Join(m.SkillsDir(), "memo-system")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	userContent := []byte("user's own SKILL.md, never materialized by us\n")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), userContent, 0644); err != nil {
		t.Fatal(err)
	}

	created, err := MaterializeEmbedded(m, "memo-system", embeddedFixture())
	if err != nil {
		t.Fatalf("MaterializeEmbedded: %v", err)
	}
	if created {
		t.Fatal("created = true for a directory with no materialization signature, want false")
	}
	got, _ := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if string(got) != string(userContent) {
		t.Fatalf("user-provided SKILL.md was overwritten: %q", string(got))
	}
}

// TestMaterializeEmbedded_WritesFilesAtomically proves each file lands via
// a temp-file-plus-rename rather than a direct truncating write, so a crash
// mid-materialize can never leave a half-written file that still satisfies
// the "already materialized" presence check on the next start. It observes
// this by racing many concurrent readers against one materialize call and
// asserting every observed read is either fully absent or fully the
// expected content — never a partial write.
func TestMaterializeEmbedded_WritesFilesAtomically(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	big := fstest.MapFS{
		"memo-system/SKILL.md": &fstest.MapFile{Data: []byte(`---
name: memo-system
description: "Memo self-management guidance"
danger_level: safe
---

# memo-system
`)},
		"memo-system/reference/notes.md": &fstest.MapFile{Data: bytes.Repeat([]byte("0123456789abcdef"), 1<<16)}, // 1MiB
	}

	target := filepath.Join(m.SkillsDir(), "memo-system", "reference", "notes.md")
	stop := make(chan struct{})
	var sawPartial atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		want := big["memo-system/reference/notes.md"].Data
		for {
			select {
			case <-stop:
				return
			default:
			}
			data, err := os.ReadFile(target)
			if err == nil && len(data) != 0 && len(data) != len(want) {
				sawPartial.Store(true)
				return
			}
			if err == nil && len(data) == len(want) && string(data) != string(want) {
				sawPartial.Store(true)
				return
			}
		}
	}()

	if _, err := MaterializeEmbedded(m, "memo-system", big); err != nil {
		t.Fatalf("MaterializeEmbedded: %v", err)
	}
	close(stop)
	wg.Wait()

	if sawPartial.Load() {
		t.Fatal("observed a partially-written file mid-materialize — write is not atomic")
	}
}

package taskloop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestParseTaskMd_Basic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Task.md")
	writeFile(t, path, "# bildirim: her-şey\n\n- [ ] ilk görev\n- [x] bitmiş görev\n\n## Grup\n1. [ ] numaralı görev\n")

	parsed, err := ParseTaskMd(path)
	if err != nil {
		t.Fatalf("ParseTaskMd: %v", err)
	}
	if parsed.NotifyLevel != NotifyEverything {
		t.Fatalf("NotifyLevel = %q, want %q", parsed.NotifyLevel, NotifyEverything)
	}
	if len(parsed.Items) != 3 {
		t.Fatalf("Items = %d, want 3", len(parsed.Items))
	}
	if parsed.Items[0].Text != "ilk görev" || parsed.Items[0].Status != "pending" {
		t.Fatalf("Items[0] = %+v", parsed.Items[0])
	}
	if parsed.Items[1].Status != "done" {
		t.Fatalf("Items[1].Status = %q, want done", parsed.Items[1].Status)
	}
	if parsed.Items[2].Text != "numaralı görev" || parsed.Items[2].Status != "pending" {
		t.Fatalf("Items[2] = %+v", parsed.Items[2])
	}
	// Line numbers are 1-based and point at the real source line.
	if parsed.Items[0].Line != 3 {
		t.Fatalf("Items[0].Line = %d, want 3", parsed.Items[0].Line)
	}
	if parsed.Items[2].Line != 7 {
		t.Fatalf("Items[2].Line = %d, want 7", parsed.Items[2].Line)
	}
}

func TestParseTaskMd_NoHeaderDefaultsImportant(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Task.md")
	writeFile(t, path, "- [ ] tek görev\n")

	parsed, err := ParseTaskMd(path)
	if err != nil {
		t.Fatalf("ParseTaskMd: %v", err)
	}
	if parsed.NotifyLevel != NotifyImportant {
		t.Fatalf("NotifyLevel = %q, want %q", parsed.NotifyLevel, NotifyImportant)
	}
	if len(parsed.Items) != 1 {
		t.Fatalf("Items = %d, want 1", len(parsed.Items))
	}
}

func TestParseTaskMd_UnknownHeaderWarnsAndDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Task.md")
	writeFile(t, path, "# bildirim: bazen\n- [ ] görev\n")

	parsed, err := ParseTaskMd(path)
	if err != nil {
		t.Fatalf("ParseTaskMd: %v", err)
	}
	if parsed.NotifyLevel != NotifyImportant {
		t.Fatalf("NotifyLevel = %q, want fallback %q", parsed.NotifyLevel, NotifyImportant)
	}
	if len(parsed.Warnings) == 0 {
		t.Fatal("expected a warning for the unrecognized bildirim level")
	}
}

func TestParseTaskMd_ZeroItems(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Task.md")
	writeFile(t, path, "# Some heading\n\nJust prose, no checkboxes.\n")

	parsed, err := ParseTaskMd(path)
	if err != nil {
		t.Fatalf("ParseTaskMd: %v", err)
	}
	if len(parsed.Items) != 0 {
		t.Fatalf("Items = %d, want 0", len(parsed.Items))
	}
}

func TestParseTaskMd_MissingFile(t *testing.T) {
	_, err := ParseTaskMd(filepath.Join(t.TempDir(), "nope.md"))
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestMarkItemDone_PreservesFormatting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Task.md")
	writeFile(t, path, "- [ ] first\n  - [ ]  indented  # trailing comment\n- [x] already done\n")

	if err := MarkItemDone(path, 2); err != nil {
		t.Fatalf("MarkItemDone: %v", err)
	}
	data, _ := os.ReadFile(path)
	got := string(data)
	want := "- [ ] first\n  - [x]  indented  # trailing comment\n- [x] already done\n"
	if got != want {
		t.Fatalf("MarkItemDone result:\n%q\nwant:\n%q", got, want)
	}
}

func TestMarkItemDone_AlreadyDoneIsNoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Task.md")
	orig := "- [x] first\n- [ ] second\n"
	writeFile(t, path, orig)

	if err := MarkItemDone(path, 1); err != nil {
		t.Fatalf("MarkItemDone: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != orig {
		t.Fatalf("file changed on a no-op: %q", string(data))
	}
}

func TestMarkItemDone_LineOutOfRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Task.md")
	writeFile(t, path, "- [ ] only line\n")

	if err := MarkItemDone(path, 99); err == nil {
		t.Fatal("expected an error for an out-of-range line")
	}
}

// TestMarkItemDone_ConcurrentCallsDoNotLoseUpdates is the regression test
// for the P2 finding that MarkItemDone did an unlocked
// read-full-file -> mutate-one-line -> write-full-file with no
// coordination — two task lists sharing the same TaskMdPath (or two calls
// for the same list racing a retry/resume) could each read a stale copy
// and one write would silently clobber the other's already-applied
// change. Fires many concurrent MarkItemDone calls against different
// lines of the same file and asserts every single one actually stuck —
// a lost update shows up as a line that's still "[ ]" afterward.
func TestMarkItemDone_ConcurrentCallsDoNotLoseUpdates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Task.md")

	const n = 50
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "- [ ] item %d\n", i)
	}
	writeFile(t, path, sb.String())

	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(line int) {
			defer wg.Done()
			if err := MarkItemDone(path, line); err != nil {
				errCh <- err
			}
		}(i + 1) // 1-based lines
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("MarkItemDone: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final Task.md: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != n {
		t.Fatalf("got %d lines, want %d", len(lines), n)
	}
	for i, line := range lines {
		if !strings.Contains(line, "[x]") {
			t.Errorf("line %d (%q) was never marked done — a concurrent write lost this update", i+1, line)
		}
	}
}

func TestReadRules_Priority(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "AGENTS.md"), "agents-rule-content")
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "claude-rule-content")
	writeFile(t, filepath.Join(dir, "memo.md"), "memo-rule-content")

	out, err := ReadRules(dir)
	if err != nil {
		t.Fatalf("ReadRules: %v", err)
	}
	ai := strings.Index(out, "agents-rule-content")
	ci := strings.Index(out, "claude-rule-content")
	mi := strings.Index(out, "memo-rule-content")
	if ai < 0 || ci < 0 || mi < 0 {
		t.Fatalf("ReadRules missing content:\n%s", out)
	}
	if !(ai < ci && ci < mi) {
		t.Fatalf("ReadRules order wrong (want AGENTS < CLAUDE < memo): ai=%d ci=%d mi=%d", ai, ci, mi)
	}
}

func TestReadRules_MissingReturnsEmpty(t *testing.T) {
	out, err := ReadRules(filepath.Join(t.TempDir(), "no-such-dir"))
	if err != nil {
		t.Fatalf("ReadRules: %v", err)
	}
	if out != "" {
		t.Fatalf("ReadRules = %q, want empty", out)
	}
}

func TestReadRules_EmptyRoot(t *testing.T) {
	out, err := ReadRules("")
	if err != nil {
		t.Fatalf("ReadRules(\"\"): %v", err)
	}
	if out != "" {
		t.Fatalf("ReadRules(\"\") = %q, want empty", out)
	}
}

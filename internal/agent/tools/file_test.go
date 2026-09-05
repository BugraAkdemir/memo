package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidatePath_RejectsUnlistedAbsolutePathOutsideBase is the regression
// guard for BUG-H2 as originally reported: it claimed any absolute path
// outside the project directory that isn't in the (short, hardcoded)
// protected-paths list is allowed through. That was true of the separate,
// dead agent.Sandbox.ValidatePath (removed — it had zero callers anywhere
// in the codebase), but this function — the one every file tool actually
// calls — rejects *any* path outside basePath regardless of whether it
// matches the protected list; that list only produces a more specific error
// message for the paths it does recognize.
func TestValidatePath_RejectsUnlistedAbsolutePathOutsideBase(t *testing.T) {
	base := t.TempDir()

	// Not in defaultProtectedPaths() (/etc/, /usr/, /home/, ...) — exactly
	// the class of path BUG-H2 claimed slipped through.
	unlisted := []string{
		"/srv/evil",
		"/data2/evil",
		"/nas/evil",
	}
	for _, p := range unlisted {
		if _, err := validatePath(p, base); err == nil {
			t.Errorf("validatePath(%q, base) = nil error, want rejection (path is outside project directory)", p)
		}
	}
}

func TestValidatePath_RejectsListedProtectedPath(t *testing.T) {
	base := t.TempDir()
	if _, err := validatePath("/etc/passwd", base); err == nil {
		t.Fatal("validatePath(\"/etc/passwd\", base) = nil error, want rejection")
	}
}

func TestValidatePath_AllowsPathWithinBase(t *testing.T) {
	base := t.TempDir()
	got, err := validatePath("notes.txt", base)
	if err != nil {
		t.Fatalf("validatePath() error = %v", err)
	}
	want := filepath.Join(base, "notes.txt")
	if got != want {
		t.Errorf("validatePath() = %q, want %q", got, want)
	}
}

func TestValidatePath_RejectsRelativeTraversal(t *testing.T) {
	base := t.TempDir()
	if _, err := validatePath("../../etc/passwd", base); err == nil {
		t.Fatal("validatePath(\"../../etc/passwd\", base) = nil error, want rejection")
	}
}

// TestValidatePath_ExpandsTildeInsteadOfCreatingALiteralDirectory is a
// regression test for a real bug found live: the model wrote
// "~/Desktop/hello.py" while its sandbox basePath was this very repo, and
// validatePath — unlike resolveChangeDirectoryTarget (changedir.go), which
// already expanded "~" correctly — treated "~" as an ordinary relative-path
// character and joined it straight onto basePath, creating a literal
// directory named "~" inside the repo instead of reaching the real home
// directory. Confirms "~/notes.txt" now resolves to the real home
// directory and (since that's outside this test's tempdir basePath) is
// correctly rejected as outside the project — not silently redirected to a
// wrong location inside it.
func TestValidatePath_ExpandsTildeInsteadOfCreatingALiteralDirectory(t *testing.T) {
	base := t.TempDir()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory available in this environment: %v", err)
	}

	_, err = validatePath("~/notes.txt", base)
	if err == nil {
		t.Fatal("expected rejection: the real home directory is outside this test's basePath")
	}
	if strings.Contains(err.Error(), filepath.Join(base, "~")) {
		t.Errorf("expected the error to reference the real home directory, not a literal \"~\" under basePath — got: %v", err)
	}

	// Sanity-check the expansion target itself, independent of the
	// outside-basePath rejection above: when basePath legitimately *is*
	// (an ancestor of) the home directory, "~" must resolve to the real
	// home directory, not a "~" subdirectory of it.
	got, err := validatePath("~", filepath.Dir(home))
	if err != nil {
		t.Fatalf("validatePath(\"~\", parent-of-home) error = %v", err)
	}
	realHome, _ := filepath.EvalSymlinks(home)
	if got != realHome {
		t.Errorf("validatePath(\"~\", ...) = %q, want the real home directory %q", got, realHome)
	}
}

// TestValidatePath_RejectsSymlinkedAncestorEscapeToNotYetExistingFile is the
// regression test for BUG-C1: a pre-existing symlink inside the project
// pointing outside it (e.g. an npm/yarn/venv symlink, or one left by another
// tool) used to let write_file (and edit_file/insert_line/delete_lines,
// which all route through validatePath) escape the sandbox entirely, as long
// as the specific file being written didn't exist yet. filepath.EvalSymlinks
// fails with os.IsNotExist as soon as the FINAL path component is missing —
// even though the symlinked ancestor directory earlier in the path very much
// exists and resolves — and the pre-fix code's fallback on that error was
// the raw, completely unresolved path, so the "is this inside basePath"
// check below saw only the literal, project-relative-looking string while
// the real write would have followed the symlink straight outside it.
func TestValidatePath_RejectsSymlinkedAncestorEscapeToNotYetExistingFile(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(base, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("os.Symlink() error = %v", err)
	}

	// "newfile.txt" deliberately does not exist yet — the exact write_file
	// scenario the bug required.
	got, err := validatePath("link/newfile.txt", base)
	if err == nil {
		t.Fatalf("validatePath(\"link/newfile.txt\", base) = %q, nil error — want rejection, since this resolves to %q, outside base %q", got, filepath.Join(outside, "newfile.txt"), base)
	}
}

// TestValidatePath_AllowsSymlinkedAncestorToNotYetExistingFileWithinBase is
// the non-adversarial counterpart: a symlinked ancestor directory that
// points to somewhere else *inside* the project must still work normally
// for a not-yet-existing target file — the fix must not turn every
// not-yet-existing path under a symlinked directory into a rejection,
// only ones that actually resolve outside basePath.
func TestValidatePath_AllowsSymlinkedAncestorToNotYetExistingFileWithinBase(t *testing.T) {
	base := t.TempDir()
	realDir := filepath.Join(base, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("os.Mkdir() error = %v", err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatalf("os.Symlink() error = %v", err)
	}

	got, err := validatePath("link/newfile.txt", base)
	if err != nil {
		t.Fatalf("validatePath(\"link/newfile.txt\", base) error = %v, want nil (target resolves inside base)", err)
	}
	want := filepath.Join(realDir, "newfile.txt")
	if got != want {
		t.Errorf("validatePath() = %q, want %q", got, want)
	}
}

func readFile(t *testing.T, base, path string, offset, limit int) string {
	t.Helper()
	args := map[string]any{"path": path}
	if offset > 0 {
		args["offset"] = offset
	}
	if limit > 0 {
		args["limit"] = limit
	}
	raw, _ := json.Marshal(args)
	out, err := ReadFile(context.Background(), raw, base, nil)
	if err != nil {
		t.Fatalf("ReadFile(%q, off=%d lim=%d): %v", path, offset, limit, err)
	}
	return out
}

func TestReadFile_WholeSmallFileUnchanged(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "s.txt"), []byte("a\nb\nc"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, base, "s.txt", 0, 0); got != "a\nb\nc" {
		t.Errorf("small file read = %q, want verbatim", got)
	}
}

func TestReadFile_OffsetLimitWindow(t *testing.T) {
	base := t.TempDir()
	var sb strings.Builder
	for i := 1; i <= 50; i++ {
		sb.WriteString("line")
		sb.WriteByte(byte('0' + i%10))
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(base, "f.txt"), []byte(sb.String()), 0644); err != nil {
		t.Fatal(err)
	}
	out := readFile(t, base, "f.txt", 10, 5)
	if !strings.HasPrefix(out, "[read_file f.txt: lines 10-14 of 51") {
		t.Errorf("missing/incorrect window header: %q", firstLine(out))
	}
	if strings.Contains(out, "pass offset/limit to read more") == false {
		t.Errorf("expected a 'read more' hint when the window ends before EOF: %q", firstLine(out))
	}
}

func TestReadFile_BigFileAutoCapped(t *testing.T) {
	base := t.TempDir()
	var sb strings.Builder
	for i := 0; i < 5000; i++ {
		sb.WriteString("some line of content here to add bytes ")
		sb.WriteString(strings.Repeat("x", 40))
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(base, "big.txt"), []byte(sb.String()), 0644); err != nil {
		t.Fatal(err)
	}
	out := readFile(t, base, "big.txt", 0, 0) // no window given
	if !strings.HasPrefix(out, "[read_file big.txt: lines 1-2000 of 5001") {
		t.Errorf("big unwindowed read should auto-cap to 2000 lines: %q", firstLine(out))
	}
}

func TestReadFile_OffsetPastEnd(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "s.txt"), []byte("a\nb\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out := readFile(t, base, "s.txt", 999, 0)
	if !strings.Contains(out, "past end of file") {
		t.Errorf("out-of-range offset should say so, got %q", out)
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

package tools

import (
	"path/filepath"
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

package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIsBlacklisted_RmRfRoot guards against the BUG-C2 bypass: the original
// \b-terminated patterns never matched "rm -rf /" (or its ~ and . variants)
// because "/", "~", and "." are non-word characters, so \b never found a
// boundary at end-of-string or before another shell operator.
func TestIsBlacklisted_RmRfRoot(t *testing.T) {
	blocked := []string{
		"rm -rf /",
		"rm -rf /*",
		"sudo rm -rf /",
		"rm -rf /; echo done",
		"rm -rf / && echo done",
		"rm -rf /|cat",
		"rm -rf ~",
		"rm -rf ~ ",
		"rm -rf ~;",
		"rm -rf .",
		"rm -rf . ",
		"rm -rf .;",
	}
	for _, cmd := range blocked {
		if _, ok := isBlacklisted(cmd); !ok {
			t.Errorf("isBlacklisted(%q) = false, want true (destructive root/home/cwd wipe)", cmd)
		}
	}
}

// TestIsBlacklisted_AllowsScopedRm confirms the fix didn't overreach into
// blocking legitimate, scoped deletions the blacklist was never meant to
// catch (e.g. removing a subdirectory of the project, or a dotfile).
func TestIsBlacklisted_AllowsScopedRm(t *testing.T) {
	allowed := []string{
		"rm -rf /home/user/foo",
		"rm -rf ./build",
		"rm -rf .git",
		"rm -rf .cache",
	}
	for _, cmd := range allowed {
		if pattern, ok := isBlacklisted(cmd); ok {
			t.Errorf("isBlacklisted(%q) = true (matched %q), want false (scoped, non-destructive path)", cmd, pattern)
		}
	}
}

// TestRunCommand_BlocksProtectedPathBypass is the regression test for
// BUG-M7: read_file correctly refused "../../../../etc/passwd" ("access
// denied: path is within protected directory"), but the exact same target
// was fully readable through run_command ("cat /etc/hostname && whoami &&
// printenv HOME") since RunCommand only validated the cwd argument, never
// paths referenced inside the command string itself.
func TestRunCommand_BlocksProtectedPathBypass(t *testing.T) {
	base := t.TempDir()
	blocked := []string{
		"cat /etc/hostname && whoami && printenv HOME",
		"cat /etc/passwd",
		"cat ~/.ssh/id_rsa",
	}
	for _, cmd := range blocked {
		args, _ := json.Marshal(RunCommandArgs{Command: cmd})
		out, err := RunCommand(context.Background(), args, base, nil)
		if err == nil {
			t.Errorf("RunCommand(%q) = %q, <nil> error — want rejection (protected path bypass)", cmd, out)
		}
	}
}

// TestRunCommand_AllowsOrdinaryProjectCommands guards against the fix above
// overreaching: the project directory itself commonly lives somewhere under
// a "protected" prefix like /home/ or /tmp/ (t.TempDir() does on Linux), so
// an ordinary relative-path command referencing files *inside* the project
// must not be rejected just because the project's own absolute path happens
// to start with a protected prefix.
// TestRunCommand_RejectsSymlinkedAncestorCWDEscape is RunCommand's
// counterpart to file_test.go's
// TestValidatePath_RejectsSymlinkedAncestorEscapeToNotYetExistingFile — the
// exact same BUG-C1 gap existed in the CWD resolution here: a pre-existing
// symlink inside the project pointing outside it let `cwd` resolve outside
// basePath as long as the specific CWD subdirectory named didn't exist yet
// (filepath.EvalSymlinks fails with os.IsNotExist on the missing final
// component, discarding resolution of the symlinked ancestor entirely).
func TestRunCommand_RejectsSymlinkedAncestorCWDEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(base, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("os.Symlink() error = %v", err)
	}

	// "notyet" deliberately does not exist under either link or outside.
	args, _ := json.Marshal(RunCommandArgs{Command: "pwd", CWD: "link/notyet"})
	out, err := RunCommand(context.Background(), args, base, nil)
	if err == nil {
		t.Errorf("RunCommand(cwd=%q) = %q, <nil> error — want rejection (cwd resolves outside project directory)", "link/notyet", out)
	}
}

func TestRunCommand_AllowsOrdinaryProjectCommands(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "notes.txt"), []byte("hi"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	allowed := []string{
		"go build ./...",
		"cat notes.txt",
		"ls ./sub/dir",
	}
	for _, cmd := range allowed {
		args, _ := json.Marshal(RunCommandArgs{Command: cmd})
		_, err := RunCommand(context.Background(), args, base, nil)
		if err != nil && strings.Contains(err.Error(), "protected directory") {
			t.Errorf("RunCommand(%q) error = %v, want no protected-directory rejection for an in-project path", cmd, err)
		}
	}
}

func TestCommandTargetsProtectedPath(t *testing.T) {
	base := t.TempDir()
	tests := []struct {
		command string
		want    bool
	}{
		{"cat /etc/passwd", true},
		{"cat /etc/hostname && whoami", true},
		{"cat ~/.ssh/id_rsa", true},
		{"cat ../../../../etc/passwd", true},
		{"go build ./...", false},
		{"ls -la", false},
		{"echo hello", false},
	}
	for _, tc := range tests {
		_, got := commandTargetsProtectedPath(tc.command, base, base)
		if got != tc.want {
			t.Errorf("commandTargetsProtectedPath(%q) blocked = %v, want %v", tc.command, got, tc.want)
		}
	}
}

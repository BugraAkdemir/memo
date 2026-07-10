package tools

import "testing"

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

package replcli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain points config.DataDir() at a throwaway temp directory before any
// test in this package runs. config.DataDir() caches its resolved value in
// a package-level sync.Once for the whole test binary's lifetime — without
// this, whichever test happens to touch it first (theme persistence here,
// but potentially others later) would permanently decide the directory for
// every other test, and on Linux the unset-env fallback is a
// process-relative "data" folder, which would mean writing a stray
// cli_theme file into the actual source tree under `go test`. Setting the
// env var here, before m.Run(), guarantees the very first resolution is
// always this safe temp directory regardless of test/file ordering.
func TestMain(m *testing.M) {
	os.Setenv("MEMO_DATA_DIR", filepath.Join(os.TempDir(), "memo-replcli-test-data"))
	os.Exit(m.Run())
}

func TestParseTheme(t *testing.T) {
	tests := []struct {
		in     string
		want   replTheme
		wantOK bool
	}{
		{"g", themeDefault, true},
		{"G", themeDefault, true},
		{"classic", themeClaudeCode, true},
		{" classic ", themeClaudeCode, true},
		{"", "", false},
		{"dark", "", false},
	}
	for _, tt := range tests {
		got, ok := parseTheme(tt.in)
		if ok != tt.wantOK || (ok && got != tt.want) {
			t.Errorf("parseTheme(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

// TestThemePersistence is the only test in this package that touches the
// real (TestMain-redirected) theme file — see TestMain's doc comment for
// why every other test that exercises saveTheme (cmdTheme's own tests)
// deliberately avoids asserting on the file itself, only on in-memory
// state, to avoid two tests racing over the same shared file.
func TestThemePersistence(t *testing.T) {
	// Other tests in this binary (cmdTheme's) also call saveTheme against
	// this same TestMain-provided directory as a side effect — clear any
	// leftover file first so this test's "no file yet" assertion below
	// doesn't depend on running before those.
	themeFile := filepath.Join(os.Getenv("MEMO_DATA_DIR"), "cli_theme")
	_ = os.Remove(themeFile)

	if got := loadSavedTheme(); got != themeDefault {
		t.Errorf("loadSavedTheme() with no saved file = %q, want default %q", got, themeDefault)
	}

	if err := saveTheme(themeClaudeCode); err != nil {
		t.Fatalf("saveTheme() error = %v", err)
	}
	if got := loadSavedTheme(); got != themeClaudeCode {
		t.Errorf("loadSavedTheme() after saveTheme(classic) = %q, want classic", got)
	}

	if err := os.WriteFile(themeFile, []byte("garbage\x00"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := loadSavedTheme(); got != themeDefault {
		t.Errorf("loadSavedTheme() with garbage contents = %q, want fallback %q", got, themeDefault)
	}

	// Leave the file back in a known-good state for any other test in this
	// binary that might (directly or via cmdTheme) call loadSavedTheme.
	_ = saveTheme(themeDefault)
}

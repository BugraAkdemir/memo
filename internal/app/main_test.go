package app

import (
	"os"
	"testing"

	"memo/internal/config"
)

// TestMain gives the whole package a throwaway data directory. Many tests
// here construct an App or an agent.Executor without choosing a data dir of
// their own, and config.DataDir() then resolved to the process-relative
// "data" — i.e. internal/app/data inside the source tree, which collected a
// machine.key, permission files and agent backups (and, before
// TestAgentWrappers_WithExecutor reset its own override, those tests
// silently shared whatever temp dir an earlier test had set instead). A test
// that sets MEMO_DATA_DIR itself and calls config.ResetForTests still gets
// its own directory; on cleanup it falls back to this one, not the tree.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "memo-app-test-data-")
	if err != nil {
		panic(err)
	}
	os.Setenv("MEMO_DATA_DIR", dir)
	// The claude-sub capability probe fires in the BACKGROUND whenever a test
	// exercises the connect surface, and its default target is the real
	// api.anthropic.com. Left alone that is real network I/O from a unit test,
	// made with whatever token is in the throwaway data dir — and the goroutine
	// outlives the test that started it, which is exactly the shape the race
	// detector complains about. An unreachable port fails instantly instead.
	os.Setenv("MEMO_CLAUDE_API_URL", "http://127.0.0.1:1")
	config.ResetForTests()
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

package agent

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"memo/internal/config"
)

// TestExecutor_PermissionFlagsConcurrentAccess guards against the bug where
// RunStream read e.bypassPermissions/e.autoPermission directly instead of
// through GetBypassPermissions/GetAutoPermission — SetBypassPermissions/
// SetAutoPermission write under e.mu (an HTTP handler goroutine), so an
// unsynchronized read elsewhere has no happens-before guarantee and can
// observe a stale value. Run with -race to catch a regression.
func TestExecutor_PermissionFlagsConcurrentAccess(t *testing.T) {
	e := NewExecutor(t.TempDir(), nil, nil, nil)

	var wg sync.WaitGroup
	for i := range 100 {
		v := i%2 == 0
		wg.Add(4)
		go func() {
			defer wg.Done()
			e.SetAutoPermission(v)
		}()
		go func() {
			defer wg.Done()
			e.SetBypassPermissions(v)
		}()
		go func() {
			defer wg.Done()
			_ = e.GetAutoPermission()
		}()
		go func() {
			defer wg.Done()
			_ = e.GetBypassPermissions()
		}()
	}
	wg.Wait()
}

// TestExecutor_LogEventPersistsToAuditFile is the H10 regression: before
// this fix, e.logs was the only copy of a tool call's audit trail — capped
// at 1000 in-memory entries, entirely gone on restart. logEvent must now
// also durably append each entry to disk.
func TestExecutor_LogEventPersistsToAuditFile(t *testing.T) {
	path := config.DataPath("agent-audit.jsonl")
	// Other tests in this package share the same process-memoized
	// config.DataDir(), so this file is a single shared log across the
	// whole test binary run — record the size before, and only inspect
	// the bytes appended after, instead of assuming the file starts empty.
	var startSize int64
	if fi, err := os.Stat(path); err == nil {
		startSize = fi.Size()
	}

	e := NewExecutor(t.TempDir(), nil, nil, nil)
	if e.auditLogFile == nil {
		t.Fatal("auditLogFile is nil — could not open the audit log")
	}
	t.Cleanup(func() { e.auditLogFile.Close() })

	e.logEvent("session-h10", AgentEvent{
		Type:       EventToolResult,
		ToolName:   "write_file",
		DurationMs: 42,
	})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	appended := string(data[startSize:])
	lines := strings.Split(strings.TrimSpace(appended), "\n")
	if len(lines) == 0 || lines[len(lines)-1] == "" {
		t.Fatalf("expected at least one new line appended to %s, got %q", path, appended)
	}

	var entry AgentLogEntry
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("last appended line is not valid JSON: %v (line: %q)", err, lines[len(lines)-1])
	}
	if entry.SessionID != "session-h10" || entry.ToolName != "write_file" || entry.DurationMs != 42 {
		t.Errorf("got entry %+v, want SessionID=session-h10 ToolName=write_file DurationMs=42", entry)
	}
}

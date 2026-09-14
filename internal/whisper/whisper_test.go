package whisper

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	s := NewServer(9877)
	if s.port != 9877 {
		t.Errorf("port = %d, want 9877", s.port)
	}
}

func TestNewServerDefaultPort(t *testing.T) {
	s := NewServer(0)
	if s.port != 9877 {
		t.Errorf("default port = %d, want 9877", s.port)
	}
}

func TestIsRunning_NewServer(t *testing.T) {
	s := NewServer(9877)
	if s.IsRunning() {
		t.Error("new server should not be running")
	}
}

func TestGetStatus_NewServer(t *testing.T) {
	s := NewServer(9877)
	status := s.GetStatus()
	if status.Running {
		t.Error("new server should not be running")
	}
	if status.Port != 9877 {
		t.Errorf("port = %d, want 9877", status.Port)
	}
}

func TestWhisperServerBinary(t *testing.T) {
	got := whisperServerBinary()
	if runtime.GOOS == "windows" {
		if got != "whisper-server.exe" {
			t.Errorf("binary = %q, want %q", got, "whisper-server.exe")
		}
	} else {
		if got != "whisper-server" {
			t.Errorf("binary = %q, want %q", got, "whisper-server")
		}
	}
}

func TestBinarySearchBases(t *testing.T) {
	bases := binarySearchBases()
	if len(bases) < 1 {
		t.Error("expected at least one base")
	}
	if bases[0] != "." {
		t.Errorf("first base = %q, want %q", bases[0], ".")
	}
}

func TestWithPrependedEnvPath(t *testing.T) {
	tests := []struct {
		name            string
		env             []string
		key             string
		dir             string
		caseInsensitive bool
		wantContains    string
	}{
		{
			name:            "prepend to existing PATH",
			env:             []string{"PATH=/usr/bin", "HOME=/home/user"},
			key:             "PATH",
			dir:             "/opt/bin",
			caseInsensitive: false,
			wantContains:    "/opt/bin",
		},
		{
			name:            "add new PATH entry",
			env:             []string{"HOME=/home/user"},
			key:             "PATH",
			dir:             "/opt/bin",
			caseInsensitive: false,
			wantContains:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := withPrependedEnvPath(tt.env, tt.key, tt.dir, tt.caseInsensitive)

			if tt.wantContains != "" {
				found := false
				for _, entry := range result {
					if strings.HasPrefix(entry, tt.key+"=") && strings.Contains(entry, tt.wantContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected entry with %s containing %s in: %v", tt.key, tt.wantContains, result)
				}
			}
		})
	}
}

func TestWithPrependedEnvPath_CaseInsensitive(t *testing.T) {
	env := []string{"Path=C:\\Windows", "HOME=C:\\Users"}
	result := withPrependedEnvPath(env, "PATH", "C:\\tools", true)

	found := false
	for _, entry := range result {
		if strings.HasPrefix(entry, "Path=") && strings.Contains(entry, "C:\\tools") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected case-insensitive match, got: %v", result)
	}
}

func TestPrependPathValue(t *testing.T) {
	tests := []struct {
		name    string
		current string
		dir     string
		want    string
	}{
		{"empty current", "", "/opt/bin", "/opt/bin"},
		{"prepend", "/usr/bin", "/opt/bin", "/opt/bin:/usr/bin"},
		{"deduplicate", "/opt/bin:/usr/bin", "/opt/bin", "/opt/bin:/usr/bin"},
		{"trailing separator", "/usr/bin/", "/opt/bin", "/opt/bin:/usr/bin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prependPathValue(tt.current, tt.dir)
			sep := string(os.PathListSeparator)
			parts := strings.Split(got, sep)

			if parts[0] != tt.dir {
				t.Errorf("first element = %q, want %q", parts[0], tt.dir)
			}
		})
	}
}

func TestSamePathEntry(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want bool
	}{
		{"/usr/bin", "/usr/bin", true},
		{"/usr/bin", "/usr/local/bin", false},
	}
	for _, tt := range tests {
		got := samePathEntry(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("samePathEntry(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestWhisperThreads_MatchesNumCPUWithinCap(t *testing.T) {
	got := whisperThreads()
	if got < 1 {
		t.Fatalf("whisperThreads() = %d, want >= 1", got)
	}
	if got > 8 {
		t.Fatalf("whisperThreads() = %d, want <= 8 cap", got)
	}
	want := runtime.NumCPU()
	if want > 8 {
		want = 8
	}
	if got != want {
		t.Errorf("whisperThreads() = %d, want %d", got, want)
	}
}

func TestPingPort_NotRunning(t *testing.T) {
	s := NewServer(19999) // unlikely to be in use
	if s.pingPort() {
		t.Error("pingPort on unused port should return false")
	}
}

func TestResolveBinary_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "whisper-server")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	got, err := resolveBinary(binPath)
	if err != nil {
		t.Fatalf("resolveBinary failed: %v", err)
	}
	if got != binPath {
		t.Errorf("got %q, want %q", got, binPath)
	}
}

func TestResolveBinary_NotFound(t *testing.T) {
	_, err := resolveBinary("/nonexistent/path/whisper-server")
	if err == nil {
		t.Error("expected error for nonexistent binary")
	}
}

func TestResolveModel_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "ggml-small.bin")
	if err := os.WriteFile(modelPath, []byte("model data"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	got := resolveModel(modelPath)
	if got != modelPath {
		t.Errorf("got %q, want %q", got, modelPath)
	}
}

func TestResolveModel_NotFound(t *testing.T) {
	got := resolveModel("/nonexistent/model.bin")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestResolveModel_EmptyConfig(t *testing.T) {
	got := resolveModel("")
	// Should not crash — returns empty when nothing found
	_ = got
}

// TestBinarySearchBasesFrom_IncludesParentOfExeDir is a regression test,
// adapted from internal/llama's identical test: the installed CLI binary
// lives at ~/.memo/bin/memo, one level deeper than the bundled binaries/
// tree it ships next to (~/.memo/binaries/...). Before this fix, only "."
// and the exe's own directory were searched, so resolveBinary/resolveModel
// never found whisper-server or its model when running as the CLI.
func TestBinarySearchBasesFrom_IncludesParentOfExeDir(t *testing.T) {
	exePath := filepath.Join("/home/user/.memo/bin", "memo")

	bases := binarySearchBasesFrom(exePath)

	wantExeDir := filepath.Join("/home/user/.memo/bin")
	wantParent := filepath.Join("/home/user/.memo")
	if !slices.Contains(bases, wantExeDir) {
		t.Errorf("bases = %v, want to contain exe dir %q", bases, wantExeDir)
	}
	if !slices.Contains(bases, wantParent) {
		t.Errorf("bases = %v, want to contain parent dir %q", bases, wantParent)
	}
}

// TestServer_MonitorRestartsAfterUnexpectedCrash is the regression test for
// the P1 finding that a crashed whisper-server was never restarted: before
// this fix, monitor() only logged the exit and cleared s.cmd, leaving
// every future Transcribe() call failing with a connection error
// indefinitely (a.whisperServer at the internal/app layer still pointed at
// this same, now-permanently-dead Server). A tiny shell script stands in
// for whisper-server: it exits immediately (simulating a crash) on its
// first launch, and stays "running" on every later launch — proving
// monitor() actually relaunched it, not just detected the exit.
func TestServer_MonitorRestartsAfterUnexpectedCrash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script stand-in for whisper-server")
	}

	dir := t.TempDir()
	marker := filepath.Join(dir, "runs")
	script := filepath.Join(dir, "fake-whisper-server")
	scriptBody := "#!/bin/sh\n" +
		"echo run >> \"" + marker + "\"\n" +
		"n=$(wc -l < \"" + marker + "\")\n" +
		"if [ \"$n\" -eq 1 ]; then\n" +
		"  exit 1\n" +
		"fi\n" +
		"exec sleep 30\n" // exec, not a plain command: replaces the shell's process image so Stop()'s SIGTERM (sent to this PID) actually reaches the sleep, instead of orphaning it as an untracked grandchild
	if err := os.WriteFile(script, []byte(scriptBody), 0755); err != nil {
		t.Fatalf("write stub script: %v", err)
	}
	model := filepath.Join(dir, "fake-model.bin")
	if err := os.WriteFile(model, []byte("not a real model, just needs to exist"), 0644); err != nil {
		t.Fatalf("write stub model: %v", err)
	}

	origDelay := whisperRestartDelay
	whisperRestartDelay = 20 * time.Millisecond
	t.Cleanup(func() { whisperRestartDelay = origDelay })

	s := NewServer(0)
	if err := s.Start(script, model, "auto", 19998); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { s.Stop() })

	deadline := time.Now().Add(3 * time.Second)
	var runs int
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(marker)
		runs = strings.Count(string(data), "\n")
		if runs >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if runs < 2 {
		t.Fatalf("stub server was launched %d time(s) within the deadline, want >= 2 (monitor() never restarted it after the crash)", runs)
	}

	// Give startLocked's own bookkeeping a moment to land, then confirm
	// the Server object reports itself running again — the whole point:
	// a.whisperServer (the same pointer) self-heals in place, no caller
	// action needed.
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.IsRunning() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("server did not report itself running again after the auto-restart")
}

// TestServer_StopDuringRestartWindowPreventsRevival is the regression test
// for the race the restart fix could otherwise introduce: Stop() must set
// s.stopping even when s.cmd is already nil (the crash cleared it, but the
// backoff-delayed restart hasn't run yet) — otherwise a Stop() landing in
// exactly that window wouldn't be seen by attemptRestart, and the server
// would come back up right after the user asked to stop it.
func TestServer_StopDuringRestartWindowPreventsRevival(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script stand-in for whisper-server")
	}

	dir := t.TempDir()
	marker := filepath.Join(dir, "runs")
	script := filepath.Join(dir, "fake-whisper-server")
	// Always exits immediately — every launch is a "crash".
	scriptBody := "#!/bin/sh\necho run >> \"" + marker + "\"\nexit 1\n"
	if err := os.WriteFile(script, []byte(scriptBody), 0755); err != nil {
		t.Fatalf("write stub script: %v", err)
	}
	model := filepath.Join(dir, "fake-model.bin")
	if err := os.WriteFile(model, []byte("stub"), 0644); err != nil {
		t.Fatalf("write stub model: %v", err)
	}

	origDelay := whisperRestartDelay
	whisperRestartDelay = 100 * time.Millisecond
	t.Cleanup(func() { whisperRestartDelay = origDelay })

	s := NewServer(0)
	if err := s.Start(script, model, "auto", 19997); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for the first (crashing) launch to be observed by monitor(),
	// then call Stop() while attemptRestart is still in its backoff sleep
	// — before it has re-acquired s.mu to check s.stopping.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(marker)
		if strings.Count(string(data), "\n") >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // land inside the 100ms backoff window
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Give attemptRestart's full backoff+retry budget time to elapse, then
	// confirm it did not bring the server back up.
	time.Sleep(whisperRestartMaxAttempts*whisperRestartDelay + 200*time.Millisecond)
	if s.IsRunning() {
		t.Fatal("server is running after Stop() — the pending auto-restart revived it")
	}
}

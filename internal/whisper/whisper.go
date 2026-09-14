package whisper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/logx"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type ServerStatus struct {
	Running   bool   `json:"running"`
	ModelPath string `json:"model_path"`
	Port      int    `json:"port"`
	PID       int    `json:"pid"`
	Language  string `json:"language"`
}

type Server struct {
	mu        sync.RWMutex
	cmd       *exec.Cmd
	port      int
	modelPath string
	language  string
	stopping  bool
	waitDone  chan struct{}
	portPid   int

	// lastBinaryPath/lastModelPath/lastLanguage/lastPort are the exact
	// arguments the most recent successful Start() call was given —
	// separate from modelPath/language/port above (which Start resolves
	// to their effective values, and monitor() clears on an unexpected
	// exit). Kept so monitor() can restart the server with the original
	// request after a crash without the caller having to remember/resupply
	// them.
	lastBinaryPath string
	lastModelPath  string
	lastLanguage   string
	lastPort       int
}

func NewServer(port int) *Server {
	if port <= 0 {
		port = 9877
	}
	return &Server{port: port}
}

func (s *Server) Start(binaryPath, modelPath, language string, port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startLocked(binaryPath, modelPath, language, port)
}

// startLocked is Start's actual implementation, callable with s.mu already
// held — used both by the public Start() and by monitor()'s crash-restart
// (attemptRestart), which needs to hold the lock across its own
// already-running/stopping check and the spawn itself.
func (s *Server) startLocked(binaryPath, modelPath, language string, port int) error {
	if s.cmd != nil && s.cmd.Process != nil {
		return fmt.Errorf("whisper: server already running (PID %d)", s.cmd.Process.Pid)
	}

	bin, err := resolveBinary(binaryPath)
	if err != nil {
		return fmt.Errorf("whisper: %w", err)
	}

	model := modelPath
	if model != "" {
		if _, err := os.Stat(model); err != nil {
			model = resolveModel(modelPath)
		}
	}
	if model == "" {
		model = resolveModel("")
	}
	if model == "" {
		return fmt.Errorf("whisper: ggml model file not found — place ggml-small.bin in binaries/{os}/cpu/")
	}

	lang := language
	if lang == "" {
		lang = "auto"
	}

	actualPort := port
	if actualPort <= 0 {
		actualPort = s.port
	}

	// Clear the port before spawning — the same pre-flight llama.Server.Start
	// does, for the same reason, which whisper never got. newSysProcAttr sets
	// Setpgid but deliberately not Pdeathsig (incompatible in Go: runtime
	// thread reuse triggers premature child death), so a whisper-server whose
	// parent died abnormally — a crash, kill -9, the SIGHUP of a closed
	// terminal, `go run` tearing down — is NOT reaped by the OS. It is
	// orphaned, re-parented to init, and keeps holding this port forever.
	//
	// Without this, the next Start() spawns a whisper-server that cannot bind
	// and exits within a second, while cmd.Start() itself succeeded — so
	// nothing below reports an error and voice input is silently dead until
	// the machine reboots. Confirmed live on a user's machine: an orphaned
	// whisper-server with PPID 1 holding :9877 with no Memo backend running
	// at all. Safe to call unconditionally — killByPort no-ops on a free port.
	if err := s.killByPort(actualPort); err != nil {
		logx.Printf("whisper: could not clear port %d before starting: %v", actualPort, err)
	}

	s.port = actualPort
	s.language = lang

	args := []string{
		"--model", model,
		"--port", fmt.Sprintf("%d", actualPort),
		"--host", "127.0.0.1",
		"--language", lang,
		"--threads", fmt.Sprintf("%d", whisperThreads()),
	}

	logx.Printf("whisper: launching %s %s", bin, strings.Join(args, " "))

	s.cmd = exec.Command(bin, args...)
	s.cmd.Stdout = os.Stdout
	s.cmd.Stderr = os.Stderr
	s.modelPath = model
	s.stopping = false
	s.waitDone = make(chan struct{})

	binDir := filepath.Dir(bin)
	absBinDir, err := filepath.Abs(binDir)
	if err == nil {
		binDir = absBinDir
	}

	env := os.Environ()
	if runtime.GOOS == "windows" {
		env = withPrependedEnvPath(env, "PATH", binDir, true)
	} else if runtime.GOOS == "darwin" {
		env = withPrependedEnvPath(env, "DYLD_LIBRARY_PATH", binDir, false)
		env = withPrependedEnvPath(env, "DYLD_FALLBACK_LIBRARY_PATH", binDir, false)
	} else {
		env = withPrependedEnvPath(env, "LD_LIBRARY_PATH", binDir, false)
	}
	s.cmd.Env = env

	s.cmd.SysProcAttr = newSysProcAttr()

	if err := s.cmd.Start(); err != nil {
		s.cmd = nil
		return fmt.Errorf("whisper: start failed: %w", err)
	}

	s.portPid = s.cmd.Process.Pid
	s.lastBinaryPath, s.lastModelPath, s.lastLanguage, s.lastPort = binaryPath, modelPath, language, port

	logx.Printf("whisper: server started (PID %d, port %d)", s.cmd.Process.Pid, s.port)

	logx.GoRecover("whisper.Server.monitor", s.monitor)

	return nil
}

func (s *Server) WaitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/", s.port)

	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			logx.Printf("whisper: server ready on port %d", s.port)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("whisper: server failed to become ready within %v", timeout)
}

func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Set unconditionally, before the branch below — attemptRestart's
	// crash-restart (monitor.go) checks this flag between backoff
	// attempts, and only takes s.mu itself right before checking it. If
	// Stop() is called while s.cmd is already nil (a crash left it that
	// way, restart hasn't happened yet) and only set stopping inside the
	// "still running" branch below, a Stop() landing in exactly that
	// window would silently fail to prevent the pending auto-restart from
	// bringing the server back up right after the user asked to stop it.
	s.stopping = true

	if s.cmd == nil || s.cmd.Process == nil {
		if s.port > 0 {
			if s.portPid > 0 {
				if err := killPID(s.portPid); err != nil {
					logx.Printf("whisper: kill stored PID %d: %v, trying port discovery", s.portPid, err)
				} else {
					s.portPid = 0
					return nil
				}
			}
			if err := s.killByPort(s.port); err != nil {
				return fmt.Errorf("whisper: stop by port: %w", err)
			}
		}
		return nil
	}

	logx.Printf("whisper: stopping server (PID %d)", s.cmd.Process.Pid)

	processSignalTerm(s.cmd.Process)

	select {
	case <-s.waitDone:
		logx.Printf("whisper: server stopped gracefully")
	case <-time.After(5 * time.Second):
		logx.Printf("whisper: graceful shutdown timed out, force killing")
		s.forceKill()
	}

	s.cmd = nil
	s.modelPath = ""
	s.portPid = 0
	return nil
}

func (s *Server) killByPort(port int) error {
	pid := s.pidOnPort(port)
	if pid <= 0 {
		logx.Printf("whisper: nothing found on port %d", port)
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	logx.Printf("whisper: killing external PID %d on port %d", pid, port)
	processSignalTerm(proc)

	for i := 0; i < 6; i++ {
		time.Sleep(500 * time.Millisecond)
		if !processIsAlive(proc) {
			logx.Printf("whisper: process %d exited cleanly", pid)
			return nil
		}
	}
	logx.Printf("whisper: force-killing PID %d", pid)
	proc.Kill()
	return nil
}

func (s *Server) pidOnPort(port int) int {
	return pidListeningOnPort(port)
}

func (s *Server) forceKill() {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	forceKillCmd(s.cmd, s.waitDone)
}

// whisperRestartMaxAttempts/-Delay bound monitor()'s crash-restart below.
// Before this existed, an unexpected exit (OOM, segfault, killed by the
// OS) was only ever logged: s.cmd/s.modelPath were cleared but a.whisperServer
// at the internal/app layer kept pointing at this same, now-dead Server,
// so every subsequent transcription request failed with a connection
// error indefinitely — the only fix was manually toggling Whisper off/on
// in Settings or restarting the whole app. A short, bounded retry (not
// unbounded like internal/telegram's network reconnect — a local
// subprocess that keeps crashing immediately, e.g. a missing shared
// library, won't fix itself no matter how many times it's retried) covers
// the common transient case without looping forever on a broken install.
const whisperRestartMaxAttempts = 3

// whisperRestartDelay is a var (not const), the same trick used elsewhere
// in this codebase (e.g. google.Client's SessionBaseURL) so tests can
// shrink it instead of taking real wall-clock seconds per retry.
var whisperRestartDelay = 2 * time.Second

func (s *Server) monitor() {
	s.mu.Lock()
	cmd := s.cmd
	waitDone := s.waitDone
	s.mu.Unlock()

	if cmd == nil {
		return
	}

	err := cmd.Wait()

	// Signal exit BEFORE acquiring s.mu: Stop() may hold the lock while waiting
	// on waitDone, so closing it only after taking the lock would deadlock and
	// force every Stop()/shutdown to burn the full graceful + force-kill timeout.
	if waitDone != nil {
		select {
		case <-waitDone:
		default:
			close(waitDone)
		}
	}

	s.mu.Lock()
	stopping := s.stopping
	binaryPath, modelPath, language, port := s.lastBinaryPath, s.lastModelPath, s.lastLanguage, s.lastPort
	if !stopping {
		s.cmd = nil
		s.modelPath = ""
	}
	s.mu.Unlock()

	if stopping {
		return
	}

	if err != nil {
		logx.Printf("whisper: server exited unexpectedly: %v", err)
	} else {
		logx.Printf("whisper: server exited unexpectedly (exit 0)")
	}

	logx.GoRecover("whisper.Server.restart", func() {
		s.attemptRestart(binaryPath, modelPath, language, port)
	})
}

// attemptRestart tries to bring the whisper-server back up after monitor()
// observed an unexpected exit — see whisperRestartMaxAttempts/-Delay.
// Deliberately runs outside s.mu between attempts (startLocked/Stop each
// take it themselves for their own critical section) so IsRunning()/Stop()
// aren't blocked for the whole retry sequence, only for each individual
// spawn attempt.
func (s *Server) attemptRestart(binaryPath, modelPath, language string, port int) {
	for attempt := 1; attempt <= whisperRestartMaxAttempts; attempt++ {
		time.Sleep(whisperRestartDelay)

		s.mu.Lock()
		if s.stopping || (s.cmd != nil && s.cmd.Process != nil) {
			// Stop() ran, or something else (a user toggling Whisper
			// off/on in Settings) already got a server running again —
			// don't fight it.
			s.mu.Unlock()
			return
		}
		startErr := s.startLocked(binaryPath, modelPath, language, port)
		s.mu.Unlock()

		if startErr == nil {
			logx.Printf("whisper: auto-restarted successfully (attempt %d/%d)", attempt, whisperRestartMaxAttempts)
			return
		}
		logx.Printf("whisper: auto-restart attempt %d/%d failed: %v", attempt, whisperRestartMaxAttempts, startErr)
	}
	logx.Printf("whisper: auto-restart gave up after %d attempts — STT will stay down until manually toggled off/on or the app is restarted", whisperRestartMaxAttempts)
}

func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}
	return processIsAlive(s.cmd.Process)
}

func (s *Server) GetStatus() ServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := ServerStatus{
		Port:     s.port,
		Language: s.language,
	}

	if s.cmd != nil && s.cmd.Process != nil {
		if processIsAlive(s.cmd.Process) {
			status.Running = true
			status.PID = s.cmd.Process.Pid
			status.ModelPath = s.modelPath
		}
		return status
	}

	if s.port > 0 && s.pingPort() {
		status.Running = true
	}

	return status
}

func (s *Server) pingPort() bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/", s.port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

func (s *Server) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	s.mu.RLock()
	port := s.port
	s.mu.RUnlock()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "recording.wav")
	if err != nil {
		return "", fmt.Errorf("whisper: form create: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return "", fmt.Errorf("whisper: form write: %w", err)
	}
	writer.Close()

	url := fmt.Sprintf("http://127.0.0.1:%d/inference", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", fmt.Errorf("whisper: request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("whisper: transcribe request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper: server returned %d: %s", resp.StatusCode, string(msg))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("whisper: decode: %w", err)
	}

	return result.Text, nil
}

// whisperThreads returns the thread count passed to whisper-server's
// --threads flag. Previously omitted entirely, silently leaving
// whisper-server on its own hardcoded default of 4 regardless of the host's
// actual core count -- a real STT-latency cost on any machine with more
// cores available. Capped at 8: whisper.cpp's own docs note that beyond
// roughly the audio decode pipeline's parallelizable width, additional
// threads mostly add scheduling overhead rather than speed.
func whisperThreads() int {
	n := runtime.NumCPU()
	if n > 8 {
		return 8
	}
	if n < 1 {
		return 1
	}
	return n
}

func whisperServerBinary() string {
	if runtime.GOOS == "windows" {
		return "whisper-server.exe"
	}
	return "whisper-server"
}

func resolveBinary(configured string) (string, error) {
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured, nil
		}
	}

	for _, base := range binarySearchBases() {
		for _, dir := range []string{"cpu", "amd", "nvidia"} {
			p := filepath.Join(base, "binaries", runtime.GOOS, dir, whisperServerBinary())
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
		p := filepath.Join(base, "bin", whisperServerBinary())
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	if path, err := exec.LookPath("whisper-server"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("whisper-server binary not found — place it in binaries/{os}/cpu/")
}

func resolveModel(configured string) string {
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured
		}
	}

	modelNames := []string{"ggml-small.bin", "ggml-base.bin", "ggml-tiny.bin", "ggml-medium.bin", "ggml-large-v3-turbo.bin"}

	for _, base := range binarySearchBases() {
		for _, dir := range []string{"cpu", "amd", "nvidia"} {
			for _, name := range modelNames {
				p := filepath.Join(base, "binaries", runtime.GOOS, dir, name)
				if _, err := os.Stat(p); err == nil {
					return p
				}
			}
		}
		for _, name := range modelNames {
			p := filepath.Join(base, "data", "models", name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	return ""
}

// binarySearchBases returns directories to search for bundled binaries.
func binarySearchBases() []string {
	exePath, err := os.Executable()
	if err != nil {
		return []string{"."}
	}
	return binarySearchBasesFrom(exePath)
}

// binarySearchBasesFrom is the pure part of binarySearchBases, split out so
// it can be tested without mocking os.Executable: the current working
// directory, the executable's own directory, and that directory's parent.
// The parent matters for the installed CLI, whose binary lives one level
// deeper (~/.memo/bin/memo) than the bundled "binaries/" tree it ships next
// to (~/.memo/binaries/...) — the GUI/AppImage binary sits flush with
// "binaries/" already, so exeDir alone covers it, but the CLI needs the
// parent to find the very same bundle. Mirrors internal/llama's
// binarySearchBasesFrom (same bug, fixed there first).
func binarySearchBasesFrom(exePath string) []string {
	bases := []string{"."}
	exeDir := filepath.Dir(exePath)
	if exeDir != "." {
		bases = append(bases, exeDir)
		if parent := filepath.Dir(exeDir); parent != exeDir {
			bases = append(bases, parent)
		}
	}
	return bases
}

func withPrependedEnvPath(env []string, key, dir string, caseInsensitive bool) []string {
	out := make([]string, 0, len(env)+1)
	found := false

	for _, entry := range env {
		entryKey, entryValue, ok := strings.Cut(entry, "=")
		if !ok {
			out = append(out, entry)
			continue
		}

		matches := entryKey == key
		if caseInsensitive {
			matches = strings.EqualFold(entryKey, key)
		}
		if !matches {
			out = append(out, entry)
			continue
		}

		if found {
			continue
		}
		out = append(out, entryKey+"="+prependPathValue(entryValue, dir))
		found = true
	}

	if !found {
		out = append(out, key+"="+dir)
	}
	return out
}

func prependPathValue(current, dir string) string {
	if current == "" {
		return dir
	}
	parts := []string{dir}
	for _, part := range filepath.SplitList(current) {
		if part == "" || samePathEntry(part, dir) {
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, string(os.PathListSeparator))
}

func samePathEntry(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

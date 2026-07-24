package llama

import (
	"fmt"
	"memo/internal/config"
	"memo/internal/gguf"
	"memo/internal/logx"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ServerStatus represents the current state of the llama-server.
type ServerStatus struct {
	Running   bool    `json:"running"`
	ModelPath string  `json:"model_path"`
	ModelName string  `json:"model_name"`
	Port      int     `json:"port"`
	PID       int     `json:"pid"`
	GPU       GPUInfo `json:"gpu"`
}

// Server manages a llama-server subprocess.
type Server struct {
	mu        sync.RWMutex
	cmd       *exec.Cmd
	port      int
	ctxSize   int
	modelPath string
	gpu       GPUInfo
	stopping  bool
	waitDone  chan struct{} // Closed when the process actually exits
	portPid   int           // Last known PID for the port (fallback when cmd is nil)
	logFile   *os.File      // llama-server's own stdout/stderr, never the REPL's
}

// clampContextSize returns ctxSize clamped to modelPath's own trained max
// context length (read from its GGUF header, see internal/gguf), when that
// can be determined. Returns ctxSize unchanged when the file can't be
// parsed or its architecture's max context isn't recognized (maxCtx==0,
// "unknown") — there's nothing to clamp against in that case, same as
// before this existed.
func clampContextSize(modelPath string, ctxSize int) int {
	meta, err := gguf.Read(modelPath)
	if err != nil || meta.ContextLength <= 0 || ctxSize <= meta.ContextLength {
		return ctxSize
	}
	logx.Printf("llama: requested ctx-size %d exceeds %s's max context %d, clamping", ctxSize, filepath.Base(modelPath), meta.ContextLength)
	return meta.ContextLength
}

// NewServer creates a new llama-server manager.
func NewServer(port, ctxSize int) *Server {
	if port <= 0 {
		port = 8081
	}
	if ctxSize <= 0 {
		ctxSize = 4096
	}
	return &Server{
		port:    port,
		ctxSize: ctxSize,
	}
}

// Start launches llama-server with the given model file and configuration.
// If gpuLayers is -1, it auto-detects maximum capacity. If port/ctxSize are 0, they use defaults.
// Set embedding=true only for dedicated embedding models; chat models must NOT get --embedding.
func (s *Server) Start(binaryPath, modelPath string, ctxSize, port, gpuLayers int, embedding bool, mode string) error {
	return s.startInternal(binaryPath, modelPath, ctxSize, port, gpuLayers, embedding, mode, nil)
}

// RPCOptions configures the swarm coordinator's llama-server invocation —
// see StartWithRPC. Order in both slices is meaningful (see PLAN_memo_swarm.md):
// index 0 of the resulting --tensor-split is always the coordinator's own
// local share; Servers[i] corresponds to TensorSplit[i+1].
type RPCOptions struct {
	// Servers is "host:port" per worker, in list order.
	Servers []string
	// TensorSplit has len(Servers)+1 entries: [0] is the coordinator's own
	// share, [1:] align positionally with Servers.
	TensorSplit []float64
}

// StartWithRPC is Start, plus llama.cpp RPC flags to pool compute across
// the given worker machines (Memo Swarm — see PLAN_memo_swarm.md). Always
// forces --split-mode layer (never "row" — see buildRPCArgs). Binary
// resolution goes through resolveCoordinatorBinary instead of the plain
// resolveBinary, since not every bundled flavor is built with RPC support
// (verified: the plain linux/cpu release rejects --rpc outright — see
// rpc_probe.go) — mode is still used for GPU-layer/detection purposes below,
// just not for binary flavor selection in this path. embedding is always
// false: an embedding-only server has no reason to run as a swarm
// coordinator.
func (s *Server) StartWithRPC(binaryPath, modelPath string, ctxSize, port, gpuLayers int, mode string, rpc RPCOptions) error {
	return s.startInternal(binaryPath, modelPath, ctxSize, port, gpuLayers, false, mode, &rpc)
}

func (s *Server) startInternal(binaryPath, modelPath string, ctxSize, port, gpuLayers int, embedding bool, mode string, rpc *RPCOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil && s.cmd.Process != nil {
		return fmt.Errorf("llama: server already running (PID %d)", s.cmd.Process.Pid)
	}

	// Resolve binary path — the coordinator role needs an RPC-capable
	// build, which isn't guaranteed for every flavor (see resolveCoordinatorBinary).
	var bin string
	var err error
	if rpc != nil {
		bin, err = resolveCoordinatorBinary(binaryPath, mode)
	} else {
		bin, err = resolveBinary(binaryPath, mode)
	}
	if err != nil {
		return fmt.Errorf("Llama.cpp motoru (llama-server) bulunamadı. Lütfen motorun kurulu olduğundan emin olun veya Ayarlar -> Llama kısmından yolu kontrol edin. (Hata: %w)", err)
	}

	// Detect GPU
	s.gpu = DetectGPU()
	if mode == "cpu" {
		s.gpu = GPUInfo{Type: GPUTypeCPU, Name: "CPU (Zorunlu)"}
	} else if mode == "nvidia" {
		s.gpu.Type = GPUTypeNVIDIA
	} else if mode == "amd" {
		s.gpu.Type = GPUTypeAMD
	} else if mode == "metal" {
		s.gpu = GPUInfo{Type: GPUTypeMetal, Name: "Apple Silicon (Metal)", GPULayers: 999}
	}

	// Apply overrides
	actualGPU := gpuLayers
	if actualGPU < 0 {
		actualGPU = s.gpu.GPULayers
	}
	// If mode is CPU, force 0 layers
	if mode == "cpu" {
		actualGPU = 0
	}
	actualCtx := ctxSize
	if actualCtx <= 0 {
		actualCtx = 4096
	}
	// Defense in depth: clamp to the model's own trained context length,
	// independent of whatever the caller (a UI slider, an older client, a
	// direct API call) already validated or didn't. Without this, a value
	// exceeding what the model can actually support (the frontend used to
	// accept anything typed into a free-text field, with no bound at all)
	// crashed llama-server outright at startup instead of failing cleanly.
	actualCtx = clampContextSize(modelPath, actualCtx)
	actualPort := port
	if actualPort <= 0 {
		actualPort = s.port
	}

	// If something is already listening on this port, clear it before
	// binding. This is deliberately unconditional (every attempt, not just
	// retries): Pdeathsig is not used on Linux (see newSysProcAttr) so a
	// llama-server whose parent Memo process died abnormally (crash, kill
	// -9, OOM) is orphaned and keeps the port forever. Without this
	// pre-flight check, a fresh Server here has no memory of that orphan —
	// cmd.Start() succeeds at the OS level, but llama-server itself fails
	// to bind and exits within ~1s; WaitReady sees the exit almost
	// immediately, and the resulting Stop() only SIGTERMs *this* attempt's
	// own (already-dead) PID, since s.cmd is non-nil by then — it never
	// reaches killByPort's port-discovery fallback. Every retry repeats the
	// identical failure and the orphan is left running until reboot. Safe
	// to call every time: killByPort is a no-op when the port is free.
	if err := s.killByPort(actualPort); err != nil {
		logx.Printf("llama: could not clear port %d before starting: %v", actualPort, err)
	}

	// Update active state
	s.port = actualPort
	s.ctxSize = actualCtx

	// Build command args
	args := []string{
		"--model", modelPath,
		"--port", fmt.Sprintf("%d", actualPort),
		"--host", "127.0.0.1",
		"--ctx-size", fmt.Sprintf("%d", actualCtx),
		"--parallel", "1",
	}
	// Offload layers to the GPU when one is available. Without this flag
	// llama-server runs entirely on the CPU regardless of the detected GPU,
	// which makes both chat and embedding startup/inference much slower.
	if actualGPU > 0 {
		args = append(args, "--n-gpu-layers", fmt.Sprintf("%d", actualGPU))
	}
	if embedding {
		// Embedding-only mode: enables /embeddings endpoint, disables chat.
		// Skip the warmup decode — it noticeably slows startup and is pointless
		// for an embedding model.
		args = append(args, "--embedding", "--no-warmup")
	} else {
		// Use the model's own chat template (Jinja). This is what enables
		// OpenAI-style tool calling on /v1/chat/completions, which the agent
		// mode relies on when running against a local model.
		args = append(args, "--jinja")
	}

	// Auto-detect multimodal projector (mmproj) file next to the model
	mmproj := findMmproj(modelPath)
	if mmproj != "" {
		args = append(args, "--mmproj", mmproj)
		logx.Printf("llama: detected mmproj: %s", mmproj)
	}

	if rpc != nil {
		args = append(args, buildRPCArgs(*rpc)...)
	}

	logx.Printf("llama: launching %s %s", bin, strings.Join(args, " "))

	s.cmd = exec.Command(bin, args...)
	// llama-server logs its own verbose C++-side output straight to whatever
	// stdout/stderr it inherits. In headless mode that's fine — the shell
	// wrapper redirects the whole process's output to backend.log anyway —
	// but the terminal REPL's own stdout IS the live chat UI, so inheriting
	// it here would interleave llama-server's raw log lines character-by-
	// character with the REPL's output (exactly what a user reported: model
	// load logs and chat replies garbled together mid-response). Give it its
	// own log file instead, unconditionally, so this can never happen.
	if s.logFile != nil {
		s.logFile.Close()
		s.logFile = nil
	}
	logPath := config.DataPath(fmt.Sprintf("llama-%d.log", actualPort))
	if f, ferr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); ferr == nil {
		s.logFile = f
		s.cmd.Stdout = f
		s.cmd.Stderr = f
	} else {
		logx.Printf("llama: could not open %s (%v), falling back to inherited stdout/stderr", logPath, ferr)
		s.cmd.Stdout = os.Stdout
		s.cmd.Stderr = os.Stderr
	}
	s.modelPath = modelPath
	s.stopping = false
	s.waitDone = make(chan struct{})

	// Add binary directory to the shared library search path.
	// Windows uses PATH for DLL discovery.
	// On Linux/macOS: the bundled binary has its RUNPATH set at build time.
	binDir := filepath.Dir(bin)
	absBinDir, err := filepath.Abs(binDir)
	if err == nil {
		binDir = absBinDir
	}

	env := os.Environ()
	if runtime.GOOS == "windows" {
		env = withPrependedEnvPath(env, "PATH", binDir, true)
	} else if runtime.GOOS == "darwin" {
		// macOS ignores LD_LIBRARY_PATH; dyld uses DYLD_LIBRARY_PATH instead.
		env = withPrependedEnvPath(env, "DYLD_LIBRARY_PATH", binDir, false)
		env = withPrependedEnvPath(env, "DYLD_FALLBACK_LIBRARY_PATH", binDir, false)
	} else {
		// The bundled binary has RUNPATH=/tmp/llama.cpp/build/bin: which
		// doesn't exist after deployment. Put the real directory first.
		env = withPrependedEnvPath(env, "LD_LIBRARY_PATH", binDir, false)
	}
	s.cmd.Env = env

	// Setpgid puts the child in its own process group so forceKill/killByPort
	// can signal the whole tree without affecting the parent Memo process.
	// None of the platform newSysProcAttr() implementations set Pdeathsig —
	// it's incompatible with Setpgid in Go (runtime thread reuse triggers
	// premature child death) — so a llama-server whose parent dies abnormally
	// (crash, kill -9, OOM) is NOT auto-killed by the OS; it's orphaned and
	// keeps holding its port until something explicitly kills it. The
	// port-clearing pre-flight above exists precisely to catch that case on
	// the next Start().
	s.cmd.SysProcAttr = newSysProcAttr()

	if err := s.cmd.Start(); err != nil {
		s.cmd = nil
		return fmt.Errorf("llama: start failed: %w", err)
	}

	s.portPid = s.cmd.Process.Pid

	logx.Printf("llama: server started (PID %d, port %d, GPU layers: %d)",
		s.cmd.Process.Pid, s.port, s.gpu.GPULayers)

	// Monitor process in background for unexpected exits
	logx.GoRecover("llama.Server.monitor", s.monitor)

	return nil
}

// WaitReady polls the server's health endpoint until it responds or timeout.
func (s *Server) WaitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	s.mu.Lock()
	port := s.port
	s.mu.Unlock()
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/models", port)

	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		if !s.IsRunning() {
			return fmt.Errorf("llama: server process exited during startup")
		}
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				logx.Printf("llama: server ready on port %d", port)
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("llama: server failed to become ready within %v", timeout)
}

// Stop gracefully shuts down the llama-server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		if s.logFile != nil {
			s.logFile.Close()
			s.logFile = nil
		}
	}()

	if s.cmd == nil || s.cmd.Process == nil {
		// No tracked cmd — try last known PID first, then port discovery.
		if s.port > 0 {
			if s.portPid > 0 {
				if err := killPID(s.portPid); err != nil {
					logx.Printf("llama: kill stored PID %d: %v, trying port discovery", s.portPid, err)
				} else {
					s.portPid = 0
					return nil
				}
			}
			if err := s.killByPort(s.port); err != nil {
				return fmt.Errorf("llama: stop by port: %w", err)
			}
		}
		return nil
	}

	s.stopping = true
	logx.Printf("llama: stopping server (PID %d)", s.cmd.Process.Pid)

	// Step 1: Send SIGTERM to the process for graceful shutdown.
	processSignalTerm(s.cmd.Process)

	// Step 2: Wait up to 5 seconds for graceful exit (monitored by monitor loop)
	select {
	case <-s.waitDone:
		logx.Printf("llama: server stopped gracefully")
	case <-time.After(5 * time.Second):
		logx.Printf("llama: graceful shutdown timed out, force killing")
		s.forceKill()
	}

	s.cmd = nil
	s.modelPath = ""
	s.portPid = 0
	return nil
}

// killByPort finds the PID listening on the given TCP port and kills it.
// Uses lsof (primary) or fuser (fallback) to locate the process. Delegates
// to the package-level KillByPort (process_export.go), which is also
// exposed for other packages in this module to reuse.
func (s *Server) killByPort(port int) error {
	return KillByPort(port)
}

// pidOnPort returns the PID of the process listening on the given TCP port, or 0.
func (s *Server) pidOnPort(port int) int {
	return pidListeningOnPort(port)
}

// forceKill sends SIGKILL to the process (and its group if possible).
func (s *Server) forceKill() {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	forceKillCmd(s.cmd, s.waitDone)
}

// monitor watches for unexpected process exit.
func (s *Server) monitor() {
	s.mu.Lock()
	cmd := s.cmd
	waitDone := s.waitDone
	s.mu.Unlock()

	if cmd == nil {
		return
	}

	err := cmd.Wait()

	// Signal that the process has exited BEFORE acquiring s.mu. Stop() may be
	// holding s.mu while blocked on <-s.waitDone; if we waited for the lock here
	// first, waitDone would never close and every Stop()/shutdown would burn the
	// full 5s graceful timeout (plus the 3s force-kill wait) on a process that
	// already exited.
	if waitDone != nil {
		select {
		case <-waitDone:
			// already closed
		default:
			close(waitDone)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopping {
		return // Expected shutdown
	}

	if err != nil {
		logx.Printf("llama: server exited unexpectedly: %v", err)
	} else {
		logx.Printf("llama: server exited unexpectedly (exit 0)")
	}
	s.cmd = nil
	s.modelPath = ""
}

// IsRunning checks if the server process is still alive.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}
	return processIsAlive(s.cmd.Process)
}

// GetStatus returns the current server status.
// If the app lost track of the process (e.g. after restart), it falls back to
// an HTTP health check on the configured port so we never show "not running"
// when something is actually listening.
func (s *Server) GetStatus() ServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := ServerStatus{
		Port: s.port,
		GPU:  s.gpu,
	}

	if s.cmd != nil && s.cmd.Process != nil {
		// Process is tracked — use direct info.
		if processIsAlive(s.cmd.Process) {
			status.Running = true
			status.PID = s.cmd.Process.Pid
			status.ModelPath = s.modelPath
			status.ModelName = extractModelName(s.modelPath)
		}
		return status
	}

	// No tracked process — ping the port to detect externally started servers.
	if s.port > 0 && s.pingPort() {
		status.Running = true
		status.ModelName = "llama-server (harici)"
	}

	return status
}

// pingPort does a quick HTTP check on /v1/models to see if a server is live.
func (s *Server) pingPort() bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/v1/models", s.port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// GetBaseURL returns the OpenAI-compatible API base URL.
func (s *Server) GetBaseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/v1", s.port)
}

// ─── Binary Resolution ──────────────────────────────────────────

// resolveBinary finds the llama-server binary.
// It searches relative to the current working directory first, then relative to
// the running executable's location (for bundled AppImage/tar.gz deployments
// where CWD is $HOME/.memo but binaries live next to memo-backend).
func resolveBinary(configured string, mode string) (string, error) {
	// 1. If a specific mode is requested, look in bundled paths first
	currentOS := runtime.GOOS
	if mode != "" && mode != "auto" {
		for _, base := range binarySearchBases() {
			p := filepath.Join(base, "binaries", currentOS, mode, llamaServerBinary())
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
			p = filepath.Join(base, "bin", llamaServerBinary())
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}

	// 2. Use configured path if provided
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured, nil
		}
	}

	// 3. Check Bundled Paths (Auto-detect order) in all search bases
	for _, base := range binarySearchBases() {
		bundledDirs := []string{
			filepath.Join(base, "binaries", currentOS, "amd", llamaServerBinary()),
			filepath.Join(base, "binaries", currentOS, "nvidia", llamaServerBinary()),
			filepath.Join(base, "binaries", currentOS, "cpu", llamaServerBinary()),
			filepath.Join(base, "bin", llamaServerBinary()),
		}
		for _, p := range bundledDirs {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}

	// 4. Check PATH (exec.LookPath handles .exe on Windows automatically)
	if path, err := exec.LookPath("llama-server"); err == nil {
		return path, nil
	}

	// 5. Check common install locations
	homeDir, _ := os.UserHomeDir()
	var commonPaths []string
	switch runtime.GOOS {
	case "windows":
		commonPaths = []string{
			filepath.Join(homeDir, "llama.cpp", "build", "bin", "llama-server.exe"),
			filepath.Join(homeDir, "scoop", "apps", "llama.cpp", "current", "llama-server.exe"),
			`C:\Program Files\llama.cpp\llama-server.exe`,
			filepath.Join(homeDir, ".local", "bin", "llama-server.exe"),
		}
	case "darwin":
		commonPaths = []string{
			"/opt/homebrew/bin/llama-server",
			"/usr/local/opt/llama.cpp/bin/llama-server",
			filepath.Join(homeDir, "llama.cpp", "build", "bin", "llama-server"),
		}
	default:
		commonPaths = []string{
			"/usr/local/bin/llama-server",
			"/usr/bin/llama-server",
			filepath.Join(homeDir, ".local", "bin", "llama-server"),
			filepath.Join(homeDir, "llama.cpp", "build", "bin", "llama-server"),
		}
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("llama-server binary not found — install llama.cpp or set the binary path in settings")
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
// parent to find the very same bundle.
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

// llamaServerBinary returns the platform-specific llama-server binary name.
func llamaServerBinary() string {
	if runtime.GOOS == "windows" {
		return "llama-server.exe"
	}
	return "llama-server"
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

// extractModelName gets a clean name from a model file path.
func extractModelName(path string) string {
	if path == "" {
		return ""
	}
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".gguf")
	return name
}

// findMmproj looks for a multimodal projector GGUF file next to the model.
func findMmproj(modelPath string) string {
	dir := filepath.Dir(modelPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.Contains(lower, "mmproj") || strings.Contains(lower, "multimodal") {
			if strings.HasSuffix(lower, ".gguf") {
				return filepath.Join(dir, e.Name())
			}
		}
	}
	return ""
}

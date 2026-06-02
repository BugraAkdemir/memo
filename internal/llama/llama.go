package llama

import (
	"fmt"
	"log"
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil && s.cmd.Process != nil {
		return fmt.Errorf("llama: server already running (PID %d)", s.cmd.Process.Pid)
	}

	// Resolve binary path
	bin, err := resolveBinary(binaryPath, mode)
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
	actualPort := port
	if actualPort <= 0 {
		actualPort = s.port
	}

	// Update active state
	s.port = actualPort
	s.ctxSize = actualCtx

	// Build command args
	args := []string{
		"--model", modelPath,
		"--port", fmt.Sprintf("%d", actualPort),
		"--host", "127.0.0.1",
		"--n-gpu-layers", fmt.Sprintf("%d", actualGPU),
		"--ctx-size", fmt.Sprintf("%d", actualCtx),
		"--parallel", "1",
	}
	if embedding {
		// Embedding-only mode: enables /embeddings endpoint, disables chat.
		args = append(args, "--embedding")
	}

	log.Printf("llama: launching %s %s", bin, strings.Join(args, " "))

	s.cmd = exec.Command(bin, args...)
	s.cmd.Stdout = os.Stdout
	s.cmd.Stderr = os.Stderr
	s.modelPath = modelPath
	s.stopping = false
	s.waitDone = make(chan struct{})

	// Add binary directory to the shared library search path.
	// Windows uses PATH for DLL discovery; Linux/macOS use LD_LIBRARY_PATH.
	binDir := filepath.Dir(bin)
	absBinDir, err := filepath.Abs(binDir)
	if err == nil {
		binDir = absBinDir
	}
	
	env := os.Environ()
	if runtime.GOOS == "windows" {
		for i, e := range env {
			if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
				env[i] = e[:5] + binDir + ";" + e[5:]
				break
			}
		}
		s.cmd.Env = env
	} else {
		ldPath := binDir
		for _, e := range env {
			if strings.HasPrefix(e, "LD_LIBRARY_PATH=") {
				ldPath = binDir + ":" + strings.TrimPrefix(e, "LD_LIBRARY_PATH=")
				break
			}
		}
		s.cmd.Env = append(env, "LD_LIBRARY_PATH="+ldPath)
	}

	// Pdeathsig: child receives SIGKILL when parent dies (even force-kill).
	// NOTE: Setpgid must NOT be set — it clears Pdeathsig on Linux.
	s.cmd.SysProcAttr = newSysProcAttr()

	if err := s.cmd.Start(); err != nil {
		s.cmd = nil
		return fmt.Errorf("llama: start failed: %w", err)
	}

	log.Printf("llama: server started (PID %d, port %d, GPU layers: %d)",
		s.cmd.Process.Pid, s.port, s.gpu.GPULayers)

	// Monitor process in background for unexpected exits
	go s.monitor()

	return nil
}

// WaitReady polls the server's health endpoint until it responds or timeout.
func (s *Server) WaitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/models", s.port)

	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				log.Printf("llama: server ready on port %d", s.port)
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

	if s.cmd == nil || s.cmd.Process == nil {
		// No tracked process — try to kill whatever is on the port.
		if s.port > 0 {
			if err := s.killByPort(s.port); err != nil {
				return fmt.Errorf("llama: stop by port: %w", err)
			}
		}
		return nil
	}

	s.stopping = true
	log.Printf("llama: stopping server (PID %d)", s.cmd.Process.Pid)

	// Step 1: Send SIGTERM to the process for graceful shutdown.
	processSignalTerm(s.cmd.Process)

	// Step 2: Wait up to 5 seconds for graceful exit (monitored by monitor loop)
	select {
	case <-s.waitDone:
		log.Printf("llama: server stopped gracefully")
	case <-time.After(5 * time.Second):
		log.Printf("llama: graceful shutdown timed out, force killing")
		s.forceKill()
	}

	s.cmd = nil
	s.modelPath = ""
	return nil
}

// killByPort finds the PID listening on the given TCP port and kills it.
// Uses lsof (primary) or fuser (fallback) to locate the process.
func (s *Server) killByPort(port int) error {
	pid := s.pidOnPort(port)
	if pid <= 0 {
		log.Printf("llama: nothing found on port %d", port)
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	log.Printf("llama: killing external PID %d on port %d", pid, port)
	processSignalTerm(proc)

	// Give it 3s to die gracefully, then force kill.
	for i := 0; i < 6; i++ {
		time.Sleep(500 * time.Millisecond)
		if !processIsAlive(proc) {
			log.Printf("llama: process %d exited cleanly", pid)
			return nil
		}
	}
	log.Printf("llama: force-killing PID %d", pid)
	proc.Kill()
	return nil
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
	s.mu.Unlock()

	if cmd == nil {
		return
	}

	err := cmd.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Signal that the process has exited
	if s.waitDone != nil {
		select {
		case <-s.waitDone:
			// already closed
		default:
			close(s.waitDone)
		}
	}

	if s.stopping {
		return // Expected shutdown
	}

	if err != nil {
		log.Printf("llama: server exited unexpectedly: %v", err)
	} else {
		log.Printf("llama: server exited unexpectedly (exit 0)")
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
	var commonPaths []string
	switch runtime.GOOS {
	case "windows":
		home := os.Getenv("USERPROFILE")
		commonPaths = []string{
			filepath.Join(home, "llama.cpp", "build", "bin", "llama-server.exe"),
			filepath.Join(home, "scoop", "apps", "llama.cpp", "current", "llama-server.exe"),
			`C:\Program Files\llama.cpp\llama-server.exe`,
			filepath.Join(home, ".local", "bin", "llama-server.exe"),
		}
	case "darwin":
		home := os.Getenv("HOME")
		commonPaths = []string{
			"/opt/homebrew/bin/llama-server",
			"/usr/local/opt/llama.cpp/bin/llama-server",
			filepath.Join(home, "llama.cpp", "build", "bin", "llama-server"),
		}
	default:
		home := os.Getenv("HOME")
		commonPaths = []string{
			"/usr/local/bin/llama-server",
			"/usr/bin/llama-server",
			filepath.Join(home, ".local", "bin", "llama-server"),
			filepath.Join(home, "llama.cpp", "build", "bin", "llama-server"),
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
// First the current working directory, then the directory of the running executable.
func binarySearchBases() []string {
	bases := []string{"."}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		if exeDir != "." {
			bases = append(bases, exeDir)
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

// extractModelName gets a clean name from a model file path.
func extractModelName(path string) string {
	if path == "" {
		return ""
	}
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".gguf")
	return name
}

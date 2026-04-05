package llama

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
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
func (s *Server) Start(binaryPath, modelPath string, ctxSize, port, gpuLayers int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil && s.cmd.Process != nil {
		return fmt.Errorf("llama: server already running (PID %d)", s.cmd.Process.Pid)
	}

	// Resolve binary path
	bin, err := resolveBinary(binaryPath)
	if err != nil {
		return fmt.Errorf("llama: %w", err)
	}

	// Detect GPU
	s.gpu = DetectGPU()
	
	// Apply overrides
	actualGPU := gpuLayers
	if actualGPU < 0 {
		actualGPU = s.gpu.GPULayers
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
		"--embedding",
		"--pooling", "mean",
	}

	log.Printf("llama: launching %s %s", bin, strings.Join(args, " "))

	s.cmd = exec.Command(bin, args...)
	s.cmd.Stdout = os.Stdout
	s.cmd.Stderr = os.Stderr
	s.modelPath = modelPath
	s.stopping = false

	// Set LD_LIBRARY_PATH to include the binary's directory (shared libs live next to it)
	binDir := filepath.Dir(bin)
	env := os.Environ()
	ldPath := binDir
	for _, e := range env {
		if strings.HasPrefix(e, "LD_LIBRARY_PATH=") {
			ldPath = binDir + ":" + strings.TrimPrefix(e, "LD_LIBRARY_PATH=")
			break
		}
	}
	s.cmd.Env = append(env, "LD_LIBRARY_PATH="+ldPath)

	// Set process group so we can kill the entire tree
	s.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
		return nil
	}

	s.stopping = true
	pid := s.cmd.Process.Pid
	log.Printf("llama: stopping server (PID %d)", pid)

	// Step 1: Send SIGTERM for graceful shutdown
	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("llama: SIGTERM failed: %v, trying SIGKILL", err)
		s.forceKill()
		return nil
	}

	// Step 2: Wait up to 5 seconds for graceful exit
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()

	select {
	case <-done:
		log.Printf("llama: server stopped gracefully")
	case <-time.After(5 * time.Second):
		log.Printf("llama: graceful shutdown timed out, force killing")
		s.forceKill()
	}

	s.cmd = nil
	s.modelPath = ""
	return nil
}

// forceKill sends SIGKILL to the entire process group.
func (s *Server) forceKill() {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}

	// Kill the entire process group
	pgid, err := syscall.Getpgid(s.cmd.Process.Pid)
	if err == nil {
		syscall.Kill(-pgid, syscall.SIGKILL)
	} else {
		s.cmd.Process.Kill()
	}

	// Wait for the process to actually exit
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	waitDone := make(chan struct{})
	go func() {
		s.cmd.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
	case <-ctx.Done():
		log.Printf("llama: WARNING — process may not have exited cleanly")
	}
}

// monitor watches for unexpected process exit.
func (s *Server) monitor() {
	if s.cmd == nil {
		return
	}

	err := s.cmd.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()

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

	// Check if process is still alive
	err := s.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// GetStatus returns the current server status.
func (s *Server) GetStatus() ServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := ServerStatus{
		Port: s.port,
		GPU:  s.gpu,
	}

	if s.cmd != nil && s.cmd.Process != nil {
		status.Running = true
		status.PID = s.cmd.Process.Pid
		status.ModelPath = s.modelPath
		status.ModelName = extractModelName(s.modelPath)
	}

	return status
}

// GetBaseURL returns the OpenAI-compatible API base URL.
func (s *Server) GetBaseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/v1", s.port)
}

// ─── Binary Resolution ──────────────────────────────────────────

// resolveBinary finds the llama-server binary.
func resolveBinary(configured string) (string, error) {
	// 1. Use configured path if provided
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured, nil
		}
		return "", fmt.Errorf("configured binary not found: %s", configured)
	}

	// 2. Check PATH
	if path, err := exec.LookPath("llama-server"); err == nil {
		return path, nil
	}

	// 3. Check common install locations
	commonPaths := []string{
		"/usr/local/bin/llama-server",
		"/usr/bin/llama-server",
		filepath.Join(os.Getenv("HOME"), ".local", "bin", "llama-server"),
		filepath.Join(os.Getenv("HOME"), "llama.cpp", "build", "bin", "llama-server"),
	}

	if runtime.GOOS == "darwin" {
		commonPaths = append(commonPaths,
			"/opt/homebrew/bin/llama-server",
			"/usr/local/opt/llama.cpp/bin/llama-server",
		)
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("llama-server binary not found — install llama.cpp or set the binary path in settings")
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

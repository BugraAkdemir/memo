package whisper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

	s.port = actualPort
	s.language = lang

	args := []string{
		"--model", model,
		"--port", fmt.Sprintf("%d", actualPort),
		"--host", "127.0.0.1",
		"--language", lang,
	}

	log.Printf("whisper: launching %s %s", bin, strings.Join(args, " "))

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

	log.Printf("whisper: server started (PID %d, port %d)", s.cmd.Process.Pid, s.port)

	go s.monitor()

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
			log.Printf("whisper: server ready on port %d", s.port)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("whisper: server failed to become ready within %v", timeout)
}

func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd == nil || s.cmd.Process == nil {
		if s.port > 0 {
			if s.portPid > 0 {
				if err := killPID(s.portPid); err != nil {
					log.Printf("whisper: kill stored PID %d: %v, trying port discovery", s.portPid, err)
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

	s.stopping = true
	log.Printf("whisper: stopping server (PID %d)", s.cmd.Process.Pid)

	processSignalTerm(s.cmd.Process)

	select {
	case <-s.waitDone:
		log.Printf("whisper: server stopped gracefully")
	case <-time.After(5 * time.Second):
		log.Printf("whisper: graceful shutdown timed out, force killing")
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
		log.Printf("whisper: nothing found on port %d", port)
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	log.Printf("whisper: killing external PID %d on port %d", pid, port)
	processSignalTerm(proc)

	for i := 0; i < 6; i++ {
		time.Sleep(500 * time.Millisecond)
		if !processIsAlive(proc) {
			log.Printf("whisper: process %d exited cleanly", pid)
			return nil
		}
	}
	log.Printf("whisper: force-killing PID %d", pid)
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

	if s.waitDone != nil {
		select {
		case <-s.waitDone:
		default:
			close(s.waitDone)
		}
	}

	if s.stopping {
		return
	}

	if err != nil {
		log.Printf("whisper: server exited unexpectedly: %v", err)
	} else {
		log.Printf("whisper: server exited unexpectedly (exit 0)")
	}
	s.cmd = nil
	s.modelPath = ""
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

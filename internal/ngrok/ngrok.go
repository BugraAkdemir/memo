package ngrok

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type Server struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	port     int
	token    string
	binPath  string
	publicURL string
	running  bool
	apiPort  int
}

func NewServer(binPath string) *Server {
	return &Server{
		binPath: binPath,
	}
}

func (s *Server) Start(port int, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("ngrok already running")
	}

	s.port = port
	s.token = token
	s.apiPort = 4040

	bin := s.binPath
	if bin == "" {
		bin = "ngrok"
	}

	// Step 1: Configure auth token
	configCmd := exec.Command(bin, "config", "add-authtoken", token)
	if out, err := configCmd.CombinedOutput(); err != nil {
		log.Printf("[ngrok] config add-authtoken output: %s", string(out))
		return fmt.Errorf("ngrok auth config failed: %w", err)
	}

	// Step 2: Start ngrok
	cmd := exec.Command(bin, "http", fmt.Sprintf("%d", port), "--log=stdout")
	cmd.Stdout = ngrokLogWriter{}
	cmd.Stderr = ngrokLogWriter{}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ngrok start: %w", err)
	}
	s.cmd = cmd
	s.running = true

	// Step 3: Wait for tunnel and get public URL
	go s.waitForTunnel()

	log.Printf("[ngrok] Starting on port %d", port)
	return nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.cmd == nil {
		return nil
	}

	if err := s.cmd.Process.Kill(); err != nil {
		log.Printf("[ngrok] kill: %v", err)
	}
	s.running = false
	s.publicURL = ""
	log.Println("[ngrok] Stopped")
	return nil
}

func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Server) PublicURL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.publicURL
}

func (s *Server) Status() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]interface{}{
		"running":    s.running,
		"port":       s.port,
		"public_url": s.publicURL,
	}
}

func (s *Server) waitForTunnel() {
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)

		s.mu.Lock()
		if !s.running {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		url, err := s.fetchTunnelURL()
		if err == nil && url != "" {
			s.mu.Lock()
			s.publicURL = url
			s.mu.Unlock()
			log.Printf("[ngrok] Public URL: %s", url)
			return
		}
	}

	log.Println("[ngrok] Failed to get public URL after 30s")
}

type ngrokTunnel struct {
	PublicURL string `json:"public_url"`
}

type ngrokAPIResp struct {
	Tunnels []ngrokTunnel `json:"tunnels"`
}

func (s *Server) fetchTunnelURL() (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/tunnels", s.apiPort))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var data ngrokAPIResp
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}

	for _, t := range data.Tunnels {
		if t.PublicURL != "" {
			return t.PublicURL, nil
		}
	}

	return "", fmt.Errorf("no tunnels found")
}

type ngrokLogWriter struct{}

func (ngrokLogWriter) Write(p []byte) (int, error) {
	log.Printf("[ngrok] %s", string(p))
	return len(p), nil
}

func DefaultBinaryDir() string {
	return filepath.Join(".", "binaries", runtimeGOOS())
}

func runtimeGOOS() string {
	return "" // filled at build time or use runtime.GOOS
}

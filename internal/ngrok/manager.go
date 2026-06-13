package ngrok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type Manager struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	port      int
	binPath   string
	publicURL string
	running   bool
	apiPort   int
	errMsg    string
	errCapture bytes.Buffer
}

func NewManager(binPath string) *Manager {
	if binPath == "" {
		binPath = findBinary()
	}
	return &Manager{
		binPath: binPath,
		apiPort: 4040,
	}
}

func findBinary() string {
	name := "ngrok"
	candidates := []string{
		filepath.Join(".", "binaries", runtime.GOOS, name),
		filepath.Join(".", "binaries", runtime.GOOS, name+".exe"),
		name,
		name + ".exe",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return name
}

func (m *Manager) Start(port int, authtoken string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("ngrok already running")
	}

	m.port = port
	m.errMsg = ""
	m.errCapture.Reset()

	// Step 1: configure auth token
	configCmd := exec.Command(m.binPath, "config", "add-authtoken", authtoken)
	if out, err := configCmd.CombinedOutput(); err != nil {
		log.Printf("[ngrok] config output: %s", string(out))
		return fmt.Errorf("ngrok auth config failed: %w", err)
	}

	// Step 2: start ngrok
	cmd := exec.Command(m.binPath, "http",
		fmt.Sprintf("%d", port),
		"--log=stdout",
		"--log-level=info",
	)
	cmd.Stderr = io.MultiWriter(&m.errCapture, logWriter{})
	cmd.Stdout = logWriter{}
	cmd.SysProcAttr = newSysProcAttr()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ngrok start: %w", err)
	}
	m.cmd = cmd
	m.running = true

	go m.pollPublicURL()
	go func() {
		err := m.cmd.Wait()
		m.mu.Lock()
		m.running = false
		if err != nil {
			if m.errCapture.Len() > 0 {
				m.errMsg = m.errCapture.String()
			} else {
				m.errMsg = err.Error()
			}
		}
		m.mu.Unlock()
	}()
	log.Printf("[ngrok] Starting tunnel for port %d", port)
	return nil
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	cmd := m.cmd
	running := m.running
	m.mu.Unlock()

	if !running || cmd == nil || cmd.Process == nil {
		return nil
	}

	killProcessTree(cmd)

	m.mu.Lock()
	m.running = false
	m.publicURL = ""
	m.mu.Unlock()
	log.Println("[ngrok] Stopped")
	return nil
}

func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *Manager) PublicURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.publicURL
}

func (m *Manager) pollPublicURL() {
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)

		m.mu.Lock()
		if !m.running {
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()

		url, err := m.fetchURL()
		if err == nil && url != "" {
			m.mu.Lock()
			m.publicURL = url
			m.mu.Unlock()
			log.Printf("[ngrok] Public URL: %s", url)
			return
		}
	}
	log.Println("[ngrok] timed out waiting for tunnel URL")
}

func (m *Manager) LastError() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errMsg
}

type apiTunnel struct {
	PublicURL string `json:"public_url"`
}

type apiResp struct {
	Tunnels []apiTunnel `json:"tunnels"`
}

func (m *Manager) fetchURL() (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/tunnels", m.apiPort))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data apiResp
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}
	for _, t := range data.Tunnels {
		if t.PublicURL != "" {
			return t.PublicURL, nil
		}
	}
	return "", fmt.Errorf("no tunnel")
}

type logWriter struct{}

func (logWriter) Write(p []byte) (int, error) {
	log.Printf("[ngrok] %s", string(p))
	return len(p), nil
}

package ngrok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/logx"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type Manager struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	port       int
	binPath    string
	publicURL  string
	running    bool
	apiPort    int
	errMsg     string
	errCapture bytes.Buffer
	stopping   bool
	authToken  string
	stopCh     chan struct{}
	stopOnce   sync.Once
}

func NewManager(binPath string) *Manager {
	if binPath == "" {
		binPath = findBinary()
	}
	return &Manager{
		binPath: binPath,
		apiPort: 4040,
		stopCh:  make(chan struct{}),
	}
}

func findBinary() string {
	name := "ngrok"
	var bases []string
	// Prefer exe-relative path so packaged builds find the binary regardless of CWD.
	if exe, err := os.Executable(); err == nil {
		bases = append(bases, filepath.Dir(exe))
	}
	bases = append(bases, ".")
	var candidates []string
	for _, base := range bases {
		candidates = append(candidates,
			filepath.Join(base, "binaries", runtime.GOOS, name),
			filepath.Join(base, "binaries", runtime.GOOS, name+".exe"),
		)
	}
	candidates = append(candidates, name, name+".exe")
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
	m.authToken = authtoken
	m.stopping = false
	m.errMsg = ""
	m.errCapture.Reset()

	if err := m.startLocked(); err != nil {
		return err
	}

	go m.monitor()
	logx.Printf("[ngrok] Starting tunnel for port %d", port)
	return nil
}

func (m *Manager) startLocked() error {
	configCmd := exec.Command(m.binPath, "config", "add-authtoken", m.authToken)
	if out, err := configCmd.CombinedOutput(); err != nil {
		logx.Printf("[ngrok] config output: %s", string(out))
		return fmt.Errorf("ngrok auth config failed: %w", err)
	}

	cmd := exec.Command(m.binPath, "http",
		fmt.Sprintf("%d", m.port),
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
	return nil
}

func (m *Manager) monitor() {
	for {
		m.mu.Lock()
		cmd := m.cmd
		m.mu.Unlock()

		if cmd == nil {
			return
		}
		err := cmd.Wait()

		m.mu.Lock()
		if m.stopping {
			m.running = false
			m.mu.Unlock()
			return
		}
		if err != nil {
			if m.errCapture.Len() > 0 {
				m.errMsg = m.errCapture.String()
			} else {
				m.errMsg = err.Error()
			}
		}
		logx.Printf("[ngrok] process exited (err=%v), restarting in 5s...", err)
		m.errCapture.Reset()
		m.mu.Unlock()

		select {
		case <-m.stopCh:
			return
		case <-time.After(5 * time.Second):
		}

		m.mu.Lock()
		if m.stopping {
			m.mu.Unlock()
			return
		}
		if err := m.startLocked(); err != nil {
			logx.Printf("[ngrok] restart failed: %v", err)
			m.running = false
			m.errMsg = fmt.Sprintf("restart failed: %v", err)
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()
	}
}

func (m *Manager) Stop() error {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
	m.mu.Lock()
	m.stopping = true
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
	logx.Info("[ngrok] Stopped")
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

func (m *Manager) LastError() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errMsg
}

func (m *Manager) pollPublicURL() {
	for i := 0; i < 30; i++ {
		select {
		case <-m.stopCh:
			return
		case <-time.After(time.Second):
		}
		m.mu.Lock()
		r := m.running
		m.mu.Unlock()
		if !r {
			return
		}
		url, err := m.fetchURL()
		if err == nil && url != "" {
			m.mu.Lock()
			m.publicURL = url
			m.mu.Unlock()
			logx.Printf("[ngrok] Public URL: %s", url)
			return
		}
	}
	logx.Info("[ngrok] timed out waiting for tunnel URL")
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
	logx.Printf("[ngrok] %s", string(p))
	return len(p), nil
}

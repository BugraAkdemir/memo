package webserver

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// AppBridge defines the interface that the main App must implement
// for the web server to call its methods.
type AppBridge interface {
	SendMessage(userMsg string) string
	SendMessageWithImage(userMsg string, imagePath string) string
	SendMessageWithFile(userMsg string, filePath string) string
	NewChat() string
	WebListChats() interface{}
	SwitchChat(id string) error
	DeleteChat(id string) error
	RenameChat(id, title string) error
	UpdateMessage(index int, content string) error
	DeleteMessage(index int) error
	WebGetActiveMessages() interface{}
	GetActiveChatID() string
	WebCheckConnection() interface{}
	GetMemoryCount() int
	ToggleIncognito(enabled bool)
	TranscribeAudio(audioData []byte) (string, error)
}

type Server struct {
	mu         sync.Mutex
	srv        *http.Server
	bridge     AppBridge
	fullBridge FullBridge
	assets     fs.FS
	running    bool
	port       int
	localIPs   []string
}

func New(bridge AppBridge, assets fs.FS) *Server {
	s := &Server{
		bridge: bridge,
		assets: assets,
	}
	if fb, ok := bridge.(FullBridge); ok {
		s.fullBridge = fb
	}
	return s
}

func (s *Server) Start(port int) error {
	return fmt.Errorf("remote access is disabled in this version")
}

// StartHTTP starts a plain HTTP server (no TLS) for local Flutter desktop communication.
// This avoids self-signed certificate issues when Flutter connects to localhost.
func (s *Server) StartHTTP(port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server already running on port %d", s.port)
	}

	mux := http.NewServeMux()

	// API endpoints (original)
	mux.HandleFunc("/api/send", s.handleSend)
	mux.HandleFunc("/api/chats", s.handleChats)
	mux.HandleFunc("/api/chats/new", s.handleNewChat)
	mux.HandleFunc("/api/chats/switch", s.handleSwitchChat)
	mux.HandleFunc("/api/chats/delete", s.handleDeleteChat)
	mux.HandleFunc("/api/chats/rename", s.handleRenameChat)
	mux.HandleFunc("/api/chats/active", s.handleActiveChat)
	mux.HandleFunc("/api/messages", s.handleMessages)
	mux.HandleFunc("/api/messages/update", s.handleUpdateMessage)
	mux.HandleFunc("/api/messages/delete", s.handleDeleteMessage)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/incognito", s.handleIncognito)
	mux.HandleFunc("/api/transcribe", s.handleTranscribe)
	mux.HandleFunc("/api/send_file", s.handleSendFile)

	// Flutter-specific endpoints
	mux.HandleFunc("/api/send/stream", s.handleSendStream)
	mux.HandleFunc("/api/system-prompt", s.handleSystemPrompt)
	mux.HandleFunc("/api/system-prompt/reset", s.handleResetSystemPrompt)
	mux.HandleFunc("/api/incognito-prompt", s.handleIncognitoPrompt)
	mux.HandleFunc("/api/memory/files", s.handleMemoryFiles)
	mux.HandleFunc("/api/memory/clear", s.handleMemoryClear)
	mux.HandleFunc("/api/memory/settings", s.handleMemorySettings)
	mux.HandleFunc("/api/memory/enabled", s.handleMemoryEnabled)
	mux.HandleFunc("/api/version", s.handleVersion)
	mux.HandleFunc("/api/image", s.handleImage)
	mux.HandleFunc("/api/chat/export", s.handleExportChat)
	mux.HandleFunc("/api/chat/title", s.handleGenerateTitle)
	mux.HandleFunc("/api/models/local", s.handleLocalModels)
	mux.HandleFunc("/api/models/import", s.handleModelImport)
	mux.HandleFunc("/api/models/start", s.handleModelStart)
	mux.HandleFunc("/api/models/stop", s.handleModelStop)
	mux.HandleFunc("/api/models/status", s.handleModelStatus)
	mux.HandleFunc("/api/models/embedding/start", s.handleEmbeddingStart)
	mux.HandleFunc("/api/models/embedding/stop", s.handleEmbeddingStop)
	mux.HandleFunc("/api/models/embedding/status", s.handleEmbeddingStatus)
	mux.HandleFunc("/api/gpu", s.handleGPU)
	mux.HandleFunc("/api/models/search", s.handleModelSearch)
	mux.HandleFunc("/api/models/files", s.handleModelFiles)
	mux.HandleFunc("/api/models/download", s.handleModelDownload)
	mux.HandleFunc("/api/models/download/progress", s.handleDownloadProgress)
	mux.HandleFunc("/api/models/download/cancel", s.handleDownloadCancel)
	mux.HandleFunc("/api/models/llama/check", s.handleLlamaCheck)
	mux.HandleFunc("/api/models/llama/install", s.handleLlamaInstall)
	mux.HandleFunc("/api/models/llama/skip", s.handleLlamaSkip)
	mux.HandleFunc("/api/models/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.handleLlamaConfigGet(w, r)
		} else if r.Method == http.MethodPut {
			s.handleLlamaConfigUpdate(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/remote-access", s.handleRemoteAccess)
	mux.HandleFunc("/api/sync/settings", s.handleSyncSettings)
	mux.HandleFunc("/api/sync/auth", s.handleSyncAuth)
	mux.HandleFunc("/api/sync/account", s.handleSyncAccount)
	mux.HandleFunc("/api/sync/trigger", s.handleSyncTrigger)
	mux.HandleFunc("/api/sync/pull", s.handleSyncPull)
	mux.HandleFunc("/api/sync/now", s.handleSyncNow)
	mux.HandleFunc("/api/sync/disconnect", s.handleSyncDisconnect)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/recording/start", s.handleRecordingStart)
	mux.HandleFunc("/api/recording/stop", s.handleRecordingStop)

	// Provider management
	mux.HandleFunc("/api/providers", s.handleProviders)
	mux.HandleFunc("/api/providers/test", s.handleProviderTest)
	mux.HandleFunc("/api/providers/active", s.handleActiveProvider)

	// OpenRouter
	mux.HandleFunc("/api/openrouter/connect", s.handleOpenRouterConnect)
	mux.HandleFunc("/api/openrouter/models", s.handleOpenRouterModels)

	// Orchestra mode
	mux.HandleFunc("/api/orchestra/config", s.handleOrchestraConfig)

	// Agent mode
	mux.HandleFunc("/api/agent/chat", s.handleAgentChat)
	mux.HandleFunc("/api/agent/enabled", s.handleAgentEnabled)
	mux.HandleFunc("/api/agent/permission", s.handleAgentPermission)
	mux.HandleFunc("/api/agent/permissions", s.handleAgentPermissions)
	mux.HandleFunc("/api/agent/undo", s.handleAgentUndo)

	s.srv = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: corsMiddleware(mux),
	}
	s.port = port

	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("cannot listen on port %d: %w", port, err)
	}

	s.running = true
	go func() {
		log.Printf("Flutter API server (HTTP) started on http://127.0.0.1:%d", port)
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("Flutter server error: %v", err)
		}
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	return nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.srv == nil {
		return nil
	}

	log.Println("Stopping remote access server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := s.srv.Shutdown(ctx)
	s.running = false
	return err
}

func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Server) GetPort() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

func (s *Server) GetAddresses() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var addrs []string
	for _, ip := range s.localIPs {
		addrs = append(addrs, fmt.Sprintf("https://%s:%d", ip, s.port))
	}
	return addrs
}

// ─── Handlers ───────────────────────────────────────────────────

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	reply := s.bridge.SendMessage(req.Message)
	writeJSON(w, map[string]string{"reply": reply})
}

func (s *Server) handleSendFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(50 << 20) // 50 MB
	if err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	msg := r.FormValue("message")

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "memo_web_*_"+header.Filename)
	if err != nil {
		http.Error(w, "tmp error", http.StatusInternalServerError)
		return
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, file); err != nil {
		http.Error(w, "copy error", http.StatusInternalServerError)
		return
	}
	tmpFilePath := tmpFile.Name()
	tmpFile.Close() // Close before sending to bridge

	mimeType := header.Header.Get("Content-Type")
	isImage := false
	if mimeType != "" {
		if len(mimeType) >= 5 && mimeType[:5] == "image" {
			isImage = true
		}
	} else {
		// Use file extension guess if MIME not provided
		ext := tmpFilePath[len(tmpFilePath)-4:]
		if ext == ".png" || ext == ".jpg" || ext == "jpeg" || ext == ".gif" {
			isImage = true
		}
	}

	var reply string
	if isImage {
		reply = s.bridge.SendMessageWithImage(msg, tmpFilePath)
	} else {
		reply = s.bridge.SendMessageWithFile(msg, tmpFilePath)
	}

	writeJSON(w, map[string]string{"reply": reply})
}

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.bridge.WebListChats())
}

func (s *Server) handleNewChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	id := s.bridge.NewChat()
	writeJSON(w, map[string]string{"id": id})
}

func (s *Server) handleSwitchChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.bridge.SwitchChat(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleDeleteChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.bridge.DeleteChat(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleRenameChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.bridge.RenameChat(req.ID, req.Title); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleActiveChat(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"id": s.bridge.GetActiveChatID()})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	msgs := s.bridge.WebGetActiveMessages()
	if msgs == nil {
		writeJSON(w, []struct{}{})
		return
	}
	writeJSON(w, msgs)
}

func (s *Server) handleUpdateMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Index   int    `json:"index"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.bridge.UpdateMessage(req.Index, req.Content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Index int `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.bridge.DeleteMessage(req.Index); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"connection":   s.bridge.WebCheckConnection(),
		"memory_count": s.bridge.GetMemoryCount(),
	})
}

func (s *Server) handleIncognito(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	s.bridge.ToggleIncognito(req.Enabled)
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	audioData, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	text, err := s.bridge.TranscribeAudio(audioData)
	if err != nil {
		log.Printf("Transcribe error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"text": text})
}

// ─── Helpers ────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "http://localhost"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getLocalIPs() []string {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return []string{"localhost"}
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			ips = append(ips, ipnet.IP.String())
		}
	}
	if len(ips) == 0 {
		ips = []string{"localhost"}
	}
	return ips
}

func generateSelfSignedCert(ips []string) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Memo Local Engine"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	for _, ip := range ips {
		if parsedIP := net.ParseIP(ip); parsedIP != nil {
			template.IPAddresses = append(template.IPAddresses, parsedIP)
		}
	}
	template.DNSNames = append(template.DNSNames, "localhost")

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	return tls.X509KeyPair(certPEM, keyPEM)
}

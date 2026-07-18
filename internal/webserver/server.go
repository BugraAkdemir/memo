package webserver

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"memo/internal/logx"
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
	GetIncognito() bool
	ToggleIncognito(enabled bool)
	TranscribeAudio(audioData []byte) (string, error)

	// Client tracking (see internal/app/clients.go) — lets a backend spawned
	// on demand for a terminal session know when the last attached CLI/GUI
	// client has gone, so it can shut itself down instead of staying tied
	// to whichever process happened to start it.
	RegisterClient() string
	HeartbeatClient(clientID string) error
	UnregisterClient(clientID string)
}

type Server struct {
	mu          sync.Mutex
	srv         *http.Server
	bridge      AppBridge
	fullBridge  FullBridge
	assets      fs.FS
	running     bool
	port        int
	listenAddr  string
	localIPs    []string
	stopCleaner chan struct{}
}

func New(bridge AppBridge) *Server {
	s := &Server{
		bridge: bridge,
	}
	if fb, ok := bridge.(FullBridge); ok {
		s.fullBridge = fb
	}
	return s
}

func (s *Server) Start(port int) error {
	return s.StartHTTPWithAddr(port, "127.0.0.1")
}

// StartHTTP starts a plain HTTP server (no TLS) for local Flutter desktop communication.
func (s *Server) StartHTTP(port int) error {
	return s.StartHTTPWithAddr(port, "127.0.0.1")
}

// StartHTTPWithAddr starts HTTP server on the given address and port.
// addr: "127.0.0.1" for local-only, "0.0.0.0" for LAN access.
func (s *Server) StartHTTPWithAddr(port int, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server already running on port %d", s.port)
	}

	mux := http.NewServeMux()

	// route registers handler at pattern (e.g. "/api/foo") and, if pattern
	// starts with "/api/", also under an "/api/v1/" alias (e.g.
	// "/api/v1/foo") — the actual API versioning strategy TD-2
	// (BUG_REPORT.md) asked for. /api/ itself keeps meaning "whatever the
	// latest stable version is" and every existing client (Flutter desktop,
	// mobile, the Go CLI) keeps working completely unchanged; new clients,
	// or a client that wants to pin against breaking changes, can start
	// using /api/v1/ today. A future breaking change gets its own
	// /api/v2/... routes registered the same way, with /api/ (unversioned)
	// re-pointed at v2's handlers and /api/v1/ kept serving the old
	// behavior for as long as it's supported.
	route := func(pattern string, handler http.HandlerFunc) {
		mux.HandleFunc(pattern, handler)
		if rest, ok := strings.CutPrefix(pattern, "/api/"); ok {
			mux.HandleFunc("/api/v1/"+rest, handler)
		}
	}

	// API endpoints (original)
	route("/api/send", s.handleSend)
	route("/api/chats", s.handleChats)
	route("/api/chats/new", s.handleNewChat)
	route("/api/chats/switch", s.handleSwitchChat)
	route("/api/chats/delete", s.handleDeleteChat)
	route("/api/chats/rename", s.handleRenameChat)
	route("/api/chats/active", s.handleActiveChat)
	route("/api/messages", s.handleMessages)
	route("/api/messages/update", s.handleUpdateMessage)
	route("/api/messages/delete", s.handleDeleteMessage)
	route("/api/status", s.handleStatus)
	route("/api/incognito", s.handleIncognito)
	route("/api/transcribe", s.handleTranscribe)
	route("/api/send_file", s.handleSendFile)
	route("/api/send_file/stream", s.handleSendFileStream)

	// Flutter-specific endpoints
	route("/api/send/stream", s.handleSendStream)
	route("/api/system-prompt", s.handleSystemPrompt)
	route("/api/system-prompt/reset", s.handleResetSystemPrompt)
	route("/api/system-prompt/minimal-mode", s.handleMinimalMode)
	route("/api/system-prompt/minimal-mode/overrides", s.handleMinimalModeOverrides)
	route("/api/incognito-prompt", s.handleIncognitoPrompt)
	route("/api/memory/files", s.handleMemoryFiles)
	route("/api/memory/clear", s.handleMemoryClear)
	route("/api/memory/settings", s.handleMemorySettings)
	route("/api/memory/enabled", s.handleMemoryEnabled)
	route("/api/memory/debug-search", s.handleMemoryDebugSearch)
	route("/api/memory/explicit/save", s.handleMemoryExplicitSave)
	route("/api/memory/explicit/delete", s.handleMemoryExplicitDelete)
	route("/api/memory/import-text", s.handleMemoryImportText)
	route("/api/memory/export", s.handleMemoryExport)
	route("/api/memory/import", s.handleMemoryImport)
	route("/api/memory/stats", s.handleMemoryStats)
	route("/api/memory/search", s.handleMemoryFilteredSearch)
	route("/api/stats/usage", s.handleUsageStats)
	route("/api/version", s.handleVersion)
	route("/api/version/check", s.handleVersionCheck)
	route("/api/image", s.handleImage)
	route("/api/chat/export", s.handleExportChat)
	route("/api/chat/title", s.handleGenerateTitle)
	route("/api/models/local", s.handleLocalModels)
	route("/api/models/import", s.handleModelImport)
	route("/api/models/start", s.handleModelStart)
	route("/api/models/stop", s.handleModelStop)
	route("/api/models/status", s.handleModelStatus)
	route("/api/models/embedding/start", s.handleEmbeddingStart)
	route("/api/models/embedding/stop", s.handleEmbeddingStop)
	route("/api/models/embedding/status", s.handleEmbeddingStatus)
	route("/api/gpu", s.handleGPU)
	route("/api/models/search", s.handleModelSearch)
	route("/api/models/files", s.handleModelFiles)
	route("/api/models/download", s.handleModelDownload)
	route("/api/models/download/progress", s.handleDownloadProgress)
	route("/api/models/download/cancel", s.handleDownloadCancel)
	route("/api/models/llama/check", s.handleLlamaCheck)
	route("/api/models/llama/install", s.handleLlamaInstall)
	route("/api/models/llama/skip", s.handleLlamaSkip)
	route("/api/models/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.handleLlamaConfigGet(w, r)
		} else if r.Method == http.MethodPut {
			s.handleLlamaConfigUpdate(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	route("/api/remote-access", s.handleRemoteAccess)
	route("/api/sync/settings", s.handleSyncSettings)
	route("/api/sync/auth", s.handleSyncAuth)
	route("/api/sync/account", s.handleSyncAccount)
	route("/api/sync/trigger", s.handleSyncTrigger)
	route("/api/sync/pull", s.handleSyncPull)
	route("/api/sync/now", s.handleSyncNow)
	route("/api/sync/disconnect", s.handleSyncDisconnect)
	route("/api/events", s.handleEvents)

	// Shutdown via HTTP
	route("/api/shutdown", s.handleShutdown)

	// Client tracking (auto-shutdown when no CLI/GUI clients remain)
	route("/api/clients/register", s.handleClientRegister)
	route("/api/clients/heartbeat", s.handleClientHeartbeat)
	route("/api/clients/unregister", s.handleClientUnregister)

	// Provider management
	route("/api/providers", s.handleProviders)
	route("/api/providers/test", s.handleProviderTest)
	route("/api/providers/models", s.handleProviderModels)
	route("/api/providers/active", s.handleActiveProvider)

	// OpenRouter
	route("/api/openrouter/connect", s.handleOpenRouterConnect)
	route("/api/openrouter/models", s.handleOpenRouterModels)

	// Orchestra mode
	route("/api/orchestra/config", s.handleOrchestraConfig)

	// Proactive learning system
	route("/api/proactive/settings", s.handleProactiveSettings)
	route("/api/proactive/pending", s.handleProactivePending)
	route("/api/proactive/respond", s.handleProactiveRespond)
	route("/api/proactive/patterns", s.handleProactivePatterns)
	route("/api/proactive/patterns/forget", s.handleProactiveForget)
	route("/api/proactive/clear", s.handleProactiveClear)

	// Agent mode
	route("/api/agent/chat", s.handleAgentChat)
	route("/api/agent/enabled", s.handleAgentEnabled)
	route("/api/agent/permission", s.handleAgentPermission)
	route("/api/agent/permissions", s.handleAgentPermissions)
	route("/api/agent/auto-permission", s.handleAgentAutoPermission)
	route("/api/agent/undo", s.handleAgentUndo)

	// Task lists
	route("/api/tasklists", s.handleTaskLists)
	route("/api/tasklists/", s.handleTaskListByID)

	// WhatsApp
	route("/api/whatsapp/status", s.handleWhatsAppStatus)
	route("/api/whatsapp/start", s.handleWhatsAppStart)
	route("/api/whatsapp/stop", s.handleWhatsAppStop)
	route("/api/whatsapp/logout", s.handleWhatsAppLogout)
	route("/api/whatsapp/send", s.handleWhatsAppSend)
	route("/api/whatsapp/search", s.handleWhatsAppSearch)
	route("/api/whatsapp/chats", s.handleWhatsAppChats)
	route("/api/whatsapp/messages", s.handleWhatsAppMessages)
	route("/api/whatsapp/avatar", s.handleWhatsAppAvatar)
	route("/api/websearch", s.handleWebSearchSettings)
	route("/api/whatsapp/stats", s.handleWhatsAppStats)
	route("/api/whatsapp/chat-mode", s.handleWhatsAppChatMode)
	route("/api/whatsapp/chat-stream", s.handleWhatsAppChatStream)
	route("/api/export", s.handleExport)

	// Skills
	route("/api/skills/list", s.handleListSkills)
	route("/api/skills/install", s.handleInstallSkill)
	route("/api/skills/remove/{name}", s.handleRemoveSkill)
	route("/api/skills/get/{name}", s.handleGetSkill)
	route("/api/skills/active", s.handleSetActiveSkills)
	route("/api/skills/active-list", s.handleGetActiveSkills)
	route("/api/import", s.handleImport)
	route("/api/wipe", s.handleWipe)
	route("/api/cli/remove", s.handleCLIRemove)
	route("/api/cli/reinstall", s.handleCLIReinstall)
	route("/api/uninstall", s.handleUninstall)

	// Calendar
	route("/api/calendar/events", s.handleCalendarEvents)
	route("/api/calendar/events/", s.handleCalendarEvent)
	route("/api/calendar/settings", s.handleCalendarSettings)

	// Learning settings
	route("/api/learning/settings", s.handleLearningSettings)

	// Routines (scheduled automations)
	route("/api/routines", s.handleRoutines)
	route("/api/routines/parse", s.handleParseRoutine)
	route("/api/routines/", s.handleRoutine)
	route("/api/routines/mobile-ready", s.handleRoutinesMobileReady)

	// Mood
	route("/api/mood/score", s.handleMoodScore)
	route("/api/mood/settings", s.handleMoodSettings)
	route("/api/mood/self-interest", s.handleSelfInterestSettings)
	route("/api/mood/system-management", s.handleSystemManagementSettings)

	if s.stopCleaner != nil {
		close(s.stopCleaner)
	}
	s.stopCleaner = make(chan struct{})
	handler := limitBodyMiddleware(rateLimitMiddleware(s.stopCleaner, corsMiddleware(s.remoteAuthMiddleware(mux))), 50<<20) // 50 MB body limit
	s.srv = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", addr, port),
		Handler: handler,
	}
	s.port = port
	s.listenAddr = addr

	// Detect local IPs for LAN access display
	s.localIPs = nil
	if addr == "0.0.0.0" {
		ifaces, _ := net.Interfaces()
		for _, iface := range ifaces {
			addrs, _ := iface.Addrs()
			for _, a := range addrs {
				if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
					s.localIPs = append(s.localIPs, ipnet.IP.String())
				}
			}
		}
	}

	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("cannot listen on port %d: %w", port, err)
	}

	s.running = true
	var addrLog = addr
	if addr == "0.0.0.0" {
		addrLog = "0.0.0.0 (LAN accessible)"
	}
	go func() {
		logx.Info("Flutter API server (HTTP) started", "addr", fmt.Sprintf("http://%s:%d", addrLog, port))
		for _, ip := range s.localIPs {
			logx.Info("LAN address available", "ip", ip, "port", port)
		}
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			logx.Error("Flutter server error", "err", err)
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

	logx.Info("Stopping server...")
	srv := s.srv
	s.running = false
	if s.stopCleaner != nil {
		close(s.stopCleaner)
		s.stopCleaner = nil
	}
	go srv.Shutdown(context.Background())
	return nil
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
	if s.listenAddr == "127.0.0.1" {
		return []string{fmt.Sprintf("http://127.0.0.1:%d", s.port)}
	}
	var addrs []string
	for _, ip := range s.localIPs {
		addrs = append(addrs, fmt.Sprintf("http://%s:%d", ip, s.port))
	}
	return addrs
}

func (s *Server) GetListenAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listenAddr
}

func (s *Server) SetListenAddr(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listenAddr = addr
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
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, file); err != nil {
		http.Error(w, "copy error", http.StatusInternalServerError)
		return
	}
	tmpFilePath := tmpFile.Name()
	tmpFile.Close() // Close before sending to bridge

	isImage := detectIsImageFile(tmpFilePath, header.Filename)

	var reply string
	if isImage {
		reply = s.bridge.SendMessageWithImage(msg, tmpFilePath)
	} else {
		reply = s.bridge.SendMessageWithFile(msg, tmpFilePath)
	}

	writeJSON(w, map[string]string{"reply": reply})
}

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
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
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]string{"id": s.bridge.GetActiveChatID()})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
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
		"port":         s.port,
		"listen_addr":  s.listenAddr,
	})
}

func (s *Server) handleIncognito(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]bool{"enabled": s.bridge.GetIncognito()})
	case http.MethodPost:
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		s.bridge.ToggleIncognito(req.Enabled)
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
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
		logx.Error("Transcribe error", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"text": text})
}

// ─── Helpers ────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logx.Error("writeJSON error", "err", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Only allow loopback origins. Even in LAN mode (0.0.0.0) we do not
		// reflect arbitrary origins, because that would let malicious websites
		// talk to the local Memo API from the user's browser.
		isLoopback := origin == "" ||
			strings.HasPrefix(origin, "http://localhost") ||
			strings.HasPrefix(origin, "http://127.0.0.1") ||
			strings.HasPrefix(origin, "http://[::1]")
		if isLoopback {
			if origin == "" {
				origin = "http://localhost"
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Memo-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// remoteAuthOK reports whether a request may proceed given the server's
// current listen address and the configured remote-access token. Pulled out
// as a pure function (no *Server/FullBridge dependency) so the token-check
// logic is directly unit-testable without a full FullBridge mock.
//
// listenAddr != "0.0.0.0" (local-only mode) always passes — every request is
// let through exactly as before BUG-C1 was fixed. Once bound to "0.0.0.0"
// (LAN mode, or ngrok/Tailscale which target this same port), a valid
// X-Memo-Token — or "Authorization: Bearer <token>" — is required. A
// constant-time comparison avoids leaking the token's value through
// response-time differences.
func remoteAuthOK(listenAddr, wantToken string, r *http.Request) bool {
	if listenAddr != "0.0.0.0" {
		return true
	}

	got := r.Header.Get("X-Memo-Token")
	if got == "" {
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			got = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if wantToken == "" || got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(wantToken), []byte(got)) == 1
}

// remoteAuthMiddleware wires remoteAuthOK into the request chain.
// SetRemoteAccess/SetNgrokMode/SetTailscaleMode all rebind the listener to
// "0.0.0.0" when enabling, and back to "127.0.0.1" when disabling, so a
// single listenAddr check covers LAN mode and ngrok alike. The token itself
// was already generated and shown in the UI (GetRemoteAccessStatus) before
// this fix; it just wasn't checked anywhere.
func (s *Server) remoteAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.fullBridge == nil {
			next.ServeHTTP(w, r)
			return
		}
		s.mu.Lock()
		listenAddr := s.listenAddr
		s.mu.Unlock()
		if !remoteAuthOK(listenAddr, s.fullBridge.GetRemoteAccessToken(), r) {
			http.Error(w, "unauthorized: missing or invalid X-Memo-Token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// limitBodyMiddleware limits request body size to prevent DoS.
// File upload handlers that need larger limits should skip this or set their own.
func limitBodyMiddleware(next http.Handler, maxBytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware limits requests to 100/second per IP using a simple
// token bucket. Excess requests get 429 Too Many Requests.
// stop is closed when the server stops to terminate the bucket cleaner goroutine.
func rateLimitMiddleware(stop <-chan struct{}, next http.Handler) http.Handler {
	type bucket struct {
		tokens   float64
		lastSeen time.Time
	}
	var (
		mu      sync.Mutex
		buckets = make(map[string]*bucket)
	)
	const (
		rate       = 100.0 // tokens per second
		burst      = 200.0 // max burst
		cleanEvery = 60 * time.Second
	)
	go func() {
		ticker := time.NewTicker(cleanEvery)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				mu.Lock()
				now := time.Now()
				for ip, b := range buckets {
					if now.Sub(b.lastSeen) > 5*time.Minute {
						delete(buckets, ip)
					}
				}
				mu.Unlock()
			}
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.RemoteAddr includes the ephemeral source port (e.g.
		// "127.0.0.1:54321"), which is different for every new TCP
		// connection — keying the bucket map on the full RemoteAddr gave
		// every single connection its own fresh bucket, so the 100 req/s
		// limit was trivially bypassed by opening new connections (which
		// e.g. a browser or a simple retry loop does routinely) rather than
		// reusing one. Strip the port so the bucket is actually per-IP.
		ip := r.RemoteAddr
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			ip = host
		}
		// Do NOT trust X-Forwarded-For without a trusted proxy configuration —
		// on LAN deployments an attacker can rotate spoofed IPs to bypass rate
		// limiting entirely.

		mu.Lock()
		b, ok := buckets[ip]
		if !ok {
			b = &bucket{tokens: burst, lastSeen: time.Now()}
			buckets[ip] = b
		}
		now := time.Now()
		elapsed := now.Sub(b.lastSeen).Seconds()
		b.lastSeen = now
		b.tokens = min(burst, b.tokens+elapsed*rate) - 1
		allowed := b.tokens >= 0
		mu.Unlock()

		if !allowed {
			http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
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

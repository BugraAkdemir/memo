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
	"math/big"
	"net"
	"net/http"
	"net/url"
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
	// fs.Sub can only fail if "webapp" isn't a valid subtree of the embedded
	// FS — a build-time mistake (a typo in the //go:embed directive or a
	// missing file), not a runtime condition, so panicking here rather than
	// threading an error through New()'s (currently error-free) signature
	// is the right failure mode.
	webAssets, err := fs.Sub(webAppFS, "webapp")
	if err != nil {
		panic(fmt.Sprintf("webserver: embedded web app assets: %v", err))
	}
	s := &Server{
		bridge: bridge,
		assets: webAssets,
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
	route("/api/onboarding", s.handleOnboarding)
	route("/api/transcribe", s.handleTranscribe)
	route("/api/send_file", s.handleSendFile)
	route("/api/send_file/stream", s.handleSendFileStream)

	// Flutter-specific endpoints
	route("/api/send/stream", s.handleSendStream)
	route("/api/system-prompt", s.handleSystemPrompt)
	route("/api/system-prompt/reset", s.handleResetSystemPrompt)
	route("/api/system-prompt/ui-language", s.handleUILanguage)
	route("/api/system-prompt/minimal-mode", s.handleMinimalMode)
	route("/api/system-prompt/minimal-mode/overrides", s.handleMinimalModeOverrides)
	route("/api/incognito-prompt", s.handleIncognitoPrompt)
	route("/api/memory/files", s.handleMemoryFiles)
	route("/api/memory/clear", s.handleMemoryClear)
	route("/api/memory/settings", s.handleMemorySettings)
	route("/api/memory/enabled", s.handleMemoryEnabled)
	route("/api/memory/dream/settings", s.handleMemoryDreamSettings)
	route("/api/memory/dream/run", s.handleMemoryDreamRun)
	route("/api/memory/debug-search", s.handleMemoryDebugSearch)
	route("/api/memory/explicit/save", s.handleMemoryExplicitSave)
	route("/api/memory/explicit/delete", s.handleMemoryExplicitDelete)
	route("/api/memory/import-text", s.handleMemoryImportText)
	route("/api/tts/synthesize", s.handleTTSSynthesize)
	route("/api/tts/filler", s.handleTTSFiller)
	route("/api/memory/insight", s.handleMemoryInsight)
	// Swarm (Memo Swarm — multi-machine llama.cpp RPC; Beta-gated in App)
	route("/api/swarm/status", s.handleSwarmStatus)
	route("/api/swarm/host/create", s.handleSwarmHostCreate)
	route("/api/swarm/host/workers/add", s.handleSwarmAddWorker)
	route("/api/swarm/host/workers/remove", s.handleSwarmRemoveWorker)
	route("/api/swarm/host/workers/reorder", s.handleSwarmReorderWorkers)
	route("/api/swarm/host/workers/share", s.handleSwarmSetShare)
	route("/api/swarm/host/start", s.handleSwarmStart)
	route("/api/swarm/host/stop", s.handleSwarmStop)
	route("/api/swarm/host/close", s.handleSwarmClose)
	route("/api/swarm/join", s.handleSwarmJoin)
	route("/api/swarm/leave", s.handleSwarmLeave)
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
	route("/api/remote-access/devices", s.handleRemoteDevices)
	route("/api/remote-access/devices/{id}", s.handleRemoteDeviceByID)
	route("/api/auth/login", s.handleRemoteLogin)

	// Multi-account / role model (Faz 5.1, yapacam.md)
	route("/api/setup/status", s.handleSetupStatus)
	route("/api/setup/create-admin", s.handleSetupCreateAdmin)
	route("/api/setup/create-device", s.handleSetupCreateDevice)
	route("/api/accounts", s.handleAccounts)
	route("/api/accounts/{id}", s.handleAccountByID)
	route("/api/accounts/{id}/password", s.handleAccountByID)
	route("/api/cli/status", s.handleCLIStatus)
	route("/api/cli/running", s.handleCLIRunning)
	route("/api/cli/commands", s.handleCLICommands)
	route("/api/files/mentions", s.handleFileMentions)
	route("/api/files/browse", s.handleFileBrowse)
	route("/api/chats/cli-provider", s.handleChatCLIProvider)
	route("/api/chats/cli-workdir", s.handleChatCLIWorkdir)
	route("/api/chats/cli-model", s.handleChatCLIModel)
	route("/api/cli/model-options", s.handleCLIModelOptions)
	route("/api/send/cli-stream", s.handleSendCLIStream)
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
	route("/api/providers/effort-levels", s.handleProviderEffortLevels)

	// TTS provider management (Faz 2)
	route("/api/tts/providers", s.handleTTSProviders)
	route("/api/tts/providers/test", s.handleTTSProviderTest)

	// TTS local voice store (Faz 2.6)
	route("/api/tts/voices", s.handleTTSVoices)
	route("/api/tts/voices/download", s.handleTTSVoiceDownload)
	route("/api/tts/voices/select", s.handleTTSVoiceSelect)

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
	route("/api/routines/sync-offset", s.handleRoutinesSyncOffset)

	// Dev gateway (Settings > Developer): local Anthropic/OpenAI-compatible
	// API surface. /v1/messages and /v1/chat/completions (and /v1/models) are
	// NOT under /api/ — they must match Anthropic's and OpenAI's real paths
	// exactly so pointing a client's ANTHROPIC_BASE_URL/OPENAI_BASE_URL at
	// Memo works unmodified.
	route("/api/dev-gateway/config", s.handleDevGatewayConfig)
	route("/api/dev-gateway/models", s.handleDevGatewayModels)
	route("/api/dev-gateway/logs", s.handleDevGatewayLogs)
	route("/api/dev-gateway/claude-code-cli", s.handleClaudeCodeCLIConnection)
	mux.HandleFunc("/v1/messages", s.handleAnthropicMessages)
	mux.HandleFunc("/v1/chat/completions", s.handleOpenAIChatCompletions)
	mux.HandleFunc("/v1/models", s.handleOpenAIModels)

	// Mood
	route("/api/mood/score", s.handleMoodScore)
	route("/api/mood/settings", s.handleMoodSettings)
	route("/api/mood/self-interest", s.handleSelfInterestSettings)
	route("/api/mood/system-management", s.handleSystemManagementSettings)

	// Minimal browser client (index.html/app.js/style.css) — a catch-all,
	// lowest-priority route: Go's ServeMux always prefers the more specific
	// "/api/..." registrations above over this "/", so it can never shadow
	// an API endpoint no matter the registration order.
	mux.Handle("/", http.HandlerFunc(s.handleWebUIAssets))

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
		s.localIPs = getLocalIPs()
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
		defer logx.Recover("webserver.Server/HTTP serve")
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
	// Unlock before blocking Shutdown so Serve's exit path can take s.mu.
	s.mu.Unlock()
	// Wait for Shutdown so the port is free before a follow-up
	// StartHTTPWithAddr (e.g. SetRemoteAccess rebind for Swarm LAN). Fire-
	// and-forget Shutdown raced Listen and caused intermittent "address
	// already in use" on create-room.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logx.Printf("server shutdown: %v", err)
	}
	s.mu.Lock()
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

// handleWebUIAssets serves the embedded web app (webAppFS — the same
// Flutter app as the desktop build, compiled for web, see webapp.go) for
// a bare "/" and every asset path under it, with Cache-Control: no-store
// on every response. Plain http.FileServer sends no caching headers at
// all by default, so a browser that already had this page open (or just
// visited it before) can silently keep serving its own cached copy
// indefinitely, never even asking the server again, long after a
// `memo service update`/binary swap actually shipped new ones. Found
// live, against the previous hand-rolled webui this replaced: a user
// re-ran the install script, CI confirmed the new build was genuinely
// running, and they still saw the old page until a hard refresh — the
// server was serving the new files correctly the whole time, the browser
// just never asked. Flutter's own flutter_service_worker.js does real
// content-hash-based caching for the app's assets, but that mechanism
// itself depends on the browser actually re-fetching index.html/
// flutter_bootstrap.js to learn a new version exists — no-store here
// guarantees that entry point is never the thing silently going stale.
func (s *Server) handleWebUIAssets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.FileServer(http.FS(s.assets)).ServeHTTP(w, r)
}

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

// isLoopbackOrigin reports whether origin is exactly http://localhost,
// http://127.0.0.1, or http://[::1] (any port) — parsed and compared by
// exact hostname, never a substring/prefix match. A prefix check
// (the previous implementation: strings.HasPrefix(origin, "http://localhost"))
// is a classic CORS bypass (CWE-346): an attacker who registers
// "localhost.attacker.com" — a perfectly ordinary subdomain of a domain
// they own — sends Origin: http://localhost.attacker.com, which starts
// with "http://localhost" as a raw string despite being a completely
// different, attacker-controlled host. Combined with a credential-less
// endpoint like POST /api/auth/login, that let an external attacker's
// webpage use a LAN-connected victim's browser as a relay to reach and
// read responses from Memo instances the attacker has no direct network
// path to (classic browser-as-LAN-pivot). Parsing the URL and checking
// u.Hostname() exactly closes this: "localhost.attacker.com" parses to
// hostname "localhost.attacker.com", never "localhost".
func isLoopbackOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Allow loopback origins (localhost/127.0.0.1/::1 — the desktop app
		// and a locally-opened web UI) AND origins that exactly match the
		// address this request was addressed to (r.Host). The second rule
		// is what makes the embedded web UI work when the user opens it via
		// its LAN address: the page's own origin is then
		// http://192.168.1.x:8090, and every same-server call from it
		// carries that origin with Host: 192.168.1.x:8090. Reflecting the
		// origin only when it equals the request's own Host is still safe
		// against the CWE-346 browser-as-LAN-pivot attack: a malicious
		// page's Origin is set by the browser to the attacker's own origin,
		// which can never equal the Host of a request aimed at Memo — so
		// arbitrary websites stay blocked, exactly as before.
		allow := isLoopbackOrigin(origin) || isSameHostOrigin(origin, r.Host)
		if allow {
			if origin == "" {
				origin = "http://localhost"
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
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

// isSameHostOrigin reports whether origin is exactly the host:port this
// request was addressed to (the Host header) — i.e. a page served by Memo
// itself calling its own address. This is the "the embedded web app talking
// to the backend that served it" case; browsers treat such requests as
// same-origin when the wording matches exactly and need no CORS at all, but
// the same page calling the same backend under a differently-worded alias
// (http://localhost:8090 vs http://192.168.1.106:8090, or a hostname) is
// cross-origin and does need the header. Compared by exact host:port —
// never a prefix/substring match, for the same CWE-346 reason documented on
// isLoopbackOrigin.
func isSameHostOrigin(origin, host string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" {
		return false
	}
	return strings.EqualFold(u.Host, host)
}

// remoteCredential extracts the caller-presented credential — device token
// or session token alike, both travel the same two headers — from r.
func remoteCredential(r *http.Request) string {
	got := r.Header.Get("X-Memo-Token")
	if got == "" {
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			got = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	return got
}

// remoteAuthOK reports whether a request may proceed given the server's
// current listen address and configured auth mode (Faz 2, yapacam.md).
// Pulled out as a pure function (verifyDevice/validateSession passed in as
// closures rather than a *Server/FullBridge dependency) so the mode-gating
// logic is directly unit-testable without a full FullBridge mock.
//
// listenAddr != "0.0.0.0" (local-only mode) always passes — every request is
// let through exactly as before BUG-C1 was fixed, regardless of mode.
//
//   - "none":  always passes (once bound to 0.0.0.0) — the deliberately
//     insecure opt-in; callers must surface a loud warning elsewhere
//     (see warnAuthDisabled), this function only implements the gate.
//   - "token": verifyDevice only — a device token from CreateRemoteDevice.
//   - "password": validateSession only — a session token from
//     POST /api/auth/login. A device token, even a genuinely valid one,
//     does NOT satisfy this mode: the whole point of choosing
//     password-only is that pre-shared device tokens aren't the account's
//     credential anymore.
//   - "token_password": either satisfies (OR logic, per yapacam.md).
//   - anything else (shouldn't happen — validate() defaults empty to
//     "token"): fails closed.
func remoteAuthOK(listenAddr, mode string, r *http.Request, verifyDevice, validateSession func(string) bool) bool {
	if listenAddr != "0.0.0.0" {
		return true
	}
	if mode == "none" {
		return true
	}
	// Loopback requests need no credential regardless of mode: traffic
	// from 127.0.0.1/::1 can only come from software on this machine (the
	// installed desktop app talks to 127.0.0.1 exclusively), and the gate
	// exists to keep *remote* callers out — loopback is exactly as trusted
	// as the unauthenticated local-only bind. A LAN-IP client on the same
	// box ("192.168.1.xxx" bounced back locally) still gets gated, since
	// RemoteAddr then carries the interface's non-loopback address.
	//
	// Except: a local reverse proxy or tunnel (cloudflared, ngrok, nginx,
	// ssh -L) forwarding public/LAN traffic to this same loopback address
	// produces a connection that is *also*, genuinely, RemoteAddr==127.0.0.1
	// — indistinguishable from the real desktop client by IP alone. Found
	// live: a Cloudflare Tunnel ingress rule of `service: http://localhost:
	// 8090` (an ordinary, commonly-suggested way to self-host) let anyone
	// on the public internet reach every /api/ route with zero credential,
	// completely bypassing password/token auth. isForwardedRequest's
	// comment explains why checking for proxy-forwarding headers here is
	// safe to use as a "don't trust this" signal.
	if isLoopbackIP(requestIP(r)) && !isForwardedRequest(r) {
		return true
	}
	cred := remoteCredential(r)
	if cred == "" {
		return false
	}
	switch mode {
	case "token":
		return verifyDevice(cred)
	case "password":
		return validateSession(cred)
	case "token_password":
		return verifyDevice(cred) || validateSession(cred)
	default:
		return false
	}
}

// remoteAuthMiddleware wires remoteAuthOK into the request chain.
// SetRemoteAccess/SetNgrokMode/SetTailscaleMode all rebind the listener to
// "0.0.0.0" when enabling, and back to "127.0.0.1" when disabling, so a
// single listenAddr check covers LAN mode and ngrok alike.
func (s *Server) remoteAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.fullBridge == nil {
			next.ServeHTTP(w, r)
			return
		}
		// Worker join registration: a joining machine has no X-Memo-Token —
		// auth is the room id+secret pair checked inside HostSwarmAddWorker.
		// Both /api/ and /api/v1/ aliases must be skipped (route() registers both).
		if isSwarmWorkerAddPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		// Password login itself can't require a credential — that's the
		// whole point of the endpoint. Rate-limited independently by
		// internal/remoteauth's brute-force limiter inside LoginRemotePassword,
		// not by this check.
		if isRemoteLoginPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		// First-run bootstrap (Faz 5.1, yapacam.md): same reasoning as the
		// login path above — a client checking "is setup needed" or
		// creating the very first account has no credential to present yet.
		if isSetupBootstrapPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		// Minimal web UI's static assets (index.html/app.js/style.css,
		// served off the "/" catch-all) are deliberately unauthenticated
		// even in LAN mode — the page has to load before it can show a
		// token-entry screen at all. No secrets live in these files; every
		// actual data call the page makes still goes through /api/... and
		// stays fully gated below. Every registered route in this server is
		// under /api/ except that one catch-all, so this check is exact,
		// not a heuristic.
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		s.mu.Lock()
		listenAddr := s.listenAddr
		s.mu.Unlock()
		mode := s.fullBridge.GetRemoteAuthMode()
		if !remoteAuthOK(listenAddr, mode, r, s.fullBridge.VerifyRemoteDeviceToken, s.fullBridge.ValidateRemoteSession) {
			http.Error(w, "unauthorized: missing or invalid credential", http.StatusUnauthorized)
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
		defer logx.Recover("webserver rate limiter cleanup")
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

// virtualInterfacePrefixes lists interface name prefixes that are very
// unlikely to be the physical LAN link a phone/desktop client would
// actually use to reach this box: Docker/Podman bridges and veth pairs,
// libvirt's virbr0, VPN tun/tap devices, and common container-CNI bridges
// (Flannel, Calico, Kubernetes' own kube-bridge). BUG-ONB1: with no
// filtering at all, a box running Docker showed 3-4 "LAN address
// available" lines (the real 192.168.x.x alongside 172.17-19.x.x Docker
// bridge IPs) with nothing distinguishing which one the user should
// actually type into a browser — an average user could easily try a
// bridge IP first and get a connection failure.
var virtualInterfacePrefixes = []string{
	"docker", "br-", "veth", "virbr", "tun", "tap",
	"podman", "cni", "flannel", "kube-bridge", "cali",
}

func isVirtualNetworkInterface(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range virtualInterfacePrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// getLocalIPs enumerates this machine's non-loopback IPv4 addresses,
// excluding virtual/container network interfaces (see
// isVirtualNetworkInterface) so LAN-access display (the desktop Settings
// UI, `memo remote status`, and the self-hosted install scripts) surfaces
// only addresses another device on the same physical network could
// actually reach.
func getLocalIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if isVirtualNetworkInterface(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
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

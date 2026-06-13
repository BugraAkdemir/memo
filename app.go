package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"memo/internal/agent"
	"memo/internal/agent/tools"
	"memo/internal/api"
	"memo/internal/cloudsync"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/llama"
	"memo/internal/memory"
	"memo/internal/modelstore"
	"memo/internal/ngrok"
	"memo/internal/orchestra"
	"memo/internal/provider"
	"memo/internal/sessions"
	"memo/internal/webserver"
	"memo/internal/whatsapp"
)

//go:embed binaries/*
var embeddedBinaries embed.FS

//go:embed version
var versionBytes []byte

func (a *App) GetVersion() string {
	return strings.TrimSpace(string(versionBytes))
}

// CheckLatestVersion checks the remote version at version-zeta.vercel.app/version.json
// and returns the latest version string if newer, or empty string if up-to-date.
func (a *App) CheckLatestVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://version-zeta.vercel.app/version.json", nil)
	if err != nil {
		return "", fmt.Errorf("version check request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("version check http: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("version check decode: %w", err)
	}

	current := a.GetVersion()
	latest := strings.TrimSpace(body.Version)

	// Compare versions — if latest is different and not empty, notify
	if latest == "" || latest == current {
		return "", nil
	}
	return latest, nil
}

type ConnectionStatus struct {
	Connected bool     `json:"connected"`
	Models    []string `json:"models"`
	Error     string   `json:"error,omitempty"`
}

type SyncAccount struct {
	Authenticated bool   `json:"authenticated"`
	Name          string `json:"name,omitempty"`
	Email         string `json:"email,omitempty"`
}

type saveTask struct {
	userMsg string
	reply   string
}

// AppEvent represents a background notification for the UI.
type AppEvent struct {
	Name string `json:"name"`
	Data string `json:"data,omitempty"`
}

// eventRing is a fixed-size ring buffer of recent events.
type eventRing struct {
	mu   sync.Mutex
	buf  [64]AppEvent
	pos  int // next write position
	full bool
}

func (r *eventRing) push(e AppEvent) {
	r.mu.Lock()
	r.buf[r.pos] = e
	r.pos = (r.pos + 1) % len(r.buf)
	if r.pos == 0 {
		r.full = true
	}
	r.mu.Unlock()
}

func (r *eventRing) snapshot() []AppEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := len(r.buf)
	if !r.full {
		n = r.pos
	}
	out := make([]AppEvent, n)
	if r.full {
		copy(out, r.buf[r.pos:])
		copy(out[len(r.buf)-r.pos:], r.buf[:r.pos])
	} else {
		copy(out, r.buf[:r.pos])
	}
	return out
}

type App struct {
	ctx               context.Context
	client            *api.Client
	clientMu          sync.RWMutex // protects client and embeddingClient reassignment
	store             *memory.Store
	storeMu           sync.RWMutex
	identity          *identity.Identity
	cfg               *config.AppConfig
	sessions          *sessions.Manager
	incognitoMu       sync.RWMutex
	isIncognito       bool
	incognitoMessages []api.Message
	sttServer         *exec.Cmd
	webServer         *webserver.Server
	modelStore        *modelstore.Store

	waClient         *whatsapp.Client
	waMsgStore       *whatsapp.Store
	llamaServer      *llama.Server
	llamaEmbedServer *llama.Server // dedicated embedding model server
	llamaInstaller   *llama.Installer
	originalBaseURL  string      // stores the original API base URL before llama override
	embeddingClient  *api.Client // separate client for embedding server
	syncManager      *cloudsync.Manager
	syncMu           sync.RWMutex
	memorySaveCh     chan saveTask
	events           *eventRing

	providerCfgMgr *provider.ConfigManager
	providerRouter *provider.Router
	activeProvider provider.ProviderType // which provider is currently active

	orchestraConductor *orchestra.Conductor

	agentExecutor *agent.Executor
	agentEnabled  bool
	agentMu       sync.RWMutex

	providerMu     sync.RWMutex // protects providerRouter, providerCfgMgr, activeProvider
	sessionsMu     sync.RWMutex // protects sessions

	remoteAccessEnabled bool
	ngrokServer        *ngrok.Manager

	whatsappChatMode bool
	whatsappChatMu   sync.RWMutex
}

func NewApp() *App {
	return &App{events: &eventRing{}}
}

// loadDotEnv reads a .env file and sets any unset environment variables from it.
// Lines starting with # are ignored. Format: KEY=VALUE (no export keyword needed).
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // .env is optional
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Only set if not already set by the real environment.
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

func (a *App) emitEvent(name string, data ...interface{}) {
	var dataStr string
	if len(data) > 0 {
		dataStr = fmt.Sprint(data...)
	}
	a.events.push(AppEvent{Name: name, Data: dataStr})
	log.Printf("event: %s — %s", name, dataStr)
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Load .env before anything else so credentials are available via os.Getenv.
	loadDotEnv(".env")

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Printf("WARN: config: %v", err)
		a.emitEvent("config_load_error", err.Error())
		cfg = config.Default()
	}
	a.cfg = cfg
	a.originalBaseURL = cfg.API.BaseURL
	a.clientMu.Lock()
	a.client = api.NewClient(cfg.API.BaseURL, cfg.API.TimeoutSeconds)
	a.clientMu.Unlock()

	a.clientMu.RLock()
	initClient := a.client
	a.clientMu.RUnlock()
	embeddingFunc := memory.NewEmbeddingFunc(initClient, cfg.API.EmbeddingModel)

	// Memory store initialization in background (vec0 extension loading is slow)
	go func() {
		store, err := memory.NewStore(memory.StoreConfig{
			Dir:           cfg.Memory.PersistDir,
			Dimension:     cfg.Memory.EmbeddingDimension,
			EmbeddingFunc: embeddingFunc,
		})
		if err != nil {
			log.Printf("WARN: memory: %v", err)
			a.emitEvent("memory_store_error", err.Error())
			return
		}
		a.storeMu.Lock()
		a.store = store
		a.storeMu.Unlock()
		log.Println("Memory store ready")
	}()

	a.identity = identity.New(cfg.Identity.UserName, cfg.Identity.AssistantName, cfg.Identity.Style, cfg.Identity.SystemRole)

	sm, err := sessions.NewManager("data/sessions")
	if err != nil {
		log.Printf("WARN: sessions: %v", err)
		a.emitEvent("sessions_manager_error", err.Error())
	}
	a.sessions = sm

	// Initialize model store
	a.modelStore = modelstore.New(cfg.Llama.ModelsDir)

	// Initialize llama server managers and installer
	a.llamaServer = llama.NewServer(cfg.Llama.Port, cfg.Llama.CtxSize)
	a.llamaEmbedServer = llama.NewServer(cfg.Llama.EmbeddingPort, 512) // embedding models need minimal context
	a.llamaInstaller = llama.NewInstaller("data")

	// Check embedding health in background.
	// Removed: Since we are using internal models, they are started manually.
	// Running a health check here will falsely report an error.

	// Start memory save worker (channel-based, single goroutine — no lock contention)
	a.memorySaveCh = make(chan saveTask, 64)
	go a.memorySaveWorker()

	// Start STT server in background (DISABLED due to Vosk crashes)
	// go a.startSTTServer()

	// Remote access via ngrok
	if cfg.RemoteAccess.Enabled && cfg.RemoteAccess.NgrokMode && cfg.RemoteAccess.NgrokToken != "" {
		a.remoteAccessEnabled = true
		binPath, err := ngrok.Install("data")
		if err != nil {
			log.Printf("[ngrok] Install error: %v", err)
		} else {
			mgr := ngrok.NewManager(binPath)
			if err := mgr.Start(cfg.RemoteAccess.Port, cfg.RemoteAccess.NgrokToken); err != nil {
				log.Printf("[ngrok] Start error: %v", err)
			} else {
				a.ngrokServer = mgr
			}
		}
	}

	// Cross-mode: if memory is enabled and embedding model is configured,
	// auto-download (if needed) and auto-start the embedding server.
	// This lets API providers handle chat while a tiny local model handles embeddings.
	if cfg.Memory.MemoryEnabled && cfg.Memory.EmbeddingAutoStart && cfg.Memory.EmbeddingModelRepo != "" && cfg.Memory.EmbeddingModelFile != "" && !a.llamaEmbedServer.IsRunning() {
		go a.startupEmbeddingModel()
	}

	// Initialize WhatsApp integration
	if cfg.WhatsApp.Enabled {
		a.initWhatsApp()
	}

	// Initialize provider system
	a.providerCfgMgr = provider.NewConfigManager("data/providers.json", nil)
	configs := a.providerCfgMgr.GetEnabled()
	if len(configs) > 0 {
		a.providerRouter = provider.NewRouter(configs)
		// Start health check goroutine for auto-recovery
		go a.providerRouter.HealthCheck(ctx, 5*time.Minute)
		log.Printf("Provider system initialized with %d enabled provider(s)", len(configs))
		for _, cfg := range configs {
			log.Printf("  - %s (%s)", cfg.Type, cfg.Model)
		}
	} else {
		log.Println("No external providers configured, using local models")
	}

	// activeProvider starts empty — user must explicitly select from UI
	a.activeProvider = ""

	// Initialize orchestra conductor
	orchestraCfg := orchestra.LoadConfig("data/orchestra.json")
		a.orchestraConductor = orchestra.NewConductor(
		orchestraCfg,
		func(cfg provider.ProviderConfig) (provider.Provider, error) {
			if a.providerRouter == nil {
				return nil, fmt.Errorf("provider router not initialized, cannot create %s/%s", cfg.Type, cfg.Model)
			}
			p, ok := a.providerRouter.GetProvider(cfg.Type)
			if !ok {
				return nil, fmt.Errorf("provider %s not found in router (disabled or not configured), enable it in API Providers", cfg.Type)
			}
			return p, nil
		},
		func() []provider.ProviderConfig {
			if a.providerCfgMgr == nil {
				return nil
			}
			return a.providerCfgMgr.GetAll()
		},
	)
	log.Printf("Orchestra mode initialized (enabled=%v)", orchestraCfg.Enabled)

	// Initialize Agent Executor
	basePath, _ := filepath.Abs(".")
	a.agentExecutor = agent.NewExecutor(basePath, a.providerRouter, a.providerCfgMgr)
	a.agentEnabled = false // Default disabled
	log.Printf("Agent mode initialized (enabled=false)")

	log.Println("Memo ready")
}

// startWebServerHTTP starts a plain HTTP API server for the Flutter desktop frontend.
func (a *App) startWebServerHTTP(port int) {
	addr := "127.0.0.1"
	if a.remoteAccessEnabled {
		addr = "0.0.0.0"
	}
	a.webServer = webserver.New(a)
	if err := a.webServer.StartHTTPWithAddr(port, addr); err != nil {
		log.Printf("Flutter server: %v", err)
	}
}

// startWebServer starts a TLS server for remote access.
func (a *App) startWebServer(port int) {
	if a.webServer == nil {
		a.webServer = webserver.New(a)
	}
	if err := a.webServer.Start(port); err != nil {
		log.Printf("Remote access server: %v", err)
	}
}

func (a *App) startSTTServer() {
	var binName string
	if runtime.GOOS == "windows" {
		binName = "stt_server_windows.exe"
	} else if runtime.GOOS == "linux" {
		binName = "stt_server_linux"
	} else {
		log.Printf("STT disabled: OS %s not specifically supported for bundled binary yet.", runtime.GOOS)
		return
	}

	binData, err := embeddedBinaries.ReadFile("binaries/" + binName)
	if err != nil {
		log.Printf("STT: embedded binary %s not found in build. STT disabled.", binName)
		return
	}

	tempPath := filepath.Join(os.TempDir(), "memo_stt_server")
	if runtime.GOOS == "windows" {
		tempPath += ".exe"
	}

	// Always overwrite the temp file to ensure it is the latest bundled version
	err = os.WriteFile(tempPath, binData, 0700)
	if err != nil {
		log.Printf("STT server unpacking failed: %v", err)
		return
	}

	a.sttServer = exec.Command(tempPath, "tr", "9876")
	a.sttServer.Stdout = os.Stdout
	a.sttServer.Stderr = os.Stderr
	sttSetProcessGroup(a.sttServer)

	if err := a.sttServer.Start(); err != nil {
		log.Printf("STT server start failed: %v", err)
		a.sttServer = nil
		return
	}
	log.Println("STT server starting on :9876")
}

func (a *App) shutdown(ctx context.Context) {
	log.Println("Memo shutting down, cleaning up background processes...")

	// Signal memorySaveWorker to stop
	close(a.memorySaveCh)

	if a.sttServer != nil && a.sttServer.Process != nil {
		sttKillProcessGroup(a.sttServer)
	}
	// Stop llama servers if running
	if a.llamaServer != nil {
		if err := a.llamaServer.Stop(); err != nil {
			log.Printf("llama chat shutdown: %v", err)
		}
	}
	if a.llamaEmbedServer != nil {
		if err := a.llamaEmbedServer.Stop(); err != nil {
			log.Printf("llama embedding shutdown: %v", err)
		}
	}
	if a.ngrokServer != nil {
		a.ngrokServer.Stop()
		a.ngrokServer = nil
	}
	stopRecordingProcess()
}

// stopRecordingProcess kills an in-flight microphone recording (arecord/sox/ffmpeg)
// so it doesn't outlive the app and keep writing to the temp WAV forever.
func stopRecordingProcess() {
	recMu.Lock()
	defer recMu.Unlock()
	if recCmd == nil {
		return
	}
	if recStdin != nil {
		recStdin.Close()
		recStdin = nil
	}
	if recCmd.Process != nil {
		recCmd.Process.Kill()
	}
	recCmd.Wait()
	recCmd = nil
	if recFile != "" {
		os.Remove(recFile)
		recFile = ""
	}
}

// ─── Incognito ───────────────────────────────────────────────────

func (a *App) ToggleIncognito(enabled bool) {
	a.incognitoMu.Lock()
	a.isIncognito = enabled
	a.incognitoMessages = nil
	a.incognitoMu.Unlock()
	if enabled {
		log.Println("Entered Incognito Mode")
	} else {
		log.Println("Exited Incognito Mode")
	}
}

// ─── Chat ────────────────────────────────────────────────────────

func (a *App) handleIncognito(userMsg string, b64 string) string {
	a.incognitoMu.Lock()
	if b64 != "" {
		a.incognitoMessages = append(a.incognitoMessages, api.NewMultimodalMessage("user", userMsg, b64))
	} else {
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("user", userMsg))
	}
	msgs := []api.Message{api.NewTextMessage("system", a.cfg.Identity.IncognitoPrompt)}
	msgs = append(msgs, a.incognitoMessages...)
	a.incognitoMu.Unlock()

	reply := a.callLLM(context.Background(), msgs)

	a.incognitoMu.Lock()
	a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("assistant", reply))
	a.incognitoMu.Unlock()
	return reply
}

func (a *App) SendMessage(userMsg string) string {
	log.Printf(">> SendMessage: %q", userMsg)
	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognito(userMsg, "")
	}
	messages := a.buildMessages(context.Background(), userMsg, nil)
	sm := a.getSessionManager()
	if sm != nil {
		sm.AddMessage("user", userMsg, "", "")
	}
	reply := a.callLLM(context.Background(), messages)
	if sm != nil {
		sm.AddMessage("assistant", reply, "", "")
	}
	a.saveMemoryAsync(userMsg, reply)
	return reply
}

func (a *App) SendMessageStream(ctx context.Context, userMsg string) <-chan api.StreamChunk {
	log.Printf(">> SendMessageStream: %q", userMsg)

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognitoStream(ctx, userMsg, "")
	}

	messages := a.buildMessages(ctx, userMsg, nil)

	sm := a.getSessionManager()
	if sm != nil {
		sm.AddMessage("user", userMsg, "", "")
	}

	a.agentMu.RLock()
	agentActive := a.agentEnabled
	a.agentMu.RUnlock()

	orchestraEnabled := a.orchestraConductor != nil && a.orchestraConductor.Config().Enabled

	if agentActive && a.activeProvider != "" {
		if orchestraEnabled {
			return a.callAgentWithOrchestra(ctx, messages, userMsg)
		}
		return a.callAgentStream(ctx, messages, userMsg)
	}

	return a.callLLMStream(ctx, messages, userMsg, "", "")
}

func (a *App) SendMessageWithImageStream(ctx context.Context, userMsg string, imagePath string) <-chan api.StreamChunk {
	log.Printf(">> VisionStream: %q with image %s", userMsg, imagePath)

	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		ch := make(chan api.StreamChunk, 1)
		ch <- api.StreamChunk{Error: "⚠️ Cannot read image: " + err.Error(), Done: true}
		close(ch)
		return ch
	}
	mime := detectMime(imagePath, imgData)
	b64 := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imgData)

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognitoStream(ctx, userMsg, b64)
	}

	var memories []memory.MemoryResult
	if a.cfg.Memory.MemoryEnabled {
		memories = a.retrieveMemory(ctx, userMsg)
	}
	systemPrompt := a.identity.BuildSystemPrompt(memories)

	var msgs []api.Message
	msgs = append(msgs, api.NewTextMessage("system", systemPrompt))
	msgs = append(msgs, a.getSessionHistory()...)
	msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, b64))

	sm := a.getSessionManager()
	if sm != nil {
		sm.AddMessage("user", userMsg, imagePath, "")
	}

	return a.callLLMStream(ctx, msgs, userMsg, imagePath, "")
}

func (a *App) SendMessageWithFileStream(ctx context.Context, userMsg string, filePath string) <-chan api.StreamChunk {
	log.Printf(">> FileStream: %q with %s", userMsg, filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		ch := make(chan api.StreamChunk, 1)
		ch <- api.StreamChunk{Error: "⚠️ Cannot read file: " + err.Error(), Done: true}
		close(ch)
		return ch
	}

	fileName := filepath.Base(filePath)
	fileContent := string(content)
	if len(fileContent) > 10000 {
		fileContent = fileContent[:10000] + "\n\n... (truncated, file too large)"
	}

	combined := fmt.Sprintf("%s\n\n--- File: %s ---\n%s", userMsg, fileName, fileContent)

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognitoStream(ctx, combined, "")
	}

	messages := a.buildMessages(ctx, combined, nil)

	sm := a.getSessionManager()
	if sm != nil {
		sm.AddMessage("user", userMsg, "", filePath)
	}

	return a.callLLMStream(ctx, messages, userMsg, "", filePath)
}

func (a *App) handleIncognitoStream(ctx context.Context, userMsg string, b64 string) <-chan api.StreamChunk {
	a.incognitoMu.Lock()
	if b64 != "" {
		a.incognitoMessages = append(a.incognitoMessages, api.NewMultimodalMessage("user", userMsg, b64))
	} else {
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("user", userMsg))
	}
	msgs := []api.Message{api.NewTextMessage("system", a.cfg.Identity.IncognitoPrompt)}
	msgs = append(msgs, a.incognitoMessages...)
	a.incognitoMu.Unlock()

	// Note: for incognito, we don't save to memory/sessions, handled in callLLMStream via isIncognito flag
	return a.callLLMStream(ctx, msgs, userMsg, "", "")
}

func (a *App) callAgentStream(ctx context.Context, messages []api.Message, userMsg string) <-chan api.StreamChunk {
	outCh := make(chan api.StreamChunk, 128)

	go func() {
		defer close(outCh)

		// Convert api.Message to provider.Message
		pMsgs := make([]provider.Message, len(messages))
		for i, m := range messages {
			pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
		}

		sm := a.getSessionManager()
		sessionID := ""
		if sm != nil {
			sessionID = sm.GetActiveID()
		}

		modelName := ""
		if a.providerRouter != nil {
			if a.activeProvider != "" {
				for _, p := range a.providerCfgMgr.GetEnabled() {
					if p.Type == a.activeProvider {
						modelName = p.Model
						break
					}
				}
			}
			// Fallback: pick any enabled provider
			if modelName == "" {
				for _, p := range a.providerCfgMgr.GetEnabled() {
					modelName = p.Model
					break
				}
			}
		}

		// Get project path from session (agent chat support)
		projectPath := ""
		if sessionID != "" && sm != nil {
			projectPath = sm.GetProjectPath(sessionID)
		}

		// Ensure provider router is healthy
		if a.providerRouter == nil && a.providerCfgMgr != nil {
			if configs := a.providerCfgMgr.GetEnabled(); len(configs) > 0 {
				a.providerRouter = provider.NewRouter(configs)
			}
		}
		if a.providerRouter == nil || !a.providerRouter.HasActiveProvider() {
			trySend(ctx, outCh, api.StreamChunk{
				Error: "⚠️ Agent modu için bir sağlayıcı (provider) yapılandırmadınız. Ayarlar > Sağlayıcılar bölümünde bir API sağlayıcısı ekleyin.",
				Done:  true,
			})
			return
		}
		// Sync executor's router with current app router (router is replaced on provider changes)
		a.agentExecutor.SyncRouter(a.providerRouter)

		start := time.Now()
		var fullReply strings.Builder
		var agentEvents []interface{}

		streamCh, err := a.agentExecutor.RunStream(ctx, sessionID, modelName, pMsgs, func(ev agent.AgentEvent) {
			// Collect agent events for persistence
			agentEvents = append(agentEvents, ev)
			// Emit agent events as special StreamChunks so frontend can render them
			chunkData, _ := json.Marshal(ev)
			trySend(ctx, outCh, api.StreamChunk{
				Content:      string(chunkData),
				FinishReason: "agent_event", // Special flag
			})
		}, projectPath)

		if err != nil {
			log.Printf("Agent error: %v", err)
			trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
			return
		}

		// Forward final response stream
		for chunk := range streamCh {
			if chunk.Error != "" {
				trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + chunk.Error, Done: true})
				return
			}

			if chunk.Content != "" {
				fullReply.WriteString(chunk.Content)
				trySend(ctx, outCh, api.StreamChunk{Content: chunk.Content})
			}

			if chunk.Done {
				a.finishStream(start, 0, chunk.FinishReason, fullReply.String(), userMsg, agentEvents)
				trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: chunk.FinishReason})
				return
			}
		}

		if fullReply.Len() > 0 {
			a.finishStream(start, 0, "stop", fullReply.String(), userMsg, agentEvents)
			trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
		}
	}()

	return outCh
}

// callAgentWithOrchestra runs when both agent mode and orchestra mode are enabled.
// It first runs the orchestra workflow for multi-model reasoning, then feeds the
// result through the agent pipeline so the agent can execute any tool calls.
func (a *App) callAgentWithOrchestra(ctx context.Context, messages []api.Message, userMsg string) <-chan api.StreamChunk {
	outCh := make(chan api.StreamChunk, 128)

	go func() {
		defer close(outCh)

		// Build conversation context for orchestra
		var userPrompt string
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "user" {
				userPrompt = messages[i].GetTextContent()
				if userPrompt != "" {
					break
				}
			}
		}
		if userPrompt == "" {
			trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ No user message found", Done: true})
			return
		}

		conversationCtx := buildConversationContext(messages, userPrompt)

		// Run orchestra workflow
		trySend(ctx, outCh, api.StreamChunk{Content: "🧠 **Orchestra + Agent**\n"})
		trySend(ctx, outCh, api.StreamChunk{Content: fmt.Sprintf("🧙 Şef: %s/%s\n", a.orchestraConductor.Config().ChiefType, a.orchestraConductor.Config().ChiefModel)})

		var fullBuf strings.Builder
		orchestraResult, _, err := a.orchestraConductor.RunWithProgress(ctx, conversationCtx, func(up orchestra.ProgressUpdate) {
			switch up.Type {
			case orchestra.ProgressPlan:
				trySend(ctx, outCh, api.StreamChunk{Content: "🧠 Şef planlıyor...\n"})
			case orchestra.ProgressPlanChunk:
				fullBuf.WriteString(up.Content)
				trySend(ctx, outCh, api.StreamChunk{Content: up.Content})
			case orchestra.ProgressTaskStart:
				fullBuf.WriteString(up.Content)
				trySend(ctx, outCh, api.StreamChunk{Content: up.Content})
			case orchestra.ProgressTaskDone:
				if up.Error != "" {
					chunk := fmt.Sprintf("❌ %s | %s\n   ⚠️ %s\n\n", up.Role, up.ModelType, up.Error)
					fullBuf.WriteString(chunk)
					trySend(ctx, outCh, api.StreamChunk{Content: chunk})
				} else {
					chunk := fmt.Sprintf("✅ %s | %s (%dms)\n", up.Role, up.ModelType, up.DurationMs)
					fullBuf.WriteString(chunk)
					trySend(ctx, outCh, api.StreamChunk{Content: chunk})
					fullBuf.WriteString(up.Content + "\n\n")
					trySend(ctx, outCh, api.StreamChunk{Content: up.Content + "\n\n"})
				}
			case orchestra.ProgressSynthChunk:
				fullBuf.WriteString(up.Content)
				trySend(ctx, outCh, api.StreamChunk{Content: up.Content})
			}
		})
		if err != nil {
			trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ Orchestra hatası: " + err.Error(), Done: true})
			return
		}

		finalContent := fullBuf.String()
		if finalContent == "" {
			finalContent = orchestraResult
		}

		// Now feed the orchestra result through the agent pipeline for tool execution
		trySend(ctx, outCh, api.StreamChunk{Content: "\n🤖 **Agent executing tasks...**\n"})

		// Convert to provider messages
		pMsgs := make([]provider.Message, len(messages))
		for i, m := range messages {
			pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
		}
		// Add the orchestra result as an assistant message context
		pMsgs = append(pMsgs, provider.Message{Role: "assistant", Content: finalContent})

		sm := a.getSessionManager()
		sessionID := ""
		if sm != nil {
			sessionID = sm.GetActiveID()
		}

		modelName := ""
		if a.providerRouter != nil {
			if a.activeProvider != "" {
				for _, p := range a.providerCfgMgr.GetEnabled() {
					if p.Type == a.activeProvider {
						modelName = p.Model
						break
					}
				}
			}
			if modelName == "" {
				for _, p := range a.providerCfgMgr.GetEnabled() {
					modelName = p.Model
					break
				}
			}
		}

		projectPath := ""
		if sessionID != "" && sm != nil {
			projectPath = sm.GetProjectPath(sessionID)
		}

		if a.providerRouter == nil || !a.providerRouter.HasActiveProvider() {
			trySend(ctx, outCh, api.StreamChunk{
				Error: "⚠️ Agent modu için bir sağlayıcı (provider) yapılandırmadınız. Ayarlar > Sağlayıcılar bölümünde bir API sağlayıcısı ekleyin.",
				Done:  true,
			})
			return
		}
		// Sync executor's router with current app router
		a.agentExecutor.SyncRouter(a.providerRouter)

		start := time.Now()
		var agentBuf strings.Builder
		var agentEvents []interface{}

		streamCh, err := a.agentExecutor.RunStream(ctx, sessionID, modelName, pMsgs, func(ev agent.AgentEvent) {
			agentEvents = append(agentEvents, ev)
			chunkData, _ := json.Marshal(ev)
			trySend(ctx, outCh, api.StreamChunk{
				Content:      string(chunkData),
				FinishReason: "agent_event",
			})
		}, projectPath)

		if err != nil {
			trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ Agent hatası: " + err.Error(), Done: true})
			return
		}

		for chunk := range streamCh {
			if chunk.Error != "" {
				trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + chunk.Error, Done: true})
				return
			}
			if chunk.Content != "" {
				agentBuf.WriteString(chunk.Content)
				trySend(ctx, outCh, api.StreamChunk{Content: chunk.Content})
			}
			if chunk.Done {
				finalReply := finalContent + "\n\n" + agentBuf.String()
				a.finishStream(start, 0, chunk.FinishReason, finalReply, userMsg, agentEvents)
				trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: chunk.FinishReason})
				return
			}
		}

		if agentBuf.Len() > 0 {
			finalReply := finalContent + "\n\n" + agentBuf.String()
			a.finishStream(start, 0, "stop", finalReply, userMsg, agentEvents)
			trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
		}
	}()

	return outCh
}

func (a *App) callLLMStream(ctx context.Context, messages []api.Message, userMsg, imagePath, filePath string) <-chan api.StreamChunk {
	outCh := make(chan api.StreamChunk, 128)

	// Orchestra mode takes priority
	if a.orchestraConductor != nil && a.orchestraConductor.Config().Enabled {
		go func() {
			defer close(outCh)

			// Build user message and conversation context
			var userPrompt string
			for i := len(messages) - 1; i >= 0; i-- {
				if messages[i].Role == "user" {
					userPrompt = messages[i].GetTextContent()
					if userPrompt != "" {
						break
					}
				}
			}
			if userPrompt == "" {
				trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ No user message found", Done: true})
				return
			}

			// Include conversation history for context
			conversationCtx := buildConversationContext(messages, userPrompt)

			start := time.Now()

			trySend(ctx, outCh, api.StreamChunk{Content: "🎵 **Orchestra Mode Active**\n"})
			trySend(ctx, outCh, api.StreamChunk{Content: fmt.Sprintf("🧙 Şef: %s/%s\n\n", a.orchestraConductor.Config().ChiefType, a.orchestraConductor.Config().ChiefModel)})

			var fullBuf strings.Builder

			finalResponse, _, err := a.orchestraConductor.RunWithProgress(ctx, conversationCtx, func(up orchestra.ProgressUpdate) {
				switch up.Type {
				case orchestra.ProgressPlan:
					trySend(ctx, outCh, api.StreamChunk{Content: "🧠 Şef planlıyor...\n"})
				case orchestra.ProgressPlanChunk:
					fullBuf.WriteString(up.Content)
					trySend(ctx, outCh, api.StreamChunk{Content: up.Content})
				case orchestra.ProgressTaskStart:
					fullBuf.WriteString(up.Content)
					trySend(ctx, outCh, api.StreamChunk{Content: up.Content})
				case orchestra.ProgressTaskDone:
					if up.Error != "" {
						chunk := fmt.Sprintf("❌ %s | %s\n   ⚠️ %s\n\n", up.Role, up.ModelType, up.Error)
						fullBuf.WriteString(chunk)
						trySend(ctx, outCh, api.StreamChunk{Content: chunk})
					} else {
						chunk := fmt.Sprintf("✅ %s | %s (%dms)\n", up.Role, up.ModelType, up.DurationMs)
						fullBuf.WriteString(chunk)
						trySend(ctx, outCh, api.StreamChunk{Content: chunk})
						fullBuf.WriteString(up.Content + "\n\n")
						trySend(ctx, outCh, api.StreamChunk{Content: up.Content + "\n\n"})
					}
				case orchestra.ProgressSynthChunk:
					fullBuf.WriteString(up.Content)
					trySend(ctx, outCh, api.StreamChunk{Content: up.Content})
				}
			})
			if err != nil {
				a.finishStream(start, 0, "error", fullBuf.String(), userPrompt)
				trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
				return
			}

			// Use fullBuf content if non-empty, otherwise finalResponse
			finalContent := fullBuf.String()
			if finalContent == "" {
				finalContent = finalResponse
			}

			tokenCount := 0
			if finalContent != "" {
				tokenCount = len(strings.Fields(finalContent))
			}

			a.finishStream(start, tokenCount, "stop", finalContent, userPrompt)
			trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
		}()
		return outCh
	}

	// Use external provider only if user explicitly selected one
	a.providerMu.RLock()
	activeProvider := a.activeProvider
	providerRouter := a.providerRouter
	a.providerMu.RUnlock()
	if activeProvider != "" && providerRouter != nil {
		go func() {
			defer close(outCh)

			providerCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
			defer cancel()

			// Convert api.Message to provider.Message
			pMsgs := make([]provider.Message, len(messages))
			for i, m := range messages {
				pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
			}

			req := provider.ChatRequest{
				Messages:    pMsgs,
				Temperature: a.cfg.Llama.Temperature,
				TopP:        a.cfg.Llama.TopP,
				MaxTokens:   a.cfg.Llama.MaxTokens,
				Stream:      true,
			}

			ch, err := providerRouter.ChatCompletionStream(providerCtx, req)
			if err != nil {
				log.Printf("Provider stream error: %v", err)
				trySend(providerCtx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
				return
			}

			start := time.Now()
			var fullReply strings.Builder
			tokenCount := 0
			firstTokenLogged := false

		providerLoop:
			for {
				select {
				case <-providerCtx.Done():
					return
				case chunk, ok := <-ch:
					if !ok {
						break providerLoop
					}

					if chunk.Error != "" {
						trySend(providerCtx, outCh, api.StreamChunk{Error: "⚠️ " + chunk.Error, Done: true})
						return
					}

					if chunk.Content != "" {
						if !firstTokenLogged {
							firstTokenLogged = true
							log.Printf("LATENCY provider.first_token ms=%d", time.Since(start).Milliseconds())
						}
						fullReply.WriteString(chunk.Content)
						tokenCount++
						trySend(providerCtx, outCh, api.StreamChunk{Content: chunk.Content})
					}

					if chunk.Done {
						a.finishStream(start, tokenCount, chunk.FinishReason, fullReply.String(), userMsg)
						trySend(providerCtx, outCh, api.StreamChunk{Done: true, FinishReason: chunk.FinishReason})
						return
					}
				}
			}

			if fullReply.Len() > 0 {
				a.finishStream(start, tokenCount, "stop", fullReply.String(), userMsg)
				trySend(providerCtx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
			} else {
				trySend(providerCtx, outCh, api.StreamChunk{Error: "⚠️ Provider returned empty response", Done: true})
			}
		}()
		return outCh
	}

	// Fallback to local model
	a.clientMu.RLock()
	streamClient := a.client
	a.clientMu.RUnlock()

	go func() {
		defer close(outCh)

		streamCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
		defer cancel()

		requestStart := time.Now()
		ch, err := streamClient.ChatCompletionStream(streamCtx, messages, a.cfg.Llama.Temperature, a.cfg.Llama.TopP, a.cfg.Llama.MaxTokens)
		if err != nil {
			log.Printf("LATENCY llm.stream_error total_ms=%d messages=%d", time.Since(requestStart).Milliseconds(), len(messages))
			log.Printf("LLM stream error: %v", err)
			trySend(streamCtx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
			return
		}
		log.Printf("LATENCY llm.stream_ready total_ms=%d messages=%d", time.Since(requestStart).Milliseconds(), len(messages))

		start := time.Now()
		var fullReply strings.Builder
		tokenCount := 0
		firstTokenLogged := false

	localLoop:
		for {
			select {
			case <-streamCtx.Done():
				return
			case chunk, ok := <-ch:
				if !ok {
					break localLoop
				}

				if chunk.Error != "" {
					log.Printf("LATENCY llm.stream_chunk_error total_ms=%d generation_ms=%d tokens=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), tokenCount)
					log.Printf("Stream chunk error: %s", chunk.Error)
					trySend(streamCtx, outCh, api.StreamChunk{Error: "⚠️ " + chunk.Error, Done: true})
					return
				}

				if chunk.Content != "" {
					if !firstTokenLogged {
						firstTokenLogged = true
						log.Printf("LATENCY llm.first_token total_ms=%d after_stream_ready_ms=%d messages=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), len(messages))
					}
					fullReply.WriteString(chunk.Content)
					tokenCount++
					trySend(streamCtx, outCh, chunk)
				}

				if chunk.Done {
					log.Printf("LATENCY llm.stream_done total_ms=%d generation_ms=%d tokens=%d finish=%s", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), tokenCount, chunk.FinishReason)
					a.finishStream(start, tokenCount, chunk.FinishReason, fullReply.String(), userMsg)
					trySend(streamCtx, outCh, chunk)
					return
				}
			}
		}

		// Channel closed without an explicit Done chunk (some providers skip [DONE]).
		// Treat accumulated content as a complete reply.
		if fullReply.Len() > 0 {
			log.Printf("LATENCY llm.stream_closed total_ms=%d generation_ms=%d tokens=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), tokenCount)
			a.finishStream(start, tokenCount, "stop", fullReply.String(), userMsg)
			trySend(streamCtx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
		} else {
			log.Printf("LATENCY llm.stream_empty total_ms=%d generation_ms=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds())
			trySend(streamCtx, outCh, api.StreamChunk{Error: "⚠️ Model boş yanıt döndürdü", Done: true})
		}
	}()

	return outCh
}

// trySend sends a chunk to outCh or returns if the context is cancelled.
// This prevents goroutine leaks when the consumer disconnects.
func trySend(ctx context.Context, outCh chan<- api.StreamChunk, chunk api.StreamChunk) {
	select {
	case outCh <- chunk:
	case <-ctx.Done():
	}
}

func (a *App) finishStream(start time.Time, tokenCount int, finishReason, reply, userMsg string, agentEvents ...[]interface{}) {
	duration := time.Since(start).Seconds()
	tps := 0.0
	if duration > 0 && tokenCount > 0 {
		tps = float64(tokenCount) / duration
	}

	a.emitEvent("chat:done", api.StreamChunk{
		Done: true,
		Stats: &api.MessageStats{
			TokensPerSecond:  tps,
			CompletionTokens: tokenCount,
			TotalDuration:    duration,
			StopReason:       finishReason,
		},
	})

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if !incog {
		sm := a.getSessionManager()
		if sm != nil {
			sm.AddMessage("assistant", reply, "", "", agentEvents...)
			// Auto-generate smart title after first exchange
			if len(sm.GetActiveMessages()) == 2 {
				go a.GenerateChatTitle()
			}
		}
		a.saveMemoryAsync(userMsg, reply)
	} else {
		a.incognitoMu.Lock()
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("assistant", reply))
		a.incognitoMu.Unlock()
	}
}

func (a *App) SendMessageWithImage(userMsg string, imagePath string) string {
	log.Printf(">> Vision: %q with image %s", userMsg, imagePath)

	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "⚠️ Cannot read image: " + err.Error()
	}
	mime := detectMime(imagePath, imgData)
	b64 := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imgData)

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognito(userMsg, b64)
	}

	// Build multimodal messages BEFORE saving to session,
	// so getSessionHistory() doesn't include the current user message.
	var memories []memory.MemoryResult
	if a.cfg.Memory.MemoryEnabled {
		memories = a.retrieveMemory(context.Background(), userMsg)
	}
	systemPrompt := a.identity.BuildSystemPrompt(memories)

	var msgs []api.Message
	msgs = append(msgs, api.NewTextMessage("system", systemPrompt))
	msgs = append(msgs, a.getSessionHistory()...)
	msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, b64))

	// Save to session after building messages
	sm := a.getSessionManager()
	if sm != nil {
		sm.AddMessage("user", userMsg, imagePath, "")
	}

	reply := a.callLLM(context.Background(), msgs)

	// Detect vision-not-supported error and return friendly message
	if strings.Contains(reply, "image input is not supported") || strings.Contains(reply, "mmproj") {
		reply = "⚠️ Bu model görsel/resim desteklemiyor. Resim gönderebilmek için vision destekli bir model kullanmalısınız (örn: LLaVA, BakLLaVA, Llama Vision gibi)."
	}

	if sm != nil {
		sm.AddMessage("assistant", reply, "", "")
	}
	a.saveMemoryAsync(userMsg, reply)
	return reply
}

func (a *App) SendMessageWithFile(userMsg string, filePath string) string {
	log.Printf(">> File: %q with %s", userMsg, filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "⚠️ Cannot read file: " + err.Error()
	}

	fileName := filepath.Base(filePath)
	fileContent := string(content)

	if len(fileContent) > 10000 {
		fileContent = fileContent[:10000] + "\n\n... (truncated, file too large)"
	}

	combined := fmt.Sprintf("%s\n\n--- File: %s ---\n%s", userMsg, fileName, fileContent)

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognito(combined, "")
	}

	messages := a.buildMessages(context.Background(), combined, nil)

	// Save to session after building messages
	sm := a.getSessionManager()
	if sm != nil {
		sm.AddMessage("user", userMsg, "", filePath)
	}

	reply := a.callLLM(context.Background(), messages)

	if sm != nil {
		sm.AddMessage("assistant", reply, "", "")
	}
	a.saveMemoryAsync(userMsg, reply)
	return reply
}

// ─── Session Management ──────────────────────────────────────────

func (a *App) getSessionManager() *sessions.Manager {
	a.sessionsMu.RLock()
	defer a.sessionsMu.RUnlock()
	return a.sessions
}

func (a *App) NewChat() string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	return sm.NewChat()
}

func (a *App) NewAgentChat(projectPath string) string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	return sm.NewAgentChat(projectPath)
}

func (a *App) ListChats() []sessions.SessionInfo {
	sm := a.getSessionManager()
	if sm == nil {
		return nil
	}
	return sm.ListChats()
}

func (a *App) SwitchChat(id string) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("no session manager")
	}
	return sm.SwitchChat(id)
}

func (a *App) DeleteChat(id string) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("no session manager")
	}
	return sm.DeleteChat(id)
}

func (a *App) RenameChat(id, title string) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("no session manager")
	}
	return sm.RenameChat(id, title)
}

func (a *App) UpdateMessage(index int, content string) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("no session manager")
	}
	return sm.UpdateMessage(index, content)
}

func (a *App) DeleteMessage(index int) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("no session manager")
	}
	return sm.DeleteMessage(index)
}

func (a *App) GetActiveMessages() []sessions.ChatMessage {
	sm := a.getSessionManager()
	if sm == nil {
		return nil
	}
	return sm.GetActiveMessages()
}

func (a *App) GetActiveChatID() string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	return sm.GetActiveID()
}

// ExportChat returns the active chat as a Markdown string.
func (a *App) ExportChat() string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	msgs := sm.GetActiveMessages()
	if len(msgs) == 0 {
		return ""
	}

	var sb strings.Builder
	chatID := sm.GetActiveID()
	// Find title from list
	title := "Memo Chat"
	for _, s := range sm.ListChats() {
		if s.ID == chatID {
			title = s.Title
			break
		}
	}
	sb.WriteString("# " + title + "\n\n")
	sb.WriteString("_Exported from Memo — " + time.Now().Format("2006-01-02 15:04") + "_\n\n---\n\n")

	for _, m := range msgs {
		switch m.Role {
		case "user":
			sb.WriteString("**You** · " + m.Timestamp + "\n\n")
		case "assistant":
			sb.WriteString("**Memo** · " + m.Timestamp + "\n\n")
		}
		sb.WriteString(m.Content + "\n\n---\n\n")
	}
	return sb.String()
}

// GenerateChatTitle asks the LLM to produce a short title from the first
// exchange, then renames the active chat and returns the new title.
func (a *App) GenerateChatTitle() string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	msgs := sm.GetActiveMessages()
	// Only generate when we have exactly the first exchange (user + assistant).
	if len(msgs) < 2 {
		return ""
	}

	first := msgs[0].Content
	if len(first) > 300 {
		first = first[:300]
	}
	second := msgs[1].Content
	if len(second) > 300 {
		second = second[:300]
	}

	prompt := []api.Message{
		api.NewTextMessage("user", fmt.Sprintf(
			"Based on this conversation excerpt, generate a very short chat title (3–6 words max, no quotes, no punctuation at end):\n\nUser: %s\nAssistant: %s\n\nTitle:",
			first, second,
		)),
	}

	title := strings.TrimSpace(a.callLLM(context.Background(), prompt))
	// Discard error replies.
	if title == "" || strings.HasPrefix(title, "⚠️") {
		return ""
	}
	// Sanitize: remove surrounding quotes if any.
	title = strings.Trim(title, `"'`)
	// Truncate to 60 chars just in case.
	runes := []rune(title)
	if len(runes) > 60 {
		title = string(runes[:60])
	}

	chatID := sm.GetActiveID()
	if err := sm.RenameChat(chatID, title); err != nil {
		log.Printf("auto-title rename: %v", err)
		return ""
	}
	return title
}

// ─── File Dialog ─────────────────────────────────────────────────
// ─── Speech to Text ─────────────────────────────────────────────

var (
	recCmd   *exec.Cmd
	recStdin io.WriteCloser
	recFile  string
	recMu    sync.Mutex
)

// getDefaultDshowDevice enumerates ffmpeg DirectShow audio devices and returns
// the first one, or "" if none is found. Note: `-i dummy` always makes ffmpeg
// exit non-zero (the dummy input can't be opened), so the device list must be
// parsed from the output regardless of the error.
func getDefaultDshowDevice() string {
	out, _ := exec.Command("ffmpeg", "-hide_banner", "-list_devices", "true", "-f", "dshow", "-i", "dummy").CombinedOutput()
	inAudioSection := false
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "[dshow") {
			continue
		}
		// Older ffmpeg groups devices under section headers; newer ffmpeg
		// tags each line with "(audio)" / "(video)" instead.
		if strings.Contains(line, "DirectShow audio devices") {
			inAudioSection = true
			continue
		}
		if strings.Contains(line, "DirectShow video devices") {
			inAudioSection = false
			continue
		}
		if strings.Contains(line, "Alternative name") {
			continue
		}
		isAudio := strings.Contains(line, "(audio)") || (inAudioSection && !strings.Contains(line, "(video)"))
		if !isAudio {
			continue
		}
		a := strings.Index(line, "\"")
		b := strings.LastIndex(line, "\"")
		if a != -1 && b > a+1 {
			return line[a+1 : b]
		}
	}
	return ""
}

func (a *App) StartRecording() error {
	recMu.Lock()
	defer recMu.Unlock()

	if recCmd != nil {
		return fmt.Errorf("already recording")
	}

	tmpFile, err := os.CreateTemp("", "memo-stt-*.wav")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpFile.Close()
	recFile = tmpFile.Name()

	var recordArgs []string
	var recorder string
	switch runtime.GOOS {
	case "windows":
		recorder = "ffmpeg"
		dev := getDefaultDshowDevice()
		if dev == "" {
			os.Remove(recFile)
			return fmt.Errorf("no DirectShow audio device found — is a microphone connected and ffmpeg installed?")
		}
		recordArgs = []string{"-y", "-f", "dshow", "-i", "audio=" + dev, "-ar", "16000", "-ac", "1", "-acodec", "pcm_s16le", recFile}
	case "darwin":
		recorder = "sox"
		recordArgs = []string{"-d", "-b", "16", "-r", "16000", "-c", "1", recFile}
	default:
		recorder = "arecord"
		recordArgs = []string{"-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "wav", recFile}
	}
	recCmd = exec.Command(recorder, recordArgs...)
	if runtime.GOOS == "windows" {
		// ffmpeg finalizes the WAV header only on a graceful quit ('q' on
		// stdin); Process.Kill() leaves a corrupt header.
		recStdin, _ = recCmd.StdinPipe()
	}
	if err := recCmd.Start(); err != nil {
		recCmd = nil
		recStdin = nil
		os.Remove(recFile)
		return fmt.Errorf("recording start (%s): %w", recorder, err)
	}

	log.Println("Recording started")
	return nil
}

func (a *App) StopRecordingAndTranscribe() (string, error) {
	recMu.Lock()
	defer recMu.Unlock()

	if recCmd == nil {
		return "", fmt.Errorf("not recording")
	}

	// Stop recording gracefully
	if recCmd.Process != nil {
		if runtime.GOOS == "windows" {
			// Ask ffmpeg to quit via stdin so it finalizes the WAV header,
			// then force-kill if it doesn't exit in time.
			if recStdin != nil {
				io.WriteString(recStdin, "q")
				recStdin.Close()
				recStdin = nil
			}
			done := make(chan struct{})
			go func() { recCmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				recCmd.Process.Kill()
				<-done
			}
		} else {
			recCmd.Process.Signal(os.Interrupt)
			recCmd.Wait()
		}
	} else {
		recCmd.Wait()
	}
	recCmd = nil

	defer os.Remove(recFile)

	// Send WAV to the local STT server
	audioData, err := os.ReadFile(recFile)
	if err != nil {
		return "", fmt.Errorf("read recording: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:9876/transcribe", bytes.NewReader(audioData))
	if err != nil {
		return "", fmt.Errorf("stt request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("stt server unreachable (model may still be loading): %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("stt decode: %w", err)
	}

	log.Printf("STT result: %q", result.Text)
	return result.Text, nil
}

func (a *App) findPath(relative string) string {
	// Try relative to working directory first (dev mode)
	if _, err := os.Stat(relative); err == nil {
		return relative
	}
	// Try relative to binary
	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}
	full := filepath.Join(filepath.Dir(exePath), relative)
	if _, err := os.Stat(full); err == nil {
		return full
	}
	return ""
}

func (a *App) TranscribeAudio(audioData []byte) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:9876/transcribe", bytes.NewReader(audioData))
	if err != nil {
		return "", fmt.Errorf("stt request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("stt server unreachable: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("stt decode: %w", err)
	}
	return result.Text, nil
}

// ─── Other ───────────────────────────────────────────────────────

// DebugMemorySearch searches memory WITHOUT similarity filter — for debugging.
func (a *App) DebugMemorySearch(query string) []memory.MemoryResult {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.store.DebugSearch(ctx, query, 10)
}

func (a *App) GetMemoryCount() int {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return 0
	}
	return a.store.Count()
}

// ─── Provider Management ───────────────────────────────────────────

func (a *App) GetProviders() []provider.ProviderConfig {
	if a.providerCfgMgr == nil {
		return nil
	}
	configs := a.providerCfgMgr.GetAll()
	// Add connection status from router
	if a.providerRouter != nil {
		active := a.providerRouter.ActiveProviders()
		activeMap := make(map[provider.ProviderType]bool)
		for _, cfg := range active {
			activeMap[cfg.Type] = true
		}
		for i, cfg := range configs {
			configs[i].Connected = activeMap[cfg.Type]
		}
	}
	return configs
}

func (a *App) UpdateProvider(cfg provider.ProviderConfig) error {
	if a.providerCfgMgr == nil {
		return fmt.Errorf("provider system not initialized")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	a.providerCfgMgr.Set(cfg)
	// Rebuild router
	configs := a.providerCfgMgr.GetEnabled()
	a.providerMu.Lock()
	a.providerRouter = provider.NewRouter(configs)
	a.providerMu.Unlock()
	if len(configs) > 0 {
		a.providerMu.RLock()
		rt := a.providerRouter
		a.providerMu.RUnlock()
		go rt.HealthCheck(a.ctx, 5*time.Minute)
	}
	return nil
}

func (a *App) DeleteProvider(pt provider.ProviderType) error {
	if a.providerCfgMgr == nil {
		return fmt.Errorf("provider system not initialized")
	}
	a.providerCfgMgr.Delete(pt)
	configs := a.providerCfgMgr.GetEnabled()
	a.providerMu.Lock()
	a.providerRouter = provider.NewRouter(configs)
	a.providerMu.Unlock()
	return nil
}

func (a *App) TestProviderConnection(cfg provider.ProviderConfig) error {
	if a.providerCfgMgr == nil {
		return fmt.Errorf("provider system not initialized")
	}
	return a.providerCfgMgr.TestConnection(&cfg)
}

func (a *App) SetActiveProvider(pt provider.ProviderType) {
	a.providerMu.Lock()
	a.activeProvider = pt
	if a.providerRouter != nil {
		a.providerRouter.SetActiveProvider(pt)
	}
	a.providerMu.Unlock()
	log.Printf("Active provider set to: %s", pt)
}

func (a *App) GetActiveProvider() string {
	a.providerMu.RLock()
	defer a.providerMu.RUnlock()
	return string(a.activeProvider)
}

// ─── Orchestra Mode ───────────────────────────────────────────

func (a *App) GetOrchestraConfig() orchestra.OrchestraConfig {
	if a.orchestraConductor == nil {
		return orchestra.DefaultConfig()
	}
	return a.orchestraConductor.Config()
}

func (a *App) UpdateOrchestraConfig(cfg orchestra.OrchestraConfig) error {
	if a.orchestraConductor == nil {
		return fmt.Errorf("orchestra system not initialized")
	}
	a.orchestraConductor.UpdateConfig(cfg)
	if err := orchestra.SaveConfig("data/orchestra.json", a.orchestraConductor.Config()); err != nil {
		log.Printf("ORCHESTRA: config save error: %v", err)
		return err
	}
	log.Printf("Orchestra config updated: enabled=%v, chief=%s/%s", cfg.Enabled, cfg.ChiefType, cfg.ChiefModel)
	return nil
}

func (a *App) GetImageBase64(path string) string {
	// Layer 2: Only allow paths under the data directory
	dataDir := filepath.Dir(a.cfg.Memory.PersistDir)
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return ""
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return ""
	}

	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return ""
	}

	if !strings.HasPrefix(realPath, absDataDir) {
		log.Printf("WARNING: Blocked attempt to read file outside data dir: %s", path)
		return ""
	}

	imgData, err := os.ReadFile(realPath)
	if err != nil {
		return ""
	}
	mime := detectMime(path, imgData)
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imgData)
}

func (a *App) CheckConnection() ConnectionStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	a.clientMu.RLock()
	checkClient := a.client
	a.clientMu.RUnlock()
	models, err := checkClient.CheckConnection(ctx)
	if err != nil {
		return ConnectionStatus{Connected: false, Error: err.Error()}
	}
	var names []string
	for _, m := range models {
		names = append(names, m.ID)
	}
	return ConnectionStatus{Connected: true, Models: names}
}

func (a *App) GetConfig() *config.AppConfig { return a.cfg }
func (a *App) GetAvailableStyles() []string { return identity.AvailableStyles() }

func (a *App) UpdateIdentity(userName, assistantName, style string) error {
	a.identity.Update(userName, assistantName, style, a.cfg.Identity.SystemRole)
	a.cfg.Identity.UserName = userName
	a.cfg.Identity.AssistantName = assistantName
	a.cfg.Identity.Style = style
	return config.Save(a.cfg)
}

func (a *App) ClearHistory() {
	sm := a.getSessionManager()
	if sm != nil {
		sm.DeleteChat(sm.GetActiveID())
	}
}

// ─── Settings: System Prompt ─────────────────────────────────────

func (a *App) GetSystemPrompt() string {
	return a.cfg.Identity.SystemRole
}

func (a *App) GetIncognitoPrompt() string {
	return a.cfg.Identity.IncognitoPrompt
}

func (a *App) SetIncognitoPrompt(prompt string) error {
	a.cfg.Identity.IncognitoPrompt = prompt
	return config.Save(a.cfg)
}

func (a *App) SetSystemPrompt(prompt string) error {
	a.identity.Update("", "", "", prompt)
	a.cfg.Identity.SystemRole = prompt
	log.Printf("System prompt updated (%d chars)", len(prompt))
	return config.Save(a.cfg)
}

func (a *App) ResetSystemPrompt() error {
	nameSection := ""
	if a.cfg.Identity.UserName != "" {
		nameSection = fmt.Sprintf("The user's name is %s. ", a.cfg.Identity.UserName)
	}
	defaultPrompt := fmt.Sprintf(`%sYou are %s, a highly capable, privacy-first AI assistant running entirely locally on the user's device.

CORE DIRECTIVES:
1. Identity: You are always %s, regardless of the underlying LLM. Act as a smart, reliable, and direct partner.
2. Anti-Hallucination: Never invent, guess, or fabricate information. If you are unsure or do not know the answer, explicitly state that you do not know.
3. Conciseness & Structure: Keep your answers clear, well-structured, and strictly to the point. Avoid long, rambling introductions or unnecessary filler words.
4. Seamless Memory: You have access to the user's personal context. Use this information naturally to inform your answers. STRICTLY FORBIDDEN: Do not use phrases like "I remember," "As we discussed," "Based on your data," or "I recall." Simply present the information as shared context.
5. Language Mirroring: Always respond in the exact language the user communicates in (e.g., if the user asks in Turkish, your entire response must be in Turkish).`, nameSection, a.cfg.Identity.AssistantName, a.cfg.Identity.AssistantName)

	a.identity.Update("", "", "", defaultPrompt)
	a.cfg.Identity.SystemRole = defaultPrompt
	log.Println("System prompt reset to default")
	return config.Save(a.cfg)
}

// ─── Settings: Memory Management ─────────────────────────────────

func (a *App) ClearAllMemory() error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		return fmt.Errorf("no memory store")
	}
	log.Println("Clearing all memory...")
	return a.store.ClearAll()
}

func (a *App) ListMemoryFiles() []memory.GobFileInfo {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return nil
	}
	return a.store.ListGobFiles()
}

func (a *App) DeleteMemoryFile(relPath string) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		return fmt.Errorf("no memory store")
	}
	log.Printf("Deleting memory file: %s", relPath)
	return a.store.DeleteGobFile(relPath)
}

func (a *App) GetMemorySettings() config.MemoryConfig {
	return a.cfg.Memory
}

func (a *App) UpdateMemorySettings(topK int, minSimilarity float32) error {
	if topK < 1 || topK > 50 {
		return fmt.Errorf("top_k must be between 1 and 50")
	}
	if minSimilarity <= 0 || minSimilarity > 1 {
		return fmt.Errorf("min_similarity must be between 0.01 and 1")
	}

	a.cfg.Memory.TopK = topK
	a.cfg.Memory.MinSimilarity = minSimilarity
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	log.Printf("Memory settings updated: top_k=%d min_similarity=%.2f", topK, minSimilarity)
	return nil
}

func (a *App) GetMemoryEnabled() bool {
	return a.cfg.Memory.MemoryEnabled
}

func (a *App) SetMemoryEnabled(enabled bool) error {
	a.cfg.Memory.MemoryEnabled = enabled
	return config.Save(a.cfg)
}

// ─── Web Bridge (interface adapters for webserver) ───────────────

func (a *App) WebListChats() interface{}         { return a.ListChats() }
func (a *App) WebGetActiveMessages() interface{} { return a.GetActiveMessages() }
func (a *App) WebCheckConnection() interface{}   { return a.CheckConnection() }

// ─── Settings: Remote Access ─────────────────────────────────────

type RemoteAccessStatus struct {
	Enabled     bool     `json:"enabled"`
	Port        int      `json:"port"`
	Running     bool     `json:"running"`
	Addresses   []string `json:"addresses"`
	Token       string   `json:"token"`
	NgrokMode   bool     `json:"ngrok_mode"`
	NgrokToken  string   `json:"ngrok_token"`
	NgrokURL    string   `json:"ngrok_url"`
	NgrokError  string   `json:"ngrok_error"`
}

func (a *App) GetRemoteAccessStatus() interface{} {
	status := RemoteAccessStatus{
		Enabled:    a.cfg.RemoteAccess.Enabled,
		Port:       a.cfg.RemoteAccess.Port,
		Token:      a.cfg.RemoteAccess.Token,
		NgrokMode:  a.cfg.RemoteAccess.NgrokMode,
		NgrokToken: a.cfg.RemoteAccess.NgrokToken,
	}
	if a.webServer != nil {
		status.Running = a.webServer.IsRunning()
		status.Addresses = a.webServer.GetAddresses()
	}
	if a.ngrokServer != nil {
		if a.ngrokServer.IsRunning() {
			if url := a.ngrokServer.PublicURL(); url != "" {
				status.NgrokURL = url
				status.Addresses = append(status.Addresses, url)
			}
		}
		if err := a.ngrokServer.LastError(); err != "" {
			status.NgrokError = err
		}
	}
	return status
}

func (a *App) SetRemoteAccess(enabled bool, port int) error {
	if a.webServer == nil {
		return fmt.Errorf("server not initialized")
	}

	if enabled == a.remoteAccessEnabled && a.webServer.GetPort() == port && a.cfg.RemoteAccess.NgrokMode == (a.ngrokServer != nil) {
		return nil
	}

	a.remoteAccessEnabled = enabled
	a.cfg.RemoteAccess.Enabled = enabled
	a.cfg.RemoteAccess.Port = port

	// Generate token if missing
	if a.cfg.RemoteAccess.Token == "" {
		a.cfg.RemoteAccess.Token = generateToken()
	}

	// Start/stop ngrok
	if enabled && a.cfg.RemoteAccess.NgrokMode && a.cfg.RemoteAccess.NgrokToken != "" {
		if a.ngrokServer != nil {
			a.ngrokServer.Stop()
		}
		binPath, err := ngrok.Install("data")
		if err != nil {
			log.Printf("[ngrok] Install error: %v", err)
		} else {
			mgr := ngrok.NewManager(binPath)
			if err := mgr.Start(port, a.cfg.RemoteAccess.NgrokToken); err != nil {
				log.Printf("[ngrok] Start error: %v", err)
			} else {
				a.ngrokServer = mgr
			}
		}
	} else if !enabled && a.ngrokServer != nil {
		a.ngrokServer.Stop()
		a.ngrokServer = nil
	}

	config.Save(a.cfg)

	if err := a.webServer.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}
	addr := "127.0.0.1"
	if enabled {
		addr = "0.0.0.0"
	}
	return a.webServer.StartHTTPWithAddr(port, addr)
}

func (a *App) SetNgrokMode(enabled bool, port int, ngrokToken string) error {
	a.cfg.RemoteAccess.NgrokMode = enabled
	if ngrokToken != "" {
		a.cfg.RemoteAccess.NgrokToken = ngrokToken
	}

	if !enabled && a.ngrokServer != nil {
		a.ngrokServer.Stop()
		a.ngrokServer = nil
	}

	return a.SetRemoteAccess(enabled, port)
}

func generateToken() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "memo-" + hex.EncodeToString(b)
}

func (a *App) GetListenAddr() string {
	if a.webServer == nil {
		return "127.0.0.1"
	}
	return a.webServer.GetListenAddr()
}

func (a *App) SetListenAddr(addr string) {
	if a.webServer != nil {
		a.webServer.SetListenAddr(addr)
	}
}

// ─── Model Store: Search & Download ──────────────────────────────

func (a *App) SearchModels(query string) ([]modelstore.HFModelResult, error) {
	results, err := a.modelStore.SearchModels(query)
	if err != nil {
		log.Printf("SearchModels error: %v", err)
		return nil, fmt.Errorf("search failed: %w", err)
	}
	return results, nil
}

func (a *App) GetModelFiles(repoID string) []modelstore.GGUFFile {
	files, err := a.modelStore.GetModelFiles(repoID)
	if err != nil {
		log.Printf("GetModelFiles error: %v", err)
		return nil
	}
	return files
}

func (a *App) DownloadModel(repoID, filename string) error {
	return a.modelStore.DownloadModel(repoID, filename)
}

func (a *App) GetDownloadProgress() *modelstore.DownloadProgress {
	return a.modelStore.GetDownloadProgress()
}

func (a *App) CancelDownload() {
	a.modelStore.CancelDownload()
}

func (a *App) ImportLocalModel(sourcePath string) error {
	return a.modelStore.ImportLocalModel(sourcePath)
}

func (a *App) ListLocalModels() []modelstore.LocalModel {
	return a.modelStore.ListLocalModels()
}

func (a *App) DeleteLocalModel(path string) error {
	return a.modelStore.DeleteLocalModel(path)
}

// ─── llama-server: Lifecycle Management ──────────────────────────

func (a *App) StartLocalModel(modelPath string, ctxSize, port, gpuLayers int) error {
	if a.isEmbeddingModelPath(modelPath) {
		return fmt.Errorf("embedding modeli ana sohbet modeli olarak başlatılamaz; Hafıza (Embedding) Modeli olarak başlatın")
	}

	if err := a.llamaServer.Start(a.cfg.Llama.BinaryPath, modelPath, ctxSize, port, gpuLayers, false, a.cfg.Llama.EngineMode); err != nil {
		return err
	}

	// Wait up to 3 minutes for the model to load before returning success to the UI.
	// Since Flutter event system is currently disabled, we use synchronous wait.
	if err := a.llamaServer.WaitReady(180 * time.Second); err != nil {
		a.llamaServer.Stop()
		return fmt.Errorf("Model yükleme zaman aşımına uğradı (3 dk). (Hata: %w)", err)
	}

	// Redirect API client to the local llama-server
	newBaseURL := a.llamaServer.GetBaseURL()
	a.clientMu.Lock()
	a.client = api.NewClient(newBaseURL, a.cfg.API.TimeoutSeconds)
	a.clientMu.Unlock()
	log.Printf("API client redirected to local llama-server: %s", newBaseURL)

	// Auto-start embedding model if not already running and memory is enabled
	if a.cfg.Memory.MemoryEnabled && !a.llamaEmbedServer.IsRunning() {
		a.autoStartEmbeddingModel()
	} else if !a.cfg.Memory.MemoryEnabled {
		log.Println("Memory disabled — skipping embedding model auto-start")
	}

	return nil
}

func (a *App) StopLocalModel() error {
	if err := a.llamaServer.Stop(); err != nil {
		return err
	}

	// Revert API client to the original base URL
	a.clientMu.Lock()
	a.client = api.NewClient(a.originalBaseURL, a.cfg.API.TimeoutSeconds)
	revertedClient := a.client
	a.clientMu.Unlock()
	log.Printf("API client reverted to: %s", a.originalBaseURL)

	// Only re-init embedding if no dedicated embedding server is running
	if !a.llamaEmbedServer.IsRunning() {
		a.reinitMemoryStore(revertedClient, a.cfg.API.EmbeddingModel)
	}

	return nil
}

func (a *App) GetLocalModelStatus() llama.ServerStatus {
	return a.llamaServer.GetStatus()
}

func (a *App) DetectGPU() llama.GPUInfo {
	return llama.DetectGPU()
}

// ─── Settings: Llama Config ──────────────────────────────────────

func (a *App) GetLlamaConfig() config.LlamaConfig {
	return a.cfg.Llama
}

func (a *App) UpdateLlamaConfig(cfg config.LlamaConfig) error {
	// Merge partial updates — only overwrite fields with non-zero values
	if cfg.EngineMode != "" {
		a.cfg.Llama.EngineMode = cfg.EngineMode
	}
	if cfg.BinaryPath != "" {
		a.cfg.Llama.BinaryPath = cfg.BinaryPath
	}
	if cfg.Port != 0 {
		a.cfg.Llama.Port = cfg.Port
	}
	if cfg.EmbeddingPort != 0 {
		a.cfg.Llama.EmbeddingPort = cfg.EmbeddingPort
	}
	if cfg.CtxSize != 0 {
		a.cfg.Llama.CtxSize = cfg.CtxSize
	}
	if cfg.MaxHistory != 0 {
		a.cfg.Llama.MaxHistory = cfg.MaxHistory
	}
	if cfg.ModelsDir != "" {
		a.cfg.Llama.ModelsDir = cfg.ModelsDir
	}
	if cfg.Temperature != 0 {
		a.cfg.Llama.Temperature = cfg.Temperature
	}
	if cfg.TopP != 0 {
		a.cfg.Llama.TopP = cfg.TopP
	}
	if cfg.MaxTokens != 0 {
		a.cfg.Llama.MaxTokens = cfg.MaxTokens
	}
	return config.Save(a.cfg)
}

func (a *App) SetLlamaBinaryPath(path string) error {
	a.cfg.Llama.BinaryPath = path
	return config.Save(a.cfg)
}

// ─── Llama Installer ─────────────────────────────────────────────

func (a *App) CheckLlamaInstallation() bool {
	return a.llamaInstaller.IsInstalled(a.cfg.Llama.BinaryPath)
}

func (a *App) InstallLlamaServer() error {
	logger := func(msg string) {
		log.Println("INSTALL:", msg)
	}

	binPath, err := a.llamaInstaller.Install(a.ctx, logger)
	if err != nil {
		return err
	}

	// Update config to point to the newly compiled binary
	a.cfg.Llama.BinaryPath = binPath
	// If GPU installer succeeds, remove any old .force_cpu file so they run on GPU!
	_ = os.Remove("data/.force_cpu")
	return config.Save(a.cfg)
}

func (a *App) SkipLlamaGPUInstall() error {
	// Create .force_cpu file in data directory to bypass GPU checks
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}
	forceCPUFile := "data/.force_cpu"
	f, err := os.Create(forceCPUFile)
	if err != nil {
		return err
	}
	f.Close()
	log.Println("Created .force_cpu bypass file. Future starts will use CPU.")
	return nil
}

// ─── WhatsApp Integration ─────────────────────────────────────

// initWhatsApp initializes the WhatsApp client and message store,
// then auto-connects (QR codes become available via WhatsAppStatus()).
func (a *App) initWhatsApp() {
	cfg := a.cfg.WhatsApp

	dataDir := cfg.DataDir
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("WhatsApp: mkdir data dir: %v", err)
		return
	}

	msgDB := filepath.Join(dataDir, "messages.db")
	sessionDB := filepath.Join(dataDir, "session.db")

	msgStore, err := whatsapp.NewStore(msgDB)
	if err != nil {
		log.Printf("WhatsApp: store init error: %v", err)
		return
	}
	a.waMsgStore = msgStore

	waCfg := whatsapp.Config{
		DataDir:        dataDir,
		MessageStoreDB: msgDB,
		SessionDB:      sessionDB,
		AutoIndex:      cfg.AutoIndex,
		MaxHistoryDays: cfg.MaxHistoryDays,
	}

	a.waClient = whatsapp.NewClient(waCfg)
	a.waClient.SetStore(msgStore)

	// Wire up WhatsApp client for agent tools (package-level global)
	tools.WhatsAppClient = waToolAdapter{a.waClient}

	// Auto-connect: embedding model gibi startup'ta bağlan.
	// QR varsa Fluent UI QR gösterir, session varsa direkt bağlanır.
	go func() {
		if err := a.waClient.Start(context.Background()); err != nil {
			log.Printf("WhatsApp: auto-connect error: %v", err)
		}
	}()

	log.Println("WhatsApp client initialized and connecting...")
}

// StartWhatsApp connects to WhatsApp Web. Flutter reads QRCodes() for pairing.
func (a *App) StartWhatsApp(ctx context.Context) error {
	if a.waClient == nil {
		return fmt.Errorf("WhatsApp not initialized (enable in config)")
	}
	return a.waClient.Start(ctx)
}

// StopWhatsApp disconnects from WhatsApp Web.
func (a *App) StopWhatsApp() {
	if a.waClient != nil {
		a.waClient.Stop()
	}
}

// WhatsAppStatus returns QR codes (if pairing) and connection state.
func (a *App) WhatsAppStatus() map[string]interface{} {
	if a.waClient == nil {
		return map[string]interface{}{
			"initialized": false,
			"connected":   false,
			"logged_in":   false,
		}
	}
	return map[string]interface{}{
		"initialized": true,
		"connected":   a.waClient.IsConnected(),
		"logged_in":   a.waClient.IsLoggedIn(),
		"qr_codes":    a.waClient.QRCodes(),
	}
}

// WhatsAppSend sends a text message via WhatsApp.
func (a *App) WhatsAppSend(ctx context.Context, jid, text string) (string, error) {
	if a.waClient == nil {
		return "", fmt.Errorf("WhatsApp not initialized")
	}
	return a.waClient.SendMessage(ctx, jid, text)
}

// GetWhatsAppChatMode returns whether WhatsApp chat mode is active.
func (a *App) GetWhatsAppChatMode() bool {
	a.whatsappChatMu.RLock()
	defer a.whatsappChatMu.RUnlock()
	return a.whatsappChatMode
}

// SetWhatsAppChatMode enables or disables WhatsApp chat mode.
// When active, messages are handled by an agent with WhatsApp-only tools.
func (a *App) SetWhatsAppChatMode(enabled bool) {
	a.whatsappChatMu.Lock()
	defer a.whatsappChatMu.Unlock()
	a.whatsappChatMode = enabled
}

// WhatsAppChatStream handles a chat message in WhatsApp mode.
// Uses a dedicated agent executor with ONLY WhatsApp tools (no file/command tools).
func (a *App) WhatsAppChatStream(ctx context.Context, userMsg string) <-chan api.StreamChunk {
	outCh := make(chan api.StreamChunk, 128)

	go func() {
		defer close(outCh)

		trySend := func(ctx context.Context, ch chan<- api.StreamChunk, chunk api.StreamChunk) bool {
			select {
			case ch <- chunk:
				return true
			case <-ctx.Done():
				return false
			}
		}

		// Build messages with WhatsApp context in system prompt
		messages := a.buildMessages(ctx, userMsg, nil)

		sm := a.getSessionManager()
		if sm != nil {
			sm.AddMessage("user", userMsg, "", "")
		}

		// Convert to provider messages
		pMsgs := make([]provider.Message, len(messages))
		for i, m := range messages {
			pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
		}

		sessionID := ""
		if sm != nil {
			sessionID = sm.GetActiveID()
		}

		modelName := ""
		if a.providerRouter != nil {
			if a.activeProvider != "" {
				for _, p := range a.providerCfgMgr.GetEnabled() {
					if p.Type == a.activeProvider {
						modelName = p.Model
						break
					}
				}
			}
			if modelName == "" {
				for _, p := range a.providerCfgMgr.GetEnabled() {
					modelName = p.Model
					break
				}
			}
		}

		if a.providerRouter == nil || !a.providerRouter.HasActiveProvider() {
			trySend(ctx, outCh, api.StreamChunk{
				Error: "WhatsApp sohbeti için bir sağlayıcı yapılandırmadınız.",
				Done:  true,
			})
			return
		}

		// WhatsApp system prompt: tell the LLM about WhatsApp capabilities
		waPrompt := provider.Message{
			Role: "system",
			Content: `Sen bir WhatsApp asistanısın. Kullanıcının WhatsApp mesajlarını okuyabilir, arama yapabilir ve mesaj gönderebilirsin.

Kullanabileceğin araçlar:
- whatsapp_latest: En son mesajlaşılan sohbetleri listele
- whatsapp_messages: Bir sohbetin mesaj geçmişini getir
- whatsapp_search: Mesajlarda metin araması yap
- whatsapp_send: Bir kişiye mesaj gönder

Kullanıcı "bana en son kim yazdı" derse whatsapp_latest çağır.
"falana mesaj at" derse whatsapp_send çağır.
"falana ne yazmışım" derse whatsapp_messages veya whatsapp_search çağır.

NOT: Kişi JID'leri telefon numarası formatındadır (ör: 905551234567@s.whatsapp.net).
Kullanıcıya JID sormadan önce whatsapp_latest ile sohbet listesini kontrol et.`,
		}

		// Prepend WhatsApp system prompt
		allMsgs := make([]provider.Message, 0, len(pMsgs)+1)
		allMsgs = append(allMsgs, waPrompt)
		allMsgs = append(allMsgs, pMsgs...)

		// Create WhatsApp-only executor
		waExecutor := agent.NewWhatsAppExecutor(a.agentExecutor)
		waExecutor.SyncRouter(a.providerRouter)

		start := time.Now()
		var fullReply strings.Builder

		streamCh, err := waExecutor.RunStream(ctx, sessionID, modelName, allMsgs, func(ev agent.AgentEvent) {
			chunkData, _ := json.Marshal(ev)
			trySend(ctx, outCh, api.StreamChunk{
				Content:      string(chunkData),
				FinishReason: "agent_event",
			})
		})

		if err != nil {
			trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
			return
		}

		for chunk := range streamCh {
			if chunk.Content != "" {
				fullReply.WriteString(chunk.Content)
			}
			trySend(ctx, outCh, api.StreamChunk{Content: chunk.Content, FinishReason: chunk.FinishReason, Done: chunk.Done})
		}

		reply := fullReply.String()
		if reply != "" && sm != nil {
			sm.AddMessage("assistant", reply, "", "")
		}

		log.Printf("WhatsApp chat completed in %v (%d chars)", time.Since(start), len(reply))
	}()

	return outCh
}

// WhatsAppSearch searches WhatsApp messages.
func (a *App) WhatsAppSearch(query string, limit int) ([]whatsapp.Message, error) {
	if a.waMsgStore == nil {
		return nil, fmt.Errorf("WhatsApp store not available")
	}
	return a.waMsgStore.SearchMessages(query, limit)
}

// WhatsAppGetChats returns the chat list.
func (a *App) WhatsAppGetChats() ([]whatsapp.ChatSummary, error) {
	if a.waMsgStore == nil {
		return nil, fmt.Errorf("WhatsApp store not available")
	}
	return a.waMsgStore.GetChatList()
}

// WhatsAppGetMessages returns messages for a specific chat.
func (a *App) WhatsAppGetMessages(chatJID string, limit int) ([]whatsapp.Message, error) {
	if a.waMsgStore == nil {
		return nil, fmt.Errorf("WhatsApp store not available")
	}
	return a.waMsgStore.GetChatMessages(chatJID, limit)
}

// WhatsAppStats returns message statistics.
func (a *App) WhatsAppStats() (total, last24h int, err error) {
	if a.waMsgStore == nil {
		return 0, 0, fmt.Errorf("WhatsApp store not available")
	}
	return a.waMsgStore.Stats()
}

// ─── WhatsApp Agent Tool Adapter ────────────────────────────────

// waToolAdapter wraps *whatsapp.Client to satisfy tools.WhatsAppClient.
type waToolAdapter struct {
	c *whatsapp.Client
}

func (a waToolAdapter) SendMessage(ctx context.Context, jid, text string) (string, error) {
	return a.c.SendMessage(ctx, jid, text)
}

func (a waToolAdapter) SearchMessages(query string, limit int) ([]tools.WhatsAppMsg, error) {
	if a.c == nil {
		return nil, nil
	}
	msgs, err := a.c.SearchMessages(query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]tools.WhatsAppMsg, len(msgs))
	for i, m := range msgs {
		out[i] = tools.WhatsAppMsg{
			ID:         m.ID,
			ChatJID:    m.ChatJID,
			SenderJID:  m.SenderJID,
			SenderName: m.SenderName,
			Text:       m.Text,
			Timestamp:  m.Timestamp,
			FromMe:     m.FromMe,
		}
	}
	return out, nil
}

func (a waToolAdapter) GetChatList() ([]tools.WhatsAppChat, error) {
	if a.c == nil {
		return nil, nil
	}
	chats, err := a.c.GetChatList()
	if err != nil {
		return nil, err
	}
	out := make([]tools.WhatsAppChat, len(chats))
	for i, c := range chats {
		out[i] = tools.WhatsAppChat{
			JID:         c.JID,
			DisplayName: c.DisplayName,
			LastMessage: c.LastMessage,
			LastTime:    c.LastTime,
			Unread:      c.Unread,
		}
	}
	return out, nil
}

func (a waToolAdapter) GetChatMessages(chatJID string, limit int) ([]tools.WhatsAppMsg, error) {
	if a.c == nil {
		return nil, nil
	}
	msgs, err := a.c.GetChatMessages(chatJID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]tools.WhatsAppMsg, len(msgs))
	for i, m := range msgs {
		out[i] = tools.WhatsAppMsg{
			ID:         m.ID,
			ChatJID:    m.ChatJID,
			SenderJID:  m.SenderJID,
			SenderName: m.SenderName,
			Text:       m.Text,
			Timestamp:  m.Timestamp,
			FromMe:     m.FromMe,
		}
	}
	return out, nil
}

// ─── Embedding Server: Lifecycle Management ─────────────────────

func (a *App) StartEmbeddingModel(modelPath string, gpuLayers int) error {
	if a.modelStore != nil {
		for _, m := range a.modelStore.ListLocalModels() {
			if m.Path == modelPath && !m.IsEmbedding {
				return fmt.Errorf("sohbet modeli Hafıza (Embedding) Modeli olarak başlatılamaz")
			}
		}
	}

	// Stop existing embedding server if running
	if a.llamaEmbedServer.IsRunning() {
		a.llamaEmbedServer.Stop()
		time.Sleep(500 * time.Millisecond) // Give it a moment to release ports
	}

	// Use 8082 as the fixed embedding port — MUST NOT conflict with chat port (8081)
	embPort := a.cfg.Llama.EmbeddingPort
	if embPort <= 0 || embPort == a.cfg.Llama.Port {
		embPort = 8082
	}
	log.Printf("Starting embedding model on port %d", embPort)

	if err := a.llamaEmbedServer.Start(a.cfg.Llama.BinaryPath, modelPath, 512, embPort, gpuLayers, true, a.cfg.Llama.EngineMode); err != nil {
		return err
	}

	if err := a.llamaEmbedServer.WaitReady(120 * time.Second); err != nil {
		a.llamaEmbedServer.Stop()
		return fmt.Errorf("embedding model loaded but server failed to start: %w", err)
	}

	// Create dedicated embedding client and reinit memory store
	embBaseURL := a.llamaEmbedServer.GetBaseURL()
	a.clientMu.Lock()
	a.embeddingClient = api.NewClient(embBaseURL, a.cfg.API.TimeoutSeconds)
	embClient := a.embeddingClient
	a.clientMu.Unlock()
	a.reinitMemoryStore(embClient, a.cfg.API.EmbeddingModel)
	log.Printf("Embedding server ready on %s", embBaseURL)

	return nil
}

func (a *App) StopEmbeddingModel() error {
	if err := a.llamaEmbedServer.Stop(); err != nil {
		return err
	}

	a.clientMu.Lock()
	a.embeddingClient = nil
	a.clientMu.Unlock()
	log.Println("Embedding server stopped")

	// Fall back to main client for embeddings
	a.clientMu.RLock()
	mainClient := a.client
	a.clientMu.RUnlock()
	a.reinitMemoryStore(mainClient, a.cfg.API.EmbeddingModel)

	return nil
}

func (a *App) GetEmbeddingModelStatus() llama.ServerStatus {
	return a.llamaEmbedServer.GetStatus()
}

func (a *App) isEmbeddingModelPath(modelPath string) bool {
	if a.modelStore == nil {
		return false
	}
	for _, m := range a.modelStore.ListLocalModels() {
		if m.Path == modelPath {
			return m.IsEmbedding
		}
	}
	return false
}

// ─── Internal Helpers ────────────────────────────────────────────

// autoStartEmbeddingModel finds the first embedding model in the local model store
// and starts it automatically. This ensures the GOB memory/RAG system works
// without requiring the user to manually start the embedding server.
func (a *App) autoStartEmbeddingModel() {
	models := a.modelStore.ListLocalModels()
	var embeddingPath string
	for _, m := range models {
		if m.IsEmbedding {
			embeddingPath = m.Path
			break
		}
	}
	if embeddingPath == "" {
		msg := "⚠️ No embedding model found — RAG will NOT function."
		log.Println(msg)
		a.emitEvent("memory:error", msg)
		return
	}

	log.Printf("Auto-starting embedding model: %s", embeddingPath)
	if err := a.StartEmbeddingModel(embeddingPath, -1); err != nil {
		msg := fmt.Sprintf("⚠️ Failed to auto-start embedding model: %v", err)
		log.Print(msg)
		a.emitEvent("memory:error", msg)
	} else {
		log.Println("✅ Embedding model auto-started — memory/RAG is active.")
	}
}

// startupEmbeddingModel ensures the embedding model is available and running.
// It is called during startup when memory is enabled and an embedding model
// is configured (cross-mode: API provider for chat + local embed).
func (a *App) startupEmbeddingModel() {
	repoID := a.cfg.Memory.EmbeddingModelRepo
	filename := a.cfg.Memory.EmbeddingModelFile
	modelsDir := a.cfg.Llama.ModelsDir
	modelPath := filepath.Join(modelsDir, filename)

	// Check if model already exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		log.Printf("Downloading embedding model: %s/%s ...", repoID, filename)
		if err := a.downloadFile(repoID, filename, modelPath); err != nil {
			log.Printf("WARN: failed to download embedding model: %v", err)
			a.emitEvent("memory:error", fmt.Sprintf("Embedding model indirme hatası: %v", err))
			return
		}
		log.Printf("Embedding model downloaded: %s", modelPath)
	}

	// Start the embedding server
	log.Printf("Auto-starting embedding model: %s", modelPath)
	if err := a.StartEmbeddingModel(modelPath, -1); err != nil {
		msg := fmt.Sprintf("Failed to start embedding model: %v", err)
		log.Print(msg)
		a.emitEvent("memory:error", msg)
	} else {
		log.Println("Cross-mode active: API provider for chat, local model for embeddings")
	}
}

// downloadFile downloads a GGUF model from HuggingFace synchronously.
func (a *App) downloadFile(repoID, filename, destPath string) error {
	downloadURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repoID, filename)

	req, err := http.NewRequestWithContext(a.ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	dlClient := &http.Client{Timeout: 0} // no timeout for large files
	resp, err := dlClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download status %d: %s", resp.StatusCode, string(body))
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	tmpPath := destPath + ".downloading"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("download write: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpPath, destPath); err != nil {
		// Cross-volume rename fallback: copy + delete
		if copyErr := copyFile(tmpPath, destPath); copyErr != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("rename and copy fallback both failed: rename: %w, copy: %v", err, copyErr)
		}
		os.Remove(tmpPath)
	}

	log.Printf("Downloaded %s (%d bytes)", destPath, written)
	return nil
}

// copyFile copies a file from src to dst (cross-device safe).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func (a *App) reinitMemoryStore(client *api.Client, model string) {
	embeddingFunc := memory.NewEmbeddingFunc(client, model)
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store != nil {
		newStore, err := memory.NewStore(memory.StoreConfig{
			Dir:           a.cfg.Memory.PersistDir,
			Dimension:     a.cfg.Memory.EmbeddingDimension,
			EmbeddingFunc: embeddingFunc,
		})
		if err != nil {
			log.Printf("WARN: memory re-init: %v", err)
		} else {
			a.store = newStore
		}
	}
}

func (a *App) buildMessages(ctx context.Context, userMsg string, extraImageB64 []string) []api.Message {
	start := time.Now()
	var memories []memory.MemoryResult
	if a.cfg.Memory.MemoryEnabled {
		memories = a.retrieveMemory(ctx, userMsg)
	}
	systemPrompt := a.identity.BuildSystemPrompt(memories)

	history := a.getSessionHistory()
	// Defensive copy: ensure mutations below never affect session data
	history = append([]api.Message{}, history...)
	var msgs []api.Message

	if a.llamaServer.IsRunning() {
		// Local models (e.g. Gemma) require strict user/assistant alternation —
		// no "system" role allowed. Inject system prompt into the first user turn.
		if len(history) == 0 {
			// First message: prepend system prompt to user message
			combinedMsg := systemPrompt + "\n\n" + userMsg
			if len(extraImageB64) > 0 {
				msgs = append(msgs, api.NewMultimodalMessage("user", combinedMsg, extraImageB64...))
			} else {
				msgs = append(msgs, api.NewTextMessage("user", combinedMsg))
			}
		} else {
			// Subsequent messages: inject system prompt into the very first user message in history
			injected := false
			for i, h := range history {
				if !injected && h.Role == "user" {
					content := systemPrompt + "\n\n" + h.GetTextContent()
					history[i] = api.NewTextMessage("user", content)
					injected = true
				}
			}
			msgs = append(msgs, history...)
			if len(extraImageB64) > 0 {
				msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, extraImageB64...))
			} else {
				msgs = append(msgs, api.NewTextMessage("user", userMsg))
			}
		}
	} else {
		msgs = append(msgs, api.NewTextMessage("system", systemPrompt))
		msgs = append(msgs, history...)
		if len(extraImageB64) > 0 {
			msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, extraImageB64...))
		} else {
			msgs = append(msgs, api.NewTextMessage("user", userMsg))
		}
	}

	log.Printf("LATENCY chat.build_messages total_ms=%d memories=%d history=%d messages=%d system_chars=%d", time.Since(start).Milliseconds(), len(memories), len(history), len(msgs), len(systemPrompt))
	return msgs
}

// buildConversationContext extracts conversation history from messages for orchestra chief.
// It returns the latest user message with preceding conversation context.
func buildConversationContext(messages []api.Message, userPrompt string) string {
	var sb strings.Builder

	// Collect previous non-system messages (up to last 6 exchanges = 12 messages)
	var prevMsgs []api.Message
	for i := len(messages) - 2; i >= 0; i-- {
		if messages[i].Role == "system" {
			continue
		}
		prevMsgs = append(prevMsgs, messages[i])
		if len(prevMsgs) >= 12 {
			break
		}
	}

	// Reverse to get chronological order
	for i := len(prevMsgs) - 1; i >= 0; i-- {
		msg := prevMsgs[i]
		roleLabel := "Kullanıcı"
		if msg.Role == "assistant" {
			roleLabel = "Asistan"
		}
		if text, ok := msg.Content.(string); ok {
			// Filter out orchestra debug/result lines (emoji-prefixed headers)
			cleanText := stripOrchestraLines(text)
			sb.WriteString(fmt.Sprintf("%s: %s\n", roleLabel, cleanText))
		}
	}

	if sb.Len() > 0 {
		// Prepend the context header
		ctx := fmt.Sprintf("Önceki konuşma:\n%s\n---\nYeni mesaj:\nKullanıcı: %s", sb.String(), userPrompt)
		return ctx
	}

	return fmt.Sprintf("Kullanıcı: %s", userPrompt)
}

var orchestraPrefixes = []string{"🎵", "🧙", "🧠", "✅", "❌", "📝"}

// stripOrchestraLines removes lines that start with orchestra debug prefixes.
func stripOrchestraLines(text string) string {
	lines := strings.Split(text, "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		skip := false
		for _, prefix := range orchestraPrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

func (a *App) getSessionHistory() []api.Message {
	sm := a.getSessionManager()
	if sm == nil {
		return nil
	}
	history := sm.GetHistoryForAPI(a.cfg.Llama.MaxHistory)
	var msgs []api.Message
	for _, h := range history {
		msgs = append(msgs, api.NewTextMessage(h["role"], h["content"]))
	}
	return msgs
}

func (a *App) retrieveMemory(ctx context.Context, query string) []memory.MemoryResult {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		log.Println("Memory: store not initialized, skipping retrieve")
		return nil
	}
	start := time.Now()
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	m, err := a.store.RetrieveContext(rctx, query, a.cfg.Memory.TopK, a.cfg.Memory.MinSimilarity)
	if err != nil {
		log.Printf("LATENCY app.retrieve_memory total_ms=%d status=error", time.Since(start).Milliseconds())
		log.Printf("MEMORY RETRIEVE FAILED: %v", err)
		a.emitEvent("memory:error", fmt.Sprintf("Hafıza okunamadı: %v", err))
		return nil
	}
	log.Printf("LATENCY app.retrieve_memory total_ms=%d returned=%d", time.Since(start).Milliseconds(), len(m))
	if len(m) > 0 {
		log.Printf("Memory: found %d relevant memories (best=%.0f%%)", len(m), m[0].Similarity*100)
	}
	return m
}

func (a *App) callLLM(ctx context.Context, messages []api.Message) string {
	// Orchestra mode takes priority
	if a.orchestraConductor != nil && a.orchestraConductor.Config().Enabled {
		var userPrompt string
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "user" {
				userPrompt = messages[i].GetTextContent()
				if userPrompt != "" {
					break
				}
			}
		}
		if userPrompt == "" {
			return "⚠️ No user message found"
		}
		conversationCtx := buildConversationContext(messages, userPrompt)
		octx, cancel := context.WithTimeout(ctx, 300*time.Second)
		defer cancel()
		finalResponse, _, err := a.orchestraConductor.Run(octx, conversationCtx)
		if err != nil {
			return "⚠️ " + err.Error()
		}
		return finalResponse
	}

	// Use external provider only if user explicitly selected one
	if a.activeProvider != "" && a.providerRouter != nil {
		pctx, cancel := context.WithTimeout(ctx, 300*time.Second)
		defer cancel()

		pMsgs := make([]provider.Message, len(messages))
		for i, m := range messages {
			pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
		}

		req := provider.ChatRequest{
			Messages:    pMsgs,
			Temperature: a.cfg.Llama.Temperature,
			TopP:        a.cfg.Llama.TopP,
			MaxTokens:   a.cfg.Llama.MaxTokens,
		}

		resp, err := a.providerRouter.ChatCompletion(pctx, req)
		if err != nil {
			log.Printf("Provider error: %v", err)
			return "⚠️ " + err.Error()
		}
		return resp.Content
	}

	// Fallback to local model
	lctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	start := time.Now()
	a.clientMu.RLock()
	llmClient := a.client
	a.clientMu.RUnlock()
	resp, err := llmClient.ChatCompletion(lctx, messages, a.cfg.Llama.Temperature, a.cfg.Llama.TopP, a.cfg.Llama.MaxTokens)
	if err != nil {
		log.Printf("LATENCY llm.complete total_ms=%d status=error messages=%d", time.Since(start).Milliseconds(), len(messages))
		log.Printf("LLM error: %v", err)
		return "⚠️ " + err.Error()
	}
	if len(resp.Choices) == 0 {
		log.Printf("LATENCY llm.complete total_ms=%d status=empty messages=%d", time.Since(start).Milliseconds(), len(messages))
		return "⚠️ Empty response"
	}

	reply := resp.Choices[0].Message.GetTextContent()
	log.Printf("LATENCY llm.complete total_ms=%d status=ok messages=%d reply_chars=%d", time.Since(start).Milliseconds(), len(messages), len(reply))
	log.Printf("<< Reply: %d chars", len(reply))
	return reply
}

func (a *App) saveMemoryAsync(userMsg, reply string) {
	if reply == "" || !a.cfg.Memory.MemoryEnabled {
		return
	}
	select {
	case a.memorySaveCh <- saveTask{userMsg: userMsg, reply: reply}:
	default:
		log.Println("WARN: memory save channel full, dropping")
	}
}

func (a *App) memorySaveWorker() {
	for task := range a.memorySaveCh {
		a.saveMemorySync(context.Background(), task.userMsg, task.reply)
	}
}

func (a *App) saveMemorySync(ctx context.Context, userMsg, reply string) {
	if !a.cfg.Memory.MemoryEnabled {
		return
	}
	start := time.Now()

	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		log.Println("MEMORY SAVE SKIPPED: store not initialized")
		a.emitEvent("memory:error", "Hafıza kaydedilemedi: depo başlatılmamış")
		return
	}

	mctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := a.store.SaveInteraction(mctx, userMsg, reply); err != nil {
		log.Printf("LATENCY app.memory_save_sync total_ms=%d status=error", time.Since(start).Milliseconds())
		log.Printf("MEMORY SAVE FAILED: %v", err)
		a.emitEvent("memory:error", fmt.Sprintf("Hafıza kaydedilemedi: %v", err))
	} else {
		log.Printf("LATENCY app.memory_save_sync total_ms=%d status=ok", time.Since(start).Milliseconds())
		log.Printf("Memory saved: %q → %d chars reply", truncateLog(userMsg, 60), len(reply))
		a.syncMu.RLock()
		sm := a.syncManager
		a.syncMu.RUnlock()
		if sm != nil {
			sm.Increment()
		}
	}
}

func truncateLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// CheckEmbeddingHealth tests if the embedding API is reachable and working.
func (a *App) CheckEmbeddingHealth(ctx context.Context) map[string]interface{} {
	result := map[string]interface{}{
		"ok":    false,
		"error": "",
		"count": 0,
	}

	a.storeMu.RLock()
	defer a.storeMu.RUnlock()

	if a.store == nil {
		result["error"] = "memory store not initialized"
		return result
	}

	result["count"] = a.store.Count()

	// Try a test embedding
	a.clientMu.RLock()
	client := a.client
	if a.embeddingClient != nil {
		client = a.embeddingClient
	}
	// Local copy of client pointer is safe from nil (copied under lock),
	// but embedding server may shut down after releasing lock — acceptable for MVP.
	a.clientMu.RUnlock()

	ectx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := client.CreateEmbedding(ectx, a.cfg.API.EmbeddingModel, "test")
	if err != nil {
		result["error"] = err.Error()
		log.Printf("EMBEDDING HEALTH CHECK FAILED: %v", err)
		return result
	}

	result["ok"] = true
	log.Printf("Embedding health: OK (model=%s, memories=%d)", a.cfg.API.EmbeddingModel, a.store.Count())
	return result
}

func detectMime(path string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	}
	// Use http.DetectContentType as fallback
	mime := http.DetectContentType(data)
	if mime == "application/octet-stream" {
		return "image/jpeg"
	}
	return mime
}

// ─── Backup / Restore (.memo) ──────────────────────────────────────────────────

// ExportData packages all user data (except models) into a .memo zip archive.
func (a *App) ExportData(includeModels bool) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Helper to add a file to the zip
	addFile := func(name, src string) error {
		fi, err := os.Stat(src)
		if err != nil {
			return nil // skip missing files
		}
		if fi.IsDir() {
			return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(src, path)
				w, err := zw.Create(filepath.Join(name, rel))
				if err != nil {
					return err
				}
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				defer f.Close()
				_, err = io.Copy(w, f)
				return err
			})
		}
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	}

	// Include: sessions, config, providers, orchestra, memory, whatsapp
	addFile("sessions/", "data/sessions")
	addFile("config/config.yaml", "config/config.yaml")
	addFile("data/providers.json", "data/providers.json")
	addFile("data/orchestra.json", "data/orchestra.json")
	addFile("data/memory/", "data/memory")
	addFile("data/whatsapp/", "data/whatsapp")
	if includeModels {
		addFile("data/models/", "data/models")
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ImportData restores user data from a .memo zip archive.
func (a *App) ImportData(data []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("import: invalid zip: %w", err)
	}

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		// Map zip entry paths to filesystem paths
		target := f.Name
		// sessions/* -> data/sessions/*
		if strings.HasPrefix(target, "sessions/") {
			target = filepath.Join("data", target)
		}

		// Safety: prevent path traversal
		clean := filepath.Clean(target)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			continue
		}

		// Create parent dir
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("import: mkdir: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("import: open %s: %w", f.Name, err)
		}

		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return fmt.Errorf("import: create %s: %w", target, err)
		}

		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return fmt.Errorf("import: write %s: %w", target, err)
		}
	}

	// Re-initialize components after import
	a.sessionsMu.Lock()
	if a.sessions != nil {
		sm, err := sessions.NewManager("data/sessions")
		if err == nil {
			a.sessions = sm
		}
	}
	a.sessionsMu.Unlock()

	a.storeMu.RLock()
	isStoreNil := a.store == nil
	a.storeMu.RUnlock()
	if isStoreNil {
		// memory store will be initialized in background goroutine
	}

	return nil
}

// WipeAllData removes all user data: sessions, memory, whatsapp, providers.
func (a *App) WipeAllData() error {
	dirs := []string{
		"data/sessions",
		"data/memory",
		"data/whatsapp",
		"data/providers.json",
		"data/orchestra.json",
	}
	for _, d := range dirs {
		if err := os.RemoveAll(d); err != nil {
			return fmt.Errorf("wipe: %s: %w", d, err)
		}
	}

	// Re-init sessions
	a.sessionsMu.Lock()
	if a.sessions != nil {
		sm, err := sessions.NewManager("data/sessions")
		if err == nil {
			a.sessions = sm
		}
	}
	a.sessionsMu.Unlock()

	// Reset memory store
	a.storeMu.Lock()
	a.store = nil
	a.storeMu.Unlock()

	log.Println("All user data wiped")
	return nil
}

// ─── Cloud Sync ───────────────────────────────────────────────────────────────

func (a *App) resolveSyncCredentials() (string, string) {
	clientID := strings.TrimSpace(a.cfg.Sync.ClientID)
	clientSecret := strings.TrimSpace(a.cfg.Sync.ClientSecret)
	if clientID == "" {
		clientID = strings.TrimSpace(os.Getenv("MEMO_GOOGLE_CLIENT_ID"))
	}
	if clientSecret == "" {
		clientSecret = strings.TrimSpace(os.Getenv("MEMO_GOOGLE_CLIENT_SECRET"))
	}
	return clientID, clientSecret
}

func (a *App) ensureSyncManager() error {
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm != nil {
		return nil
	}
	clientID, clientSecret := a.resolveSyncCredentials()
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("cloud sync OAuth credentials missing (set MEMO_GOOGLE_CLIENT_ID and MEMO_GOOGLE_CLIENT_SECRET in app environment)")
	}
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	if a.syncManager != nil {
		return nil // double-check after acquiring write lock
	}
	a.syncManager = cloudsync.New(
		a.ctx,
		a.cfg.Memory.PersistDir,
		a.cfg.Sync.Passphrase,
		a.cfg.Sync.IntervalMessages,
		clientID,
		clientSecret,
		a.cfg.Sync.TokenPath,
	)
	return nil
}

// CheckSyncAuth reports whether the cloud sync manager is authenticated.
func (a *App) CheckSyncAuth() bool {
	if err := a.ensureSyncManager(); err != nil {
		return false
	}
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm == nil {
		return false
	}
	return sm.IsAuthenticated()
}

// CheckAuth is an alias for CheckSyncAuth exposed for cloud sync UI logic.
func (a *App) CheckAuth() bool {
	return a.CheckSyncAuth()
}

// StartSyncAuth starts the OAuth2 loopback flow and returns the URL to open.
// The frontend should open this URL in the system browser. Poll CheckSyncAuth
// to detect when the user has completed the flow.
func (a *App) StartSyncAuth() (string, error) {
	if err := a.ensureSyncManager(); err != nil {
		return "", err
	}
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm == nil {
		return "", fmt.Errorf("sync manager not initialized")
	}
	url, err := sm.StartAuthFlow()
	if err != nil {
		return "", err
	}
	return url, nil
}

// TriggerSync forces an immediate backup upload outside the automatic 50-message cycle.
func (a *App) TriggerSync() {
	if err := a.ensureSyncManager(); err != nil {
		a.emitEvent("sync:error", err.Error())
		return
	}
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm != nil {
		sm.TriggerNow()
	}
}

// PullSync downloads latest cloud backup and restores local .gob files.
func (a *App) PullSync() {
	if err := a.ensureSyncManager(); err != nil {
		a.emitEvent("sync:error", err.Error())
		return
	}
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm != nil {
		sm.TriggerPullNow()
	}
}

// SyncNow runs push then pull in background.
func (a *App) SyncNow() {
	if err := a.ensureSyncManager(); err != nil {
		a.emitEvent("sync:error", err.Error())
		return
	}
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm != nil {
		sm.TriggerFullSyncNow()
	}
}

// GetSyncAccount returns Google account identity for the connected sync session.
func (a *App) GetSyncAccount() interface{} {
	if err := a.ensureSyncManager(); err != nil {
		return SyncAccount{Authenticated: false}
	}
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm == nil {
		return SyncAccount{Authenticated: false}
	}
	if !sm.IsAuthenticated() {
		return SyncAccount{Authenticated: false}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	acc, err := sm.GetAccountInfo(ctx)
	if err != nil {
		log.Printf("cloud sync account info: %v", err)
		return SyncAccount{Authenticated: true}
	}
	return SyncAccount{
		Authenticated: true,
		Name:          acc.Name,
		Email:         acc.Email,
	}
}

func (a *App) GetSyncSettings() interface{} {
	return a.cfg.Sync
}

func (a *App) UpdateSyncSettings(enabled bool, clientID, clientSecret, passphrase, tokenPath string, intervalMessages int) error {
	if tokenPath == "" {
		tokenPath = "./data/sync_token.json"
	}
	if intervalMessages <= 0 {
		intervalMessages = 50
	}

	a.cfg.Sync.Enabled = enabled
	a.cfg.Sync.ClientID = strings.TrimSpace(clientID)
	a.cfg.Sync.ClientSecret = strings.TrimSpace(clientSecret)
	a.cfg.Sync.Passphrase = passphrase
	a.cfg.Sync.TokenPath = strings.TrimSpace(tokenPath)
	a.cfg.Sync.IntervalMessages = intervalMessages

	if err := config.Save(a.cfg); err != nil {
		return err
	}

	// Re-create manager with fresh settings.
	resolvedClientID, resolvedClientSecret := a.resolveSyncCredentials()
	a.syncMu.Lock()
	if a.cfg.Sync.Enabled && resolvedClientID != "" && resolvedClientSecret != "" {
		a.syncManager = cloudsync.New(
			a.ctx,
			a.cfg.Memory.PersistDir,
			a.cfg.Sync.Passphrase,
			a.cfg.Sync.IntervalMessages,
			resolvedClientID,
			resolvedClientSecret,
			a.cfg.Sync.TokenPath,
		)
	} else {
		a.syncManager = nil
	}
	a.syncMu.Unlock()
	return nil
}

// DisconnectSync revokes the local OAuth token and resets the sync manager.
// The user will need to re-authenticate to use cloud sync again.
func (a *App) DisconnectSync() error {
	tokenPath := a.cfg.Sync.TokenPath
	if tokenPath == "" {
		tokenPath = "./data/sync_token.json"
	}
	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("disconnect sync: remove token: %w", err)
	}
	a.syncMu.Lock()
	a.syncManager = nil
	a.syncMu.Unlock()
	return nil
}

// GetEvents returns recent background events for the frontend to display.
func (a *App) GetEvents() []map[string]string {
	evs := a.events.snapshot()
	out := make([]map[string]string, len(evs))
	for i, e := range evs {
		out[i] = map[string]string{"name": e.Name, "data": e.Data}
	}
	return out
}

var _ webserver.FullBridge = (*App)(nil)

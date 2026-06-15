// Package app implements the Memo application core logic.
package app

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/calendar"
	"memo/internal/cloudsync"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/intent"
	"memo/internal/llama"
	"memo/internal/memory"
	"memo/internal/modelstore"
	"memo/internal/ngrok"
	"memo/internal/observer"
	"memo/internal/orchestra"
	"memo/internal/proactive"
	"memo/internal/provider"
	"memo/internal/sessions"
	"memo/internal/skill"
	"memo/internal/tunnel"
	"memo/internal/webserver"
	"memo/internal/whatsapp"
)

// ConnectionStatus holds the connection state and available model list.
type ConnectionStatus struct {
	Connected bool     `json:"connected"`
	Models    []string `json:"models"`
	Error     string   `json:"error,omitempty"`
}

// SyncAccount holds Google Drive account information for the cloud sync UI.
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

// App is the central application object.
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

	// Learning system (see docs/learning-system).
	observerStore    *observer.Store
	observerRecorder *observer.Recorder
	observerPatterns *observer.PatternStore
	observerAnalyzer *observer.Analyzer

	// Proactive engine — gated by cfg.Proactive (default off).
	proactivePending *proactive.PendingStore
	proactiveEngine  *proactive.Engine

	// Intent extraction and calendar system.
	intentExtractor *intent.Extractor
	learningMu      sync.RWMutex // protects intentExtractor reassignment
	calendarStore   *calendar.Store
	calendarRemind  *calendar.ReminderLoop

	agentExecutor *agent.Executor
	agentEnabled  bool
	agentMu       sync.RWMutex

	skillManager *skill.Manager

	providerMu sync.RWMutex // protects providerRouter, providerCfgMgr, activeProvider
	sessionsMu sync.RWMutex // protects sessions

	remoteAccessEnabled bool
	ngrokServer         *ngrok.Manager
	tailscaleTunnel     *tunnel.Tailscale

	whatsappChatMode bool
	whatsappChatMu   sync.RWMutex

	// Embedded binaries and version string passed in from main.
	binaries embed.FS
	version  string
}

// NewApp creates a new App instance. The binaries embed.FS and version string
// must be provided by the caller (typically main), because go:embed directives
// cannot reference paths outside the package directory.
func NewApp(binaries embed.FS, version string) *App {
	return &App{
		events:   &eventRing{},
		binaries: binaries,
		version:  version,
	}
}

// loadDotEnv reads a .env file and sets any unset environment variables from it.
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
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

// Startup initializes all application subsystems. It must be called once before
// any other method.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	loadDotEnv(".env")

	cfg, err := config.Load(config.ConfigFilePath())
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

	sm, err := sessions.NewManager(config.DataPath("sessions"))
	if err != nil {
		log.Printf("WARN: sessions: %v", err)
		a.emitEvent("sessions_manager_error", err.Error())
	}
	a.sessions = sm

	if obsStore, oerr := observer.NewStore(observer.StoreConfig{Dir: config.DataPath("profile")}); oerr != nil {
		log.Printf("WARN: observer: %v", oerr)
		a.emitEvent("observer_store_error", oerr.Error())
	} else {
		a.observerStore = obsStore
	}
	a.observerRecorder = observer.NewRecorder(a.observerStore)
	a.observerPatterns = observer.NewPatternStore(config.DataPath("profile", "patterns.json"))
	a.observerAnalyzer = observer.NewAnalyzer(a.observerStore, a.observerPatterns)
	if a.observerStore != nil {
		go a.runObserverAnalysis(ctx)
	}

	a.proactivePending = proactive.NewPendingStore(config.DataPath("profile", "pending.json"))
	a.proactiveEngine = proactive.NewEngine(
		proactive.Config{},
		a.observerPatterns,
		a.proactivePending,
		a.proactiveDecide,
		a.proactiveEmit,
		a.proactiveLevel,
	)
	go a.proactiveEngine.Start(ctx)

	a.initLearning(ctx)

	a.modelStore = modelstore.New(cfg.Llama.ModelsDir)
	a.llamaServer = llama.NewServer(cfg.Llama.Port, cfg.Llama.CtxSize)
	a.llamaEmbedServer = llama.NewServer(cfg.Llama.EmbeddingPort, 512)
	a.llamaInstaller = llama.NewInstaller(config.DataDir())

	a.memorySaveCh = make(chan saveTask, 64)
	go a.memorySaveWorker()

	if cfg.RemoteAccess.Enabled && cfg.RemoteAccess.NgrokMode && cfg.RemoteAccess.NgrokToken != "" {
		a.remoteAccessEnabled = true
		binPath, err := ngrok.Install(config.DataDir())
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

	// Tailscale tunnel auto-start (embedded, stable URL).
	go a.startupTailscale()

	if cfg.Memory.MemoryEnabled && cfg.Memory.EmbeddingAutoStart && cfg.Memory.EmbeddingModelRepo != "" && cfg.Memory.EmbeddingModelFile != "" && !a.llamaEmbedServer.IsRunning() {
		go a.startupEmbeddingModel()
	}

	if cfg.WhatsApp.Enabled {
		a.initWhatsApp()
	}

	a.providerCfgMgr = provider.NewConfigManager(config.DataPath("providers.json"), nil)
	configs := a.providerCfgMgr.GetEnabled()
	if len(configs) > 0 {
		a.providerRouter = provider.NewRouter(configs)
		go a.providerRouter.HealthCheck(ctx, 5*time.Minute)
		log.Printf("Provider system initialized with %d enabled provider(s)", len(configs))
		for _, cfg := range configs {
			log.Printf("  - %s (%s)", cfg.Type, cfg.Model)
		}
	} else {
		log.Println("No external providers configured, using local models")
	}

	a.activeProvider = provider.ProviderType(a.cfg.ActiveProvider)
	if a.activeProvider != "" && a.providerRouter != nil {
		a.providerRouter.SetActiveProvider(a.activeProvider)
	}
	if a.activeProvider != "" {
		log.Printf("Active provider restored from config: %s", a.activeProvider)
	}

	orchestraCfg := orchestra.LoadConfig(config.DataPath("orchestra.json"))
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

	basePath, _ := filepath.Abs(".")
	a.agentExecutor = agent.NewExecutor(basePath, a.providerRouter, a.providerCfgMgr)
	a.agentEnabled = false
	log.Printf("Agent mode initialized (enabled=false)")

	a.skillManager = skill.NewManager(config.DataDir())
	if err := a.skillManager.Discover(); err != nil {
		log.Printf("skill: discover error: %v", err)
	}
	log.Println("Skill manager initialized")

	log.Println("Memo ready")
}

// StartWebServerHTTP starts a plain HTTP API server for the Flutter desktop frontend.
func (a *App) StartWebServerHTTP(port int) {
	addr := "127.0.0.1"
	if a.remoteAccessEnabled {
		addr = "0.0.0.0"
	}
	a.webServer = webserver.New(a)
	if err := a.webServer.StartHTTPWithAddr(port, addr); err != nil {
		log.Printf("Flutter server: %v", err)
	}
}

// Shutdown cleans up all running background processes and servers.
func (a *App) Shutdown(ctx context.Context) {
	log.Println("Memo shutting down, cleaning up background processes...")

	close(a.memorySaveCh)

	if a.sttServer != nil && a.sttServer.Process != nil {
		sttKillProcessGroup(a.sttServer)
	}
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
	if a.tailscaleTunnel != nil {
		a.tailscaleTunnel.Stop()
	}
	if a.observerStore != nil {
		if err := a.observerStore.Close(); err != nil {
			log.Printf("observer shutdown: %v", err)
		}
	}
	if a.calendarStore != nil {
		if err := a.calendarStore.Close(); err != nil {
			log.Printf("calendar shutdown: %v", err)
		}
	}
	stopRecordingProcess()
}

// runObserverAnalysis runs the learning system's analysis loop.
func (a *App) runObserverAnalysis(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(30 * time.Second):
	}

	analyze := func() {
		if err := a.observerAnalyzer.Run(ctx); err != nil {
			log.Printf("OBSERVER: analysis: %v", err)
		}
		if a.observerStore != nil {
			if _, err := a.observerStore.Prune(time.Now().Add(-observer.AnalysisWindow)); err != nil {
				log.Printf("OBSERVER: prune: %v", err)
			}
		}
	}

	analyze()

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			analyze()
		}
	}
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

// Interface assertion: App must implement FullBridge.
var _ webserver.FullBridge = (*App)(nil)

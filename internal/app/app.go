// Package app implements the Memo application core logic.
package app

import (
	"context"
	"embed"
	"fmt"
	"memo/internal/logx"
	"os"
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
	moodpkg "memo/internal/mood"
	"memo/internal/ngrok"
	"memo/internal/observer"
	"memo/internal/orchestra"
	"memo/internal/proactive"
	"memo/internal/whisper"
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
	shutdownOnce      sync.Once          // guards Shutdown() against double-call
	lifecycleCtx      context.Context    // goroutine lifecycle only — NOT for request-scoped operations
	lifecycleCancel   context.CancelFunc // cancels lifecycleCtx on shutdown
	client            *api.Client
	clientMu          sync.RWMutex // protects client and embeddingClient reassignment
	store             *memory.Store
	storeMu           sync.RWMutex
	identity          *identity.Identity
	mood              *moodpkg.Engine
	cfg               *config.AppConfig
	sessions          *sessions.Manager
	incognitoMu       sync.RWMutex
	isIncognito       bool
	incognitoMessages []api.Message
	whisperServer      *whisper.Server
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

	providerCfgMgr      *provider.ConfigManager
	providerRouter      *provider.Router
	activeProviderName  string // which provider is currently active (by Name)
	healthCheckCancel   context.CancelFunc

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

	providerMu sync.RWMutex // protects providerRouter, providerCfgMgr, activeProviderName
	sessionsMu sync.RWMutex // protects sessions

	remoteAccessEnabled bool
	ngrokServer         *ngrok.Manager
	tailscaleTunnel     *tunnel.Tailscale

	whatsappChatMode bool
	whatsappChatMu   sync.RWMutex
	waMu             sync.Mutex // protects waClient, waMsgStore initialization

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
	logx.Printf("event: %s — %s", name, dataStr)
}

// Startup initializes all application subsystems. It must be called once before
// any other method.
func (a *App) Startup(ctx context.Context) {
	a.lifecycleCtx, a.lifecycleCancel = context.WithCancel(ctx)

	loadDotEnv(".env")

	cfg, err := config.Load(config.ConfigFilePath())
	if err != nil {
		logx.Printf("WARN: config: %v", err)
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
			logx.Printf("WARN: memory: %v", err)
			a.emitEvent("memory_store_error", err.Error())
			return
		}
		a.storeMu.Lock()
		a.store = store
		a.storeMu.Unlock()
		logx.Info("Memory store ready")
	}()

	a.identity = identity.New(cfg.Identity.UserName, cfg.Identity.AssistantName, cfg.Identity.Style, cfg.Identity.SystemRole)

	moodCfg := moodpkg.Config{
		Enabled:          a.cfg.Mood.Enabled,
		Alpha:            a.cfg.Mood.Alpha,
		Beta:             a.cfg.Mood.Beta,
		SigmaMin:         a.cfg.Mood.SigmaMin,
		SigmaMax:         a.cfg.Mood.SigmaMax,
		DBPath:           config.DataPath("mood", "mood.db"),
		SelfInterest:     a.cfg.Mood.SelfInterest,
		SystemManagement: a.cfg.Mood.SystemManagement,
	}
	if moodEngine, err := moodpkg.New(moodCfg); err != nil {
		logx.Printf("mood engine başlatılamadı (devre dışı): %v", err)
	} else {
		a.mood = moodEngine
	}

	sm, err := sessions.NewManager(config.DataPath("sessions"))
	if err != nil {
		logx.Printf("WARN: sessions: %v", err)
		a.emitEvent("sessions_manager_error", err.Error())
	}
	a.sessionsMu.Lock()
	a.sessions = sm
	a.sessionsMu.Unlock()

	if obsStore, oerr := observer.NewStore(observer.StoreConfig{Dir: config.DataPath("profile")}); oerr != nil {
		logx.Printf("WARN: observer: %v", oerr)
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

	// Buffered generously: a single worker drains this and each save can block up
	// to 10s on a slow embedding endpoint, so a burst of replies must not overflow
	// and silently drop interactions from long-term memory.
	a.memorySaveCh = make(chan saveTask, 1024)
	go a.memorySaveWorker()

	if cfg.RemoteAccess.Enabled && cfg.RemoteAccess.NgrokMode && cfg.RemoteAccess.NgrokToken != "" {
		a.remoteAccessEnabled = true
		binPath, err := ngrok.Install(config.DataDir())
		if err != nil {
			logx.Printf("[ngrok] Install error: %v", err)
		} else {
			mgr := ngrok.NewManager(binPath)
			if err := mgr.Start(cfg.RemoteAccess.Port, cfg.RemoteAccess.NgrokToken); err != nil {
				logx.Printf("[ngrok] Start error: %v", err)
			} else {
				a.ngrokServer = mgr
			}
		}
	}

	// Tailscale tunnel auto-start (embedded, stable URL).
	go a.startupTailscale()

	go a.startSTTServer()

	if cfg.Memory.MemoryEnabled && cfg.Memory.EmbeddingAutoStart && cfg.Memory.EmbeddingModelRepo != "" && cfg.Memory.EmbeddingModelFile != "" && !a.llamaEmbedServer.IsRunning() {
		go a.startupEmbeddingModel()
	}

	// Auto-init WhatsApp when explicitly enabled OR when a paired session
	// already exists on disk. The latter means a previously-connected user no
	// longer has to click "Connect" (and re-pair) after every app restart —
	// the client reconnects with the stored session and lands straight on the
	// chat list. A fresh install has no session, so the welcome screen still
	// shows until the user opts in.
	if cfg.WhatsApp.Enabled || a.whatsAppHasStoredSession() {
		a.waMu.Lock()
		a.initWhatsApp()
		a.waMu.Unlock()
	}

	a.providerCfgMgr = provider.NewConfigManager(config.DataPath("providers.json"), nil)
	configs := a.providerCfgMgr.GetEnabled()
	if len(configs) > 0 {
		a.providerRouter = provider.NewRouter(configs)
		hctx, hcancel := context.WithCancel(ctx)
		a.healthCheckCancel = hcancel
		go a.providerRouter.HealthCheck(hctx, 5*time.Minute)
		logx.Printf("Provider system initialized with %d enabled provider(s)", len(configs))
		for _, cfg := range configs {
			logx.Printf("  - %s (%s)", cfg.Type, cfg.Model)
		}
	} else {
		logx.Info("No external providers configured, using local models")
	}

	a.activeProviderName = a.cfg.ActiveProvider
	// Legacy configs persisted the provider *type* (e.g. "openrouter") instead of
	// the provider Name ("OpenRouter"). After the type→name identification change,
	// such a saved value matches no provider, silently dropping the user's
	// selection on upgrade. Normalize it back to the matching provider Name.
	if a.activeProviderName != "" {
		matchesName := false
		for _, p := range configs {
			if p.Name == a.activeProviderName {
				matchesName = true
				break
			}
		}
		if !matchesName {
			for _, p := range configs {
				if string(p.Type) == a.activeProviderName {
					a.activeProviderName = p.Name
					a.cfg.ActiveProvider = p.Name
					break
				}
			}
		}
	}
	if a.activeProviderName != "" && a.providerRouter != nil {
		a.providerRouter.SetActiveProvider(a.activeProviderName)
	}
	if a.activeProviderName != "" {
		logx.Printf("Active provider restored from config: %s", a.activeProviderName)
	}

	orchestraCfg := orchestra.LoadConfig(config.DataPath("orchestra.json"))
	a.orchestraConductor = orchestra.NewConductor(
		orchestraCfg,
		func(cfg provider.ProviderConfig) (provider.Provider, error) {
			if a.providerRouter == nil {
				return nil, fmt.Errorf("provider router not initialized, cannot create %s/%s", cfg.Type, cfg.Model)
			}
			p, ok := a.providerRouter.GetProvider(cfg.Name)
			if !ok {
				return nil, fmt.Errorf("provider %s not found in router (disabled or not configured), enable it in API Providers", cfg.Name)
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
	logx.Printf("Orchestra mode initialized (enabled=%v)", orchestraCfg.Enabled)

	basePath, _ := filepath.Abs(".")
	a.agentExecutor = agent.NewExecutor(basePath, a.providerRouter, a.providerCfgMgr)
	a.agentExecutor.SetBypassPermissions(a.cfg.Mood.SystemManagement)
	a.agentEnabled = false
	logx.Printf("Agent mode initialized (enabled=false)")

	a.skillManager = skill.NewManager(config.DataDir())
	if err := a.skillManager.Discover(); err != nil {
		logx.Printf("skill: discover error: %v", err)
	}
	logx.Info("Skill manager initialized")

	logx.Info("Memo ready")
}

// StartWebServerHTTP starts a plain HTTP API server for the Flutter desktop frontend.
func (a *App) StartWebServerHTTP(port int) {
	addr := "127.0.0.1"
	if a.remoteAccessEnabled {
		addr = "0.0.0.0"
	}
	a.webServer = webserver.New(a)
	if err := a.webServer.StartHTTPWithAddr(port, addr); err != nil {
		logx.Printf("Flutter server: %v", err)
	}
}

// Shutdown cleans up all running background processes and servers.
func (a *App) Shutdown(ctx context.Context) {
	a.shutdownOnce.Do(func() {
		logx.Info("Memo shutting down, cleaning up background processes...")

		// Cancel lifecycle context to stop all goroutines (proactive engine, calendar
		// reminders, WhatsApp intent loop, observer analysis, etc.)
		if a.lifecycleCancel != nil {
			a.lifecycleCancel()
		}

		close(a.memorySaveCh)

		a.storeMu.Lock()
		if a.store != nil {
			if err := a.store.Close(); err != nil {
				logx.Printf("memory store shutdown: %v", err)
			}
			a.store = nil
		}
		a.storeMu.Unlock()

		if a.whisperServer != nil {
			if err := a.whisperServer.Stop(); err != nil {
				logx.Printf("whisper shutdown: %v", err)
			}
		}
		if a.llamaServer != nil {
			if err := a.llamaServer.Stop(); err != nil {
				logx.Printf("llama chat shutdown: %v", err)
			}
		}
		if a.llamaEmbedServer != nil {
			if err := a.llamaEmbedServer.Stop(); err != nil {
				logx.Printf("llama embedding shutdown: %v", err)
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
				logx.Printf("observer shutdown: %v", err)
			}
		}
		if a.calendarStore != nil {
			if err := a.calendarStore.Close(); err != nil {
				logx.Printf("calendar shutdown: %v", err)
			}
		}
		if a.mood != nil {
			if err := a.mood.Close(); err != nil {
				logx.Printf("mood shutdown: %v", err)
			}
		}
		if a.webServer != nil {
			if err := a.webServer.Stop(); err != nil {
				logx.Printf("webserver shutdown: %v", err)
			}
		}
		stopRecordingProcess()
	})
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
			logx.Printf("OBSERVER: analysis: %v", err)
		}
		if a.observerStore != nil {
			if _, err := a.observerStore.Prune(time.Now().Add(-observer.AnalysisWindow)); err != nil {
				logx.Printf("OBSERVER: prune: %v", err)
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

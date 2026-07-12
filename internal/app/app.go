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
	"sync/atomic"
	"time"

	"memo/internal/agent"
	"memo/internal/agent/tools"
	"memo/internal/api"
	"memo/internal/calendar"
	"memo/internal/cloudsync"
	"memo/internal/config"
	"memo/internal/database"
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
	"memo/internal/taskloop"
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
	cfgMu             sync.RWMutex // protects a.cfg.Llama field reassignment (UpdateLlamaConfig writes vs concurrent reads)
	sessions          *sessions.Manager
	incognitoMu       sync.RWMutex
	isIncognito       bool
	incognitoMessages []api.Message
	whisperServer      *whisper.Server
	whisperMu          sync.RWMutex
	webServer         *webserver.Server
	webMu             sync.RWMutex
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
	memorySaveWg     sync.WaitGroup
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

	taskloopStore  *taskloop.Store
	taskloopEngine *taskloop.Engine
	// taskloopRunMu serializes taskloop worker calls: the worker needs to
	// switch the single global active session to the task list's chat and
	// force agent mode on, so two lists running at once must never overlap
	// a switch+send critical section or their messages would cross-talk into
	// each other's chats.
	taskloopRunMu sync.Mutex

	skillManager *skill.Manager

	providerMu sync.RWMutex // protects providerRouter, providerCfgMgr, activeProviderName
	sessionsMu sync.RWMutex // protects sessions

	remoteAccessEnabled bool
	ngrokServer         *ngrok.Manager
	tailscaleTunnel     *tunnel.Tailscale

	whatsappChatMode  atomic.Bool
	whatsAppSessionID string       // dedicated session for WhatsApp chat context
	waMu              sync.Mutex   // protects waClient, waMsgStore initialization
	streamMu          sync.Mutex   // prevents concurrent stream goroutines (double-send)

	clients clientRegistry // see clients.go — tracks attached CLI/GUI clients for auto-shutdown

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

	// Logged here (rather than from the database package's init()) so a
	// caller that redirects log output before calling Startup — like the
	// terminal REPL keeping backend logs out of the interactive session —
	// still catches it; init() always runs before that redirect is possible.
	database.LogStatus()

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

	// Closed once the placeholder store below has landed (success or
	// failure) — reconnectEmbeddingIfAlreadyRunning (started further down,
	// once a.llamaEmbedServer exists) waits on this so it always runs
	// *after*, never racing to overwrite a.store first only to have this
	// goroutine clobber it back to the placeholder afterward.
	memStoreReady := make(chan struct{})

	go func() {
		defer close(memStoreReady)
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

	a.identity = identity.New(cfg.Identity.UserName, cfg.Identity.AssistantName, cfg.Identity.Style, cfg.Identity.SystemRole, cfg.Identity.MinimalMode)

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
		go a.runObserverAnalysis(a.lifecycleCtx)
	}

	go a.startClientSweep(a.lifecycleCtx)

	a.proactivePending = proactive.NewPendingStore(config.DataPath("profile", "pending.json"))
	a.proactiveEngine = proactive.NewEngine(
		proactive.Config{},
		a.observerPatterns,
		a.proactivePending,
		a.proactiveDecide,
		a.proactiveEmit,
		a.proactiveLevel,
	)
	go a.proactiveEngine.Start(a.lifecycleCtx)

	a.initLearning(ctx)

	a.modelStore = modelstore.New(cfg.Llama.ModelsDir)
	a.llamaServer = llama.NewServer(cfg.Llama.Port, cfg.Llama.CtxSize)
	a.llamaEmbedServer = llama.NewServer(cfg.Llama.EmbeddingPort, 512)
	a.llamaInstaller = llama.NewInstaller(config.DataDir())

	// Buffered generously: a single worker drains this and each save can block up
	// to 10s on a slow embedding endpoint, so a burst of replies must not overflow
	// and silently drop interactions from long-term memory.
	a.memorySaveCh = make(chan saveTask, 1024)
	a.memorySaveWg.Add(1)
	go func() {
		defer a.memorySaveWg.Done()
		a.memorySaveWorker()
	}()

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

	go a.startSTTServer()

	if cfg.Memory.MemoryEnabled && cfg.Memory.EmbeddingAutoStart && cfg.Memory.EmbeddingModelRepo != "" && cfg.Memory.EmbeddingModelFile != "" && !a.llamaEmbedServer.IsRunning() {
		go a.startupEmbeddingModel()
	} else if cfg.Memory.MemoryEnabled {
		// EmbeddingAutoStart only gates launching a brand-new model process
		// (a resource/consent decision — may download a model). Reconnecting
		// to one that's already alive on the configured port — started by an
		// earlier backend process this one is attaching after, or started
		// manually before this Startup() ran — is a different, much
		// lower-risk action and shouldn't need the same opt-in. Without
		// this, memory silently keeps embedding through the a.client
		// placeholder wired above until something explicitly calls
		// StartEmbeddingModel (GUI's model dialog, or the CLI's
		// /embedding) — and GetStatus()'s own pingPort() fallback reports
		// "running" the entire time without ever actually reconnecting
		// anything, misleading whoever queries it (the CLI's welcome
		// banner in particular: shows embedding as on while memory
		// search/save both silently operate on the wrong client).
		go func() {
			<-memStoreReady
			a.reconnectEmbeddingIfAlreadyRunning()
		}()
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

	// Shared with reinitProviderAndOrchestra (providers.go), which ImportData
	// and cloud restore call after replacing providers.json/orchestra.json on
	// disk — keeping this as one code path means the two can't drift apart.
	a.reinitProviderAndOrchestra()
	tools.Configurator = a

	basePath, _ := filepath.Abs(".")
	a.agentExecutor = agent.NewExecutor(basePath, a.providerRouter, a.providerCfgMgr)
	a.agentExecutor.SetBypassPermissions(a.cfg.Mood.SystemManagement)
	a.agentEnabled = false
	logx.Printf("Agent mode initialized (enabled=false)")

	tlStore, err := taskloop.NewStore(config.DataPath("tasklists"))
	if err != nil {
		logx.Printf("WARN: taskloop store: %v", err)
	} else {
		a.taskloopStore = tlStore
		a.taskloopEngine = taskloop.NewEngine(
			tlStore,
			a.buildTaskLoopRunWorker(),
			a.buildTaskLoopReviewChief(),
			func(v bool) { a.agentExecutor.SetBypassPermissions(v) },
			func(name, data string) { a.emitEvent(name, data) },
		)
		logx.Info("Task loop engine initialized")
	}

	a.skillManager = skill.NewManager(config.DataDir())
	if err := a.skillManager.Discover(); err != nil {
		logx.Printf("skill: discover error: %v", err)
	}
	a.skillManager.SetToolRegistrar(newSkillToolRegistrar(a.agentExecutor.Registry(), a.skillManager))
	logx.Info("Skill manager initialized")

	// Auto-start the embedding model at boot, not just after a local chat
	// model loads — a user routing chat through an external provider (no
	// local model ever started via StartLocalModel) would otherwise never
	// get an embedding server, silently breaking RAG/memory retrieval.
	// Backgrounded since WaitReady can take a while and must not delay the
	// rest of Startup.
	if a.GetMemoryEnabled() {
		go func() {
			if !a.llamaEmbedServer.IsRunning() {
				a.autoStartEmbeddingModel()
			}
		}()
	}

	logx.Info("Memo ready")
}

// StartWebServerHTTP starts a plain HTTP API server for the Flutter desktop frontend.
// Returns an error if the port is already in use (e.g. another Memo instance is running).
func (a *App) StartWebServerHTTP(port int) error {
	addr := "127.0.0.1"
	if a.remoteAccessEnabled {
		addr = "0.0.0.0"
	}
	ws := webserver.New(a)
	if err := ws.StartHTTPWithAddr(port, addr); err != nil {
		return fmt.Errorf("cannot start server on %s:%d: %w", addr, port, err)
	}
	a.webMu.Lock()
	a.webServer = ws
	a.webMu.Unlock()

	// Tailscale tunnel auto-start (embedded, stable URL). Must run after
	// a.webServer is set: startupTailscale checks getWebServer().IsRunning()
	// to proxy to the actual bound port, and previously ran from Startup()
	// — which always completes and returns *before* this method is even
	// called (see main.go) — so getWebServer() was always nil and the
	// auto-start silently never did anything.
	go a.startupTailscale()

	return nil
}

// shutdownTimeout bounds Shutdown()'s total worst case. If graceful cleanup
// (a wedged subprocess, a stuck network call in a queued memory save, etc.)
// doesn't finish in time, the process force-exits anyway — closing the app
// must always actually close it, not hang around indefinitely.
//
// Both vars (not consts) so a test can shrink the timeout and swap in a
// non-terminating hook to exercise the "timeout wins the race" branch
// without actually killing the test binary.
var (
	shutdownTimeout   = 15 * time.Second
	shutdownForceExit = os.Exit
)

// Shutdown cleans up all running background processes and servers.
func (a *App) Shutdown(ctx context.Context) {
	a.shutdownOnce.Do(func() {
		done := make(chan struct{})
		go func() {
			defer close(done)
			a.shutdownSync(ctx)
		}()

		select {
		case <-done:
			logx.Info("Memo shutdown completed")
		case <-time.After(shutdownTimeout):
			logx.Printf("WARN: shutdown exceeded %v, forcing exit", shutdownTimeout)
			shutdownForceExit(1)
		}
	})
}

// shutdownSync performs the actual cleanup. Split out from Shutdown so the
// watchdog above can bound it without recursing into shutdownOnce.
func (a *App) shutdownSync(ctx context.Context) {
	logx.Info("Memo shutting down, cleaning up background processes...")

	// Stop all running task lists so bypass permissions are restored before
	// the rest of shutdown tears things down.
	if a.taskloopEngine != nil {
		tlInfos := a.taskloopStore.List()
		for _, info := range tlInfos {
			if info.Status == "running" {
				a.taskloopEngine.Stop(info.ID)
			}
		}
	}

	// Cancel lifecycle context to stop all goroutines (proactive engine, calendar
	// reminders, WhatsApp intent loop, observer analysis, etc.)
	if a.lifecycleCancel != nil {
		a.lifecycleCancel()
	}

	a.webMu.RLock()
	webSrv := a.webServer
	a.webMu.RUnlock()
	if webSrv != nil {
		if err := webSrv.Stop(); err != nil {
			logx.Printf("webserver shutdown: %v", err)
		}
	}

	// Now that all HTTP handlers (including streaming goroutines) have
	// finished, it is safe to close memorySaveCh — no more sends will occur.
	close(a.memorySaveCh)
	a.memorySaveWg.Wait()

	a.storeMu.Lock()
	if a.store != nil {
		if err := a.store.Close(); err != nil {
			logx.Printf("memory store shutdown: %v", err)
		}
		a.store = nil
	}
	a.storeMu.Unlock()

	// These subsystems are independent of each other, so stop them
	// concurrently instead of one after another. Each already has its own
	// bounded graceful-then-force-kill window (e.g. llama.Server.Stop()'s
	// 5s) — running them in parallel rather than sequentially is what keeps
	// a normal shutdown fast even with several active at once.
	var wg sync.WaitGroup
	stop := func(name string, fn func() error) {
		if fn == nil {
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				logx.Printf("%s shutdown: %v", name, err)
			}
		}()
	}

	a.whisperMu.RLock()
	ws := a.whisperServer
	a.whisperMu.RUnlock()
	if ws != nil {
		stop("whisper", ws.Stop)
	}
	if a.llamaServer != nil {
		stop("llama chat", a.llamaServer.Stop)
	}
	if a.llamaEmbedServer != nil {
		stop("llama embedding", a.llamaEmbedServer.Stop)
	}
	if a.ngrokServer != nil {
		ngrokServer := a.ngrokServer
		a.ngrokServer = nil
		stop("ngrok", ngrokServer.Stop)
	}
	if a.tailscaleTunnel != nil {
		stop("tailscale", func() error { a.tailscaleTunnel.Stop(); return nil })
	}
	if a.observerStore != nil {
		stop("observer", a.observerStore.Close)
	}
	if a.calendarStore != nil {
		stop("calendar", a.calendarStore.Close)
	}
	if a.mood != nil {
		stop("mood", a.mood.Close)
	}
	wg.Wait()

	stopRecordingProcess()
}

// getWebServer returns the current web server under read lock.
func (a *App) getWebServer() *webserver.Server {
	a.webMu.RLock()
	defer a.webMu.RUnlock()
	return a.webServer
}

// setWebServer atomically sets the web server under write lock.
func (a *App) setWebServer(s *webserver.Server) {
	a.webMu.Lock()
	a.webServer = s
	a.webMu.Unlock()
}

// runObserverAnalysis runs the learning system's analysis loop.
func (a *App) runObserverAnalysis(ctx context.Context) {
	// Wait 30s before the first analysis pass, but react to shutdown
	// immediately rather than sitting on the timer — a plain time.After
	// case in the select below would technically still respond right away
	// (select doesn't block behind the "slower" case), but it also leaks
	// the timer for up to 30s after we've already returned. Use an explicit
	// timer we can stop so ctx.Done() gets a prompt, leak-free response.
	startupTimer := time.NewTimer(30 * time.Second)
	defer startupTimer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-startupTimer.C:
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

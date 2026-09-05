// Package app implements the Memo application core logic.
package app

import (
	"context"
	"embed"
	"fmt"
	"memo/internal/logx"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"memo/internal/agent"
	"memo/internal/agent/tools"
	"memo/internal/api"
	"memo/internal/browserengine"
	"memo/internal/calendar"
	"memo/internal/cloudsync"
	"memo/internal/config"
	"memo/internal/database"
	"memo/internal/identity"
	"memo/internal/intent"
	"memo/internal/livemode"
	"memo/internal/llama"
	"memo/internal/memory"
	"memo/internal/modelstore"
	moodpkg "memo/internal/mood"
	"memo/internal/ngrok"
	"memo/internal/observer"
	"memo/internal/orchestra"
	"memo/internal/proactive"
	"memo/internal/provider"
	"memo/internal/remoteauth"
	"memo/internal/routine"
	"memo/internal/sessions"
	"memo/internal/skill"
	"memo/internal/skills"
	"memo/internal/stats"
	"memo/internal/stt"
	"memo/internal/swarm"
	"memo/internal/taskloop"
	"memo/internal/telegram"
	"memo/internal/tts"
	"memo/internal/tunnel"
	"memo/internal/websearch"
	"memo/internal/webserver"
	"memo/internal/whatsapp"
	"memo/internal/whisper"
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
	// Seq is a monotonically increasing, per-process counter assigned at
	// push time — never reused, unlike Name+Data which is frequently
	// identical across events (every memory:saved carries the same empty
	// Data). Callers that need "did X happen after this point in time"
	// must key off Seq, not struct equality: two events with equal
	// Name+Data are NOT necessarily the same occurrence, and treating them
	// as interchangeable silently collapses distinct events (see
	// replcli.memorySavedSince's history for the bug this caused).
	Seq uint64 `json:"seq"`
}

// eventRing is a fixed-size ring buffer of recent events.
type eventRing struct {
	mu      sync.Mutex
	buf     [64]AppEvent
	pos     int // next write position
	full    bool
	nextSeq uint64
}

func (r *eventRing) push(e AppEvent) {
	r.mu.Lock()
	r.nextSeq++
	e.Seq = r.nextSeq
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
	shutdownOnce         sync.Once          // guards Shutdown() against double-call
	lifecycleCtx         context.Context    // goroutine lifecycle only — NOT for request-scoped operations
	lifecycleCancel      context.CancelFunc // cancels lifecycleCtx on shutdown
	client               *api.Client
	clientMu             sync.RWMutex // protects client and embeddingClient reassignment
	store                *memory.Store
	storeMu              sync.RWMutex
	identity             *identity.Identity
	mood                 *moodpkg.Engine
	cfg                  *config.AppConfig
	cfgMu                sync.RWMutex // protects a.cfg.Llama field reassignment (UpdateLlamaConfig writes vs concurrent reads)
	sessions             *sessions.Manager
	incognitoMu          sync.RWMutex
	isIncognito          bool
	incognitoMessages    []api.Message
	whisperServer        *whisper.Server
	whisperMu            sync.RWMutex
	ttsSynthesizer       *tts.Synthesizer
	ttsFillerCache       *tts.FillerCache
	ttsMu                sync.RWMutex
	ttsProviderCfgMgr    *tts.ConfigManager
	ttsRouter            *tts.Router
	ttsRouterMu          sync.RWMutex
	ttsVoiceStore        *tts.VoiceStore
	sttProviderCfgMgr    *stt.ConfigManager
	sttRouter            *stt.Router
	sttRouterMu          sync.RWMutex
	liveModeEngineCfgMgr *livemode.ConfigManager
	liveModeMu           sync.RWMutex
	// Live Mode delegation (Phase 9, PLAN_live_mode_v2.md §4) — deliberately
	// separate locks from liveModeMu (config) and a.streamMu (interactive
	// chat streaming): see livemode_delegate.go's doc comments.
	liveModeChatID string
	liveModeChatMu sync.Mutex
	liveJobsMu     sync.Mutex
	liveJobs       map[string]context.CancelFunc
	// livePermMu/livePendingPermAnswerCh: while non-nil, the next transcript
	// routed via routeLiveTranscriptToPermissionAnswer is treated as the
	// spoken answer to an outstanding voice_prompt permission question
	// rather than ordinary conversation — mirrors waPendingPermAnswerCh/
	// waPendingPermChatJID, minus the chatJID match since Live Mode only
	// ever has one active session at a time. See livemode_session_wrapper.go.
	livePermMu              sync.Mutex
	livePendingPermAnswerCh chan string
	webServer               *webserver.Server
	webMu                   sync.RWMutex
	modelStore              *modelstore.Store

	waClient         *whatsapp.Client
	waMsgStore       *whatsapp.Store
	tgClient         *telegram.Client
	tgStore          *telegram.Store
	llamaServer      *llama.Server
	llamaEmbedServer *llama.Server // dedicated embedding model server
	// swarmServer is a separate llama-server used only as the Memo Swarm
	// coordinator. Kept independent of llamaServer so starting/stopping a
	// swarm never tears down the user's normal chat model (and vice versa).
	swarmServer     *llama.Server
	llamaInstaller  *llama.Installer
	originalBaseURL string      // stores the original API base URL before llama override
	embeddingClient *api.Client // separate client for embedding server
	syncManager     *cloudsync.Manager
	syncMu          sync.RWMutex
	memorySaveCh    chan saveTask
	memorySaveWg    sync.WaitGroup
	events          *eventRing

	// factExtractBuf batches user messages for auto fact-extraction so the
	// narrow extraction LLM call fires once per Memory.FactExtractionEveryNTurns
	// turns instead of after every single saved turn. See queueFactExtraction.
	factExtractMu  sync.Mutex
	factExtractBuf []string

	providerCfgMgr     *provider.ConfigManager
	providerRouter     *provider.Router
	activeProviderName string // which provider is currently active (by Name)
	healthCheckCancel  context.CancelFunc

	orchestraConductor *orchestra.Conductor

	// Learning system (see docs/learning-system).
	observerStore    *observer.Store
	observerRecorder *observer.Recorder
	observerPatterns *observer.PatternStore
	observerAnalyzer *observer.Analyzer

	// Proactive engine — gated by cfg.Proactive (default off).
	proactivePending *proactive.PendingStore
	proactiveEngine  *proactive.Engine

	// Ambient nudges — see proactive_ambient.go. lastNudgedPattern is set by
	// buildProactiveNudgeBlock (called synchronously while assembling a
	// turn's system prompt) and taken (read + cleared) synchronously by
	// finishStream via takeNudgedPattern, on the same call path as that
	// turn's reply — never read from inside the backgrounded
	// checkAmbientNudgeSurfaced goroutine itself, which would race against a
	// fast-enough next turn's buildProactiveNudgeBlock. resolvingSuggestionID
	// guards checkAmbientNudgeOutcome against double-processing the same
	// pending suggestion from two rapid, overlapping user messages.
	proactiveNudgeMu      sync.Mutex
	lastNudgedPattern     *observer.TimePattern
	resolvingSuggestionID string

	// Intent extraction and calendar system.
	intentExtractor *intent.Extractor
	learningMu      sync.RWMutex // protects intentExtractor reassignment
	calendarStore   *calendar.Store
	calendarRemind  *calendar.ReminderLoop

	routineStore *routine.Store
	routineLoop  *routine.RoutineLoop

	statsStore *stats.Store

	devGatewayLog gatewayLog

	agentExecutor *agent.Executor
	agentEnabled  bool
	agentMu       sync.RWMutex

	// webSearchExecutor is a single-tool (web_search only) agent executor
	// used by routeStream's non-agent "web search mode" — see
	// agent.NewWebSearchExecutor's doc comment.
	webSearchExecutor *agent.Executor

	// browserMgr manages the optional headless-browser fallback fetch_page
	// uses for JavaScript-rendered pages (see internal/browserengine's doc
	// comment). Wired into internal/websearch.Browser in Startup().
	browserMgr *browserengine.Manager

	taskloopStore  *taskloop.Store
	taskloopEngine *taskloop.Engine

	// taskRunCfgs holds each running task list's private provider/model
	// snapshot + executor (see tasklist_run.go). Keyed by list ID; populated
	// lazily by the engine's WorkerConfigHook, cleared when a list finishes
	// or pauses so a resume re-snapshots.
	taskRunMu   sync.RWMutex
	taskRunCfgs map[string]*taskRunConfig

	taskNotifyBus *taskloop.NotifyBus
	// taskNotifyQ decouples the engine goroutine from Telegram/WhatsApp
	// delivery — the pump (task_notify.go) drains it in order, one bounded
	// send at a time. Without it a stalled push froze the whole task loop.
	taskNotifyQ chan taskloop.Notification
	taskFocus   taskFocusState

	// task-loop event fan-out to chat SSE clients (GET /api/tasks/events).
	taskEventMu   sync.RWMutex
	taskEventSubs []chan string

	skillManager *skill.Manager

	providerMu sync.RWMutex // protects providerRouter, providerCfgMgr, activeProviderName
	sessionsMu sync.RWMutex // protects sessions

	remoteAccessEnabled bool
	ngrokServer         *ngrok.Manager
	tailscaleTunnel     *tunnel.Tailscale

	whatsappChatMode    atomic.Bool
	whatsAppSessionID   string     // dedicated session for WhatsApp chat context
	waSelfChatSessionID string     // dedicated session for the WhatsApp self-chat assistant (see runWhatsAppIntentLoop)
	waMu                sync.Mutex // protects waClient, waMsgStore initialization
	// waPendingPermAnswerCh/waPendingPermChatJID: while non-nil, the next
	// incoming message on that exact chat JID is a y/n answer to an
	// outstanding agent permission question (see selfchat_permission.go),
	// not a new chat turn — set by awaitWhatsAppPermissionAnswer, read by
	// routeWhatsAppPermissionAnswer in runWhatsAppIntentLoop. Both guarded
	// by waMu, same as the rest of this block.
	waPendingPermAnswerCh chan string
	waPendingPermChatJID  string
	tgSelfChatSessionID   string     // dedicated session for the Telegram bot assistant (see runTelegramIntentLoop)
	tgMu                  sync.Mutex // protects tgClient, tgStore initialization
	// tgPendingPermAnswerCh/tgPendingPermChatID: Telegram's counterpart to
	// waPendingPermAnswerCh/waPendingPermChatJID above, guarded by tgMu.
	tgPendingPermAnswerCh chan string
	tgPendingPermChatID   int64
	// streamMu is now only a gate, not the stream lock (v4.6.0 Faz A, see
	// chat_locks.go): interactive and task streams take it RLock so many
	// chats stream at once; the routine and incognito paths take it Lock
	// (exclusive) because they flip a global flag. Per-chat serialisation
	// lives in chatStreamLocks.
	streamMu        sync.RWMutex
	chatStreamMu    sync.Mutex
	chatStreamLocks map[string]*sync.Mutex

	// cliJobs tracks in-flight CLI-backed background streams (see
	// cli_stream.go), keyed by chat id. Deliberately separate from streamMu
	// above: a CLI task can run far longer than a normal reply (yapacam.md
	// §7, no fixed timeout) and must not hold the single global stream slot
	// for that whole time — every other chat in the app would be blocked
	// from sending anything until it finished. Per-chat exclusivity only
	// (two messages can't race into the same chat), never global.
	cliJobsMu sync.Mutex
	cliJobs   map[string]context.CancelFunc

	// bgLLMCancel cancels whatever background (non-chat) local-model call is
	// currently in flight — auto fact extraction today. See preemptBackgroundLLM
	// (llm.go): a real chat message about to hit the local model (which runs
	// with a single inference slot) preempts this first, instead of queueing
	// behind it (BUG_REPORT TD-2). bgLLMCtx is kept alongside so a call's own
	// cleanup can tell whether it's still the one registered (by pointer
	// identity) before clearing/cancelling — otherwise an older call's
	// deferred cleanup could clobber a newer, still-running one's cancel func.
	bgLLMMu     sync.Mutex
	bgLLMCtx    context.Context
	bgLLMCancel context.CancelFunc

	clients clientRegistry // see clients.go — tracks attached CLI/GUI clients for auto-shutdown

	// Memo Swarm (Beta) — host room + worker rpc-server. See PLAN_memo_swarm.md
	// and internal/app/swarm.go. Zero-init only at Startup; heavy pieces
	// (rpc-server, coordinator llama-server) are lazy.
	swarmMu            sync.Mutex
	swarmCoordinator   *swarm.Coordinator
	swarmWorker        *swarm.RPCWorker
	swarmJoinCode      string // room code this machine joined with (worker side)
	swarmJoinHost      string // decoded host address from that code
	swarmJoinConnected bool   // true after a successful register POST

	// Embedded binaries and version string passed in from main.
	binaries embed.FS
	version  string

	// Remote-access auth (Faz 2, yapacam.md) — see remote_auth.go.
	// remoteAuthLimiter guards password-login brute-forcing; sessionKey
	// signs/validates the short-lived JWTs a successful password login
	// issues. Both are process-lifetime singletons, lazily initialized on
	// first use rather than in NewApp/Startup, since most installs never
	// touch password auth at all (default AuthMode is "token").
	remoteAuthMu      sync.Mutex
	remoteAuthLimiter *remoteauth.Limiter
	sessionKey        []byte
	// remoteDevicesMu protects a.cfg.RemoteAccess.Devices specifically: it's
	// mutated on every authenticated remote request (VerifyRemoteDeviceToken
	// touches LastSeenAt), unlike the rest of a.cfg's RemoteAccess fields,
	// which only change on rare, user-initiated admin actions.
	remoteDevicesMu sync.Mutex
	// pendingDeviceToken holds a just-minted device token's plaintext for
	// exactly one GetRemoteAccessStatus call (see SetRemoteAccess's
	// auto-provision-on-first-enable) — read-and-cleared, never persisted.
	pendingDeviceToken string
	// remoteAccountsMu protects a.cfg.RemoteAccess.Accounts (Faz 5.1,
	// yapacam.md multi-user/role model) — separate from remoteDevicesMu
	// since accounts and devices are independent lists mutated by
	// independent request paths.
	remoteAccountsMu sync.Mutex

	// Caches this install's identity (see install_id.go) so the
	// unauthenticated /api/setup/status poll doesn't hit the disk on
	// every client tick.
	installIDMu  sync.Mutex
	installIDVal string

	// In-memory, process-lifetime registry of files staged for the
	// share_file agent tool's frontend/normal-chat delivery path (a
	// download link) — see sendfile.go. Deliberately not persisted: a
	// backend restart simply invalidates any outstanding links, which is
	// an acceptable cost for what's meant to be a short-lived handoff, not
	// a durable file store.
	outboxMu sync.Mutex
	outbox   map[string]outboxEntry
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
	if a.events != nil {
		a.events.push(AppEvent{Name: name, Data: dataStr})
	}
	logx.Printf("event: %s — %s", name, dataStr)
}

// recoverPanic logs and swallows a panic in a background goroutine. An
// unrecovered panic in ANY goroutine — not just main's — crashes the entire
// process (unlike a per-request net/http handler, Go gives no free recover
// for goroutines spawned with a bare `go`), so every long-running or
// fire-and-forget goroutine this package starts needs one of these (or
// recoverStreamPanic, llm.go's streaming-aware counterpart that also signals
// the error to a listening client) as one of its deferred calls. label
// identifies which goroutine panicked in the log.
func recoverPanic(label string) {
	if r := recover(); r != nil {
		logx.Printf("PANIC in %s: %v\n%s", label, r, string(debug.Stack()))
	}
}

// goRecover starts fn in a new goroutine with recoverPanic deferred around
// it, so a panic anywhere in fn's call chain — including one that reaches
// into another package this one calls into (proactive.Engine.Start,
// routine.RoutineLoop.Start, llama.Server's monitor loop, etc.) — is logged
// and swallowed instead of crashing the whole process. Use this instead of a
// bare `go` for any call that isn't already a `go func() { defer
// recoverPanic(...); ... }()` closure with its own recover.
func goRecover(label string, fn func()) {
	go func() {
		defer recoverPanic(label)
		fn()
	}()
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
		defer recoverPanic("Startup/memory.NewStore")
		store, err := memory.NewStore(memory.StoreConfig{
			Dir:                 cfg.Memory.PersistDir,
			Dimension:           cfg.Memory.EmbeddingDimension,
			EmbeddingFunc:       embeddingFunc,
			RecencyHalfLifeDays: cfg.Memory.RecencyHalfLifeDays,
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
	a.identity.SetLearnedStyleNotes(cfg.Identity.LearnedStyleNotes)
	a.identity.SetMinimalModeOverrides(
		cfg.Identity.MinimalModeKeepPersona,
		cfg.Identity.MinimalModeKeepCapabilities,
		cfg.Identity.MinimalModeKeepPassive,
		cfg.Identity.MinimalModeKeepProactive,
	)

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
		goRecover("runObserverAnalysis", func() { a.runObserverAnalysis(a.lifecycleCtx) })
	}

	goRecover("startClientSweep", func() { a.startClientSweep(a.lifecycleCtx) })

	a.proactivePending = proactive.NewPendingStore(config.DataPath("profile", "pending.json"))
	a.proactiveEngine = proactive.NewEngine(
		proactive.Config{},
		a.observerPatterns,
		a.proactivePending,
		a.proactiveDecide,
		a.proactiveEmit,
		a.proactiveLevel,
	)
	goRecover("proactiveEngine.Start", func() { a.proactiveEngine.Start(a.lifecycleCtx) })

	a.initLearning(ctx)
	a.initRoutines(ctx)
	a.initStats()
	a.initSwarm()

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
		defer recoverPanic("memorySaveWorker")
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

	goRecover("startSTTServer", a.startSTTServer)
	a.initTTS()
	a.initTTSProviders()
	a.initSTTProviders()
	a.initLiveModeEngines()
	a.ttsVoiceStore = tts.NewVoiceStore(config.DataPath("tts_voices"))

	if cfg.Memory.MemoryEnabled && cfg.Memory.EmbeddingAutoStart && cfg.Memory.EmbeddingModelRepo != "" && cfg.Memory.EmbeddingModelFile != "" && !a.llamaEmbedServer.IsRunning() {
		goRecover("startupEmbeddingModel", a.startupEmbeddingModel)
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
			defer recoverPanic("reconnectEmbeddingIfAlreadyRunning")
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

	// Auto-reconnect Telegram only when the user had it explicitly enabled
	// (unlike WhatsApp, a saved-but-paused bot token must NOT silently
	// resume polling on every restart — see StopTelegram's doc comment).
	a.tgMu.Lock()
	a.tgStore = telegram.NewStore(config.DataPath("telegram.json"), nil)
	if a.telegramHasStoredToken() {
		a.initTelegram()
	}
	a.tgMu.Unlock()

	// Shared with reinitProviderAndOrchestra (providers.go), which ImportData
	// and cloud restore call after replacing providers.json/orchestra.json on
	// disk — keeping this as one code path means the two can't drift apart.
	a.reinitProviderAndOrchestra()
	tools.Configurator = a
	tools.Routines = routineToolAdapter{a}
	tools.FileSender = fileToolAdapter{a}
	tools.SelfDrivingTasks = selfDrivingTaskToolAdapter{a}
	tools.TaskMdEditor = taskMdEditorAdapter{a}
	tools.TaskStatus = taskStatusToolAdapter{a}

	basePath, _ := filepath.Abs(".")
	a.agentExecutor = agent.NewExecutor(basePath, a.providerRouter, a.providerCfgMgr, a.getSessionManager())
	a.agentExecutor.SetBypassPermissions(a.cfg.Mood.SystemManagement)
	// Restored from config rather than hardcoded off: the toggle is
	// persisted by SetAgentEnabled now (see config.AgentModeConfig), so a
	// backend restart no longer silently drops agent mode while every
	// client still shows it on.
	a.agentEnabled = a.cfg.AgentMode.Enabled
	logx.Printf("Agent mode initialized (enabled=%v)", a.agentEnabled)
	a.webSearchExecutor = agent.NewWebSearchExecutor(a.agentExecutor)

	a.browserMgr = browserengine.New(a.cfg.Browser.KeepAlive)
	websearch.Browser = a.browserMgr

	tlStore, err := taskloop.NewStore(config.DataPath("tasklists"))
	if err != nil {
		logx.Printf("WARN: taskloop store: %v", err)
	} else {
		a.taskloopStore = tlStore
		a.taskRunCfgs = make(map[string]*taskRunConfig)
		a.initTaskNotifyBus()
		a.taskloopEngine = taskloop.NewEngine(
			tlStore,
			a.buildTaskLoopRunWorker(),
			a.buildTaskLoopReviewChief(),
			func(v bool) { a.agentExecutor.SetBypassPermissions(v) },
			func(name, data string) {
				a.emitEvent(name, data)
				a.dispatchTaskEvent(name, data)
				a.publishTaskEvent(name, data)
				// Drop a list's provider snapshot once it stops making
				// progress, so a resume re-snapshots from the current global
				// provider. data is "listID" or "listID:extra".
				switch name {
				case "tasklist:finished", "taskloop:cancelled", "taskloop:paused", "taskloop:waiting_user":
					id := data
					if i := strings.IndexByte(id, ':'); i >= 0 {
						id = id[:i]
					}
					a.clearTaskRunConfig(id)
				}
				if name == "tasklist:finished" {
					a.postTaskFinishMessage(data)
				}
			},
			taskloop.WithRuleReader(taskloop.ReadRules),
			taskloop.WithSystemGuidance(a.memoSystemGuidance),
			taskloop.WithProjectPathFn(func(chatID string) string {
				if sm := a.getSessionManager(); sm != nil {
					return sm.GetProjectPath(chatID)
				}
				return ""
			}),
			taskloop.WithWorkerConfigHook(a.taskRunConfigFor),
			taskloop.WithSelfHeal(a.healTaskProvider),
			taskloop.WithModelLabel(a.taskModelLabel),
			taskloop.WithPlanConfig(a.planTaskConfig),
			taskloop.WithRetryScheduler(taskloop.DefaultRetryInterval),
			// v4.5.0 planner/executor mode — only engaged for a list whose
			// Mode == ModePlanner; worker-mode lists never touch this path.
			taskloop.WithPlanner(a.planTask),
			taskloop.WithStepRunner(a.runPlanStep),
			taskloop.WithAcceptanceChecker(a.acceptancecheck),
			taskloop.WithStateCompactor(a.compactPlanState),
			taskloop.WithEscalator(a.escalateStep),
			taskloop.WithGranularity(a.cfg.TaskLoop.StepGranularity),
			taskloop.WithAutoApprovePlan(a.cfg.TaskLoop.AutoApprovePlan),
			taskloop.WithMaxParallelSteps(a.cfg.TaskLoop.MaxParallelSteps),
			taskloop.WithMaxExecutorAttempts(a.cfg.TaskLoop.MaxExecutorAttempts),
			taskloop.WithStateMaxTokens(a.cfg.TaskLoop.HandoffStateMaxTokens),
			taskloop.WithMaxConcurrentLists(a.cfg.TaskLoop.MaxConcurrentLists),
		)
		if a.cfg != nil && a.cfg.TaskLoop.SubAgents {
			// Applied after construction (rather than inline above) only so the
			// whole block stays behind the config gate.
			orch := taskloop.NewSubAgentOrchestrator(&appSubAgentRunner{a: a}, 3)
			taskloop.WithSubAgents(orch, a.resolveSubAgentSpecs)(a.taskloopEngine)
		}
		// Re-arm any list left parked in a retry-driven wait by a previous run
		// (rate limit, or an offline escalation queued for a cloud re-plan).
		for _, info := range tlStore.List() {
			if info.Status == "waiting-limit" || info.Status == "waiting-escalation" {
				a.taskloopEngine.ArmRetry(info.ID)
				logx.Printf("taskloop: re-armed retry for %s (%s)", info.ID, info.Status)
			}
		}
		logx.Info("Task loop engine initialized")
	}

	a.skillManager = skill.NewManager(config.DataDir())
	// Ship the built-in memo-system skill: write it to disk if absent so it is
	// discovered like any other skill. Deleting it is allowed — it comes back
	// on the next start.
	if created, err := skill.MaterializeEmbedded(a.skillManager, "memo-system", skills.MemoSystemFS); err != nil {
		logx.Printf("skill: materialize memo-system: %v", err)
	} else if created {
		logx.Info("skill: materialized built-in memo-system skill")
	}
	if err := a.skillManager.Discover(); err != nil {
		logx.Printf("skill: discover error: %v", err)
	}
	a.skillManager.SetToolRegistrar(newSkillToolRegistrar(a.agentExecutor.Registry(), a.skillManager))
	// Tool result strings (internal/agent/tools) read the UI language from
	// their own process-wide setting — seed it from config at startup;
	// SetUILanguage keeps it in sync on later changes.
	tools.SetUILanguage(a.cfg.Identity.UILanguage)
	// Same seeding for the llama.cpp installer's progress/error strings.
	llama.SetUILanguage(a.cfg.Identity.UILanguage)
	if err := a.skillManager.LoadActiveSkills(); err != nil {
		logx.Printf("skill: load active skills error: %v", err)
	}
	if result, err := skill.SyncExternalSkills(a.skillManager, skill.KnownExternalSources()); err != nil {
		logx.Printf("skill: external sync error: %v", err)
	} else if len(result.Imported) > 0 || len(result.Updated) > 0 {
		logx.Printf("skill: imported from external tools — new: %v, updated: %v", result.Imported, result.Updated)
	}
	logx.Info("Skill manager initialized")

	// Auto-start the embedding model at boot, not just after a local chat
	// model loads — a user routing chat through an external provider (no
	// local model ever started via StartLocalModel) would otherwise never
	// get an embedding server, silently breaking RAG/memory retrieval.
	// Backgrounded since WaitReady can take a while and must not delay the
	// rest of Startup.
	if a.GetMemoryEnabled() {
		go func() {
			defer recoverPanic("Startup/autoStartEmbeddingModel")
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
	goRecover("startupTailscale", a.startupTailscale)

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
	// shutdownCleanup is the cleanup step itself, indirected so a test can
	// substitute one that genuinely never finishes. The watchdog branch used
	// to be exercised by shrinking shutdownTimeout to 1ns instead, which is a
	// scheduler race rather than a guarantee: when both select cases are
	// ready Go picks uniformly at random, and under -race on a loaded CI
	// runner the runtime can deliver the already-expired timer later than the
	// cleanup goroutine finishes. That flaked in CI — the test logged "Memo
	// shutdown completed" (only reachable from the <-done branch) and then
	// failed waiting for a force-exit that had already lost the race.
	shutdownCleanup = (*App).shutdownSync
)

// Shutdown cleans up all running background processes and servers.
func (a *App) Shutdown(ctx context.Context) {
	a.shutdownOnce.Do(func() {
		// Read on the caller's goroutine, not inside the one below, so a
		// test restoring the var afterwards is properly ordered against it
		// instead of racing a background read.
		cleanup := shutdownCleanup
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer recoverPanic("Shutdown/shutdownSync")
			cleanup(a, ctx)
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
			defer recoverPanic("shutdownSync/" + name)
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
	if a.browserMgr != nil {
		stop("browser engine", a.browserMgr.Stop)
	}
	if a.swarmServer != nil {
		stop("swarm coordinator", a.swarmServer.Stop)
	}
	if a.swarmWorker != nil {
		stop("swarm worker", a.swarmWorker.Stop)
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
	if a.statsStore != nil {
		stop("stats", a.statsStore.Close)
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
		out[i] = map[string]string{"name": e.Name, "data": e.Data, "seq": strconv.FormatUint(e.Seq, 10)}
	}
	return out
}

// Interface assertion: App must implement FullBridge.
var _ webserver.FullBridge = (*App)(nil)

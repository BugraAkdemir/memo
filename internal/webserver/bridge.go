package webserver

import (
	"context"
	"time"

	"memo/internal/agent"
	"memo/internal/agentcli"
	"memo/internal/api"
	"memo/internal/browserengine"
	"memo/internal/config"
	"memo/internal/livemode"
	"memo/internal/llama"
	"memo/internal/memory"
	"memo/internal/models"
	"memo/internal/modelstore"
	"memo/internal/observer"
	"memo/internal/orchestra"
	"memo/internal/proactive"
	"memo/internal/provider"
	"memo/internal/sessions"
	"memo/internal/skill"
	"memo/internal/stats"
	"memo/internal/stt"
	"memo/internal/taskloop"
	"memo/internal/tts"
	"memo/internal/whatsapp"
)

// FullBridge extends AppBridge with all App methods needed by the Flutter frontend.
type FullBridge interface {
	AppBridge

	// Chat
	SendMessageStream(ctx context.Context, userMsg string) <-chan api.StreamChunk
	// SendMessageStreamTo is SendMessageStream's explicit-chatID counterpart
	// (PLAN_chatid_refactor.md Faz 4) — sends into chatID's own history
	// without reading or moving which chat is globally "active", so a
	// concurrent chat switch (from this same client or a different one, e.g.
	// the GUI and internal/replcli talking to the same backend at once)
	// can't redirect an in-flight reply into the wrong chat.
	SendMessageStreamTo(ctx context.Context, chatID, userMsg string) <-chan api.StreamChunk
	SendMessageWithImageStream(ctx context.Context, userMsg string, imagePath string) <-chan api.StreamChunk
	SendMessageWithFileStream(ctx context.Context, userMsg string, filePath string) <-chan api.StreamChunk
	ExportChat() string
	GenerateChatTitle() string

	// System prompt
	GetSystemPrompt() string
	SetSystemPrompt(prompt string) error
	ResetSystemPrompt() error
	GetIncognitoPrompt() string
	SetIncognitoPrompt(prompt string) error
	GetUILanguage() string
	SetUILanguage(lang string) error
	GetMinimalMode() bool
	SetMinimalMode(enabled bool) error
	GetMinimalModeOverrides() (keepPersona, keepCapabilities, keepPassive, keepProactive bool)
	SetMinimalModeOverrides(keepPersona, keepCapabilities, keepPassive, keepProactive bool) error

	// Memory
	ClearAllMemory() error
	ListMemoryFiles() []memory.GobFileInfo
	DeleteMemoryFile(relPath string) error
	GetMemorySettings() config.MemoryConfig
	UpdateMemorySettings(topK int, minSimilarity float32) error
	GetWebSearchEnabled() bool
	UpdateWebSearchConfig(enabled bool) error
	GetBrowserKeepAlive() bool
	SetBrowserKeepAlive(keepAlive bool) error
	GetBrowserInstalled(ctx context.Context) bool
	InstallBrowser(ctx context.Context) error
	GetBrowserInstallProgress() browserengine.InstallProgress
	GetMemoryEnabled() bool
	SetMemoryEnabled(enabled bool) error
	// Dream settings are read via the existing GetMemorySettings()
	// config.MemoryConfig above (DreamEnabled/DreamInitialDelayMinutes/
	// DreamIntervalHours are just more fields on it) — only the write and
	// the manual-trigger action need their own bridge methods.
	SetMemoryDreamSettings(enabled bool, initialDelayMinutes, intervalHours int) error
	RunDreamNow(ctx context.Context) (before, after int, ran bool, err error)
	DebugMemorySearch(query string) []memory.MemoryResult
	SaveExplicitMemory(content, tags string) error
	DeleteExplicitMemory(pattern string) (int, error)
	ImportMemoryFromText(ctx context.Context, rawText string) (factsSaved int, styleUpdated bool, err error)
	SynthesizeSpeech(text string) ([]byte, error)
	GetTTSFillerSound() ([]byte, error)
	GetWhisperEnabled() bool
	SetWhisperEnabled(enabled bool) error
	GenerateSelfInsight(ctx context.Context, windowDays int, lang string) (string, error)
	ExportMemories() ([]byte, error)
	ImportMemories(data []byte) (int, error)
	GetMemoryStats() models.MemoryStats
	FilteredMemorySearch(query string, topK int, since string, tag string) []memory.MemoryResult
	GetUsageStats(days int) stats.Summary

	// Image
	GetImageBase64(path string) string

	// Version
	GetVersion() string
	CheckLatestVersion() (string, error)

	// Sessions
	ListChats() []sessions.SessionInfo
	NewAgentChat(projectPath string) string

	// Agent
	GetAgentEnabled() bool
	SetAgentEnabled(enabled bool) error
	HandleAgentPermission(requestID string, policy string) error
	GetAgentPermissions() []agent.PermissionRecord
	RevokeAgentPermission(id string) error
	ClearAgentPermissions()
	UndoLastAgentEdit() error
	SetAgentAutoPermission(enabled bool) error
	GetAgentAutoPermission() bool

	// Models
	SearchModels(query string) ([]modelstore.HFModelResult, error)
	GetModelFiles(repoID string) []modelstore.GGUFFile
	DownloadModel(repoID, filename string, expectedSize int64) error
	GetDownloadProgress() []*modelstore.DownloadProgress
	CancelDownload(repoID, filename string)
	ImportLocalModel(sourcePath string) error
	ListLocalModels() []modelstore.LocalModel
	DeleteLocalModel(path string) error
	StartLocalModel(modelPath string, ctxSize, port, gpuLayers int) error
	StopLocalModel() error
	GetLocalModelStatus() llama.ServerStatus
	StartEmbeddingModel(modelPath string, gpuLayers int) error
	StopEmbeddingModel() error
	GetEmbeddingModelStatus() llama.ServerStatus
	DetectGPU() llama.GPUInfo
	CheckLlamaInstallation() bool
	InstallLlamaServer() error
	SkipLlamaGPUInstall() error
	GetLlamaConfig() config.LlamaConfig
	UpdateLlamaConfig(cfg config.LlamaConfigUpdate) error

	// CLI-backed providers (Settings > CLI Bağlantıları)
	GetCLIStatus(cliType string) interface{}
	SetChatCLIProvider(chatID, cliType string) error
	GetChatCLIProvider(chatID string) string
	SetChatCLIWorkdir(chatID, dir string) error
	GetChatCLIWorkdir(chatID string) string
	SendCLIMessageStream(ctx context.Context, chatID, userMsg string) <-chan api.StreamChunk
	GetRunningCLIChats() []string
	ListProjectFiles(root, query string) []string
	ListCLICommands(cliType, chatID string) []agentcli.Command
	SetChatCLIModel(chatID, model string) error
	GetChatCLIModel(chatID string) string
	ListCLIModels(cliType string) []string

	// Remote access
	GetRemoteAccessStatus() interface{}
	SetRemoteAccess(enabled bool, port int) error
	SetNgrokMode(enabled bool, port int, ngrokToken string) error
	SetTailscaleMode(enabled bool, authKey, hostname string, funnel bool, port int) error
	SetBeta(enabled bool) error
	SetNgrokAutoStart(autoStart bool)
	GetListenAddr() string
	SetListenAddr(addr string)

	// Remote access auth (Faz 2, yapacam.md): four auth modes
	// (none/token/password/token_password), argon2id password login with
	// brute-force lockout, JWT sessions, and per-device tokens replacing
	// the old single shared token. See internal/app/remote_auth.go.
	GetRemoteAuthMode() string
	SetRemoteAuthConfig(mode, username, password string) error
	VerifyRemoteDeviceToken(token string) bool
	ValidateRemoteSession(token string) bool
	LoginRemotePassword(remoteAddr, username, password string, remember bool) (token, role string, err error)
	ListRemoteDevices() interface{}
	CreateRemoteDevice(name string) (string, error)
	RevokeRemoteDevice(id string) error

	// Onboarding tracks the Flutter first-run wizard (language/theme/
	// persona/model-download), a completely separate concept from
	// NeedsSetup()/CreateAdminAccount below (remote-auth account
	// bootstrap) — see config.OnboardingConfig's doc comment.
	GetOnboardingComplete() bool
	SetOnboardingComplete(completed bool) error

	// Multi-account / role model (Faz 5.1, yapacam.md) — see
	// internal/app/remote_auth.go's doc comments for the full design.
	NeedsSetup() bool
	// InstallID identifies THIS install so a client can tell that the
	// backend it stored state against was wiped and reinstalled (or that
	// it is now pointed at a different Memo). Empty when unavailable —
	// see internal/app/install_id.go.
	InstallID() string
	CreateAdminAccount(username, password string) (token string, err error)
	// BootstrapTokenAuth is the token-only counterpart to CreateAdminAccount
	// — see its own doc comment (internal/app/remote_auth.go) for why the
	// token-only first-run path needs a bootstrap call of its own.
	BootstrapTokenAuth(deviceName string) (token string, err error)
	SessionRole(token string) (role string, ok bool)
	// SessionPermissions is the granular counterpart to SessionRole (Faz
	// 5.1.1, yapacam.md) — see internal/app/remote_auth.go's doc comment.
	SessionPermissions(token string) (config.AccountPermissions, bool)
	ListAccounts() interface{}
	CreateAccount(username, password, role string, perms config.AccountPermissions) error
	// UpdateAccountPermissions edits an existing account's checkbox state —
	// a no-op for an "admin"-role account, see its own doc comment.
	UpdateAccountPermissions(id string, perms config.AccountPermissions) error
	DeleteAccount(id string) error
	ChangeAccountPassword(sessionToken, id, currentPassword, newPassword string) error

	// Server-side file browser (Faz 5.1 follow-up) — lets the Flutter
	// client browse the *backend's* filesystem instead of its own
	// OS-native file_picker, which only ever sees the connecting client's
	// own disk. See App.BrowseServerPath's doc comment for why that's
	// silently wrong against a remote self-hosted backend.
	BrowseServerPath(path string) (interface{}, error)

	// GetOutboxFile resolves a share_file download token (see
	// App.registerOutboxFile's doc comment, internal/app/sendfile.go) to
	// the staged file's real path and display filename.
	GetOutboxFile(token string) (path, filename string, ok bool)

	// Dev gateway (Settings > Developer): local OpenAI/Anthropic-compatible
	// API surface that routes to whichever model/provider is configured.
	GetDevGatewayConfig() (requireAPIKey, useMemory bool, systemPrompt string)
	SetDevGatewayConfig(requireAPIKey, useMemory bool, systemPrompt string) error
	GetDevGatewayToken() string
	ListGatewayModels() []models.GatewayModel
	DevGatewayChatStream(ctx context.Context, modelSpec string, req provider.ChatRequest) (<-chan provider.StreamChunk, string, error)
	DevGatewayChat(ctx context.Context, modelSpec string, req provider.ChatRequest) (*provider.ChatResponse, string, error)
	RecordGatewayLog(modelSpec string, stream, hasTools bool, requestText, responseText, errMsg string, duration time.Duration)
	GetGatewayLogs() []models.GatewayLogEntry
	MaybeSaveGatewayMemory(userMsg, reply string)
	GetClaudeCodeCLIConnected() bool
	GetClaudeCodeCLIModel() string
	ConnectClaudeCodeCLI(baseURL, model string) error
	DisconnectClaudeCodeCLI() error

	// Providers
	GetProviders() []provider.ProviderConfig
	UpdateProvider(cfg provider.ProviderConfig) error
	DeleteProvider(pt provider.ProviderType, name ...string) error
	TestProviderConnection(cfg provider.ProviderConfig) error
	SetActiveProvider(name string)
	GetActiveProvider() string

	// TTS providers (Faz 2 — external providers, fall back to local Piper)
	GetTTSProviders() []tts.ProviderConfig
	UpdateTTSProvider(cfg tts.ProviderConfig) error
	DeleteTTSProvider(pt tts.ProviderType, name ...string) error
	TestTTSProviderConnection(cfg tts.ProviderConfig) error
	ListTTSProviderModels(ctx context.Context, pt tts.ProviderType, apiKey string) ([]tts.ElevenLabsModel, error)
	ListTTSProviderVoices(ctx context.Context, pt tts.ProviderType, apiKey string) ([]tts.ElevenLabsVoice, error)

	// STT providers (Live Mode v2 — external providers, fall back to local
	// whisper.cpp; see docs/plans/PLAN_live_mode_v2.md §2)
	GetSTTProviders() []stt.ProviderConfig
	UpdateSTTProvider(cfg stt.ProviderConfig) error
	DeleteSTTProvider(pt stt.ProviderType, name ...string) error

	// TTS voice store (Faz 2.6 — local, offline Piper voice models)
	GetTTSVoiceCatalog() []tts.Voice
	GetSelectedTTSVoicePath() string
	GetLocalTTSVoices() []tts.LocalVoice
	GetTTSVoiceDownloadProgress() []*tts.VoiceDownloadProgress
	DownloadTTSVoice(locale, name, quality string) error
	DeleteTTSVoice(id string) error
	SelectTTSVoice(id string) error

	// Live Mode (v2 — graduated out of Beta, own config independent of it;
	// see docs/plans/PLAN_live_mode_v2.md). Phase 1 only: the top-level
	// Enabled/ActiveEngine/WorkMode/AgentPermissionPolicy selector. Per-engine
	// config (API keys, model/voice choice) is a later phase's own
	// GetLiveModeEngines/UpdateLiveModeEngine-shaped addition here.
	GetLiveModeConfig() config.LiveModeConfig
	UpdateLiveModeConfig(cfg config.LiveModeConfig) error

	// Live Mode engine configs (Phase 3 — per-engine API keys/model/voice
	// for the four non-local engines, see PLAN_live_mode_v2.md §3)
	GetLiveModeEngines() []livemode.EngineConfig
	UpdateLiveModeEngine(cfg livemode.EngineConfig) error
	DeleteLiveModeEngine(t livemode.EngineType) error
	ListLiveModeEngineModels(ctx context.Context, t livemode.EngineType, apiKey string) ([]livemode.ModelInfo, error)

	// NewLiveModeSession builds the realtime session (real client or
	// EchoSession fallback) for the WS bridge handler — see
	// internal/app/livemode_session.go's doc comment. Phase 10.
	NewLiveModeSession(ctx context.Context) livemode.Session

	// Orchestra mode
	GetOrchestraConfig() orchestra.OrchestraConfig
	UpdateOrchestraConfig(cfg orchestra.OrchestraConfig) error

	// Proactive learning system
	GetProactiveSettings() config.ProactiveConfig
	SetProactiveSettings(enabled bool, level string) error
	GetPendingSuggestion() *proactive.PendingSuggestion
	RespondToSuggestion(id, response string) error
	ListLearnedPatterns() []observer.TimePattern
	ForgetPattern(id string) error
	ClearLearningData() error

	// Events
	GetEvents() []map[string]string

	// Sync
	CheckAuth() bool
	StartSyncAuth() (string, error)
	GetSyncAccount() interface{}
	GetSyncSettings() interface{}
	UpdateSyncSettings(enabled bool, clientID, clientSecret, passphrase, tokenPath string, intervalMessages int) error
	TriggerSync()
	PullSync()
	SyncNow()
	DisconnectSync() error

	// WhatsApp
	StartWhatsApp(ctx context.Context) error
	StopWhatsApp()
	LogoutWhatsApp() error
	WhatsAppStatus() map[string]interface{}
	WhatsAppSend(ctx context.Context, jid, text string) (string, error)
	WhatsAppSearch(query string, limit int) ([]whatsapp.Message, error)
	WhatsAppGetChats() ([]whatsapp.ChatSummary, error)
	WhatsAppGetMessages(chatJID string, limit int) ([]whatsapp.Message, error)
	WhatsAppAvatar(jid string, full bool) ([]byte, error)
	WhatsAppStats() (total, last24h int, err error)
	GetWhatsAppChatMode() bool
	SetWhatsAppChatMode(enabled bool)
	WhatsAppChatStream(ctx context.Context, userMsg string) <-chan api.StreamChunk
	GetWhatsAppSelfChatAssistant() bool
	SetWhatsAppSelfChatAssistant(enabled bool) error

	// Telegram
	StartTelegram(ctx context.Context, botToken string) error
	ReconnectTelegram(ctx context.Context) error
	StopTelegram()
	DisconnectTelegram() error
	GetTelegramStatus() map[string]interface{}

	// Backup
	ExportData(includeModels bool) ([]byte, error)
	ImportData(data []byte) error
	WipeAllData() error

	// CLI / uninstall
	RemoveCLI() error
	ReinstallCLI() error
	UninstallMemo(keepMemory bool) error

	// Skills
	ListSkills() []skill.SkillDefinition
	InstallSkill(path string) (*skill.SkillDefinition, error)
	RemoveSkill(name string) error
	GetSkill(name string) (*skill.SkillDefinition, error)
	SetActiveSkills(names []string) error
	GetActiveSkills() []string

	// Shutdown gracefully stops all background processes and the HTTP server.
	Shutdown(ctx context.Context)

	// Task lists (autonomous task loop)
	CreateTaskList(chatID, title string, items []string) (*taskloop.TaskList, error)
	CreateTaskListFromTaskMd(chatID, title, taskMdPath string) (*taskloop.TaskList, error)
	GetTaskList(id string) (*taskloop.TaskList, error)
	ListTaskLists() []taskloop.TaskListInfo
	DeleteTaskList(id string) error
	StartTaskList(ctx context.Context, listID string) error
	StopTaskList(listID string)
	CancelTaskList(listID string) error
	SkipCurrentItem(listID string) error
	InjectTaskMessage(ctx context.Context, listID, text string) (string, error)
	ListRunningTasks() []taskloop.RunningTaskInfo
	// SubscribeTaskEvents returns a channel of JSON task-loop event lines and
	// an unsubscribe func; RunningTaskEventSnapshot returns one JSON line per
	// currently-running list so a fresh subscriber isn't blind.
	SubscribeTaskEvents() (<-chan string, func())
	RunningTaskEventSnapshot() []string
	// Planner/executor mode (v4.5.0)
	SetTaskListMode(listID, mode string) error
	ApproveTaskPlan(listID string) error
	GetTaskPlanMd(listID string) (string, error)
	SaveTaskPlanMd(listID, md string) error
	GetTaskLoopSettings() config.TaskLoopConfig
	UpdateTaskLoopSettings(config.TaskLoopConfig) error

	// Swarm (Memo Swarm — multi-machine llama.cpp RPC pool). Beta-gated in App.
	// Status methods return interface{} (JSON-serializable structs) so this
	// package never imports internal/app (cycle: app → webserver).
	HostSwarmCreate(modelPath string) (roomCode string, err error)
	HostSwarmAddWorker(id, secret, myRPCAddress, label string) error
	HostSwarmRemoveWorker(id string) error
	HostSwarmReorderWorkers(fromIdx, toIdx int) error
	HostSwarmSetShare(id string, pct float64) error
	HostSwarmStart(ctxSize int) error
	HostSwarmStop() error
	HostSwarmClose() error
	HostSwarmStatus() interface{}
	JoinSwarm(code string) error
	LeaveSwarm() error
	JoinSwarmStatus() interface{}
	SwarmStatusSnapshot() interface{}
}

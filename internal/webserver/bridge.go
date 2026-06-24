package webserver

import (
	"context"
	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/llama"
	"memo/internal/memory"
	"memo/internal/modelstore"
	"memo/internal/observer"
	"memo/internal/orchestra"
	"memo/internal/proactive"
	"memo/internal/provider"
	"memo/internal/sessions"
	"memo/internal/skill"
	"memo/internal/whatsapp"
)

// FullBridge extends AppBridge with all App methods needed by the Flutter frontend.
type FullBridge interface {
	AppBridge

	// Chat
	SendMessageStream(ctx context.Context, userMsg string) <-chan api.StreamChunk
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

	// Memory
	ClearAllMemory() error
	ListMemoryFiles() []memory.GobFileInfo
	DeleteMemoryFile(relPath string) error
	GetMemorySettings() config.MemoryConfig
	UpdateMemorySettings(topK int, minSimilarity float32) error
	GetWebSearchEnabled() bool
	UpdateWebSearchConfig(enabled bool) error
	GetMemoryEnabled() bool
	SetMemoryEnabled(enabled bool) error

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
	DownloadModel(repoID, filename string) error
	GetDownloadProgress() *modelstore.DownloadProgress
	CancelDownload()
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
	UpdateLlamaConfig(cfg config.LlamaConfig) error

	// Remote access
	GetRemoteAccessStatus() interface{}
	SetRemoteAccess(enabled bool, port int) error
	SetNgrokMode(enabled bool, port int, ngrokToken string) error
	SetTailscaleMode(enabled bool, authKey, hostname string, funnel bool, port int) error
	SetBeta(enabled bool) error
	SetNgrokAutoStart(autoStart bool)
	GetListenAddr() string
	SetListenAddr(addr string)

	// Providers
	GetProviders() []provider.ProviderConfig
	UpdateProvider(cfg provider.ProviderConfig) error
	DeleteProvider(pt provider.ProviderType, name ...string) error
	TestProviderConnection(cfg provider.ProviderConfig) error
	SetActiveProvider(name string)
	GetActiveProvider() string

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

	// Backup
	ExportData(includeModels bool) ([]byte, error)
	ImportData(data []byte) error
	WipeAllData() error

	// Skills
	ListSkills() []skill.SkillDefinition
	InstallSkill(path string) (*skill.SkillDefinition, error)
	RemoveSkill(name string) error
	GetSkill(name string) (*skill.SkillDefinition, error)
	SetActiveSkills(names []string) error
	GetActiveSkills() []string
}

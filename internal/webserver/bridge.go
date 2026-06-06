package webserver

import (
	"context"
	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/llama"
	"memo/internal/memory"
	"memo/internal/modelstore"
	"memo/internal/orchestra"
	"memo/internal/provider"
	"memo/internal/sessions"
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
	GetMemoryEnabled() bool
	SetMemoryEnabled(enabled bool) error

	// Image
	GetImageBase64(path string) string

	// Version
	GetVersion() string

	// Sessions
	ListChats() []sessions.SessionInfo
	NewAgentChat(projectPath string) string

	// Recording
	StartRecording() error
	StopRecordingAndTranscribe() (string, error)

	// Agent
	GetAgentEnabled() bool
	SetAgentEnabled(enabled bool) error
	HandleAgentPermission(requestID string, policy string) error
	GetAgentPermissions() []agent.PermissionRecord
	RevokeAgentPermission(id string) error
	ClearAgentPermissions()
	UndoLastAgentEdit() error

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

	// Providers
	GetProviders() []provider.ProviderConfig
	UpdateProvider(cfg provider.ProviderConfig) error
	DeleteProvider(pt provider.ProviderType) error
	TestProviderConnection(cfg provider.ProviderConfig) error
	SetActiveProvider(pt provider.ProviderType)
	GetActiveProvider() string

	// Orchestra mode
	GetOrchestraConfig() orchestra.OrchestraConfig
	UpdateOrchestraConfig(cfg orchestra.OrchestraConfig) error

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
}

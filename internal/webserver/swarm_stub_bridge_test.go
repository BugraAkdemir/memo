package webserver

import (
	"context"
	"time"

	"memo/internal/agent"
	"memo/internal/agentcli"
	"memo/internal/api"
	"memo/internal/config"
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
	"memo/internal/taskloop"
	"memo/internal/tts"
	"memo/internal/whatsapp"
)

// swarmStubBridge is a FullBridge used only by handlers_swarm_test.go.
// Non-swarm methods are no-ops; swarm methods delegate to optional funcs.
type swarmStubBridge struct {
	mockBridge
	token      string
	uiLanguage string

	hostCreate   func(modelPath string) (string, error)
	addWorker    func(id, secret, myRPCAddress, label string) error
	removeWorker func(id string) error
	reorder      func(fromIdx, toIdx int) error
	setShare     func(id string, pct float64) error
	start        func(ctxSize int) error
	stop         func() error
	close        func() error
	hostStatus   func() interface{}
	join         func(code string) error
	leave        func() error
	joinStatus   func() interface{}
	status       func() interface{}
	synthesize   func(text string) ([]byte, error)

	sendMessageStream   func(ctx context.Context, userMsg string) <-chan api.StreamChunk
	sendMessageStreamTo func(ctx context.Context, chatID, userMsg string) <-chan api.StreamChunk
}

func (b *swarmStubBridge) GetRemoteAccessToken() string { return b.token }

func (b *swarmStubBridge) HostSwarmCreate(modelPath string) (string, error) {
	if b.hostCreate != nil {
		return b.hostCreate(modelPath)
	}
	return "", nil
}
func (b *swarmStubBridge) HostSwarmAddWorker(id, secret, myRPCAddress, label string) error {
	if b.addWorker != nil {
		return b.addWorker(id, secret, myRPCAddress, label)
	}
	return nil
}
func (b *swarmStubBridge) HostSwarmRemoveWorker(id string) error {
	if b.removeWorker != nil {
		return b.removeWorker(id)
	}
	return nil
}
func (b *swarmStubBridge) HostSwarmReorderWorkers(fromIdx, toIdx int) error {
	if b.reorder != nil {
		return b.reorder(fromIdx, toIdx)
	}
	return nil
}
func (b *swarmStubBridge) HostSwarmSetShare(id string, pct float64) error {
	if b.setShare != nil {
		return b.setShare(id, pct)
	}
	return nil
}
func (b *swarmStubBridge) HostSwarmStart(ctxSize int) error {
	if b.start != nil {
		return b.start(ctxSize)
	}
	return nil
}
func (b *swarmStubBridge) HostSwarmStop() error {
	if b.stop != nil {
		return b.stop()
	}
	return nil
}
func (b *swarmStubBridge) HostSwarmClose() error {
	if b.close != nil {
		return b.close()
	}
	return nil
}
func (b *swarmStubBridge) HostSwarmStatus() interface{} {
	if b.hostStatus != nil {
		return b.hostStatus()
	}
	return map[string]string{"role": "none"}
}
func (b *swarmStubBridge) JoinSwarm(code string) error {
	if b.join != nil {
		return b.join(code)
	}
	return nil
}
func (b *swarmStubBridge) LeaveSwarm() error {
	if b.leave != nil {
		return b.leave()
	}
	return nil
}
func (b *swarmStubBridge) JoinSwarmStatus() interface{} {
	if b.joinStatus != nil {
		return b.joinStatus()
	}
	return map[string]string{"role": "none"}
}
func (b *swarmStubBridge) SwarmStatusSnapshot() interface{} {
	if b.status != nil {
		return b.status()
	}
	return map[string]string{"role": "none"}
}

// ─── FullBridge no-ops (everything not covered above) ───
func (b *swarmStubBridge) SendMessageStream(ctx context.Context, userMsg string) <-chan api.StreamChunk {
	if b.sendMessageStream != nil {
		return b.sendMessageStream(ctx, userMsg)
	}
	ch := make(chan api.StreamChunk)
	close(ch)
	return ch
}
func (b *swarmStubBridge) SendMessageStreamTo(ctx context.Context, chatID, userMsg string) <-chan api.StreamChunk {
	if b.sendMessageStreamTo != nil {
		return b.sendMessageStreamTo(ctx, chatID, userMsg)
	}
	return b.SendMessageStream(ctx, userMsg)
}
func (b *swarmStubBridge) SendMessageWithImageStream(ctx context.Context, userMsg string, imagePath string) <-chan api.StreamChunk {
	return b.SendMessageStream(ctx, userMsg)
}
func (b *swarmStubBridge) SendMessageWithFileStream(ctx context.Context, userMsg string, filePath string) <-chan api.StreamChunk {
	return b.SendMessageStream(ctx, userMsg)
}
func (b *swarmStubBridge) ExportChat() string                     { return "" }
func (b *swarmStubBridge) GenerateChatTitle() string              { return "" }
func (b *swarmStubBridge) GetSystemPrompt() string                { return "" }
func (b *swarmStubBridge) SetSystemPrompt(prompt string) error    { return nil }
func (b *swarmStubBridge) ResetSystemPrompt() error               { return nil }
func (b *swarmStubBridge) GetIncognitoPrompt() string             { return "" }
func (b *swarmStubBridge) SetIncognitoPrompt(prompt string) error { return nil }
func (b *swarmStubBridge) GetUILanguage() string                  { return b.uiLanguage }
func (b *swarmStubBridge) SetUILanguage(lang string) error        { b.uiLanguage = lang; return nil }
func (b *swarmStubBridge) GetMinimalMode() bool                   { return false }
func (b *swarmStubBridge) SetMinimalMode(enabled bool) error      { return nil }
func (b *swarmStubBridge) GetMinimalModeOverrides() (bool, bool, bool, bool) {
	return false, false, false, false
}
func (b *swarmStubBridge) SetMinimalModeOverrides(a, b2, c, d bool) error             { return nil }
func (b *swarmStubBridge) ClearAllMemory() error                                      { return nil }
func (b *swarmStubBridge) ListMemoryFiles() []memory.GobFileInfo                      { return nil }
func (b *swarmStubBridge) DeleteMemoryFile(relPath string) error                      { return nil }
func (b *swarmStubBridge) GetMemorySettings() config.MemoryConfig                     { return config.MemoryConfig{} }
func (b *swarmStubBridge) UpdateMemorySettings(topK int, minSimilarity float32) error { return nil }
func (b *swarmStubBridge) GetWebSearchEnabled() bool                                  { return false }
func (b *swarmStubBridge) UpdateWebSearchConfig(enabled bool) error                   { return nil }
func (b *swarmStubBridge) GetMemoryEnabled() bool                                     { return false }
func (b *swarmStubBridge) SetMemoryEnabled(enabled bool) error                        { return nil }
func (b *swarmStubBridge) DebugMemorySearch(query string) []memory.MemoryResult       { return nil }
func (b *swarmStubBridge) SaveExplicitMemory(content, tags string) error              { return nil }
func (b *swarmStubBridge) DeleteExplicitMemory(pattern string) (int, error)           { return 0, nil }
func (b *swarmStubBridge) ImportMemoryFromText(ctx context.Context, rawText string) (int, bool, error) {
	return 0, false, nil
}
func (b *swarmStubBridge) GenerateSelfInsight(ctx context.Context, windowDays int, lang string) (string, error) {
	return "", nil
}
func (b *swarmStubBridge) SynthesizeSpeech(text string) ([]byte, error) {
	if b.synthesize != nil {
		return b.synthesize(text)
	}
	return nil, nil
}
func (b *swarmStubBridge) GetTTSFillerSound() ([]byte, error)      { return nil, nil }
func (b *swarmStubBridge) ExportMemories() ([]byte, error)         { return nil, nil }
func (b *swarmStubBridge) ImportMemories(data []byte) (int, error) { return 0, nil }
func (b *swarmStubBridge) GetMemoryStats() models.MemoryStats      { return models.MemoryStats{} }
func (b *swarmStubBridge) FilteredMemorySearch(query string, topK int, since string, tag string) []memory.MemoryResult {
	return nil
}
func (b *swarmStubBridge) GetUsageStats(days int) stats.Summary                        { return stats.Summary{} }
func (b *swarmStubBridge) GetImageBase64(path string) string                           { return "" }
func (b *swarmStubBridge) GetVersion() string                                          { return "test" }
func (b *swarmStubBridge) CheckLatestVersion() (string, error)                         { return "", nil }
func (b *swarmStubBridge) ListChats() []sessions.SessionInfo                           { return nil }
func (b *swarmStubBridge) NewAgentChat(projectPath string) string                      { return "" }
func (b *swarmStubBridge) GetAgentEnabled() bool                                       { return false }
func (b *swarmStubBridge) SetAgentEnabled(enabled bool) error                          { return nil }
func (b *swarmStubBridge) HandleAgentPermission(requestID string, policy string) error { return nil }
func (b *swarmStubBridge) GetAgentPermissions() []agent.PermissionRecord               { return nil }
func (b *swarmStubBridge) RevokeAgentPermission(id string) error                       { return nil }
func (b *swarmStubBridge) ClearAgentPermissions()                                      {}
func (b *swarmStubBridge) UndoLastAgentEdit() error                                    { return nil }
func (b *swarmStubBridge) SetAgentAutoPermission(enabled bool) error                   { return nil }
func (b *swarmStubBridge) GetAgentAutoPermission() bool                                { return false }
func (b *swarmStubBridge) SearchModels(query string) ([]modelstore.HFModelResult, error) {
	return nil, nil
}
func (b *swarmStubBridge) GetModelFiles(repoID string) []modelstore.GGUFFile { return nil }
func (b *swarmStubBridge) DownloadModel(repoID, filename string, expectedSize int64) error {
	return nil
}
func (b *swarmStubBridge) GetDownloadProgress() []*modelstore.DownloadProgress { return nil }
func (b *swarmStubBridge) CancelDownload(repoID, filename string)              {}
func (b *swarmStubBridge) ImportLocalModel(sourcePath string) error            { return nil }
func (b *swarmStubBridge) ListLocalModels() []modelstore.LocalModel            { return nil }
func (b *swarmStubBridge) DeleteLocalModel(path string) error                  { return nil }
func (b *swarmStubBridge) StartLocalModel(modelPath string, ctxSize, port, gpuLayers int) error {
	return nil
}
func (b *swarmStubBridge) StopLocalModel() error                                     { return nil }
func (b *swarmStubBridge) GetLocalModelStatus() llama.ServerStatus                   { return llama.ServerStatus{} }
func (b *swarmStubBridge) StartEmbeddingModel(modelPath string, gpuLayers int) error { return nil }
func (b *swarmStubBridge) StopEmbeddingModel() error                                 { return nil }
func (b *swarmStubBridge) GetEmbeddingModelStatus() llama.ServerStatus               { return llama.ServerStatus{} }
func (b *swarmStubBridge) DetectGPU() llama.GPUInfo                                  { return llama.GPUInfo{} }
func (b *swarmStubBridge) CheckLlamaInstallation() bool                              { return false }
func (b *swarmStubBridge) InstallLlamaServer() error                                 { return nil }
func (b *swarmStubBridge) SkipLlamaGPUInstall() error                                { return nil }
func (b *swarmStubBridge) GetLlamaConfig() config.LlamaConfig                        { return config.LlamaConfig{} }
func (b *swarmStubBridge) UpdateLlamaConfig(cfg config.LlamaConfigUpdate) error      { return nil }
func (b *swarmStubBridge) GetCLIStatus(cliType string) interface{}                   { return nil }
func (b *swarmStubBridge) SetChatCLIProvider(chatID, cliType string) error           { return nil }
func (b *swarmStubBridge) GetChatCLIProvider(chatID string) string                   { return "" }
func (b *swarmStubBridge) SetChatCLIWorkdir(chatID, dir string) error                { return nil }
func (b *swarmStubBridge) GetChatCLIWorkdir(chatID string) string                    { return "" }
func (b *swarmStubBridge) SendCLIMessageStream(ctx context.Context, chatID, userMsg string) <-chan api.StreamChunk {
	ch := make(chan api.StreamChunk)
	close(ch)
	return ch
}
func (b *swarmStubBridge) GetRunningCLIChats() []string                                 { return nil }
func (b *swarmStubBridge) ListProjectFiles(root, query string) []string                 { return nil }
func (b *swarmStubBridge) ListCLICommands(cliType, chatID string) []agentcli.Command    { return nil }
func (b *swarmStubBridge) SetChatCLIModel(chatID, model string) error                   { return nil }
func (b *swarmStubBridge) GetChatCLIModel(chatID string) string                         { return "" }
func (b *swarmStubBridge) ListCLIModels(cliType string) []string                        { return nil }
func (b *swarmStubBridge) GetRemoteAccessStatus() interface{}                           { return nil }
func (b *swarmStubBridge) SetRemoteAccess(enabled bool, port int) error                 { return nil }
func (b *swarmStubBridge) SetNgrokMode(enabled bool, port int, ngrokToken string) error { return nil }
func (b *swarmStubBridge) SetTailscaleMode(enabled bool, authKey, hostname string, funnel bool, port int) error {
	return nil
}
func (b *swarmStubBridge) SetBeta(enabled bool) error                              { return nil }
func (b *swarmStubBridge) SetNgrokAutoStart(autoStart bool)                        {}
func (b *swarmStubBridge) GetListenAddr() string                                   { return "127.0.0.1" }
func (b *swarmStubBridge) SetListenAddr(addr string)                               {}
func (b *swarmStubBridge) GetDevGatewayConfig() (bool, bool)                       { return false, false }
func (b *swarmStubBridge) SetDevGatewayConfig(requireAPIKey, useMemory bool) error { return nil }
func (b *swarmStubBridge) GetDevGatewayToken() string                              { return "" }
func (b *swarmStubBridge) ListGatewayModels() []models.GatewayModel                { return nil }
func (b *swarmStubBridge) DevGatewayChatStream(ctx context.Context, modelSpec string, req provider.ChatRequest) (<-chan provider.StreamChunk, string, error) {
	ch := make(chan provider.StreamChunk)
	close(ch)
	return ch, "", nil
}
func (b *swarmStubBridge) DevGatewayChat(ctx context.Context, modelSpec string, req provider.ChatRequest) (*provider.ChatResponse, string, error) {
	return nil, "", nil
}
func (b *swarmStubBridge) RecordGatewayLog(modelSpec string, stream, hasTools bool, requestText, responseText, errMsg string, duration time.Duration) {
}
func (b *swarmStubBridge) GetGatewayLogs() []models.GatewayLogEntry                      { return nil }
func (b *swarmStubBridge) MaybeSaveGatewayMemory(userMsg, reply string)                  {}
func (b *swarmStubBridge) GetProviders() []provider.ProviderConfig                       { return nil }
func (b *swarmStubBridge) UpdateProvider(cfg provider.ProviderConfig) error              { return nil }
func (b *swarmStubBridge) DeleteProvider(pt provider.ProviderType, name ...string) error { return nil }
func (b *swarmStubBridge) TestProviderConnection(cfg provider.ProviderConfig) error      { return nil }
func (b *swarmStubBridge) SetActiveProvider(name string)                                 {}
func (b *swarmStubBridge) GetActiveProvider() string                                     { return "" }
func (b *swarmStubBridge) GetTTSProviders() []tts.ProviderConfig                         { return nil }
func (b *swarmStubBridge) UpdateTTSProvider(cfg tts.ProviderConfig) error                { return nil }
func (b *swarmStubBridge) DeleteTTSProvider(pt tts.ProviderType, name ...string) error   { return nil }
func (b *swarmStubBridge) TestTTSProviderConnection(cfg tts.ProviderConfig) error        { return nil }
func (b *swarmStubBridge) GetTTSVoiceCatalog() []tts.Voice                               { return nil }
func (b *swarmStubBridge) GetSelectedTTSVoicePath() string                               { return "" }
func (b *swarmStubBridge) GetLocalTTSVoices() []tts.LocalVoice                           { return nil }
func (b *swarmStubBridge) GetTTSVoiceDownloadProgress() []*tts.VoiceDownloadProgress     { return nil }
func (b *swarmStubBridge) DownloadTTSVoice(locale, name, quality string) error           { return nil }
func (b *swarmStubBridge) DeleteTTSVoice(id string) error                                { return nil }
func (b *swarmStubBridge) SelectTTSVoice(id string) error                                { return nil }
func (b *swarmStubBridge) GetOrchestraConfig() orchestra.OrchestraConfig {
	return orchestra.OrchestraConfig{}
}
func (b *swarmStubBridge) UpdateOrchestraConfig(cfg orchestra.OrchestraConfig) error { return nil }
func (b *swarmStubBridge) GetProactiveSettings() config.ProactiveConfig {
	return config.ProactiveConfig{}
}
func (b *swarmStubBridge) SetProactiveSettings(enabled bool, level string) error { return nil }
func (b *swarmStubBridge) GetPendingSuggestion() *proactive.PendingSuggestion    { return nil }
func (b *swarmStubBridge) RespondToSuggestion(id, response string) error         { return nil }
func (b *swarmStubBridge) ListLearnedPatterns() []observer.TimePattern           { return nil }
func (b *swarmStubBridge) ForgetPattern(id string) error                         { return nil }
func (b *swarmStubBridge) ClearLearningData() error                              { return nil }
func (b *swarmStubBridge) GetEvents() []map[string]string                        { return nil }
func (b *swarmStubBridge) CheckAuth() bool                                       { return false }
func (b *swarmStubBridge) StartSyncAuth() (string, error)                        { return "", nil }
func (b *swarmStubBridge) GetSyncAccount() interface{}                           { return nil }
func (b *swarmStubBridge) GetSyncSettings() interface{}                          { return nil }
func (b *swarmStubBridge) UpdateSyncSettings(enabled bool, clientID, clientSecret, passphrase, tokenPath string, intervalMessages int) error {
	return nil
}
func (b *swarmStubBridge) TriggerSync()                            {}
func (b *swarmStubBridge) PullSync()                               {}
func (b *swarmStubBridge) SyncNow()                                {}
func (b *swarmStubBridge) DisconnectSync() error                   { return nil }
func (b *swarmStubBridge) StartWhatsApp(ctx context.Context) error { return nil }
func (b *swarmStubBridge) StopWhatsApp()                           {}
func (b *swarmStubBridge) LogoutWhatsApp() error                   { return nil }
func (b *swarmStubBridge) WhatsAppStatus() map[string]interface{}  { return map[string]interface{}{} }
func (b *swarmStubBridge) WhatsAppSend(ctx context.Context, jid, text string) (string, error) {
	return "", nil
}
func (b *swarmStubBridge) WhatsAppSearch(query string, limit int) ([]whatsapp.Message, error) {
	return nil, nil
}
func (b *swarmStubBridge) WhatsAppGetChats() ([]whatsapp.ChatSummary, error) { return nil, nil }
func (b *swarmStubBridge) WhatsAppGetMessages(chatJID string, limit int) ([]whatsapp.Message, error) {
	return nil, nil
}
func (b *swarmStubBridge) WhatsAppAvatar(jid string, full bool) ([]byte, error) { return nil, nil }
func (b *swarmStubBridge) WhatsAppStats() (int, int, error)                     { return 0, 0, nil }
func (b *swarmStubBridge) GetWhatsAppChatMode() bool                            { return false }
func (b *swarmStubBridge) SetWhatsAppChatMode(enabled bool)                     {}
func (b *swarmStubBridge) WhatsAppChatStream(ctx context.Context, userMsg string) <-chan api.StreamChunk {
	return b.SendMessageStream(ctx, userMsg)
}
func (b *swarmStubBridge) ExportData(includeModels bool) ([]byte, error)            { return nil, nil }
func (b *swarmStubBridge) ImportData(data []byte) error                             { return nil }
func (b *swarmStubBridge) WipeAllData() error                                       { return nil }
func (b *swarmStubBridge) RemoveCLI() error                                         { return nil }
func (b *swarmStubBridge) ReinstallCLI() error                                      { return nil }
func (b *swarmStubBridge) UninstallMemo(keepMemory bool) error                      { return nil }
func (b *swarmStubBridge) ListSkills() []skill.SkillDefinition                      { return nil }
func (b *swarmStubBridge) InstallSkill(path string) (*skill.SkillDefinition, error) { return nil, nil }
func (b *swarmStubBridge) RemoveSkill(name string) error                            { return nil }
func (b *swarmStubBridge) GetSkill(name string) (*skill.SkillDefinition, error)     { return nil, nil }
func (b *swarmStubBridge) SetActiveSkills(names []string) error                     { return nil }
func (b *swarmStubBridge) GetActiveSkills() []string                                { return nil }
func (b *swarmStubBridge) Shutdown(ctx context.Context)                             {}
func (b *swarmStubBridge) CreateTaskList(chatID, title string, items []string) (*taskloop.TaskList, error) {
	return nil, nil
}
func (b *swarmStubBridge) GetTaskList(id string) (*taskloop.TaskList, error)      { return nil, nil }
func (b *swarmStubBridge) ListTaskLists() []taskloop.TaskListInfo                 { return nil }
func (b *swarmStubBridge) DeleteTaskList(id string) error                         { return nil }
func (b *swarmStubBridge) StartTaskList(ctx context.Context, listID string) error { return nil }
func (b *swarmStubBridge) StopTaskList(listID string)                             {}

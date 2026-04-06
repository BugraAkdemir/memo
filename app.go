package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/llama"
	"memo/internal/memory"
	"memo/internal/modelstore"
	"memo/internal/sessions"
	"memo/internal/webserver"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed binaries/*
var embeddedBinaries embed.FS

//go:embed version
var versionBytes []byte

func (a *App) GetVersion() string {
	return strings.TrimSpace(string(versionBytes))
}

type ConnectionStatus struct {
	Connected bool     `json:"connected"`
	Models    []string `json:"models"`
	Error     string   `json:"error,omitempty"`
}

type App struct {
	ctx      context.Context
	client   *api.Client
	store    *memory.Store
	identity *identity.Identity
	cfg      *config.AppConfig
	sessions          *sessions.Manager
	isIncognito       bool
	incognitoMessages []api.Message
	sttServer         *exec.Cmd
	webServer         *webserver.Server
	embeddedAssets    embed.FS
	modelStore        *modelstore.Store
	llamaServer       *llama.Server
	llamaEmbedServer  *llama.Server  // dedicated embedding model server
	llamaInstaller    *llama.Installer
	originalBaseURL   string // stores the original API base URL before llama override
	embeddingClient   *api.Client // separate client for embedding server
}

func NewApp() *App {
	return &App{}
}

func (a *App) SetEmbeddedAssets(assets embed.FS) {
	a.embeddedAssets = assets
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Printf("WARN: config: %v", err)
		cfg = config.Default()
	}
	a.cfg = cfg
	a.originalBaseURL = cfg.API.BaseURL
	a.client = api.NewClient(cfg.API.BaseURL, cfg.API.TimeoutSeconds)

	embeddingFunc := memory.NewEmbeddingFunc(a.client, cfg.API.EmbeddingModel)
	store, err := memory.NewStore(cfg.Memory.PersistDir, embeddingFunc)
	if err != nil {
		log.Printf("WARN: memory: %v", err)
	}
	a.store = store

	a.identity = identity.New(cfg.Identity.UserName, cfg.Identity.AssistantName, cfg.Identity.Style, cfg.Identity.SystemRole)

	sm, err := sessions.NewManager("data/sessions")
	if err != nil {
		log.Printf("WARN: sessions: %v", err)
	}
	a.sessions = sm

	// Initialize model store
	a.modelStore = modelstore.New(cfg.Llama.ModelsDir)

	// Initialize llama server managers and installer
	a.llamaServer = llama.NewServer(cfg.Llama.Port, cfg.Llama.CtxSize)
	a.llamaEmbedServer = llama.NewServer(cfg.Llama.EmbeddingPort, 512) // embedding models need minimal context
	a.llamaInstaller = llama.NewInstaller("data")

	// Check embedding health in background
	go func() {
		time.Sleep(2 * time.Second) // wait for LM Studio / embedding server to be ready
		h := a.CheckEmbeddingHealth()
		if h["ok"] != true {
			log.Printf("⚠️  EMBEDDING NOT WORKING: %v — memory will NOT function!", h["error"])
		}
	}()

	// Start STT server in background
	go a.startSTTServer()

	// Start remote access server if enabled
	if cfg.RemoteAccess.Enabled {
		go a.startWebServer(cfg.RemoteAccess.Port)
	}

	log.Println("Memo ready")
}

func (a *App) startWebServer(port int) {
	subFS, err := fs.Sub(a.embeddedAssets, "frontend/dist")
	if err != nil {
		log.Printf("Remote access: cannot access frontend assets: %v", err)
		return
	}
	a.webServer = webserver.New(a, subFS)
	if err := a.webServer.Start(port); err != nil {
		log.Printf("Remote access: %v", err)
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
	err = os.WriteFile(tempPath, binData, 0755)
	if err != nil {
		log.Printf("STT server unpacking failed: %v", err)
		return
	}

	a.sttServer = exec.Command(tempPath, "tr", "9876")
	a.sttServer.Stdout = os.Stdout
	a.sttServer.Stderr = os.Stderr

	if err := a.sttServer.Start(); err != nil {
		log.Printf("STT server start failed: %v", err)
		a.sttServer = nil
		return
	}
	log.Println("STT server starting on :9876")
}

func (a *App) shutdown(ctx context.Context) {
	log.Println("Memo shutting down, cleaning up background processes...")
	if a.sttServer != nil && a.sttServer.Process != nil {
		a.sttServer.Process.Kill()
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
}

// ─── Incognito ───────────────────────────────────────────────────

func (a *App) ToggleIncognito(enabled bool) {
	a.isIncognito = enabled
	if enabled {
		a.incognitoMessages = nil
		log.Println("Entered Incognito Mode")
	} else {
		a.incognitoMessages = nil
		log.Println("Exited Incognito Mode")
	}
}

// ─── Chat ────────────────────────────────────────────────────────

func (a *App) handleIncognito(userMsg string, b64 string) string {
	if b64 != "" {
		a.incognitoMessages = append(a.incognitoMessages, api.NewMultimodalMessage("user", userMsg, b64))
	} else {
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("user", userMsg))
	}

	msgs := []api.Message{api.NewTextMessage("system", a.cfg.Identity.IncognitoPrompt)}
	msgs = append(msgs, a.incognitoMessages...)

	reply := a.callLLM(msgs)
	a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("assistant", reply))
	return reply
}

func (a *App) SendMessage(userMsg string) string {
	log.Printf(">> SendMessage: %q", userMsg)
	if a.isIncognito {
		return a.handleIncognito(userMsg, "")
	}
	messages := a.buildMessages(userMsg, nil)
	if a.sessions != nil {
		a.sessions.AddMessage("user", userMsg, "", "")
	}
	reply := a.callLLM(messages)
	if a.sessions != nil {
		a.sessions.AddMessage("assistant", reply, "", "")
	}
	a.saveMemoryAsync(userMsg, reply)
	return reply
}

func (a *App) SendMessageStream(userMsg string) {
	log.Printf(">> SendMessageStream: %q", userMsg)

	if a.isIncognito {
		a.handleIncognitoStream(userMsg, "")
		return
	}

	messages := a.buildMessages(userMsg, nil)

	if a.sessions != nil {
		a.sessions.AddMessage("user", userMsg, "", "")
	}

	a.callLLMStream(context.Background(), messages, userMsg, "", "")
}

func (a *App) SendMessageWithImageStream(userMsg string, imagePath string) {
	log.Printf(">> VisionStream: %q with image %s", userMsg, imagePath)

	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		wailsRuntime.EventsEmit(a.ctx, "chat:error", "⚠️ Cannot read image: "+err.Error())
		return
	}
	mime := detectMime(imagePath, imgData)
	b64 := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imgData)

	if a.isIncognito {
		a.handleIncognitoStream(userMsg, b64)
		return
	}

	memories := a.retrieveMemory(userMsg)
	systemPrompt := a.identity.BuildSystemPrompt(memories)

	var msgs []api.Message
	msgs = append(msgs, api.NewTextMessage("system", systemPrompt))
	msgs = append(msgs, a.getSessionHistory()...)
	msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, b64))

	if a.sessions != nil {
		a.sessions.AddMessage("user", userMsg, imagePath, "")
	}

	a.callLLMStream(context.Background(), msgs, userMsg, imagePath, "")
}

func (a *App) SendMessageWithFileStream(userMsg string, filePath string) {
	log.Printf(">> FileStream: %q with %s", userMsg, filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		wailsRuntime.EventsEmit(a.ctx, "chat:error", "⚠️ Cannot read file: "+err.Error())
		return
	}

	fileName := filepath.Base(filePath)
	fileContent := string(content)
	if len(fileContent) > 10000 {
		fileContent = fileContent[:10000] + "\n\n... (truncated, file too large)"
	}

	combined := fmt.Sprintf("%s\n\n--- File: %s ---\n%s", userMsg, fileName, fileContent)

	if a.isIncognito {
		a.handleIncognitoStream(combined, "")
		return
	}

	messages := a.buildMessages(combined, nil)

	if a.sessions != nil {
		a.sessions.AddMessage("user", userMsg, "", filePath)
	}

	a.callLLMStream(context.Background(), messages, userMsg, "", filePath)
}

func (a *App) handleIncognitoStream(userMsg string, b64 string) {
	if b64 != "" {
		a.incognitoMessages = append(a.incognitoMessages, api.NewMultimodalMessage("user", userMsg, b64))
	} else {
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("user", userMsg))
	}

	msgs := []api.Message{api.NewTextMessage("system", a.cfg.Identity.IncognitoPrompt)}
	msgs = append(msgs, a.incognitoMessages...)

	// Note: for incognito, we don't save to memory/sessions, handled in callLLMStream via isIncognito flag
	a.callLLMStream(context.Background(), msgs, userMsg, "", "")
}

func (a *App) callLLMStream(ctx context.Context, messages []api.Message, userMsg, imagePath, filePath string) {
	streamCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	ch, err := a.client.ChatCompletionStream(streamCtx, messages)
	if err != nil {
		log.Printf("LLM stream error: %v", err)
		wailsRuntime.EventsEmit(a.ctx, "chat:error", "⚠️ "+err.Error())
		return
	}

	start := time.Now()
	var fullReply strings.Builder
	tokenCount := 0

	for chunk := range ch {
		if chunk.Error != "" {
			log.Printf("Stream chunk error: %s", chunk.Error)
			wailsRuntime.EventsEmit(a.ctx, "chat:error", "⚠️ "+chunk.Error)
			return
		}

		if chunk.Content != "" {
			fullReply.WriteString(chunk.Content)
			tokenCount++
			wailsRuntime.EventsEmit(a.ctx, "chat:chunk", api.StreamChunk{
				Content: chunk.Content,
			})
		}

		if chunk.Done {
			duration := time.Since(start).Seconds()
			tps := 0.0
			if duration > 0 {
				tps = float64(tokenCount) / duration
			}

			reply := fullReply.String()
			stats := &api.MessageStats{
				TokensPerSecond:  tps,
				CompletionTokens: tokenCount,
				TotalDuration:    duration,
				StopReason:       chunk.FinishReason,
			}

			// Final chunk with stats
			wailsRuntime.EventsEmit(a.ctx, "chat:done", api.StreamChunk{
				Done:  true,
				Stats: stats,
			})

			// Post-stream persistence
			if !a.isIncognito {
				if a.sessions != nil {
					a.sessions.AddMessage("assistant", reply, "", "")
				}
				a.saveMemoryAsync(userMsg, reply)
			} else {
				// For incognito, we still need to track the conversation in memory (volatile)
				a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("assistant", reply))
			}
			return
		}
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

	if a.isIncognito {
		return a.handleIncognito(userMsg, b64)
	}

	// Build multimodal messages BEFORE saving to session,
	// so getSessionHistory() doesn't include the current user message.
	memories := a.retrieveMemory(userMsg)
	systemPrompt := a.identity.BuildSystemPrompt(memories)

	var msgs []api.Message
	msgs = append(msgs, api.NewTextMessage("system", systemPrompt))
	msgs = append(msgs, a.getSessionHistory()...)
	msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, b64))

	// Save to session after building messages
	if a.sessions != nil {
		a.sessions.AddMessage("user", userMsg, imagePath, "")
	}

	reply := a.callLLM(msgs)

	if a.sessions != nil {
		a.sessions.AddMessage("assistant", reply, "", "")
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

	if a.isIncognito {
		return a.handleIncognito(combined, "")
	}

	messages := a.buildMessages(combined, nil)

	// Save to session after building messages
	if a.sessions != nil {
		a.sessions.AddMessage("user", userMsg, "", filePath)
	}

	reply := a.callLLM(messages)

	if a.sessions != nil {
		a.sessions.AddMessage("assistant", reply, "", "")
	}
	a.saveMemoryAsync(userMsg, reply)
	return reply
}

// ─── Session Management ──────────────────────────────────────────

func (a *App) NewChat() string {
	if a.sessions == nil {
		return ""
	}
	return a.sessions.NewChat()
}

func (a *App) ListChats() []sessions.SessionInfo {
	if a.sessions == nil {
		return nil
	}
	return a.sessions.ListChats()
}

func (a *App) SwitchChat(id string) error {
	if a.sessions == nil {
		return fmt.Errorf("no session manager")
	}
	return a.sessions.SwitchChat(id)
}

func (a *App) DeleteChat(id string) error {
	if a.sessions == nil {
		return fmt.Errorf("no session manager")
	}
	return a.sessions.DeleteChat(id)
}

func (a *App) RenameChat(id, title string) error {
	if a.sessions == nil {
		return fmt.Errorf("no session manager")
	}
	return a.sessions.RenameChat(id, title)
}

func (a *App) GetActiveMessages() []sessions.ChatMessage {
	if a.sessions == nil {
		return nil
	}
	return a.sessions.GetActiveMessages()
}

func (a *App) GetActiveChatID() string {
	if a.sessions == nil {
		return ""
	}
	return a.sessions.GetActiveID()
}

// ─── File Dialog ─────────────────────────────────────────────────

func (a *App) SelectImage() string {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Image",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Images", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.bmp"},
		},
	})
	if err != nil {
		log.Printf("file dialog error: %v", err)
		return ""
	}
	return path
}

func (a *App) SelectFile() string {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select File",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Text Files", Pattern: "*.txt;*.md;*.py;*.js;*.ts;*.go;*.json;*.yaml;*.yml;*.html;*.css;*.csv;*.log;*.sh;*.bat;*.xml;*.toml;*.sql"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
	if err != nil {
		log.Printf("file dialog error: %v", err)
		return ""
	}
	return path
}

// ─── Speech to Text ─────────────────────────────────────────────

var (
	recCmd  *exec.Cmd
	recFile string
	recMu   sync.Mutex
)

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

	recCmd = exec.Command("arecord", "-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "wav", recFile)
	if err := recCmd.Start(); err != nil {
		recCmd = nil
		os.Remove(recFile)
		return fmt.Errorf("arecord start: %w", err)
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

	// Stop arecord gracefully with SIGINT
	if recCmd.Process != nil {
		recCmd.Process.Signal(os.Interrupt)
	}
	recCmd.Wait()
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
	exePath, _ := os.Executable()
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
	var result struct{ Text string `json:"text"` }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("stt decode: %w", err)
	}
	return result.Text, nil
}

// ─── Other ───────────────────────────────────────────────────────

// DebugMemorySearch searches memory WITHOUT similarity filter — for debugging.
func (a *App) DebugMemorySearch(query string) []memory.MemoryResult {
	if a.store == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.store.DebugSearch(ctx, query, 10)
}

func (a *App) GetMemoryCount() int {
	if a.store == nil {
		return 0
	}
	return a.store.Count()
}

func (a *App) GetImageBase64(path string) string {
	imgData, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	mime := detectMime(path, imgData)
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imgData)
}

func (a *App) CheckConnection() ConnectionStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	models, err := a.client.CheckConnection(ctx)
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
func (a *App) GetAvailableStyles() []string  { return identity.AvailableStyles() }

func (a *App) UpdateIdentity(userName, assistantName, style string) error {
	a.identity.Update(userName, assistantName, style, "")
	a.cfg.Identity.UserName = userName
	a.cfg.Identity.AssistantName = assistantName
	a.cfg.Identity.Style = style
	return config.Save(a.cfg)
}

func (a *App) ClearHistory() {
	if a.sessions != nil {
		a.sessions.DeleteChat(a.sessions.GetActiveID())
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
	a.identity.Update("", "", "", "")
	a.cfg.Identity.SystemRole = ""
	log.Println("System prompt reset to default")
	return config.Save(a.cfg)
}

// ─── Settings: Memory Management ─────────────────────────────────

func (a *App) ClearAllMemory() error {
	if a.store == nil {
		return fmt.Errorf("no memory store")
	}
	log.Println("Clearing all memory...")
	return a.store.ClearAll()
}

func (a *App) ListMemoryFiles() []memory.GobFileInfo {
	if a.store == nil {
		return nil
	}
	return a.store.ListGobFiles()
}

func (a *App) DeleteMemoryFile(relPath string) error {
	if a.store == nil {
		return fmt.Errorf("no memory store")
	}
	log.Printf("Deleting memory file: %s", relPath)
	return a.store.DeleteGobFile(relPath)
}

// ─── Web Bridge (interface adapters for webserver) ───────────────

func (a *App) WebListChats() interface{}          { return a.ListChats() }
func (a *App) WebGetActiveMessages() interface{}   { return a.GetActiveMessages() }
func (a *App) WebCheckConnection() interface{}     { return a.CheckConnection() }

// ─── Settings: Remote Access ─────────────────────────────────────

type RemoteAccessStatus struct {
	Enabled   bool     `json:"enabled"`
	Port      int      `json:"port"`
	Running   bool     `json:"running"`
	Addresses []string `json:"addresses"`
}

func (a *App) GetRemoteAccessStatus() RemoteAccessStatus {
	status := RemoteAccessStatus{
		Enabled: a.cfg.RemoteAccess.Enabled,
		Port:    a.cfg.RemoteAccess.Port,
	}
	if a.webServer != nil {
		status.Running = a.webServer.IsRunning()
		status.Addresses = a.webServer.GetAddresses()
	}
	return status
}

func (a *App) SetRemoteAccess(enabled bool, port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}

	// Stop existing server if running
	if a.webServer != nil && a.webServer.IsRunning() {
		a.webServer.Stop()
	}

	a.cfg.RemoteAccess.Enabled = enabled
	a.cfg.RemoteAccess.Port = port

	if enabled {
		a.startWebServer(port)
	}

	return config.Save(a.cfg)
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

func (a *App) ListLocalModels() []modelstore.LocalModel {
	return a.modelStore.ListLocalModels()
}

func (a *App) DeleteLocalModel(path string) error {
	return a.modelStore.DeleteLocalModel(path)
}

// ─── llama-server: Lifecycle Management ──────────────────────────

func (a *App) StartLocalModel(modelPath string, ctxSize, port, gpuLayers int) error {
	if err := a.llamaServer.Start(a.cfg.Llama.BinaryPath, modelPath, ctxSize, port, gpuLayers); err != nil {
		return err
	}

	// Wait for the server to become ready (up to 120s for large models)
	if err := a.llamaServer.WaitReady(120 * time.Second); err != nil {
		a.llamaServer.Stop()
		return fmt.Errorf("model loaded but server failed to start: %w", err)
	}

	// Redirect API client to the local llama-server
	newBaseURL := a.llamaServer.GetBaseURL()
	a.client = api.NewClient(newBaseURL, a.cfg.API.TimeoutSeconds)
	log.Printf("API client redirected to local llama-server: %s", newBaseURL)

	// Keep embeddings on the original API (e.g. LM Studio) — chat model doesn't support /embeddings
	if !a.llamaEmbedServer.IsRunning() {
		origClient := api.NewClient(a.originalBaseURL, a.cfg.API.TimeoutSeconds)
		a.reinitMemoryStore(origClient, a.cfg.API.EmbeddingModel)
	}

	return nil
}

func (a *App) StopLocalModel() error {
	if err := a.llamaServer.Stop(); err != nil {
		return err
	}

	// Revert API client to the original base URL
	a.client = api.NewClient(a.originalBaseURL, a.cfg.API.TimeoutSeconds)
	log.Printf("API client reverted to: %s", a.originalBaseURL)

	// Only re-init embedding if no dedicated embedding server is running
	if !a.llamaEmbedServer.IsRunning() {
		a.reinitMemoryStore(a.client, a.cfg.API.EmbeddingModel)
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
		llama.StreamToFrontend(a.ctx, msg)
	}

	binPath, err := a.llamaInstaller.Install(a.ctx, logger)
	if err != nil {
		return err
	}

	// Update config to point to the newly compiled binary
	a.cfg.Llama.BinaryPath = binPath
	return config.Save(a.cfg)
}

// ─── Embedding Server: Lifecycle Management ─────────────────────

func (a *App) StartEmbeddingModel(modelPath string, gpuLayers int) error {
	if err := a.llamaEmbedServer.Start(a.cfg.Llama.BinaryPath, modelPath, 512, a.cfg.Llama.EmbeddingPort, gpuLayers); err != nil {
		return err
	}

	if err := a.llamaEmbedServer.WaitReady(60 * time.Second); err != nil {
		a.llamaEmbedServer.Stop()
		return fmt.Errorf("embedding model loaded but server failed to start: %w", err)
	}

	// Create dedicated embedding client and reinit memory store
	embBaseURL := a.llamaEmbedServer.GetBaseURL()
	a.embeddingClient = api.NewClient(embBaseURL, a.cfg.API.TimeoutSeconds)
	a.reinitMemoryStore(a.embeddingClient, a.cfg.API.EmbeddingModel)
	log.Printf("Embedding server ready on %s", embBaseURL)

	return nil
}

func (a *App) StopEmbeddingModel() error {
	if err := a.llamaEmbedServer.Stop(); err != nil {
		return err
	}

	a.embeddingClient = nil
	log.Println("Embedding server stopped")

	// Fall back to main client for embeddings
	a.reinitMemoryStore(a.client, a.cfg.API.EmbeddingModel)

	return nil
}

func (a *App) GetEmbeddingModelStatus() llama.ServerStatus {
	return a.llamaEmbedServer.GetStatus()
}

// ─── Internal Helpers ────────────────────────────────────────────

func (a *App) reinitMemoryStore(client *api.Client, model string) {
	embeddingFunc := memory.NewEmbeddingFunc(client, model)
	if a.store != nil {
		newStore, err := memory.NewStore(a.cfg.Memory.PersistDir, embeddingFunc)
		if err != nil {
			log.Printf("WARN: memory re-init: %v", err)
		} else {
			a.store = newStore
		}
	}
}

func (a *App) buildMessages(userMsg string, extraImageB64 []string) []api.Message {
	memories := a.retrieveMemory(userMsg)
	systemPrompt := a.identity.BuildSystemPrompt(memories)

	history := a.getSessionHistory()
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

	return msgs
}

func (a *App) getSessionHistory() []api.Message {
	if a.sessions == nil {
		return nil
	}
	history := a.sessions.GetHistoryForAPI(20)
	var msgs []api.Message
	for _, h := range history {
		msgs = append(msgs, api.NewTextMessage(h["role"], h["content"]))
	}
	return msgs
}

func (a *App) retrieveMemory(query string) []memory.MemoryResult {
	if a.store == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m, err := a.store.RetrieveContext(ctx, query, a.cfg.Memory.TopK, a.cfg.Memory.MinSimilarity)
	if err != nil {
		log.Printf("MEMORY RETRIEVE FAILED: %v", err)
		wailsRuntime.EventsEmit(a.ctx, "memory:error", fmt.Sprintf("Hafıza okunamadı: %v", err))
		return nil
	}
	if len(m) > 0 {
		log.Printf("Memory: found %d relevant memories (best=%.0f%%)", len(m), m[0].Similarity*100)
	}
	return m
}

func (a *App) callLLM(messages []api.Message) string {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := a.client.ChatCompletion(ctx, messages)
	if err != nil {
		log.Printf("LLM error: %v", err)
		return "⚠️ " + err.Error()
	}
	if len(resp.Choices) == 0 {
		return "⚠️ Empty response"
	}

	reply := resp.Choices[0].Message.GetTextContent()
	log.Printf("<< Reply: %d chars", len(reply))
	return reply
}

func (a *App) saveMemoryAsync(userMsg, reply string) {
	if a.store == nil || reply == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.store.SaveInteraction(ctx, userMsg, reply); err != nil {
			log.Printf("MEMORY SAVE FAILED: %v", err)
			wailsRuntime.EventsEmit(a.ctx, "memory:error", fmt.Sprintf("Hafıza kaydedilemedi: %v", err))
		} else {
			log.Printf("Memory saved: %q → %d chars reply", truncateLog(userMsg, 60), len(reply))
		}
	}()
}

func truncateLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// CheckEmbeddingHealth tests if the embedding API is reachable and working.
func (a *App) CheckEmbeddingHealth() map[string]interface{} {
	result := map[string]interface{}{
		"ok":    false,
		"error": "",
		"count": 0,
	}

	if a.store == nil {
		result["error"] = "memory store not initialized"
		return result
	}

	result["count"] = a.store.Count()

	// Try a test embedding
	client := a.client
	if a.embeddingClient != nil {
		client = a.embeddingClient
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.CreateEmbedding(ctx, a.cfg.API.EmbeddingModel, "test")
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

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/memory"
	"memo/internal/sessions"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Printf("WARN: config: %v", err)
		cfg = config.Default()
	}
	a.cfg = cfg
	a.client = api.NewClient(cfg.API.BaseURL, cfg.API.TimeoutSeconds)

	embeddingFunc := memory.NewLMStudioEmbeddingFunc(a.client, cfg.API.EmbeddingModel)
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

	// Start STT server in background
	go a.startSTTServer()

	log.Println("Memo ready")
}

func (a *App) startSTTServer() {
	pythonPath := a.findPath("stt-env/bin/python")
	scriptPath := a.findPath("scripts/stt_server.py")
	if pythonPath == "" || scriptPath == "" {
		log.Println("STT: stt-env or stt_server.py not found, speech-to-text disabled")
		return
	}

	a.sttServer = exec.Command(pythonPath, scriptPath, "tr", "9876")
	a.sttServer.Stdout = os.Stdout
	a.sttServer.Stderr = os.Stderr

	if err := a.sttServer.Start(); err != nil {
		log.Printf("STT server start failed: %v", err)
		a.sttServer = nil
		return
	}
	log.Println("STT server starting on :9876")
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

	// Save user message to session
	if a.sessions != nil {
		a.sessions.AddMessage("user", userMsg, "", "")
	}

	messages := a.buildMessages(userMsg, nil)
	reply := a.callLLM(messages)

	// Save assistant reply to session
	if a.sessions != nil {
		a.sessions.AddMessage("assistant", reply, "", "")
	}

	a.saveMemoryAsync(userMsg, reply)
	return reply
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

	// Save to session
	if a.sessions != nil {
		a.sessions.AddMessage("user", userMsg, imagePath, "")
	}

	// Build multimodal messages
	memories := a.retrieveMemory(userMsg)
	systemPrompt := a.identity.BuildSystemPrompt(memories)

	var msgs []api.Message
	msgs = append(msgs, api.NewTextMessage("system", systemPrompt))
	msgs = append(msgs, a.getSessionHistory()...)
	msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, b64))

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

	if a.sessions != nil {
		a.sessions.AddMessage("user", userMsg, "", filePath)
	}

	messages := a.buildMessages(combined, nil)
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
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Image",
		Filters: []runtime.FileFilter{
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
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select File",
		Filters: []runtime.FileFilter{
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

// ─── Other ───────────────────────────────────────────────────────

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

// ─── Internal Helpers ────────────────────────────────────────────

func (a *App) buildMessages(userMsg string, extraImageB64 []string) []api.Message {
	memories := a.retrieveMemory(userMsg)
	systemPrompt := a.identity.BuildSystemPrompt(memories)

	var msgs []api.Message
	msgs = append(msgs, api.NewTextMessage("system", systemPrompt))
	msgs = append(msgs, a.getSessionHistory()...)

	if len(extraImageB64) > 0 {
		msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, extraImageB64...))
	} else {
		msgs = append(msgs, api.NewTextMessage("user", userMsg))
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	m, err := a.store.RetrieveContext(ctx, query, a.cfg.Memory.TopK, a.cfg.Memory.MinSimilarity)
	if err != nil {
		log.Printf("memory skip: %v", err)
		return nil
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.store.SaveInteraction(ctx, userMsg, reply); err != nil {
			log.Printf("memory save skip: %v", err)
		}
	}()
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

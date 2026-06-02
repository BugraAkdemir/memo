package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	"memo/internal/cloudsync"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/llama"
	"memo/internal/memory"
	"memo/internal/modelstore"
	"memo/internal/sessions"
	"memo/internal/webserver"
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

type SyncAccount struct {
	Authenticated bool   `json:"authenticated"`
	Name          string `json:"name,omitempty"`
	Email         string `json:"email,omitempty"`
}

type App struct {
	ctx               context.Context
	client            *api.Client
	store             *memory.Store
	storeMu           sync.RWMutex
	identity          *identity.Identity
	cfg               *config.AppConfig
	sessions          *sessions.Manager
	isIncognito       bool
	incognitoMessages []api.Message
	sttServer         *exec.Cmd
	webServer         *webserver.Server
	modelStore        *modelstore.Store
	llamaServer       *llama.Server
	llamaEmbedServer  *llama.Server // dedicated embedding model server
	llamaInstaller    *llama.Installer
	originalBaseURL   string      // stores the original API base URL before llama override
	embeddingClient   *api.Client // separate client for embedding server
	syncManager       *cloudsync.Manager
}

func NewApp() *App {
	return &App{}
}

// loadDotEnv reads a .env file and sets any unset environment variables from it.
// Lines starting with # are ignored. Format: KEY=VALUE (no export keyword needed).
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // .env is optional
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
		// Only set if not already set by the real environment.
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

func (a *App) emitEvent(name string, data ...interface{}) {
	// Wails runtime calls cause fatal exits (os.Exit(1)) when context is invalid (headless mode)
	// We no longer use Wails frontend events since Flutter migration
	// log.Printf("APP EVENT: %s - %v", name, data)
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Load .env before anything else so credentials are available via os.Getenv.
	loadDotEnv(".env")

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

	// Check embedding health in background.
	// Removed: Since we are using internal models, they are started manually.
	// Running a health check here will falsely report an error.

	// Start STT server in background (DISABLED due to Vosk crashes)
	// go a.startSTTServer()

	// Start remote access server if enabled
	if cfg.RemoteAccess.Enabled {
		go a.startWebServer(cfg.RemoteAccess.Port)
	}

	// Initialize cloud sync (credentials may come from app-level env vars).
	if cfg.Sync.Enabled {
		clientID, clientSecret := a.resolveSyncCredentials()
		if clientID != "" && clientSecret != "" {
			a.syncManager = cloudsync.New(
				ctx,
				cfg.Memory.PersistDir,
				cfg.Sync.Passphrase,
				cfg.Sync.IntervalMessages,
				clientID,
				clientSecret,
				cfg.Sync.TokenPath,
			)
			log.Println("Cloud sync enabled")
		} else {
			log.Println("Cloud sync enabled in config but OAuth credentials are not available")
		}
	}

	log.Println("Memo ready")
}

// startWebServerHTTP starts a plain HTTP API server for the Flutter desktop frontend.
// Unlike startWebServer, this does NOT use TLS and does NOT serve static assets —
// it only exposes the REST API on localhost for Flutter to consume.
func (a *App) startWebServerHTTP(port int) {
	a.webServer = webserver.New(a, nil)
	if err := a.webServer.StartHTTP(port); err != nil {
		log.Printf("Flutter server: %v", err)
	}
}

// startWebServer starts a TLS server for remote access.
func (a *App) startWebServer(port int) {
	if a.webServer == nil {
		a.webServer = webserver.New(a, nil)
	}
	if err := a.webServer.Start(port); err != nil {
		log.Printf("Remote access server: %v", err)
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

func (a *App) SendMessageStream(ctx context.Context, userMsg string) <-chan api.StreamChunk {
	log.Printf(">> SendMessageStream: %q", userMsg)

	if a.isIncognito {
		return a.handleIncognitoStream(ctx, userMsg, "")
	}

	messages := a.buildMessages(userMsg, nil)

	if a.sessions != nil {
		a.sessions.AddMessage("user", userMsg, "", "")
	}

	return a.callLLMStream(ctx, messages, userMsg, "", "")
}

func (a *App) SendMessageWithImageStream(ctx context.Context, userMsg string, imagePath string) <-chan api.StreamChunk {
	log.Printf(">> VisionStream: %q with image %s", userMsg, imagePath)

	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		ch := make(chan api.StreamChunk, 1)
		ch <- api.StreamChunk{Error: "⚠️ Cannot read image: " + err.Error(), Done: true}
		close(ch)
		return ch
	}
	mime := detectMime(imagePath, imgData)
	b64 := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imgData)

	if a.isIncognito {
		return a.handleIncognitoStream(ctx, userMsg, b64)
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

	return a.callLLMStream(ctx, msgs, userMsg, imagePath, "")
}

func (a *App) SendMessageWithFileStream(ctx context.Context, userMsg string, filePath string) <-chan api.StreamChunk {
	log.Printf(">> FileStream: %q with %s", userMsg, filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		ch := make(chan api.StreamChunk, 1)
		ch <- api.StreamChunk{Error: "⚠️ Cannot read file: " + err.Error(), Done: true}
		close(ch)
		return ch
	}

	fileName := filepath.Base(filePath)
	fileContent := string(content)
	if len(fileContent) > 10000 {
		fileContent = fileContent[:10000] + "\n\n... (truncated, file too large)"
	}

	combined := fmt.Sprintf("%s\n\n--- File: %s ---\n%s", userMsg, fileName, fileContent)

	if a.isIncognito {
		return a.handleIncognitoStream(ctx, combined, "")
	}

	messages := a.buildMessages(combined, nil)

	if a.sessions != nil {
		a.sessions.AddMessage("user", userMsg, "", filePath)
	}

	return a.callLLMStream(context.Background(), messages, userMsg, "", filePath)
}

func (a *App) handleIncognitoStream(ctx context.Context, userMsg string, b64 string) <-chan api.StreamChunk {
	if b64 != "" {
		a.incognitoMessages = append(a.incognitoMessages, api.NewMultimodalMessage("user", userMsg, b64))
	} else {
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("user", userMsg))
	}

	msgs := []api.Message{api.NewTextMessage("system", a.cfg.Identity.IncognitoPrompt)}
	msgs = append(msgs, a.incognitoMessages...)

	// Note: for incognito, we don't save to memory/sessions, handled in callLLMStream via isIncognito flag
	return a.callLLMStream(ctx, msgs, userMsg, "", "")
}

func (a *App) callLLMStream(ctx context.Context, messages []api.Message, userMsg, imagePath, filePath string) <-chan api.StreamChunk {
	outCh := make(chan api.StreamChunk, 128)

	go func() {
		defer close(outCh)

		streamCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
		defer cancel()

		requestStart := time.Now()
		ch, err := a.client.ChatCompletionStream(streamCtx, messages)
		if err != nil {
			log.Printf("LATENCY llm.stream_error total_ms=%d messages=%d", time.Since(requestStart).Milliseconds(), len(messages))
			log.Printf("LLM stream error: %v", err)
			outCh <- api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true}
			return
		}
		log.Printf("LATENCY llm.stream_ready total_ms=%d messages=%d", time.Since(requestStart).Milliseconds(), len(messages))

		start := time.Now()
		var fullReply strings.Builder
		tokenCount := 0
		firstTokenLogged := false

		for chunk := range ch {
			if chunk.Error != "" {
				log.Printf("LATENCY llm.stream_chunk_error total_ms=%d generation_ms=%d tokens=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), tokenCount)
				log.Printf("Stream chunk error: %s", chunk.Error)
				outCh <- api.StreamChunk{Error: "⚠️ " + chunk.Error, Done: true}
				return
			}

			if chunk.Content != "" {
				if !firstTokenLogged {
					firstTokenLogged = true
					log.Printf("LATENCY llm.first_token total_ms=%d after_stream_ready_ms=%d messages=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), len(messages))
				}
				fullReply.WriteString(chunk.Content)
				tokenCount++
				outCh <- chunk
			}

			if chunk.Done {
				log.Printf("LATENCY llm.stream_done total_ms=%d generation_ms=%d tokens=%d finish=%s", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), tokenCount, chunk.FinishReason)
				a.finishStream(start, tokenCount, chunk.FinishReason, fullReply.String(), userMsg)
				outCh <- chunk
				return
			}
		}

		// Channel closed without an explicit Done chunk (some providers skip [DONE]).
		// Treat accumulated content as a complete reply.
		if fullReply.Len() > 0 {
			log.Printf("LATENCY llm.stream_closed total_ms=%d generation_ms=%d tokens=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), tokenCount)
			a.finishStream(start, tokenCount, "stop", fullReply.String(), userMsg)
			outCh <- api.StreamChunk{Done: true, FinishReason: "stop"}
		} else {
			log.Printf("LATENCY llm.stream_empty total_ms=%d generation_ms=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds())
			outCh <- api.StreamChunk{Error: "⚠️ Model boş yanıt döndürdü", Done: true}
		}
	}()

	return outCh
}

func (a *App) finishStream(start time.Time, tokenCount int, finishReason, reply, userMsg string) {
	duration := time.Since(start).Seconds()
	tps := 0.0
	if duration > 0 && tokenCount > 0 {
		tps = float64(tokenCount) / duration
	}

	a.emitEvent("chat:done", api.StreamChunk{
		Done: true,
		Stats: &api.MessageStats{
			TokensPerSecond:  tps,
			CompletionTokens: tokenCount,
			TotalDuration:    duration,
			StopReason:       finishReason,
		},
	})

	if !a.isIncognito {
		if a.sessions != nil {
			a.sessions.AddMessage("assistant", reply, "", "")
		}
		a.saveMemoryAsync(userMsg, reply)
	} else {
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("assistant", reply))
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

	// Detect vision-not-supported error and return friendly message
	if strings.Contains(reply, "image input is not supported") || strings.Contains(reply, "mmproj") {
		reply = "⚠️ Bu model görsel/resim desteklemiyor. Resim gönderebilmek için vision destekli bir model kullanmalısınız (örn: LLaVA, BakLLaVA, Llama Vision gibi)."
	}

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

// ExportChat returns the active chat as a Markdown string.
func (a *App) ExportChat() string {
	if a.sessions == nil {
		return ""
	}
	msgs := a.sessions.GetActiveMessages()
	if len(msgs) == 0 {
		return ""
	}

	var sb strings.Builder
	chatID := a.sessions.GetActiveID()
	// Find title from list
	title := "Memo Chat"
	for _, s := range a.sessions.ListChats() {
		if s.ID == chatID {
			title = s.Title
			break
		}
	}
	sb.WriteString("# " + title + "\n\n")
	sb.WriteString("_Exported from Memo — " + time.Now().Format("2006-01-02 15:04") + "_\n\n---\n\n")

	for _, m := range msgs {
		switch m.Role {
		case "user":
			sb.WriteString("**You** · " + m.Timestamp + "\n\n")
		case "assistant":
			sb.WriteString("**Memo** · " + m.Timestamp + "\n\n")
		}
		sb.WriteString(m.Content + "\n\n---\n\n")
	}
	return sb.String()
}

// GenerateChatTitle asks the LLM to produce a short title from the first
// exchange, then renames the active chat and returns the new title.
func (a *App) GenerateChatTitle() string {
	if a.sessions == nil {
		return ""
	}
	msgs := a.sessions.GetActiveMessages()
	// Only generate when we have exactly the first exchange (user + assistant).
	if len(msgs) < 2 {
		return ""
	}

	first := msgs[0].Content
	if len(first) > 300 {
		first = first[:300]
	}
	second := msgs[1].Content
	if len(second) > 300 {
		second = second[:300]
	}

	prompt := []api.Message{
		api.NewTextMessage("user", fmt.Sprintf(
			"Based on this conversation excerpt, generate a very short chat title (3–6 words max, no quotes, no punctuation at end):\n\nUser: %s\nAssistant: %s\n\nTitle:",
			first, second,
		)),
	}

	title := strings.TrimSpace(a.callLLM(prompt))
	// Discard error replies.
	if title == "" || strings.HasPrefix(title, "⚠️") {
		return ""
	}
	// Sanitize: remove surrounding quotes if any.
	title = strings.Trim(title, `"'`)
	// Truncate to 60 chars just in case.
	runes := []rune(title)
	if len(runes) > 60 {
		title = string(runes[:60])
	}

	chatID := a.sessions.GetActiveID()
	if err := a.sessions.RenameChat(chatID, title); err != nil {
		log.Printf("auto-title rename: %v", err)
		return ""
	}
	return title
}

// ─── File Dialog ─────────────────────────────────────────────────
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

	var recordArgs []string
	var recorder string
	switch runtime.GOOS {
	case "windows":
		// ffmpeg with DirectShow — ships with many Windows systems; graceful fallback if missing
		recorder = "ffmpeg"
		recordArgs = []string{"-y", "-f", "dshow", "-i", "audio=@device_cm_{33D9A762-90C8-11D0-BD43-00A0C911CE86}\\wave_{00000000-0000-0000-0000-000000000000}", "-ar", "16000", "-ac", "1", "-acodec", "pcm_s16le", recFile}
	case "darwin":
		recorder = "sox"
		recordArgs = []string{"-d", "-b", "16", "-r", "16000", "-c", "1", recFile}
	default:
		recorder = "arecord"
		recordArgs = []string{"-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "wav", recFile}
	}
	recCmd = exec.Command(recorder, recordArgs...)
	if err := recCmd.Start(); err != nil {
		recCmd = nil
		os.Remove(recFile)
		return fmt.Errorf("recording start (%s): %w", recorder, err)
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
	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("stt decode: %w", err)
	}
	return result.Text, nil
}

// ─── Other ───────────────────────────────────────────────────────

// DebugMemorySearch searches memory WITHOUT similarity filter — for debugging.
func (a *App) DebugMemorySearch(query string) []memory.MemoryResult {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.store.DebugSearch(ctx, query, 10)
}

func (a *App) GetMemoryCount() int {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return 0
	}
	return a.store.Count()
}

func (a *App) GetImageBase64(path string) string {
	// Layer 2: Only allow paths under the data directory
	dataDir := filepath.Dir(a.cfg.Memory.PersistDir)
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return ""
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return ""
	}

	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return ""
	}

	if !strings.HasPrefix(realPath, absDataDir) {
		log.Printf("WARNING: Blocked attempt to read file outside data dir: %s", path)
		return ""
	}

	imgData, err := os.ReadFile(realPath)
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
func (a *App) GetAvailableStyles() []string { return identity.AvailableStyles() }

func (a *App) UpdateIdentity(userName, assistantName, style string) error {
	a.identity.Update(userName, assistantName, style, a.cfg.Identity.SystemRole)
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
	nameSection := ""
	if a.cfg.Identity.UserName != "" {
		nameSection = fmt.Sprintf("The user's name is %s. ", a.cfg.Identity.UserName)
	}
	defaultPrompt := fmt.Sprintf(`%sYou are %s, a highly capable, privacy-first AI assistant running entirely locally on the user's device.

CORE DIRECTIVES:
1. Identity: You are always %s, regardless of the underlying LLM. Act as a smart, reliable, and direct partner.
2. Anti-Hallucination: Never invent, guess, or fabricate information. If you are unsure or do not know the answer, explicitly state that you do not know.
3. Conciseness & Structure: Keep your answers clear, well-structured, and strictly to the point. Avoid long, rambling introductions or unnecessary filler words.
4. Seamless Memory: You have access to the user's personal context. Use this information naturally to inform your answers. STRICTLY FORBIDDEN: Do not use phrases like "I remember," "As we discussed," "Based on your data," or "I recall." Simply present the information as shared context.
5. Language Mirroring: Always respond in the exact language the user communicates in (e.g., if the user asks in Turkish, your entire response must be in Turkish).`, nameSection, a.cfg.Identity.AssistantName, a.cfg.Identity.AssistantName)

	a.identity.Update("", "", "", defaultPrompt)
	a.cfg.Identity.SystemRole = defaultPrompt
	log.Println("System prompt reset to default")
	return config.Save(a.cfg)
}

// ─── Settings: Memory Management ─────────────────────────────────

func (a *App) ClearAllMemory() error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		return fmt.Errorf("no memory store")
	}
	log.Println("Clearing all memory...")
	return a.store.ClearAll()
}

func (a *App) ListMemoryFiles() []memory.GobFileInfo {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return nil
	}
	return a.store.ListGobFiles()
}

func (a *App) DeleteMemoryFile(relPath string) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		return fmt.Errorf("no memory store")
	}
	log.Printf("Deleting memory file: %s", relPath)
	return a.store.DeleteGobFile(relPath)
}

func (a *App) GetMemorySettings() config.MemoryConfig {
	return a.cfg.Memory
}

func (a *App) UpdateMemorySettings(topK int, minSimilarity float32) error {
	if topK < 1 || topK > 50 {
		return fmt.Errorf("top_k must be between 1 and 50")
	}
	if minSimilarity <= 0 || minSimilarity > 1 {
		return fmt.Errorf("min_similarity must be between 0.01 and 1")
	}

	a.cfg.Memory.TopK = topK
	a.cfg.Memory.MinSimilarity = minSimilarity
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	log.Printf("Memory settings updated: top_k=%d min_similarity=%.2f", topK, minSimilarity)
	return nil
}

// ─── Web Bridge (interface adapters for webserver) ───────────────

func (a *App) WebListChats() interface{}         { return a.ListChats() }
func (a *App) WebGetActiveMessages() interface{} { return a.GetActiveMessages() }
func (a *App) WebCheckConnection() interface{}   { return a.CheckConnection() }

// ─── Settings: Remote Access ─────────────────────────────────────

type RemoteAccessStatus struct {
	Enabled   bool     `json:"enabled"`
	Port      int      `json:"port"`
	Running   bool     `json:"running"`
	Addresses []string `json:"addresses"`
}

func (a *App) GetRemoteAccessStatus() interface{} {
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

func (a *App) ImportLocalModel(sourcePath string) error {
	return a.modelStore.ImportLocalModel(sourcePath)
}

func (a *App) ListLocalModels() []modelstore.LocalModel {
	return a.modelStore.ListLocalModels()
}

func (a *App) DeleteLocalModel(path string) error {
	return a.modelStore.DeleteLocalModel(path)
}

// ─── llama-server: Lifecycle Management ──────────────────────────

func (a *App) StartLocalModel(modelPath string, ctxSize, port, gpuLayers int) error {
	if err := a.llamaServer.Start(a.cfg.Llama.BinaryPath, modelPath, ctxSize, port, gpuLayers, false, a.cfg.Llama.EngineMode); err != nil {
		return err
	}

	// Wait up to 3 minutes for the model to load before returning success to the UI.
	// Since Flutter event system is currently disabled, we use synchronous wait.
	if err := a.llamaServer.WaitReady(180 * time.Second); err != nil {
		a.llamaServer.Stop()
		return fmt.Errorf("Model yükleme zaman aşımına uğradı (3 dk). (Hata: %w)", err)
	}

	// Redirect API client to the local llama-server
	newBaseURL := a.llamaServer.GetBaseURL()
	a.client = api.NewClient(newBaseURL, a.cfg.API.TimeoutSeconds)
	log.Printf("API client redirected to local llama-server: %s", newBaseURL)

	// Auto-start embedding model if not already running
	if !a.llamaEmbedServer.IsRunning() {
		a.autoStartEmbeddingModel()
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

func (a *App) UpdateLlamaConfig(cfg config.LlamaConfig) error {
	// Merge partial updates — only overwrite fields with non-zero values
	if cfg.EngineMode != "" {
		a.cfg.Llama.EngineMode = cfg.EngineMode
	}
	if cfg.BinaryPath != "" {
		a.cfg.Llama.BinaryPath = cfg.BinaryPath
	}
	if cfg.Port != 0 {
		a.cfg.Llama.Port = cfg.Port
	}
	if cfg.EmbeddingPort != 0 {
		a.cfg.Llama.EmbeddingPort = cfg.EmbeddingPort
	}
	if cfg.CtxSize != 0 {
		a.cfg.Llama.CtxSize = cfg.CtxSize
	}
	if cfg.MaxHistory != 0 {
		a.cfg.Llama.MaxHistory = cfg.MaxHistory
	}
	if cfg.ModelsDir != "" {
		a.cfg.Llama.ModelsDir = cfg.ModelsDir
	}
	return config.Save(a.cfg)
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
		log.Println("INSTALL:", msg)
	}

	binPath, err := a.llamaInstaller.Install(a.ctx, logger)
	if err != nil {
		return err
	}

	// Update config to point to the newly compiled binary
	a.cfg.Llama.BinaryPath = binPath
	// If GPU installer succeeds, remove any old .force_cpu file so they run on GPU!
	_ = os.Remove("data/.force_cpu")
	return config.Save(a.cfg)
}

func (a *App) SkipLlamaGPUInstall() error {
	// Create .force_cpu file in data directory to bypass GPU checks
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}
	forceCPUFile := "data/.force_cpu"
	f, err := os.Create(forceCPUFile)
	if err != nil {
		return err
	}
	f.Close()
	log.Println("Created .force_cpu bypass file. Future starts will use CPU.")
	return nil
}

// ─── Embedding Server: Lifecycle Management ─────────────────────

func (a *App) StartEmbeddingModel(modelPath string, gpuLayers int) error {
	// Stop existing embedding server if running
	if a.llamaEmbedServer.IsRunning() {
		a.llamaEmbedServer.Stop()
		time.Sleep(500 * time.Millisecond) // Give it a moment to release ports
	}

	if err := a.llamaEmbedServer.Start(a.cfg.Llama.BinaryPath, modelPath, 512, a.cfg.Llama.EmbeddingPort, gpuLayers, true, a.cfg.Llama.EngineMode); err != nil {
		return err
	}

	if err := a.llamaEmbedServer.WaitReady(120 * time.Second); err != nil {
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

// autoStartEmbeddingModel finds the first embedding model in the local model store
// and starts it automatically. This ensures the GOB memory/RAG system works
// without requiring the user to manually start the embedding server.
func (a *App) autoStartEmbeddingModel() {
	models := a.modelStore.ListLocalModels()
	var embeddingPath string
	for _, m := range models {
		if m.IsEmbedding {
			embeddingPath = m.Path
			break
		}
	}
	if embeddingPath == "" {
		msg := "⚠️ No embedding model found — RAG will NOT function."
		log.Println(msg)
		a.emitEvent("memory:error", msg)
		return
	}

	log.Printf("Auto-starting embedding model: %s", embeddingPath)
	if err := a.StartEmbeddingModel(embeddingPath, -1); err != nil {
		msg := fmt.Sprintf("⚠️ Failed to auto-start embedding model: %v", err)
		log.Print(msg)
		a.emitEvent("memory:error", msg)
	} else {
		log.Println("✅ Embedding model auto-started — memory/RAG is active.")
	}
}

func (a *App) reinitMemoryStore(client *api.Client, model string) {
	embeddingFunc := memory.NewEmbeddingFunc(client, model)
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
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
	start := time.Now()
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

	log.Printf("LATENCY chat.build_messages total_ms=%d memories=%d history=%d messages=%d system_chars=%d", time.Since(start).Milliseconds(), len(memories), len(history), len(msgs), len(systemPrompt))
	return msgs
}

func (a *App) getSessionHistory() []api.Message {
	if a.sessions == nil {
		return nil
	}
	history := a.sessions.GetHistoryForAPI(a.cfg.Llama.MaxHistory)
	var msgs []api.Message
	for _, h := range history {
		msgs = append(msgs, api.NewTextMessage(h["role"], h["content"]))
	}
	return msgs
}

func (a *App) retrieveMemory(query string) []memory.MemoryResult {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return nil
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m, err := a.store.RetrieveContext(ctx, query, a.cfg.Memory.TopK, a.cfg.Memory.MinSimilarity)
	if err != nil {
		log.Printf("LATENCY app.retrieve_memory total_ms=%d status=error", time.Since(start).Milliseconds())
		log.Printf("MEMORY RETRIEVE FAILED: %v", err)
		a.emitEvent("memory:error", fmt.Sprintf("Hafıza okunamadı: %v", err))
		return nil
	}
	log.Printf("LATENCY app.retrieve_memory total_ms=%d returned=%d", time.Since(start).Milliseconds(), len(m))
	if len(m) > 0 {
		log.Printf("Memory: found %d relevant memories (best=%.0f%%)", len(m), m[0].Similarity*100)
	}
	return m
}

func (a *App) callLLM(messages []api.Message) string {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	start := time.Now()
	resp, err := a.client.ChatCompletion(ctx, messages)
	if err != nil {
		log.Printf("LATENCY llm.complete total_ms=%d status=error messages=%d", time.Since(start).Milliseconds(), len(messages))
		log.Printf("LLM error: %v", err)
		return "⚠️ " + err.Error()
	}
	if len(resp.Choices) == 0 {
		log.Printf("LATENCY llm.complete total_ms=%d status=empty messages=%d", time.Since(start).Milliseconds(), len(messages))
		return "⚠️ Empty response"
	}

	reply := resp.Choices[0].Message.GetTextContent()
	log.Printf("LATENCY llm.complete total_ms=%d status=ok messages=%d reply_chars=%d", time.Since(start).Milliseconds(), len(messages), len(reply))
	log.Printf("<< Reply: %d chars", len(reply))
	return reply
}

func (a *App) saveMemoryAsync(userMsg, reply string) {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil || reply == "" {
		return
	}
	go func() {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		a.storeMu.Lock()
		defer a.storeMu.Unlock()
		if a.store == nil {
			return
		}

		if err := a.store.SaveInteraction(ctx, userMsg, reply); err != nil {
			log.Printf("LATENCY app.memory_save_async total_ms=%d status=error", time.Since(start).Milliseconds())
			log.Printf("MEMORY SAVE FAILED: %v", err)
			a.emitEvent("memory:error", fmt.Sprintf("Hafıza kaydedilemedi: %v", err))
		} else {
			log.Printf("LATENCY app.memory_save_async total_ms=%d status=ok", time.Since(start).Milliseconds())
			log.Printf("Memory saved: %q → %d chars reply", truncateLog(userMsg, 60), len(reply))
			if a.syncManager != nil {
				a.syncManager.Increment()
			}
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

	a.storeMu.RLock()
	defer a.storeMu.RUnlock()

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

// ─── Cloud Sync ───────────────────────────────────────────────────────────────

func (a *App) resolveSyncCredentials() (string, string) {
	clientID := strings.TrimSpace(a.cfg.Sync.ClientID)
	clientSecret := strings.TrimSpace(a.cfg.Sync.ClientSecret)
	if clientID == "" {
		clientID = strings.TrimSpace(os.Getenv("MEMO_GOOGLE_CLIENT_ID"))
	}
	if clientSecret == "" {
		clientSecret = strings.TrimSpace(os.Getenv("MEMO_GOOGLE_CLIENT_SECRET"))
	}
	return clientID, clientSecret
}

func (a *App) ensureSyncManager() error {
	if a.syncManager != nil {
		return nil
	}
	clientID, clientSecret := a.resolveSyncCredentials()
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("cloud sync OAuth credentials missing (set MEMO_GOOGLE_CLIENT_ID and MEMO_GOOGLE_CLIENT_SECRET in app environment)")
	}
	a.syncManager = cloudsync.New(
		a.ctx,
		a.cfg.Memory.PersistDir,
		a.cfg.Sync.Passphrase,
		a.cfg.Sync.IntervalMessages,
		clientID,
		clientSecret,
		a.cfg.Sync.TokenPath,
	)
	return nil
}

// CheckSyncAuth reports whether the cloud sync manager is authenticated.
func (a *App) CheckSyncAuth() bool {
	if err := a.ensureSyncManager(); err != nil {
		return false
	}
	return a.syncManager.IsAuthenticated()
}

// CheckAuth is an alias for CheckSyncAuth exposed for cloud sync UI logic.
func (a *App) CheckAuth() bool {
	return a.CheckSyncAuth()
}

// StartSyncAuth starts the OAuth2 loopback flow and returns the URL to open.
// The frontend should open this URL in the system browser. Poll CheckSyncAuth
// to detect when the user has completed the flow.
func (a *App) StartSyncAuth() (string, error) {
	if err := a.ensureSyncManager(); err != nil {
		return "", err
	}
	url, err := a.syncManager.StartAuthFlow()
	if err != nil {
		return "", err
	}
	return url, nil
}

// TriggerSync forces an immediate backup upload outside the automatic 50-message cycle.
func (a *App) TriggerSync() {
	if err := a.ensureSyncManager(); err != nil {
		a.emitEvent("sync:error", err.Error())
		return
	}
	a.syncManager.TriggerNow()
}

// PullSync downloads latest cloud backup and restores local .gob files.
func (a *App) PullSync() {
	if err := a.ensureSyncManager(); err != nil {
		a.emitEvent("sync:error", err.Error())
		return
	}
	a.syncManager.TriggerPullNow()
}

// SyncNow runs push then pull in background.
func (a *App) SyncNow() {
	if err := a.ensureSyncManager(); err != nil {
		a.emitEvent("sync:error", err.Error())
		return
	}
	a.syncManager.TriggerFullSyncNow()
}

// GetSyncAccount returns Google account identity for the connected sync session.
func (a *App) GetSyncAccount() interface{} {
	if err := a.ensureSyncManager(); err != nil {
		return SyncAccount{Authenticated: false}
	}
	if !a.syncManager.IsAuthenticated() {
		return SyncAccount{Authenticated: false}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	acc, err := a.syncManager.GetAccountInfo(ctx)
	if err != nil {
		log.Printf("cloud sync account info: %v", err)
		return SyncAccount{Authenticated: true}
	}
	return SyncAccount{
		Authenticated: true,
		Name:          acc.Name,
		Email:         acc.Email,
	}
}

func (a *App) GetSyncSettings() interface{} {
	return a.cfg.Sync
}

func (a *App) UpdateSyncSettings(enabled bool, clientID, clientSecret, passphrase, tokenPath string, intervalMessages int) error {
	if tokenPath == "" {
		tokenPath = "./data/sync_token.json"
	}
	if intervalMessages <= 0 {
		intervalMessages = 50
	}

	a.cfg.Sync.Enabled = enabled
	a.cfg.Sync.ClientID = strings.TrimSpace(clientID)
	a.cfg.Sync.ClientSecret = strings.TrimSpace(clientSecret)
	a.cfg.Sync.Passphrase = passphrase
	a.cfg.Sync.TokenPath = strings.TrimSpace(tokenPath)
	a.cfg.Sync.IntervalMessages = intervalMessages

	if err := config.Save(a.cfg); err != nil {
		return err
	}

	// Re-create manager with fresh settings.
	resolvedClientID, resolvedClientSecret := a.resolveSyncCredentials()
	if a.cfg.Sync.Enabled && resolvedClientID != "" && resolvedClientSecret != "" {
		a.syncManager = cloudsync.New(
			a.ctx,
			a.cfg.Memory.PersistDir,
			a.cfg.Sync.Passphrase,
			a.cfg.Sync.IntervalMessages,
			resolvedClientID,
			resolvedClientSecret,
			a.cfg.Sync.TokenPath,
		)
	} else {
		a.syncManager = nil
	}
	return nil
}

// DisconnectSync revokes the local OAuth token and resets the sync manager.
// The user will need to re-authenticate to use cloud sync again.
func (a *App) DisconnectSync() error {
	tokenPath := a.cfg.Sync.TokenPath
	if tokenPath == "" {
		tokenPath = "./data/sync_token.json"
	}
	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("disconnect sync: remove token: %w", err)
	}
	a.syncManager = nil
	return nil
}

var _ webserver.FullBridge = (*App)(nil)

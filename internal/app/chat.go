package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"memo/internal/api"
	"memo/internal/memory"
)

// ToggleIncognito enables or disables incognito mode.
func (a *App) ToggleIncognito(enabled bool) {
	a.incognitoMu.Lock()
	a.isIncognito = enabled
	a.incognitoMessages = nil
	a.incognitoMu.Unlock()
	if enabled {
		log.Println("Entered Incognito Mode")
	} else {
		log.Println("Exited Incognito Mode")
	}
}

func (a *App) handleIncognito(userMsg string, b64 string) string {
	a.incognitoMu.Lock()
	if b64 != "" {
		a.incognitoMessages = append(a.incognitoMessages, api.NewMultimodalMessage("user", userMsg, b64))
	} else {
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("user", userMsg))
	}
	msgs := []api.Message{api.NewTextMessage("system", a.cfg.Identity.IncognitoPrompt)}
	msgs = append(msgs, a.incognitoMessages...)
	a.incognitoMu.Unlock()

	reply := a.callLLM(context.Background(), msgs)

	a.incognitoMu.Lock()
	a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("assistant", reply))
	a.incognitoMu.Unlock()
	return reply
}

// SendMessage sends a plain-text user message and returns the reply.
func (a *App) SendMessage(userMsg string) string {
	log.Printf(">> SendMessage: %q", userMsg)
	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognito(userMsg, "")
	}
	a.observerRecorder.RecordMessage(userMsg)
	messages := a.buildMessages(context.Background(), userMsg, nil)
	sm := a.getSessionManager()
	if sm != nil {
		sm.AddMessage("user", userMsg, "", "")
	}
	reply := a.callLLM(context.Background(), messages)
	if sm != nil {
		sm.AddMessage("assistant", reply, "", "")
	}
	a.saveMemoryAsync(userMsg, reply)
	return reply
}

// SendMessageStream sends a user message and streams the reply token by token.
func (a *App) SendMessageStream(ctx context.Context, userMsg string) <-chan api.StreamChunk {
	log.Printf(">> SendMessageStream: %q", userMsg)

	// Handle skill commands
	if ch := a.handleSkillCommand(ctx, userMsg); ch != nil {
		return ch
	}

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognitoStream(ctx, userMsg, "")
	}
	a.observerRecorder.RecordMessage(userMsg)

	// Inject active skill instructions into system prompt
	messages := a.buildMessages(ctx, userMsg, nil)
	if skillPrompt := a.buildActiveSkillPrompt(); skillPrompt != "" {
		for i, msg := range messages {
			if msg.Role == "system" {
				if content, ok := msg.Content.(string); ok {
					messages[i].Content = content + skillPrompt
				}
				break
			}
		}
	}

	sm := a.getSessionManager()
	if sm != nil {
		sm.AddMessage("user", userMsg, "", "")
	}

	a.agentMu.RLock()
	agentActive := a.agentEnabled
	a.agentMu.RUnlock()

	orchestraEnabled := a.orchestraConductor != nil && a.orchestraConductor.Config().Enabled

	localModelRunning := a.llamaServer != nil && a.llamaServer.IsRunning()

	if agentActive && (a.activeProvider != "" || localModelRunning) {
		if orchestraEnabled {
			a.observerRecorder.RecordOrchestraRun(userMsg)
			return a.callAgentWithOrchestra(ctx, messages, userMsg)
		}
		a.observerRecorder.RecordAgentRun(userMsg)
		return a.callAgentStream(ctx, messages, userMsg)
	}

	return a.callLLMStream(ctx, messages, userMsg, "", "")
}

// SendMessageWithImageStream sends a user message together with an image file.
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

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognitoStream(ctx, userMsg, b64)
	}

	var memories []memory.MemoryResult
	if a.cfg.Memory.MemoryEnabled {
		memories = a.retrieveMemory(ctx, userMsg)
	}
	systemPrompt := a.identity.BuildSystemPrompt(memories)

	var msgs []api.Message
	msgs = append(msgs, api.NewTextMessage("system", systemPrompt))
	msgs = append(msgs, a.getSessionHistory()...)
	msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, b64))

	sm := a.getSessionManager()
	if sm != nil {
		sm.AddMessage("user", userMsg, imagePath, "")
	}

	return a.callLLMStream(ctx, msgs, userMsg, imagePath, "")
}

// SendMessageWithFileStream attaches a file's content to the message.
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

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognitoStream(ctx, combined, "")
	}

	messages := a.buildMessages(ctx, combined, nil)

	sm := a.getSessionManager()
	if sm != nil {
		sm.AddMessage("user", userMsg, "", filePath)
	}

	return a.callLLMStream(ctx, messages, userMsg, "", filePath)
}

func (a *App) handleIncognitoStream(ctx context.Context, userMsg string, b64 string) <-chan api.StreamChunk {
	a.incognitoMu.Lock()
	if b64 != "" {
		a.incognitoMessages = append(a.incognitoMessages, api.NewMultimodalMessage("user", userMsg, b64))
	} else {
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("user", userMsg))
	}
	msgs := []api.Message{api.NewTextMessage("system", a.cfg.Identity.IncognitoPrompt)}
	msgs = append(msgs, a.incognitoMessages...)
	a.incognitoMu.Unlock()

	return a.callLLMStream(ctx, msgs, userMsg, "", "")
}

// SendMessageWithImage sends a vision message (non-streaming).
func (a *App) SendMessageWithImage(userMsg string, imagePath string) string {
	log.Printf(">> Vision: %q with image %s", userMsg, imagePath)

	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "⚠️ Cannot read image: " + err.Error()
	}
	mime := detectMime(imagePath, imgData)
	b64 := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imgData)

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognito(userMsg, b64)
	}

	var memories []memory.MemoryResult
	if a.cfg.Memory.MemoryEnabled {
		memories = a.retrieveMemory(context.Background(), userMsg)
	}
	systemPrompt := a.identity.BuildSystemPrompt(memories)

	var msgs []api.Message
	msgs = append(msgs, api.NewTextMessage("system", systemPrompt))
	msgs = append(msgs, a.getSessionHistory()...)
	msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, b64))

	sm := a.getSessionManager()
	if sm != nil {
		sm.AddMessage("user", userMsg, imagePath, "")
	}

	reply := a.callLLM(context.Background(), msgs)

	if strings.Contains(reply, "image input is not supported") || strings.Contains(reply, "mmproj") {
		reply = "⚠️ Bu model görsel/resim desteklemiyor. Resim gönderebilmek için vision destekli bir model kullanmalısınız (örn: LLaVA, BakLLaVA, Llama Vision gibi)."
	}

	if sm != nil {
		sm.AddMessage("assistant", reply, "", "")
	}
	a.saveMemoryAsync(userMsg, reply)
	return reply
}

// SendMessageWithFile sends a file-attached message (non-streaming).
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

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognito(combined, "")
	}

	messages := a.buildMessages(context.Background(), combined, nil)

	sm := a.getSessionManager()
	if sm != nil {
		sm.AddMessage("user", userMsg, "", filePath)
	}

	reply := a.callLLM(context.Background(), messages)

	if sm != nil {
		sm.AddMessage("assistant", reply, "", "")
	}
	a.saveMemoryAsync(userMsg, reply)
	return reply
}

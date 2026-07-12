package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"memo/internal/logx"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memo/internal/api"
	"memo/internal/memory"
	moodpkg "memo/internal/mood"
)

// forwardStream drains inner into out, preferring delivery of each chunk
// over honoring ctx cancellation. A plain
// `select { case out <- chunk: case <-ctx.Done(): return }` lets Go's
// random tie-breaking between simultaneously-ready cases silently drop a
// chunk — including the final Done:true one — if ctx happens to become
// Done at the exact moment out's reader is also ready to receive it. The
// HTTP handler reading the outer channel then just sees it close with no
// final chunk ever written to the SSE response, so the client never learns
// the stream finished and its "sending" UI state is stuck forever (see the
// matching fix and full explanation on trySend in llm.go). out is always
// created with a generous buffer (128) by every caller of this function, so
// the non-blocking attempt below succeeds immediately in the overwhelming
// majority of real cases, sidestepping the race entirely.
func forwardStream(ctx context.Context, inner <-chan api.StreamChunk, out chan<- api.StreamChunk) {
	for chunk := range inner {
		select {
		case out <- chunk:
			continue
		default:
		}
		select {
		case out <- chunk:
		case <-ctx.Done():
			return
		}
	}
}

// GetIncognito reports whether incognito mode is currently active.
func (a *App) GetIncognito() bool {
	a.incognitoMu.RLock()
	defer a.incognitoMu.RUnlock()
	return a.isIncognito
}

// ToggleIncognito enables or disables incognito mode.
func (a *App) ToggleIncognito(enabled bool) {
	a.incognitoMu.Lock()
	a.isIncognito = enabled
	a.incognitoMessages = nil
	a.incognitoMu.Unlock()
	if enabled {
		logx.Info("Entered Incognito Mode")
	} else {
		logx.Info("Exited Incognito Mode")
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
	logx.Printf(">> SendMessage: %q", userMsg)
	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return a.handleIncognito(userMsg, "")
	}
	a.observerRecorder.RecordMessage(userMsg)
	go a.processMessageIntent(userMsg, "chat", "", time.Now())
	messages := a.buildMessages(context.Background(), userMsg, nil)
	sm := a.getSessionManager()
	if sm != nil {
		sm.AddMessage("user", userMsg, "", "")
	}
	reply := a.callLLM(context.Background(), messages)
	if a.mood != nil && a.mood.Enabled() {
		go a.updateMoodAsync(userMsg)
	}
	if sm != nil {
		sm.AddMessage("assistant", reply, "", "")
	}
	a.saveMemoryAsync(userMsg, reply)
	return reply
}

// SendMessageStream sends a user message and streams the reply token by token.
func (a *App) SendMessageStream(ctx context.Context, userMsg string) <-chan api.StreamChunk {
	logx.Printf(">> SendMessageStream: %q", userMsg)

	// Handle skill commands
	if ch := a.handleSkillCommand(ctx, userMsg); ch != nil {
		return ch
	}

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		if !a.streamMu.TryLock() {
			errCh := make(chan api.StreamChunk, 1)
			errCh <- api.StreamChunk{Error: "⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", Done: true}
			close(errCh)
			return errCh
		}
		innerCh := a.handleIncognitoStream(ctx, userMsg, "")
		out := make(chan api.StreamChunk, 128)
		go func() {
			defer close(out)
			defer a.streamMu.Unlock()
			forwardStream(ctx, innerCh, out)
		}()
		return out
	}

	// When web-search mode is on, surface a "searching the web" status before
	// the (blocking) search runs inside buildMessages, then splice the real
	// LLM stream behind it. The frontend shows this like the "thinking" line.
	if a.GetWebSearchEnabled() {
		out := make(chan api.StreamChunk, 128)
		go func() {
			defer close(out)
			trySend(ctx, out, api.StreamChunk{FinishReason: "status", Content: "web_search"})
			forwardStream(ctx, a.sendMessageStreamInner(ctx, userMsg), out)
		}()
		return out
	}

	return a.sendMessageStreamInner(ctx, userMsg)
}

// routeStream applies skill/agent system-prompt injection to messages and
// dispatches to the agent pipeline, agent+orchestra, or a plain LLM stream,
// based on the current agent-mode/provider/local-model state.
//
// Shared by sendMessageStreamInner, SendMessageWithImageStream and
// SendMessageWithFileStream so an image- or file-attached message gets
// identical agent routing to a plain text one — the image/file streams used
// to build their own message list and call callLLMStream directly, so
// agent tools (file read/write, command execution) silently did not run
// for those message types even with agent mode on.
func (a *App) routeStream(ctx context.Context, messages []api.Message, userMsg, imagePath, filePath, sessionID string) <-chan api.StreamChunk {
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

	a.agentMu.RLock()
	agentActive := a.agentEnabled
	a.agentMu.RUnlock()

	if agentActive {
		for i, msg := range messages {
			if msg.Role == "system" {
				if content, ok := msg.Content.(string); ok {
					messages[i].Content = content + buildAgentSystemPrompt()
				}
				break
			}
		}
	}

	orchestraEnabled := a.orchestraConductor != nil && a.orchestraConductor.Config().Enabled
	localModelRunning := a.llamaServer != nil && a.llamaServer.IsRunning()

	a.providerMu.RLock()
	hasProvider := a.activeProviderName != ""
	a.providerMu.RUnlock()

	if agentActive && (hasProvider || localModelRunning) {
		if orchestraEnabled {
			a.observerRecorder.RecordOrchestraRun(userMsg)
			return a.callAgentWithOrchestra(ctx, messages, userMsg, sessionID)
		}
		a.observerRecorder.RecordAgentRun(userMsg)
		return a.callAgentStream(ctx, messages, userMsg, sessionID)
	}
	return a.callLLMStream(ctx, messages, userMsg, imagePath, filePath, sessionID)
}

// sendMessageStreamInner is the core of SendMessageStream: it records the
// message, builds the prompt (including any web-search context), and dispatches
// to the agent, orchestra, or plain LLM stream.
func (a *App) sendMessageStreamInner(ctx context.Context, userMsg string) <-chan api.StreamChunk {
	// Prevent concurrent stream goroutines. If a stream is already running,
	// return an error immediately instead of racing two parallel streams that
	// would interleave user/user/assistant/assistant into the session history.
	if !a.streamMu.TryLock() {
		errCh := make(chan api.StreamChunk, 1)
		errCh <- api.StreamChunk{Error: "⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", Done: true}
		close(errCh)
		return errCh
	}

	a.observerRecorder.RecordMessage(userMsg)
	go a.processMessageIntent(userMsg, "chat", "", time.Now())

	// Capture the target chat once, up front — buildMessagesForSession
	// (history read) and AddMessageToSession (the user's own message write)
	// must both act on the exact same chat, not whatever happens to be
	// "active" at each individual call's own point in time. Otherwise a
	// chat switch mid-call (BUG-H1) can read one chat's history but write
	// the message/reply to a different, newly-active one.
	sm := a.getSessionManager()
	var chatID string
	if sm != nil {
		chatID = sm.GetActiveID()
	}

	messages := a.buildMessagesForSession(ctx, chatID, userMsg, nil)
	if sm != nil {
		sm.AddMessageToSession(chatID, "user", userMsg, "", "")
	}

	innerCh := a.routeStream(ctx, messages, userMsg, "", "", chatID)

	// Wrap the inner channel so streamMu is released when the stream completes.
	out := make(chan api.StreamChunk, 128)
	go func() {
		defer close(out)
		defer a.streamMu.Unlock()
		forwardStream(ctx, innerCh, out)
	}()
	return out
}

// SendMessageWithImageStream sends a user message together with an image file.
func (a *App) SendMessageWithImageStream(ctx context.Context, userMsg string, imagePath string) <-chan api.StreamChunk {
	logx.Printf(">> VisionStream: %q with image %s", userMsg, imagePath)

	if !a.streamMu.TryLock() {
		errCh := make(chan api.StreamChunk, 1)
		errCh <- api.StreamChunk{Error: "⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", Done: true}
		close(errCh)
		return errCh
	}

	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		a.streamMu.Unlock()
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
		innerCh := a.handleIncognitoStream(ctx, userMsg, b64)
		out := make(chan api.StreamChunk, 128)
		go func() {
			defer close(out)
			defer a.streamMu.Unlock()
			forwardStream(ctx, innerCh, out)
		}()
		return out
	}

	// Captured once, up front — see the matching comment in
	// sendMessageStreamInner (BUG-H1).
	sm := a.getSessionManager()
	var chatID string
	if sm != nil {
		chatID = sm.GetActiveID()
	}

	// buildMessagesForSession (not a hand-rolled system+history+user list) so
	// image messages get the same mood directive, web search context, and
	// token-aware history truncation as plain text ones — the manual
	// construction this replaced skipped all three (BUG-QL5).
	msgs := a.buildMessagesForSession(ctx, chatID, userMsg, []string{b64})
	if sm != nil {
		sm.AddMessageToSession(chatID, "user", userMsg, imagePath, "")
	}

	innerCh := a.routeStream(ctx, msgs, userMsg, imagePath, "", chatID)

	out := make(chan api.StreamChunk, 128)
	go func() {
		defer close(out)
		defer a.streamMu.Unlock()
		forwardStream(ctx, innerCh, out)
	}()
	return out
}

// SendMessageWithFileStream attaches a file's content to the message.
func (a *App) SendMessageWithFileStream(ctx context.Context, userMsg string, filePath string) <-chan api.StreamChunk {
	logx.Printf(">> FileStream: %q with %s", userMsg, filePath)

	if !a.streamMu.TryLock() {
		errCh := make(chan api.StreamChunk, 1)
		errCh <- api.StreamChunk{Error: "⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", Done: true}
		close(errCh)
		return errCh
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		a.streamMu.Unlock()
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
		innerCh := a.handleIncognitoStream(ctx, combined, "")
		out := make(chan api.StreamChunk, 128)
		go func() {
			defer close(out)
			defer a.streamMu.Unlock()
			forwardStream(ctx, innerCh, out)
		}()
		return out
	}

	// Captured once, up front — see the matching comment in
	// sendMessageStreamInner (BUG-H1).
	sm := a.getSessionManager()
	var chatID string
	if sm != nil {
		chatID = sm.GetActiveID()
	}

	messages := a.buildMessagesForSession(ctx, chatID, combined, nil)
	if sm != nil {
		sm.AddMessageToSession(chatID, "user", userMsg, "", filePath)
	}

	innerCh := a.routeStream(ctx, messages, userMsg, "", filePath, chatID)

	out := make(chan api.StreamChunk, 128)
	go func() {
		defer close(out)
		defer a.streamMu.Unlock()
		forwardStream(ctx, innerCh, out)
	}()
	return out
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

	return a.callLLMStream(ctx, msgs, userMsg, "", "", "")
}

// SendMessageWithImage sends a vision message (non-streaming).
func (a *App) SendMessageWithImage(userMsg string, imagePath string) string {
	logx.Printf(">> Vision: %q with image %s", userMsg, imagePath)

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
	if a.GetMemoryEnabled() {
		memories = a.retrieveMemory(context.Background(), userMsg)
	}
	systemPrompt := a.identity.BuildSystemPrompt(memories, false, a.GetAgentEnabled(), a.GetWebSearchEnabled())

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
	logx.Printf(">> File: %q with %s", userMsg, filePath)

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

// updateMoodAsync duygu skorunu arka planda asenkron günceller.
func (a *App) updateMoodAsync(userMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	scorer := moodpkg.NewScorer(func(ctx context.Context, sys, user string) (string, error) {
		msgs := []api.Message{
			api.NewTextMessage("system", sys),
			api.NewTextMessage("user", user),
		}
		return a.callLLM(ctx, msgs), nil
	})

	iAnlik := scorer.Score(ctx, userMsg)
	if err := a.mood.Update(ctx, iAnlik); err != nil {
		logx.Printf("mood.Update: %v", err)
	}
}

// buildAgentSystemPrompt returns agent-specific instructions that guide the
// model to behave as an efficient coding agent with proper tool usage.
func buildAgentSystemPrompt() string {
	return `

## Çalışma Modu: Kodlama Ajanı

Şu anda dosya okuma/yazma/düzenleme ve terminal komutu çalıştırma yetkisine sahip bir
kodlama ajanı olarak çalışıyorsun. Aşağıdaki kurallara uy:

### Çalışma Sırası
1. **Önce oku, sonra yaz.** Mevcut dosyaları ve proje yapısını anlamadan kod yazma.
2. **Plan yap.** Neyi nasıl yapacağını adım adım düşün, sonra uygula.
3. **Kullanıcı ne dediyse onu kullan.** "Go ile yap" dediyse Go, "Python" dediyse Python.
   Kendi kafana göre dil/framework değiştirme.
4. **Her adımda test et.** Kod yazdıktan sonra derle/çalıştır, hata varsa düzelt.
5. **İşin bitince özet geç.** Ne yaptığını, nereye yazdığını kısaca söyle.

### Verimlilik
- Gereksiz araç çağrısı yapma. Bir kere okumak yeterliyse 5 kere okuma.
- Aynı dosyayı art arda okumak yerine içeriği aklında tut.
- Uzun işleri parçalara böl, her parçayı tamamla, sonra devam et.
- Bütün araçları aynı anda kullanman gerekmiyor — sadece ihtiyacın olanı kullan.`
}

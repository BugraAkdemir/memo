package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memo/internal/agent"
	"memo/internal/agent/tools"
	"memo/internal/api"
	"memo/internal/provider"
	"memo/internal/whatsapp"
)

// whatsAppHasStoredSession reports whether a paired WhatsApp session already
// exists on disk, so startup can auto-reconnect without a user click.
func (a *App) whatsAppHasStoredSession() bool {
	sessionDB := filepath.Join(a.cfg.WhatsApp.DataDir, "session.db")
	if _, err := os.Stat(sessionDB); err != nil {
		return false // no session file → never paired
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return whatsapp.HasRegisteredDevice(ctx, sessionDB)
}

// initWhatsApp initializes the WhatsApp client and message store.
func (a *App) initWhatsApp() {
	cfg := a.cfg.WhatsApp

	dataDir := cfg.DataDir
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("WhatsApp: mkdir data dir: %v", err)
		return
	}

	msgDB := filepath.Join(dataDir, "messages.db")
	sessionDB := filepath.Join(dataDir, "session.db")

	msgStore, err := whatsapp.NewStore(msgDB)
	if err != nil {
		log.Printf("WhatsApp: store init error: %v", err)
		return
	}
	a.waMsgStore = msgStore

	waCfg := whatsapp.Config{
		DataDir:        dataDir,
		MessageStoreDB: msgDB,
		SessionDB:      sessionDB,
		AutoIndex:      cfg.AutoIndex,
		MaxHistoryDays: cfg.MaxHistoryDays,
	}

	a.waClient = whatsapp.NewClient(waCfg)
	a.waClient.SetStore(msgStore)

	// Wire up WhatsApp client for agent tools (package-level global)
	tools.WhatsAppClient = waToolAdapter{a.waClient}

	go func() {
		if err := a.waClient.Start(context.Background()); err != nil {
			log.Printf("WhatsApp: auto-connect error: %v", err)
		}
	}()

	// Consume incoming messages for intent extraction and observer recording.
	go a.runWhatsAppIntentLoop(a.lifecycleCtx)

	log.Println("WhatsApp client initialized and connecting...")
}

// runWhatsAppIntentLoop drains the WhatsApp message channel and runs intent
// extraction on each message. It exits when ctx is cancelled.
func (a *App) runWhatsAppIntentLoop(ctx context.Context) {
	if a.waClient == nil {
		return
	}
	ch := a.waClient.MessageChannel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			// Record in observer regardless of intent.
			if a.observerRecorder != nil {
				a.observerRecorder.RecordWhatsAppMessage(msg.Text, msg.FromMe, msg.Timestamp)
			}
			// Run intent extraction asynchronously so the channel never blocks.
			go a.processMessageIntent(msg.Text, "whatsapp", msg.SenderName, msg.Timestamp)
		}
	}
}

// StartWhatsApp connects to WhatsApp Web.
func (a *App) StartWhatsApp(ctx context.Context) error {
	a.waMu.Lock()
	defer a.waMu.Unlock()
	if a.waClient == nil {
		a.initWhatsApp()
		if a.waClient == nil {
			return fmt.Errorf("WhatsApp initialization failed")
		}
		return nil
	}
	return a.waClient.Start(ctx)
}

// StopWhatsApp disconnects from WhatsApp Web.
func (a *App) StopWhatsApp() {
	a.waMu.Lock()
	defer a.waMu.Unlock()
	if a.waClient != nil {
		a.waClient.Stop()
	}
}

// LogoutWhatsApp removes the local session.
func (a *App) LogoutWhatsApp() error {
	a.waMu.Lock()
	defer a.waMu.Unlock()
	if a.waClient == nil {
		return nil
	}
	return a.waClient.Logout()
}

// WhatsAppStatus returns QR codes (if pairing) and connection state.
func (a *App) WhatsAppStatus() map[string]interface{} {
	if a.waClient == nil {
		return map[string]interface{}{
			"initialized":  false,
			"connected":    false,
			"logged_in":    false,
			"reconnecting": false,
			"last_error":   "",
		}
	}
	return map[string]interface{}{
		"initialized":  true,
		"connected":    a.waClient.IsConnected(),
		"logged_in":    a.waClient.IsLoggedIn(),
		"qr_codes":     a.waClient.QRCodes(),
		"reconnecting": a.waClient.IsReconnecting(),
		"last_error":   a.waClient.LastError(),
	}
}

// WhatsAppSend sends a text message via WhatsApp.
func (a *App) WhatsAppSend(ctx context.Context, jid, text string) (string, error) {
	if a.waClient == nil {
		return "", fmt.Errorf("WhatsApp not initialized")
	}
	return a.waClient.SendMessage(ctx, jid, text)
}

// GetWhatsAppChatMode returns whether WhatsApp chat mode is active.
func (a *App) GetWhatsAppChatMode() bool {
	a.whatsappChatMu.RLock()
	defer a.whatsappChatMu.RUnlock()
	return a.whatsappChatMode
}

// SetWhatsAppChatMode enables or disables WhatsApp chat mode.
func (a *App) SetWhatsAppChatMode(enabled bool) {
	a.whatsappChatMu.Lock()
	defer a.whatsappChatMu.Unlock()
	a.whatsappChatMode = enabled
}

// WhatsAppChatStream handles a chat message in WhatsApp mode.
func (a *App) WhatsAppChatStream(ctx context.Context, userMsg string) <-chan api.StreamChunk {
	outCh := make(chan api.StreamChunk, 128)

	go func() {
		defer close(outCh)

		localTrySend := func(ctx context.Context, ch chan<- api.StreamChunk, chunk api.StreamChunk) bool {
			select {
			case ch <- chunk:
				return true
			case <-ctx.Done():
				return false
			}
		}

		messages := a.buildMessages(ctx, userMsg, nil)

		sm := a.getSessionManager()
		if sm != nil {
			sm.AddMessage("user", userMsg, "", "")
		}

		pMsgs := make([]provider.Message, len(messages))
		for i, m := range messages {
			pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
		}

		sessionID := ""
		if sm != nil {
			sessionID = sm.GetActiveID()
		}

		modelName := ""
		// Snapshot the router and config manager under the lock; they can be
		// reassigned concurrently by UpdateProvider/SetActiveProvider, so reading
		// the fields directly below the lock would be a data race (and could nil
		// out the router between the check and use).
		a.providerMu.RLock()
		router := a.providerRouter
		cfgMgr := a.providerCfgMgr
		activeName := a.activeProviderName
		a.providerMu.RUnlock()
		if router != nil && cfgMgr != nil {
			if activeName != "" {
				for _, p := range cfgMgr.GetEnabled() {
					if p.Name == activeName {
						modelName = p.Model
						break
					}
				}
			}
			if modelName == "" {
				for _, p := range cfgMgr.GetEnabled() {
					modelName = p.Model
					break
				}
			}
		}

		if router == nil || !router.HasActiveProvider() {
			localTrySend(ctx, outCh, api.StreamChunk{
				Error: "WhatsApp sohbeti için bir sağlayıcı yapılandırmadınız.",
				Done:  true,
			})
			return
		}

		waPrompt := provider.Message{
			Role: "system",
			Content: `Sen bir WhatsApp asistanısın. Kullanıcının WhatsApp mesajlarını okuyabilir, arama yapabilir ve mesaj gönderebilirsin.

Kullanabileceğin araçlar:
- whatsapp_latest: En son mesajlaşılan sohbetleri listele
- whatsapp_messages: Bir sohbetin mesaj geçmişini getir
- whatsapp_search: Mesajlarda metin araması yap
- whatsapp_send: Bir kişiye mesaj gönder

Kullanıcı "bana en son kim yazdı" derse whatsapp_latest çağır.
"falana mesaj at" derse whatsapp_send çağır.
"falana ne yazmışım" derse whatsapp_messages veya whatsapp_search çağır.

NOT: whatsapp_latest çıktısında her sohbet "(jid: ...)" bilgisiyle gelir; mesaj
gönderirken veya geçmiş getirirken bu jid'i kullan. Kişiyi sadece ismiyle de
belirtebilirsin (örn. "Berra") — sistem ismi otomatik olarak doğru kişiye eşler.
Kullanıcıya ASLA JID sorma; önce whatsapp_latest ile sohbet listesini kontrol et.`,
		}

		allMsgs := make([]provider.Message, 0, len(pMsgs)+1)
		allMsgs = append(allMsgs, waPrompt)
		allMsgs = append(allMsgs, pMsgs...)

		waExecutor := agent.NewWhatsAppExecutor(a.agentExecutor)
		waExecutor.SyncRouter(router)

		start := time.Now()
		var fullReply strings.Builder

		streamCh, err := waExecutor.RunStream(ctx, sessionID, modelName, allMsgs, func(ev agent.AgentEvent) {
			chunkData, _ := json.Marshal(ev)
			localTrySend(ctx, outCh, api.StreamChunk{
				Content:      string(chunkData),
				FinishReason: "agent_event",
			})
		})

		if err != nil {
			localTrySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
			return
		}

		for chunk := range streamCh {
			if chunk.Content != "" {
				fullReply.WriteString(chunk.Content)
			}
			localTrySend(ctx, outCh, api.StreamChunk{Content: chunk.Content, FinishReason: chunk.FinishReason, Done: chunk.Done})
		}

		reply := fullReply.String()
		if reply != "" && sm != nil {
			sm.AddMessage("assistant", reply, "", "")
		}

		log.Printf("WhatsApp chat completed in %v (%d chars)", time.Since(start), len(reply))
		if a.mood != nil && a.mood.Enabled() && userMsg != "" {
			go a.updateMoodAsync(userMsg)
		}
	}()

	return outCh
}

// WhatsAppSearch searches WhatsApp messages.
func (a *App) WhatsAppSearch(query string, limit int) ([]whatsapp.Message, error) {
	if a.waMsgStore == nil {
		return nil, fmt.Errorf("WhatsApp store not available")
	}
	return a.waMsgStore.SearchMessages(query, limit)
}

// WhatsAppGetChats returns the chat list.
func (a *App) WhatsAppGetChats() ([]whatsapp.ChatSummary, error) {
	if a.waMsgStore == nil {
		return nil, fmt.Errorf("WhatsApp store not available")
	}
	return a.waMsgStore.GetChatList()
}

// WhatsAppGetMessages returns messages for a specific chat.
func (a *App) WhatsAppGetMessages(chatJID string, limit int) ([]whatsapp.Message, error) {
	if a.waMsgStore == nil {
		return nil, fmt.Errorf("WhatsApp store not available")
	}
	return a.waMsgStore.GetChatMessages(chatJID, limit)
}

// WhatsAppAvatar returns the JPEG bytes of a chat's profile picture, or
// (nil, nil) when there is none. Result is cached on disk by the client.
// full selects the full-resolution photo (for the enlarged preview) over the
// list thumbnail.
func (a *App) WhatsAppAvatar(jid string, full bool) ([]byte, error) {
	if a.waClient == nil {
		return nil, fmt.Errorf("WhatsApp not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return a.waClient.GetProfilePicture(ctx, jid, !full)
}

// WhatsAppStats returns message statistics.
func (a *App) WhatsAppStats() (total, last24h int, err error) {
	if a.waMsgStore == nil {
		return 0, 0, fmt.Errorf("WhatsApp store not available")
	}
	return a.waMsgStore.Stats()
}

// ─── WhatsApp Agent Tool Adapter ────────────────────────────────

// waToolAdapter wraps *whatsapp.Client to satisfy tools.WhatsAppClient.
type waToolAdapter struct {
	c *whatsapp.Client
}

func (a waToolAdapter) SendMessage(ctx context.Context, jid, text string) (string, error) {
	return a.c.SendMessage(ctx, jid, text)
}

func (a waToolAdapter) SearchMessages(query string, limit int) ([]tools.WhatsAppMsg, error) {
	if a.c == nil {
		return nil, nil
	}
	msgs, err := a.c.SearchMessages(query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]tools.WhatsAppMsg, len(msgs))
	for i, m := range msgs {
		out[i] = tools.WhatsAppMsg{
			ID:         m.ID,
			ChatJID:    m.ChatJID,
			SenderJID:  m.SenderJID,
			SenderName: m.SenderName,
			Text:       m.Text,
			Timestamp:  m.Timestamp,
			FromMe:     m.FromMe,
		}
	}
	return out, nil
}

func (a waToolAdapter) GetChatList() ([]tools.WhatsAppChat, error) {
	if a.c == nil {
		return nil, nil
	}
	chats, err := a.c.GetChatList()
	if err != nil {
		return nil, err
	}
	out := make([]tools.WhatsAppChat, len(chats))
	for i, c := range chats {
		out[i] = tools.WhatsAppChat{
			JID:         c.JID,
			DisplayName: c.DisplayName,
			LastMessage: c.LastMessage,
			LastTime:    c.LastTime,
			Unread:      c.Unread,
		}
	}
	return out, nil
}

func (a waToolAdapter) GetChatMessages(chatJID string, limit int) ([]tools.WhatsAppMsg, error) {
	if a.c == nil {
		return nil, nil
	}
	msgs, err := a.c.GetChatMessages(chatJID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]tools.WhatsAppMsg, len(msgs))
	for i, m := range msgs {
		out[i] = tools.WhatsAppMsg{
			ID:         m.ID,
			ChatJID:    m.ChatJID,
			SenderJID:  m.SenderJID,
			SenderName: m.SenderName,
			Text:       m.Text,
			Timestamp:  m.Timestamp,
			FromMe:     m.FromMe,
		}
	}
	return out, nil
}

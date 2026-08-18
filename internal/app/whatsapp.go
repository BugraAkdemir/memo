package app

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/logx"
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
		logx.Printf("WhatsApp: mkdir data dir: %v", err)
		return
	}

	msgDB := filepath.Join(dataDir, "messages.db")
	sessionDB := filepath.Join(dataDir, "session.db")

	msgStore, err := whatsapp.NewStore(msgDB)
	if err != nil {
		logx.Printf("WhatsApp: store init error: %v", err)
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
		defer recoverPanic("waClient.Start")
		if err := a.waClient.Start(a.lifecycleCtx); err != nil {
			logx.Printf("WhatsApp: auto-connect error: %v", err)
		}
	}()

	// Consume incoming messages for intent extraction and observer recording.
	goRecover("runWhatsAppIntentLoop", func() { a.runWhatsAppIntentLoop(a.lifecycleCtx) })

	logx.Info("WhatsApp client initialized and connecting...")
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
			// Recovered per-message: a panic while recording one WhatsApp
			// message must not permanently kill this loop for the rest of
			// the process's life — every future WhatsApp message would
			// silently stop reaching the observer/intent pipeline.
			func() {
				defer recoverPanic("runWhatsAppIntentLoop/RecordWhatsAppMessage")
				// Record in observer regardless of intent.
				if a.observerRecorder != nil {
					a.observerRecorder.RecordWhatsAppMessage(msg.Text, msg.FromMe, msg.Timestamp)
				}
			}()
			// Run intent extraction asynchronously so the channel never blocks.
			goRecover("processMessageIntent", func() {
				a.processMessageIntent(msg.Text, "whatsapp", msg.SenderName, msg.Timestamp)
			})
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
	// Reset the dedicated WhatsApp chat session so the next connect starts a
	// fresh session instead of silently reusing (or leaking) the old one.
	defer func() { a.whatsAppSessionID = "" }()
	if a.waClient != nil {
		a.waClient.Stop()
	}
}

// LogoutWhatsApp removes the local session.
func (a *App) LogoutWhatsApp() error {
	a.waMu.Lock()
	defer a.waMu.Unlock()
	// A logout pairs a (possibly different) account next time, so the
	// dedicated WhatsApp chat session must not be reused across accounts.
	defer func() { a.whatsAppSessionID = "" }()
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
	return a.whatsappChatMode.Load()
}

// SetWhatsAppChatMode enables or disables WhatsApp chat mode.
func (a *App) SetWhatsAppChatMode(enabled bool) {
	a.whatsappChatMode.Store(enabled)
}

// whatsAppAssistantSystemPrompt is the system message for the dedicated
// WhatsApp-chat assistant (as opposed to the general agent's own whatsapp_send
// tool, which this shares registration/permission machinery with via
// agent.NewWhatsAppExecutor).
//
// The explicit-consent paragraph is the fix for BUG-L2: live testing
// (Session 40) found the model refusing whatsapp_send on its own
// conversation-level judgment across three separate, increasingly direct
// rephrasings of an explicit user request ("annenin haberi olmadan otomatik
// mesaj göndermek doğru değil") — even with --auto-allow already granted, so
// the tool's own permission gate (DangerLevel: Medium, see
// internal/agent/tools.go) was never even reached; the refusal happened
// purely in the model's own reply, before any tool call was attempted. The
// same message sent instead through the direct REST endpoint
// (handleWhatsAppSend) succeeded immediately with no such second-guessing —
// a real inconsistency for a user who has to explain themselves to their own
// assistant to get it to do what they already unambiguously asked for.
// This does not weaken the actual safety boundary (the permission prompt /
// DangerLevel gate, or auto-allow if the user has explicitly turned that on
// — both untouched) — it only stops the model from adding an extra,
// unrequested layer of conversational refusal on top of a request that was
// already clear.
const whatsAppAssistantSystemPrompt = `You are a WhatsApp assistant. You can read the user's WhatsApp messages, search them, and send messages on their behalf.

Tools available to you:
- whatsapp_latest: list the most recently messaged chats
- whatsapp_messages: fetch a chat's message history
- whatsapp_search: search message text
- whatsapp_send: send a message to a contact

If the user asks "who messaged me last", call whatsapp_latest.
If they say "send X a message", call whatsapp_send.
If they ask "what did I write to X", call whatsapp_messages or whatsapp_search.

NOTE: every chat in whatsapp_latest's output comes with a "(jid: ...)" — use
that JID when sending a message or fetching history. You can also refer to a
contact by name alone (e.g. "Sarah") — the system automatically maps the name
to the right contact. NEVER ask the user for a JID; check whatsapp_latest's
chat list first instead.

IMPORTANT — whose decision this is: when the user tells you directly and
clearly who to message and what to say (e.g. "tell X: ...", "message my mom,
...", "send Y: ...") that is already the user's own explicit consent — call
whatsapp_send, and don't add your own conversational gatekeeping on top of it
("are you sure", "I don't think I should send this"). The actual approval
layer is a separate system (a permission prompt, or auto-approval the user
has explicitly turned on) — you don't need to be an additional one. Only ask
for clarification when the recipient or content is genuinely ambiguous (the
user didn't give a clear recipient or message); as long as the request itself
is clear, don't refuse to send it on your own judgment.`

// WhatsAppChatStream handles a chat message in WhatsApp mode.
func (a *App) WhatsAppChatStream(ctx context.Context, userMsg string) <-chan api.StreamChunk {
	outCh := make(chan api.StreamChunk, 128)

	// Prevent concurrent WhatsApp chat streams from racing each other or the
	// main chat stream (they share the same session manager via AddMessageToSession).
	if !a.streamMu.TryLock() {
		errCh := make(chan api.StreamChunk, 1)
		errCh <- api.StreamChunk{Error: "⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", Done: true}
		close(errCh)
		return errCh
	}

	// Use a dedicated WhatsApp session so WhatsApp messages don't pollute the
	// active chat session the user is viewing. whatsAppSessionID is also
	// written (reset to "") by StopWhatsApp/LogoutWhatsApp under waMu, so it
	// must be read/written under the same lock here — otherwise a concurrent
	// Stop/Logout racing this check is a data race and can leave waSessionID
	// pointing at a session that Stop just invalidated.
	sm := a.getSessionManager()
	a.waMu.Lock()
	if a.whatsAppSessionID == "" && sm != nil {
		a.whatsAppSessionID = sm.NewChat()
		if a.whatsAppSessionID != "" {
			sm.RenameChat(a.whatsAppSessionID, "WhatsApp Chat")
		}
	}
	waSessionID := a.whatsAppSessionID
	a.waMu.Unlock()

	go func() {
		defer close(outCh)
		defer a.streamMu.Unlock()
		defer recoverPanic("WhatsAppChatStream")

		// Prefer delivery over ctx cancellation (BUG-H2). A bare
		// `select { case ch <- chunk: case <-ctx.Done(): }` drops the
		// final Done:true chunk ~half the time when both cases are ready —
		// same class fixed in llm.trySend / provider.trySend / streamSSE.
		localTrySend := func(ctx context.Context, ch chan<- api.StreamChunk, chunk api.StreamChunk) bool {
			select {
			case ch <- chunk:
				return true
			default:
			}
			select {
			case ch <- chunk:
				return true
			case <-ctx.Done():
				return false
			}
		}

		messages := a.buildMessages(ctx, userMsg, nil)

		if sm != nil && waSessionID != "" {
			sm.AddMessageToSession(waSessionID, "user", userMsg, "", "")
		}

		pMsgs := make([]provider.Message, len(messages))
		for i, m := range messages {
			pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
		}

		modelName := ""
		a.providerMu.RLock()
		router := a.providerRouter
		cfgMgr := a.providerCfgMgr
		activeName := a.activeProviderName
		a.providerMu.RUnlock()

		// Fall back to local model if no external provider is available.
		if router == nil || !router.HasActiveProvider() {
			a.clientMu.RLock()
			localRunning := a.llamaServer != nil && a.llamaServer.IsRunning()
			a.clientMu.RUnlock()
			if localRunning {
				if router == nil && cfgMgr != nil {
					configs := cfgMgr.GetEnabled()
					if len(configs) == 0 {
						r := provider.NewRouter(nil)
						router = r
					}
				}
			}
		}

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
			if sm != nil && waSessionID != "" {
				sm.AddMessageToSession(waSessionID, "assistant", "⚠️ WhatsApp sohbeti için bir sağlayıcı yapılandırmadınız.", "", "")
			}
			localTrySend(ctx, outCh, api.StreamChunk{
				Error: "WhatsApp sohbeti için bir sağlayıcı yapılandırmadınız.",
				Done:  true,
			})
			return
		}

		waPrompt := provider.Message{
			Role:    "system",
			Content: whatsAppAssistantSystemPrompt,
		}

		allMsgs := make([]provider.Message, 0, len(pMsgs)+1)
		allMsgs = append(allMsgs, waPrompt)
		allMsgs = append(allMsgs, pMsgs...)

		waExecutor := agent.NewWhatsAppExecutor(a.agentExecutor)
		waExecutor.SyncRouter(router)

		effortLevel := a.activeProviderEffortLevel(activeName)

		start := time.Now()
		var fullReply strings.Builder
		var agentEvents []interface{}

		streamCh, err := waExecutor.RunStream(ctx, waSessionID, modelName, effortLevel, allMsgs, func(ev agent.AgentEvent) {
			agentEvents = append(agentEvents, ev)
			chunkData, _ := json.Marshal(ev)
			localTrySend(ctx, outCh, api.StreamChunk{
				Content:      string(chunkData),
				FinishReason: "agent_event",
			})
		})

		if err != nil {
			if sm != nil && waSessionID != "" {
				sm.AddMessageToSession(waSessionID, "assistant", "⚠️ "+err.Error(), "", "", agentEvents)
			}
			localTrySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
			return
		}

	waLoop:
		for {
			// Prefer an already-ready chunk over ctx cancellation (same
			// race class as trySend / streamSSE). A bare select that races
			// ctx.Done() against <-streamCh can drop a simultaneous final
			// Done:true and leave the Flutter client stuck.
			var chunk provider.StreamChunk
			var ok bool
			select {
			case chunk, ok = <-streamCh:
			default:
				select {
				case chunk, ok = <-streamCh:
				case <-ctx.Done():
					if sm != nil && waSessionID != "" {
						sm.AddMessageToSession(waSessionID, "assistant", "⏹️ Cevap durduruldu.", "", "", agentEvents)
					}
					localTrySend(ctx, outCh, api.StreamChunk{Error: "⏹️ Cevap durduruldu.", Done: true})
					return
				}
			}
			if !ok {
				break waLoop
			}
			if chunk.Error != "" {
				if sm != nil && waSessionID != "" {
					sm.AddMessageToSession(waSessionID, "assistant", "⚠️ "+chunk.Error, "", "", agentEvents)
				}
				localTrySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + chunk.Error, Done: true})
				return
			}
			if chunk.Content != "" {
				fullReply.WriteString(chunk.Content)
			}
			localTrySend(ctx, outCh, api.StreamChunk{Content: chunk.Content, FinishReason: chunk.FinishReason, Done: chunk.Done})
			if chunk.Done {
				break waLoop
			}
		}

		reply := fullReply.String()
		if reply != "" && sm != nil && waSessionID != "" {
			sm.AddMessageToSession(waSessionID, "assistant", reply, "", "", agentEvents)
		}

		logx.Printf("WhatsApp chat completed in %v (%d chars)", time.Since(start), len(reply))
		if a.mood != nil && a.mood.Enabled() && userMsg != "" {
			goRecover("updateMoodAsync", func() { a.updateMoodAsync(userMsg) })
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
			JID:           c.JID,
			DisplayName:   c.DisplayName,
			LastMessage:   c.LastMessage,
			LastTime:      c.LastTime,
			TotalReceived: c.TotalReceived,
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

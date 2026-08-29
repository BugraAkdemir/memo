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
	"memo/internal/config"
	"memo/internal/provider"
	"memo/internal/whatsapp"
)

// whatsappReachable reports whether the WhatsApp bridge is live right now
// (connected + logged in) — same check GetWhatsAppStatus's status text
// already uses — so the system prompt (identity.BuildSystemPrompt) can tell
// the model it's actually reachable there.
func (a *App) whatsappReachable() bool {
	return a.waClient != nil && a.waClient.IsConnected() && a.waClient.IsLoggedIn()
}

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
		// Retry the initial connect instead of giving up on the first
		// failure. Right after a reboot the network/DNS often isn't ready
		// when Memo starts, so a one-shot Start() left the bridge dead until
		// a manual reconnect. Client.Start cleans up its own sqlstore handle
		// on failure, so it is safe to call again.
		ok := retryWithBackoff(a.lifecycleCtx, 5*time.Second, 2*time.Minute, func() bool {
			err := a.waClient.Start(a.lifecycleCtx)
			if err != nil {
				logx.Printf("WhatsApp: startup connect failed (%v); will retry", err)
			}
			return err == nil
		})
		if ok {
			logx.Info("WhatsApp client connected")
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
			if a.routeWhatsAppPermissionAnswer(msg) {
				continue
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
			if a.shouldAutoReplyToWhatsApp(msg) {
				goRecover("whatsAppSelfChatReply", func() {
					a.handleWhatsAppSelfChatMessage(msg)
				})
			}
		}
	}
}

// shouldAutoReplyToWhatsApp reports whether msg is a genuine incoming turn
// in the user's own "Message Yourself" WhatsApp chat that the self-chat
// assistant (config.WhatsApp.SelfChatAssistant) should reply to.
//
// A self-chat message is always FromMe (only the account owner can write
// there), so FromMe alone can't distinguish it from "I sent someone else a
// message" — the real signal is ChatJID matching one of the account's own
// bare JIDs (see Client.OwnJIDs' doc comment on why that's a list, not a
// single value — a self-chat's ChatJID can legitimately be either the
// account's phone-number JID or its newer @lid form). IsSelfSentRecently
// additionally guards against treating Memo's own just-sent reply as a new
// incoming command, regardless of whether the underlying protocol ever
// actually echoes a send back through the event stream.
func (a *App) shouldAutoReplyToWhatsApp(msg whatsapp.Message) bool {
	if !a.GetWhatsAppSelfChatAssistant() {
		return false
	}
	if a.waClient == nil {
		return false
	}
	if !isSelfChatMessage(msg, a.waClient.OwnJIDs()) {
		return false
	}
	return !a.waClient.IsSelfSentRecently(msg.ID)
}

// isSelfChatMessage is shouldAutoReplyToWhatsApp's pure matching logic,
// split out so it's unit-testable without a live, connected whatsmeow
// client (internal/app has no way to construct one with a working
// Client.OwnJIDs — see whatsapp_test.go's TestShouldAutoReplyToWhatsApp_Guards
// doc comment). ownJIDs is exactly what Client.OwnJIDs returns.
func isSelfChatMessage(msg whatsapp.Message, ownJIDs []string) bool {
	if !msg.FromMe || strings.TrimSpace(msg.Text) == "" {
		return false
	}
	for _, jid := range ownJIDs {
		if msg.ChatJID == jid {
			return true
		}
	}
	return false
}

// handleWhatsAppSelfChatMessage generates a normal chat reply to a
// self-chat message and sends it back to the same chat, so the user's own
// "Message Yourself" WhatsApp conversation works as another way to talk to
// Memo. Runs in its own dedicated, background session (created without
// switching the user's active chat — see NewBackgroundChat's doc comment)
// so it never collides with or hijacks whatever chat is open in the UI.
//
// A leading slash command (/new, /agent, /status, /help — see
// handleWhatsAppSelfChatCommand) is handled and replied to directly,
// without ever touching the LLM pipeline. Anything else routes through
// SendMessageStreamTo — the same full pipeline (memory, intent, mood,
// session recording) normal chat uses — rather than a raw LLM call, so a
// self-chat conversation gets the same assistant behavior as chatting
// inside Memo itself.
func (a *App) handleWhatsAppSelfChatMessage(msg whatsapp.Message) {
	text := strings.TrimSpace(msg.Text)

	if reply, handled := a.handleWhatsAppSelfChatCommand(text); handled {
		if reply == "" {
			return
		}
		ctx, cancel := context.WithTimeout(a.lifecycleCtx, 15*time.Second)
		defer cancel()
		if _, err := a.WhatsAppSend(ctx, msg.ChatJID, reply); err != nil {
			logx.Printf("WhatsApp self-chat: send command reply error: %v", err)
		}
		return
	}

	// Task-loop control (see the Telegram mirror for the contract).
	if reply, handled := a.handleTaskControl(a.lifecycleCtx, taskSurfaceWhatsApp, text); handled {
		if reply != "" {
			ctx, cancel := context.WithTimeout(a.lifecycleCtx, 300*time.Second)
			defer cancel()
			if _, err := a.WhatsAppSend(ctx, msg.ChatJID, reply); err != nil {
				logx.Printf("WhatsApp self-chat: send task-control reply error: %v", err)
			}
		}
		return
	}

	sm := a.getSessionManager()
	if sm == nil {
		return
	}

	a.waMu.Lock()
	if a.waSelfChatSessionID == "" || !sm.SessionExists(a.waSelfChatSessionID) {
		a.waSelfChatSessionID = sm.NewBackgroundChat("WhatsApp Self-Chat")
	}
	chatID := a.waSelfChatSessionID
	a.waMu.Unlock()
	if chatID == "" {
		logx.Printf("WhatsApp self-chat: could not create session, dropping message")
		return
	}

	// 300s to match llm.go's own generation-budget contract, not the 120s
	// this used to be — a turn that also has to ask a permission question
	// and wait up to 45s for a y/n reply (see selfchat_permission.go) needs
	// real headroom beyond just the LLM call itself.
	ctx, cancel := context.WithTimeout(a.lifecycleCtx, 300*time.Second)
	defer cancel()
	ctx = withSelfChatSource(ctx, SelfChatSource{WhatsApp: true, WhatsAppJID: msg.ChatJID})

	// Shows WhatsApp's native "typing..." indicator for as long as the
	// reply is being generated, so a self-chat turn doesn't just sit there
	// with no feedback until the answer suddenly appears — cleared as soon
	// as generation finishes, deferred so it's cleared on every exit path
	// (including the empty-reply early return below).
	stopComposing := a.startWhatsAppComposing(ctx, msg.ChatJID)
	defer stopComposing()

	reply := a.drainSelfChatReply(
		a.SendMessageStreamToAsAgent(ctx, chatID, text),
		a.GetWhatsAppAutoApprovePermissions(),
		a.whatsAppPermissionQuestion,
		func(q string) error {
			_, err := a.WhatsAppSend(ctx, msg.ChatJID, q)
			return err
		},
		func(waitCtx context.Context) (string, bool) {
			return a.awaitWhatsAppPermissionAnswer(waitCtx, msg.ChatJID)
		},
	)
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return
	}

	if _, err := a.WhatsAppSend(ctx, msg.ChatJID, reply); err != nil {
		logx.Printf("WhatsApp self-chat: send reply error: %v", err)
	}
}

// handleWhatsAppSelfChatCommand recognizes a leading-slash command in a
// self-chat message and returns its reply text directly — handled reports
// whether text was actually a command at all (false means "route this to
// the LLM as a normal message instead", the caller's existing behavior).
// An empty reply with handled=true is valid (nothing to say back).
//
// Replies follow a.GetUILanguage() — the same source of truth the Flutter
// GUI's own language toggle writes to (Identity.UILanguage), read fresh on
// every call rather than snapshotted once, since a headless backend serves
// the GUI and WhatsApp self-chat concurrently and the self-chat surface has
// no startup moment of its own to snapshot a language at (see
// internal/replcli/l10n.go for the CLI's version of this same idea, which
// *does* snapshot once — appropriate there since a CLI session is a single
// short-lived process). See waT's doc comment for the TR/EN table itself.
func (a *App) handleWhatsAppSelfChatCommand(text string) (reply string, handled bool) {
	fields := strings.Fields(text)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return "", false
	}
	cmd := strings.ToLower(fields[0])
	arg := ""
	if len(fields) > 1 {
		arg = strings.ToLower(fields[1])
	}
	lang := waLang(a.GetUILanguage())

	switch cmd {
	case "/new":
		sm := a.getSessionManager()
		if sm == nil {
			return waT(lang, "wa_new_no_sessions"), true
		}
		a.waMu.Lock()
		a.waSelfChatSessionID = sm.NewBackgroundChat("WhatsApp Self-Chat")
		a.waMu.Unlock()
		return waT(lang, "wa_new_ok"), true

	case "/agent":
		switch arg {
		case "on":
			if err := a.SetAgentEnabled(true); err != nil {
				return fmt.Sprintf(waT(lang, "wa_agent_on_err"), err), true
			}
			return waT(lang, "wa_agent_on"), true
		case "off":
			if err := a.SetAgentEnabled(false); err != nil {
				return fmt.Sprintf(waT(lang, "wa_agent_off_err"), err), true
			}
			return waT(lang, "wa_agent_off"), true
		default:
			return fmt.Sprintf(waT(lang, "wa_agent_status"), waOnOff(lang, a.GetAgentEnabled())), true
		}

	case "/web":
		switch arg {
		case "on":
			if err := a.UpdateWebSearchConfig(true); err != nil {
				return fmt.Sprintf(waT(lang, "wa_web_on_err"), err), true
			}
			return waT(lang, "wa_web_on"), true
		case "off":
			if err := a.UpdateWebSearchConfig(false); err != nil {
				return fmt.Sprintf(waT(lang, "wa_web_off_err"), err), true
			}
			return waT(lang, "wa_web_off"), true
		default:
			return fmt.Sprintf(waT(lang, "wa_web_status"), waOnOff(lang, a.GetWebSearchEnabled())), true
		}

	case "/auto-perm":
		switch arg {
		case "on":
			if err := a.SetWhatsAppAutoApprovePermissions(true); err != nil {
				return fmt.Sprintf(waT(lang, "wa_autoperm_on_err"), err), true
			}
			return waT(lang, "wa_autoperm_on"), true
		case "off":
			if err := a.SetWhatsAppAutoApprovePermissions(false); err != nil {
				return fmt.Sprintf(waT(lang, "wa_autoperm_off_err"), err), true
			}
			return waT(lang, "wa_autoperm_off"), true
		default:
			return fmt.Sprintf(waT(lang, "wa_autoperm_status"), waOnOff(lang, a.GetWhatsAppAutoApprovePermissions())), true
		}

	case "/status":
		return a.whatsAppSelfChatStatusText(lang), true

	case "/help":
		return waT(lang, "wa_help"), true

	default:
		// An unrecognized "/word" — still a command attempt, not a chat
		// message someone genuinely wants answered by the model (e.g. a
		// typo'd command shouldn't silently turn into an LLM prompt).
		return fmt.Sprintf(waT(lang, "wa_unknown_command"), fields[0], waT(lang, "wa_help")), true
	}
}

// whatsAppSelfChatStatusText builds the /status reply: a snapshot of
// exactly the state a user would otherwise have to open the app to check.
func (a *App) whatsAppSelfChatStatusText(lang string) string {
	a.clientMu.RLock()
	localRunning := a.llamaServer != nil && a.llamaServer.IsRunning()
	a.clientMu.RUnlock()

	a.providerMu.RLock()
	activeProvider := a.activeProviderName
	a.providerMu.RUnlock()

	var model string
	switch {
	case activeProvider != "":
		model = fmt.Sprintf(waT(lang, "wa_model_cloud"), activeProvider)
	case localRunning:
		model = waT(lang, "wa_model_local")
	default:
		model = waT(lang, "wa_model_none")
	}

	waConnected := a.waClient != nil && a.waClient.IsConnected() && a.waClient.IsLoggedIn()

	return fmt.Sprintf(
		waT(lang, "wa_status_template"),
		model,
		waOnOff(lang, a.GetMemoryEnabled()),
		waOnOff(lang, a.GetAgentEnabled()),
		waOnOff(lang, a.GetWebSearchEnabled()),
		waOnOff(lang, a.GetWhatsAppAutoApprovePermissions()),
		waOnOff(lang, waConnected),
		a.version,
	)
}

func waOnOff(lang string, v bool) string {
	if v {
		return waT(lang, "wa_on")
	}
	return waT(lang, "wa_off")
}

// startWhatsAppComposing shows WhatsApp's native "typing..." chat-presence
// indicator in jid's chat and keeps refreshing it every few seconds for as
// long as a reply is being generated — WhatsApp clears a composing state
// client-side if it isn't renewed for a while, the same reason the real app
// keeps resending it while you're actually typing, and a reply can
// realistically take much longer than that single-shot window. Returns a
// stop function that clears the indicator; callers must defer it
// immediately so it fires on every return path, including an early one.
func (a *App) startWhatsAppComposing(ctx context.Context, jid string) (stop func()) {
	if a.waClient == nil {
		return func() {}
	}
	stopCh := make(chan struct{})
	goRecover("whatsAppComposingLoop", func() {
		if err := a.waClient.SetComposing(ctx, jid, true); err != nil {
			logx.Printf("WhatsApp self-chat: set composing error: %v", err)
			return
		}
		ticker := time.NewTicker(8 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := a.waClient.SetComposing(ctx, jid, true); err != nil {
					return
				}
			}
		}
	})
	return func() {
		close(stopCh)
		// A short, detached context: by the time the reply is ready to
		// send, the caller's own ctx may already be at (or very close to)
		// its deadline, and clearing the indicator matters more than
		// honoring that same deadline here.
		clearCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.waClient.SetComposing(clearCtx, jid, false); err != nil {
			logx.Printf("WhatsApp self-chat: clear composing error: %v", err)
		}
	}
}

// GetWhatsAppSelfChatAssistant reports whether Memo auto-replies to
// messages in the user's own WhatsApp "Message Yourself" chat.
func (a *App) GetWhatsAppSelfChatAssistant() bool {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.WhatsApp.SelfChatAssistant
}

// SetWhatsAppSelfChatAssistant toggles the self-chat assistant, persisting
// to config.yaml. Turning it on also turns web search on — a self-chat
// conversation has no other UI affordance to flip that on (unlike normal
// chat's own toggle button), and "ask Memo something on WhatsApp and it
// can't look anything up" is a worse default than having it already on.
// Unconditional on every enable, not just a first-time default: simpler
// and more predictable than tracking whether this is genuinely the first
// enable ever, at the cost of nudging web search back on if the user
// explicitly turned it off (via /web off or the app's switch) and then
// toggles the assistant off and back on again. It's still just the
// regular, user-visible global toggle underneath — /web on|off from the
// self-chat itself, or the app's own switch, both work normally otherwise.
func (a *App) SetWhatsAppSelfChatAssistant(enabled bool) error {
	a.cfgMu.Lock()
	a.cfg.WhatsApp.SelfChatAssistant = enabled
	if enabled {
		a.cfg.WebSearch.Enabled = true
	}
	a.cfgMu.Unlock()
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	logx.Printf("WhatsApp self-chat assistant: enabled=%v", enabled)
	return nil
}

// GetWhatsAppAutoApprovePermissions reports whether the WhatsApp self-chat
// assistant skips agent tool-call permission prompts (AllowOnce every
// request) instead of asking a y/n question in the chat.
func (a *App) GetWhatsAppAutoApprovePermissions() bool {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.WhatsApp.AutoApprovePermissions
}

// SetWhatsAppAutoApprovePermissions toggles it, persisting to config.yaml.
func (a *App) SetWhatsAppAutoApprovePermissions(enabled bool) error {
	a.cfgMu.Lock()
	a.cfg.WhatsApp.AutoApprovePermissions = enabled
	a.cfgMu.Unlock()
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	logx.Printf("WhatsApp self-chat auto-approve permissions: enabled=%v", enabled)
	return nil
}

// whatsAppPermissionQuestion formats the y/n prompt sent to the self-chat
// when a tool call needs approval — see selfchat_permission.go.
func (a *App) whatsAppPermissionQuestion(ev agent.AgentEvent) string {
	lang := waLang(a.GetUILanguage())
	preview := ev.Preview
	if preview == "" {
		preview = ev.ToolName
	}
	return fmt.Sprintf(waT(lang, "wa_perm_question"), ev.ToolName, preview)
}

// awaitWhatsAppPermissionAnswer blocks until routeWhatsAppPermissionAnswer
// delivers the next message on chatJID, or ctx is done. See the
// waPendingPermAnswerCh/waPendingPermChatJID fields' doc comment (app.go).
func (a *App) awaitWhatsAppPermissionAnswer(ctx context.Context, chatJID string) (string, bool) {
	answerCh := make(chan string, 1)
	a.waMu.Lock()
	a.waPendingPermAnswerCh = answerCh
	a.waPendingPermChatJID = chatJID
	a.waMu.Unlock()
	defer func() {
		a.waMu.Lock()
		if a.waPendingPermAnswerCh == answerCh {
			a.waPendingPermAnswerCh = nil
			a.waPendingPermChatJID = ""
		}
		a.waMu.Unlock()
	}()

	select {
	case ans := <-answerCh:
		return ans, true
	case <-ctx.Done():
		return "", false
	}
}

// routeWhatsAppPermissionAnswer delivers msg to an outstanding permission
// question instead of letting runWhatsAppIntentLoop treat it as a new chat
// turn, if one is currently pending for this exact chat. Reports whether it
// consumed the message.
func (a *App) routeWhatsAppPermissionAnswer(msg whatsapp.Message) bool {
	a.waMu.Lock()
	answerCh := a.waPendingPermAnswerCh
	pendingJID := a.waPendingPermChatJID
	a.waMu.Unlock()
	if answerCh == nil || msg.ChatJID != pendingJID {
		return false
	}
	select {
	case answerCh <- strings.TrimSpace(msg.Text):
	default:
	}
	return true
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
	// Reset the dedicated WhatsApp chat/self-chat sessions so the next
	// connect starts fresh instead of silently reusing (or leaking) the old
	// ones.
	defer func() {
		a.whatsAppSessionID = ""
		a.waSelfChatSessionID = ""
	}()
	if a.waClient != nil {
		a.waClient.Stop()
	}
}

// LogoutWhatsApp removes the local session.
func (a *App) LogoutWhatsApp() error {
	a.waMu.Lock()
	defer a.waMu.Unlock()
	// A logout pairs a (possibly different) account next time, so the
	// dedicated WhatsApp chat/self-chat sessions must not be reused across
	// accounts.
	defer func() {
		a.whatsAppSessionID = ""
		a.waSelfChatSessionID = ""
	}()
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
		errCh <- api.StreamChunk{Error: a.t("⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", "⏳ Please wait until the previous response finishes."), Done: true}
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
				sm.AddMessageToSession(waSessionID, "assistant", a.t("⚠️ WhatsApp sohbeti için bir sağlayıcı yapılandırmadınız.", "⚠️ No provider configured for WhatsApp chat."), "", "")
			}
			localTrySend(ctx, outCh, api.StreamChunk{
				Error: a.t("WhatsApp sohbeti için bir sağlayıcı yapılandırmadınız.", "No provider configured for WhatsApp chat."),
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

		// Same projectPath lookup as the Flutter agent chat path
		// (llm.go's callAgentStream) — without this, a change_directory call
		// from a previous WhatsApp turn would silently be ignored on every
		// later turn, since RunStream falls back to the executor's fixed
		// basePath whenever projectPath is empty.
		waProjectPath := ""
		if sm != nil && waSessionID != "" {
			waProjectPath = sm.GetProjectPath(waSessionID)
		}

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
		}, waProjectPath)

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

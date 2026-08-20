package app

import (
	"context"
	"fmt"
	"memo/internal/logx"
	"strings"
	"time"

	"memo/internal/config"
	"memo/internal/telegram"
)

// telegramHasStoredToken reports whether a bot has previously been
// connected (token saved, previously left enabled) — so Startup can
// auto-reconnect without the user re-pasting the token after every restart,
// the Telegram equivalent of whatsAppHasStoredSession.
func (a *App) telegramHasStoredToken() bool {
	if a.tgStore == nil {
		return false
	}
	st := a.tgStore.Get()
	return st.Enabled && st.BotToken != ""
}

// initTelegram starts polling with the token already on disk. Assumes the
// caller holds tgMu and that a.tgStore is already set — mirrors
// initWhatsApp's contract with StartWhatsApp/Startup.
func (a *App) initTelegram() {
	if a.tgStore == nil {
		a.tgStore = telegram.NewStore(config.DataPath("telegram.json"), nil)
	}
	st := a.tgStore.Get()
	if st.BotToken == "" {
		return
	}

	client := telegram.NewClient(st.BotToken)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.Start(ctx); err != nil {
		logx.Printf("Telegram: auto-connect error: %v", err)
		return
	}
	a.tgClient = client

	goRecover("runTelegramIntentLoop", func() { a.runTelegramIntentLoop(a.lifecycleCtx) })
	logx.Info("Telegram client initialized and connecting...")
}

// runTelegramIntentLoop drains the Telegram message channel, locking in the
// first sender as the bot's permanent owner if none is linked yet, and
// otherwise ignoring anyone who isn't that owner — see
// shouldReplyToTelegram's doc comment for why this lock exists at all
// (unlike WhatsApp self-chat, a Telegram bot token can be messaged by
// anyone who finds the bot).
func (a *App) runTelegramIntentLoop(ctx context.Context) {
	if a.tgClient == nil {
		return
	}
	ch := a.tgClient.MessageChannel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if !a.shouldReplyToTelegram(msg) {
				continue
			}
			goRecover("telegramMessageReply", func() {
				a.handleTelegramMessage(msg)
			})
		}
	}
}

// shouldReplyToTelegram links the very first incoming message's sender as
// the permanent owner (if no owner is linked yet) and otherwise reports
// whether msg came from that already-linked owner. This is the entire
// access-control boundary for the bot: since Telegram bots are reachable by
// anyone who knows the username or link, without this check a stranger
// could talk to Memo through the user's own bot.
func (a *App) shouldReplyToTelegram(msg telegram.Message) bool {
	if a.tgStore == nil {
		return false
	}
	st := a.tgStore.Get()
	if !st.Linked() {
		a.tgStore.SetOwner(msg.ChatID, msg.FromName)
		logx.Printf("Telegram: owner linked (chat_id=%d, name=%q)", msg.ChatID, msg.FromName)
		return true
	}
	return isTelegramOwnerMessage(msg.ChatID, st.OwnerChatID)
}

// isTelegramOwnerMessage is shouldReplyToTelegram's pure matching logic,
// split out for unit testing without a live client — mirrors
// isSelfChatMessage in whatsapp.go.
func isTelegramOwnerMessage(chatID, ownerChatID int64) bool {
	return chatID == ownerChatID
}

// handleTelegramMessage generates a reply to an owner message and sends it
// back, the same way handleWhatsAppSelfChatMessage does for WhatsApp
// self-chat — a leading-slash command is handled directly
// (handleTelegramCommand), anything else routes through
// SendMessageStreamTo using a dedicated background session so it never
// collides with the chat open in the UI.
func (a *App) handleTelegramMessage(msg telegram.Message) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	if reply, handled := a.handleTelegramCommand(text); handled {
		if reply == "" {
			return
		}
		ctx, cancel := context.WithTimeout(a.lifecycleCtx, 15*time.Second)
		defer cancel()
		if err := a.TelegramSend(ctx, msg.ChatID, reply); err != nil {
			logx.Printf("Telegram: send command reply error: %v", err)
		}
		return
	}

	sm := a.getSessionManager()
	if sm == nil {
		return
	}

	a.tgMu.Lock()
	if a.tgSelfChatSessionID == "" || !sm.SessionExists(a.tgSelfChatSessionID) {
		a.tgSelfChatSessionID = sm.NewBackgroundChat("Telegram Asistanı")
	}
	chatID := a.tgSelfChatSessionID
	a.tgMu.Unlock()
	if chatID == "" {
		logx.Printf("Telegram: could not create session, dropping message")
		return
	}

	ctx, cancel := context.WithTimeout(a.lifecycleCtx, 120*time.Second)
	defer cancel()

	stopComposing := a.startTelegramComposing(ctx, msg.ChatID)
	defer stopComposing()

	reply := drainToReply(a.SendMessageStreamTo(ctx, chatID, text))
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return
	}

	if err := a.TelegramSend(ctx, msg.ChatID, reply); err != nil {
		logx.Printf("Telegram: send reply error: %v", err)
	}
}

// handleTelegramCommand mirrors handleWhatsAppSelfChatCommand — same
// command set (/new /agent /web /status /help), same
// command-vs-chat-message split, replies via a.GetUILanguage(). See that
// function's doc comment for the fuller rationale.
func (a *App) handleTelegramCommand(text string) (reply string, handled bool) {
	fields := strings.Fields(text)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return "", false
	}
	cmd := strings.ToLower(fields[0])
	arg := ""
	if len(fields) > 1 {
		arg = strings.ToLower(fields[1])
	}
	lang := tgLang(a.GetUILanguage())

	switch cmd {
	case "/new":
		sm := a.getSessionManager()
		if sm == nil {
			return tgT(lang, "tg_new_no_sessions"), true
		}
		a.tgMu.Lock()
		a.tgSelfChatSessionID = sm.NewBackgroundChat("Telegram Asistanı")
		a.tgMu.Unlock()
		return tgT(lang, "tg_new_ok"), true

	case "/agent":
		switch arg {
		case "on":
			if err := a.SetAgentEnabled(true); err != nil {
				return fmt.Sprintf(tgT(lang, "tg_agent_on_err"), err), true
			}
			return tgT(lang, "tg_agent_on"), true
		case "off":
			if err := a.SetAgentEnabled(false); err != nil {
				return fmt.Sprintf(tgT(lang, "tg_agent_off_err"), err), true
			}
			return tgT(lang, "tg_agent_off"), true
		default:
			return fmt.Sprintf(tgT(lang, "tg_agent_status"), tgOnOff(lang, a.GetAgentEnabled())), true
		}

	case "/web":
		switch arg {
		case "on":
			if err := a.UpdateWebSearchConfig(true); err != nil {
				return fmt.Sprintf(tgT(lang, "tg_web_on_err"), err), true
			}
			return tgT(lang, "tg_web_on"), true
		case "off":
			if err := a.UpdateWebSearchConfig(false); err != nil {
				return fmt.Sprintf(tgT(lang, "tg_web_off_err"), err), true
			}
			return tgT(lang, "tg_web_off"), true
		default:
			return fmt.Sprintf(tgT(lang, "tg_web_status"), tgOnOff(lang, a.GetWebSearchEnabled())), true
		}

	case "/status":
		return a.telegramStatusText(lang), true

	case "/help":
		return tgT(lang, "tg_help"), true

	default:
		return fmt.Sprintf(tgT(lang, "tg_unknown_command"), fields[0], tgT(lang, "tg_help")), true
	}
}

// telegramStatusText builds the /status reply — same shape as
// whatsAppSelfChatStatusText, reporting the Telegram poll connection
// instead of a WhatsApp session.
func (a *App) telegramStatusText(lang string) string {
	a.clientMu.RLock()
	localRunning := a.llamaServer != nil && a.llamaServer.IsRunning()
	a.clientMu.RUnlock()

	a.providerMu.RLock()
	activeProvider := a.activeProviderName
	a.providerMu.RUnlock()

	var model string
	switch {
	case activeProvider != "":
		model = fmt.Sprintf(tgT(lang, "tg_model_cloud"), activeProvider)
	case localRunning:
		model = tgT(lang, "tg_model_local")
	default:
		model = tgT(lang, "tg_model_none")
	}

	a.tgMu.Lock()
	tgConnected := a.tgClient != nil && a.tgClient.IsRunning()
	a.tgMu.Unlock()

	return fmt.Sprintf(
		tgT(lang, "tg_status_template"),
		model,
		tgOnOff(lang, a.GetMemoryEnabled()),
		tgOnOff(lang, a.GetAgentEnabled()),
		tgOnOff(lang, a.GetWebSearchEnabled()),
		tgOnOff(lang, tgConnected),
		a.version,
	)
}

func tgOnOff(lang string, v bool) string {
	if v {
		return tgT(lang, "tg_on")
	}
	return tgT(lang, "tg_off")
}

// startTelegramComposing sends Telegram's "typing…" chat action and keeps
// refreshing it for as long as a reply is being generated — Telegram clears
// it client-side after ~5s if not renewed. Unlike WhatsApp's composing
// indicator, Telegram's Bot API has no explicit "clear" action, so stop()
// here only needs to stop the refresh loop; the indicator expires on its
// own shortly after.
func (a *App) startTelegramComposing(ctx context.Context, chatID int64) (stop func()) {
	a.tgMu.Lock()
	client := a.tgClient
	a.tgMu.Unlock()
	if client == nil {
		return func() {}
	}
	stopCh := make(chan struct{})
	goRecover("telegramComposingLoop", func() {
		if err := client.SetTyping(ctx, chatID); err != nil {
			logx.Printf("Telegram: set typing error: %v", err)
			return
		}
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := client.SetTyping(ctx, chatID); err != nil {
					return
				}
			}
		}
	})
	return func() { close(stopCh) }
}

// StartTelegram validates botToken against the Telegram API, persists it
// (encrypted, via tgStore) and begins polling. Reconnecting with a new
// token first stops any existing connection.
func (a *App) StartTelegram(ctx context.Context, botToken string) error {
	a.tgMu.Lock()
	defer a.tgMu.Unlock()

	botToken = strings.TrimSpace(botToken)
	if botToken == "" {
		return fmt.Errorf("bot token boş olamaz")
	}
	if a.tgStore == nil {
		a.tgStore = telegram.NewStore(config.DataPath("telegram.json"), nil)
	}

	if a.tgClient != nil {
		a.tgClient.Stop()
	}

	client := telegram.NewClient(botToken)
	info, err := client.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("bot token geçersiz: %w", err)
	}
	if err := client.Start(ctx); err != nil {
		return err
	}

	st := a.tgStore.Get()
	st.Enabled = true
	st.BotToken = botToken
	st.BotUsername = info.Username
	a.tgStore.Set(st)

	a.tgClient = client
	goRecover("runTelegramIntentLoop", func() { a.runTelegramIntentLoop(a.lifecycleCtx) })
	logx.Printf("Telegram: connected as @%s", info.Username)
	return nil
}

// StopTelegram pauses polling. The bot token and any linked owner are kept
// on disk (a later StartTelegram call skips re-linking), but Enabled is
// persisted as false so a subsequent app restart does NOT auto-resume a
// connection the user explicitly paused.
func (a *App) StopTelegram() {
	a.tgMu.Lock()
	defer a.tgMu.Unlock()
	if a.tgClient != nil {
		a.tgClient.Stop()
	}
	if a.tgStore != nil {
		st := a.tgStore.Get()
		st.Enabled = false
		a.tgStore.Set(st)
	}
}

// DisconnectTelegram stops polling and wipes the stored bot token and owner
// link entirely — the Telegram equivalent of LogoutWhatsApp. A future
// reconnect (even with the same token) re-links whoever messages the bot
// first, exactly like a fresh setup.
func (a *App) DisconnectTelegram() error {
	a.tgMu.Lock()
	defer a.tgMu.Unlock()
	if a.tgClient != nil {
		a.tgClient.Stop()
		a.tgClient = nil
	}
	if a.tgStore != nil {
		a.tgStore.Clear()
	}
	a.tgSelfChatSessionID = ""
	return nil
}

// GetTelegramStatus reports the bot's configuration/connection state for
// the Settings UI.
func (a *App) GetTelegramStatus() map[string]interface{} {
	a.tgMu.Lock()
	client := a.tgClient
	store := a.tgStore
	a.tgMu.Unlock()

	if store == nil {
		return map[string]interface{}{
			"configured":   false,
			"connected":    false,
			"reconnecting": false,
			"last_error":   "",
			"bot_username": "",
			"owner_linked": false,
			"owner_name":   "",
		}
	}
	st := store.Get()
	connected := client != nil && client.IsRunning()
	reconnecting := client != nil && client.IsReconnecting()
	lastErr := ""
	if client != nil {
		lastErr = client.LastError()
	}
	return map[string]interface{}{
		"configured":   st.BotToken != "",
		"connected":    connected,
		"reconnecting": reconnecting,
		"last_error":   lastErr,
		"bot_username": st.BotUsername,
		"owner_linked": st.Linked(),
		"owner_name":   st.OwnerName,
	}
}

// TelegramSend sends a text message via the Telegram bot.
func (a *App) TelegramSend(ctx context.Context, chatID int64, text string) error {
	a.tgMu.Lock()
	client := a.tgClient
	a.tgMu.Unlock()
	if client == nil {
		return fmt.Errorf("Telegram not initialized")
	}
	return client.SendMessage(ctx, chatID, text)
}

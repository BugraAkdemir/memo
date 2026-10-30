package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"memo/internal/calendar"
	"memo/internal/config"
	"memo/internal/intent"
	"memo/internal/logx"
	"memo/internal/mail"
	"memo/internal/memory"
)

func (a *App) initMail(ctx context.Context) {
	mgr, err := mail.NewManager(config.DataPath("mail", "accounts.json"))
	if err != nil {
		logx.Printf("WARN: mail manager: %v", err)
		return
	}

	gk := mail.NewGatekeeper(mail.BuildLLMSemanticClassifier(a.callMailClassifierLLM))

	a.cfgMu.RLock()
	mailCfg := config.MailConfig{
		Enabled:             true,
		PollIntervalSeconds: 60,
		AutoIndexMemory:     true,
		MaxInitialSync:      30,
	}
	if a.cfg != nil {
		mailCfg = a.cfg.Mail
	}
	a.cfgMu.RUnlock()

	watcher := mail.NewWatcher(mgr, gk, mail.WatcherConfig{
		Enabled:         mailCfg.Enabled,
		PollInterval:    time.Duration(mailCfg.PollIntervalSeconds) * time.Second,
		MaxScanPerPoll:  mailCfg.MaxInitialSync,
		MaxAutonomyAge:  6 * time.Hour,
		AutoIndexMemory: mailCfg.AutoIndexMemory,
	})
	watcher.SetMemoryIndexer(a.indexMailInMemory)
	watcher.SetEventHandler(a.onIncomingMailEvent)

	a.mailMu.Lock()
	a.mailManager = mgr
	a.mailGatekeeper = gk
	a.mailWatcher = watcher
	a.mailMu.Unlock()

	goRecover("mailWatcher.Start", func() { watcher.Start(ctx) })
	logx.Info("Mail subsystem initialized")
}

func (a *App) ensureMailManager() *mail.Manager {
	a.mailMu.RLock()
	mgr := a.mailManager
	a.mailMu.RUnlock()
	if mgr != nil {
		return mgr
	}
	a.mailMu.Lock()
	defer a.mailMu.Unlock()
	if a.mailManager == nil {
		m, err := mail.NewManager(config.DataPath("mail", "accounts.json"))
		if err == nil {
			a.mailManager = m
		}
		if a.mailGatekeeper == nil {
			a.mailGatekeeper = mail.NewGatekeeper(nil)
		}
		if a.mailWatcher == nil && a.mailManager != nil {
			w := mail.NewWatcher(a.mailManager, a.mailGatekeeper, mail.WatcherConfig{
				Enabled:         true,
				PollInterval:    60 * time.Second,
				MaxScanPerPoll:  25,
				MaxAutonomyAge:  6 * time.Hour,
				AutoIndexMemory: true,
			})
			w.SetMemoryIndexer(a.indexMailInMemory)
			w.SetEventHandler(a.onIncomingMailEvent)
			a.mailWatcher = w
		}
	}
	return a.mailManager
}

func (a *App) callMailClassifierLLM(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	cfg := a.decisionSettings()
	if cfg.DecisionEnabled {
		if res, err := a.callDecisionModel(ctx, systemPrompt, userPrompt, 256); err == nil && strings.TrimSpace(res) != "" {
			return res, nil
		}
	}
	return "", fmt.Errorf("mail semantic classifier disabled or unavailable")
}

func (a *App) indexMailInMemory(ctx context.Context, account mail.Account, detail *mail.MessageDetail, decision mail.TriageDecision) error {
	if !a.GetMemoryEnabled() || detail == nil {
		return fmt.Errorf("mail memory indexing disabled or missing message")
	}
	userMsg, _ := mail.FormatMailForMemory(account, detail, decision)
	a.storeMu.RLock()
	store := a.store
	a.storeMu.RUnlock()
	if store == nil {
		return fmt.Errorf("mail memory store not initialized")
	}
	// Triage has already selected this record. Do not run conversational
	// decision/fact extraction or semantic dedup on somebody else's email.
	mctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := store.SaveMail(mctx, memory.Scope{}, userMsg); err != nil {
		return err
	}
	a.emitEvent("memory:saved", "")
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm != nil {
		sm.Increment()
	}
	return nil
}

func (a *App) recordMailCalendarAdded() {
	a.mailMu.RLock()
	watcher := a.mailWatcher
	a.mailMu.RUnlock()
	if watcher != nil {
		watcher.RecordCalendarAdded()
	}
}

func (a *App) onIncomingMailEvent(ctx context.Context, account mail.Account, detail *mail.MessageDetail, decision mail.TriageDecision) {
	if detail == nil {
		return
	}
	if decision.DropFromRAG {
		payload, _ := json.Marshal(map[string]any{
			"account_id": account.ID,
			"uid":        detail.UID,
			"from":       detail.From,
			"subject":    detail.Subject,
			"category":   decision.Category,
			"reason":     decision.Reason,
		})
		a.emitEvent("mail:spam_filtered", string(payload))
		return
	}

	if decision.ExtractCalendar {
		a.extractMailStructuredToCalendar(ctx, detail)
	}

	payload, _ := json.Marshal(map[string]any{
		"account_id": account.ID,
		"email":      account.Email,
		"uid":        detail.UID,
		"seq_num":    detail.SeqNum,
		"from":       detail.From,
		"subject":    detail.Subject,
		"category":   decision.Category,
	})
	a.emitEvent("mail:received", string(payload))
}

func (a *App) extractMailStructuredToCalendar(ctx context.Context, detail *mail.MessageDetail) {
	if a.calendarStore == nil || detail == nil {
		return
	}

	addedStructured := false
	for _, inv := range detail.Structured.CalendarInvites {
		if inv.StartTime.IsZero() {
			continue
		}
		title := strings.TrimSpace(inv.Summary)
		if title == "" {
			title = strings.TrimSpace(detail.Subject)
		}
		var endPtr *time.Time
		if !inv.EndTime.IsZero() {
			t := inv.EndTime
			endPtr = &t
		}
		desc := strings.TrimSpace(inv.Description)
		if inv.Location != "" {
			if desc != "" {
				desc += "\n"
			}
			desc += "Konum: " + inv.Location
		}
		ev, err := a.calendarStore.Add(ctx, calendar.Event{
			Title:       title,
			StartTime:   inv.StartTime,
			EndTime:     endPtr,
			Description: desc,
			Source:      calendar.SourceMail,
			ContactName: detail.From,
		})
		if err == nil {
			addedStructured = true
			a.recordMailCalendarAdded()
			p, _ := json.Marshal(calendar.AddedPayload{ID: ev.ID, Title: ev.Title})
			a.emitEvent("calendar:added", string(p))
		}
	}

	for _, fl := range detail.Structured.Flights {
		if fl.DepartureTime.IsZero() {
			continue
		}
		title := fmt.Sprintf("✈️ %s (%s → %s)", strings.TrimSpace(fl.FlightNumber), fl.DepartureAirport, fl.ArrivalAirport)
		if fl.ReservationNumber != "" {
			title += fmt.Sprintf(" [PNR: %s]", fl.ReservationNumber)
		}
		var endPtr *time.Time
		if !fl.ArrivalTime.IsZero() {
			t := fl.ArrivalTime
			endPtr = &t
		}
		ev, err := a.calendarStore.Add(ctx, calendar.Event{
			Title:       title,
			StartTime:   fl.DepartureTime,
			EndTime:     endPtr,
			Description: detail.Subject,
			Source:      calendar.SourceMail,
			ContactName: detail.From,
		})
		if err == nil {
			addedStructured = true
			a.recordMailCalendarAdded()
			p, _ := json.Marshal(calendar.AddedPayload{ID: ev.ID, Title: ev.Title})
			a.emitEvent("calendar:added", string(p))
		}
	}

	for _, inv := range detail.Structured.Invoices {
		if inv.DueDate.IsZero() {
			continue
		}
		prov := inv.ProviderName
		if prov == "" {
			prov = detail.From
		}
		title := fmt.Sprintf("💳 Fatura Son Ödeme: %s (%s %s)", prov, inv.TotalAmount, inv.Currency)
		ev, err := a.calendarStore.Add(ctx, calendar.Event{
			Title:       strings.TrimSpace(title),
			StartTime:   inv.DueDate,
			Description: detail.Subject,
			Source:      calendar.SourceMail,
			ContactName: detail.From,
		})
		if err == nil {
			addedStructured = true
			a.recordMailCalendarAdded()
			p, _ := json.Marshal(calendar.AddedPayload{ID: ev.ID, Title: ev.Title})
			a.emitEvent("calendar:added", string(p))
		}
	}

	if !addedStructured {
		text := strings.TrimSpace(detail.Subject + "\n" + detail.TextBody)
		ts := detail.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}
		a.processMessageIntent(text, intent.Source("mail"), detail.From, ts)
	}
}

// --- FullBridge Mail Methods ---

func (a *App) MailListAccounts() []mail.Account {
	mgr := a.ensureMailManager()
	if mgr == nil {
		return nil
	}
	return mgr.ListAccounts()
}

func (a *App) MailRemoveAccount(accountID string) error {
	mgr := a.ensureMailManager()
	if mgr == nil {
		return fmt.Errorf("mail subsystem not initialized")
	}
	return mgr.RemoveAccount(accountID)
}

func (a *App) MailAutodiscover(ctx context.Context, emailAddr string) mail.ServerConfig {
	cfg, _ := mail.Autodiscover(ctx, emailAddr)
	return cfg
}

func (a *App) MailStartOAuth(ctx context.Context, provider, emailAddr string) (mail.OAuthSession, error) {
	mgr := a.ensureMailManager()
	if mgr == nil {
		return mail.OAuthSession{}, fmt.Errorf("mail subsystem not initialized")
	}
	var pt mail.ProviderType
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "google", "gmail":
		pt = mail.ProviderGoogle
	case "microsoft", "outlook", "hotmail", "live", "office365":
		pt = mail.ProviderMicrosoft
	default:
		return mail.OAuthSession{}, fmt.Errorf("unsupported oauth provider: %s", provider)
	}
	sess, err := mgr.StartOAuthFlow(pt, emailAddr)
	if err != nil {
		return mail.OAuthSession{}, err
	}
	return sess.Snapshot(), nil
}

func (a *App) MailGetOAuthStatus(state string) (mail.OAuthSession, bool) {
	mgr := a.ensureMailManager()
	if mgr == nil {
		return mail.OAuthSession{}, false
	}
	return mgr.GetSession(state)
}

func (a *App) MailCompleteManualOAuth(ctx context.Context, state, callbackURL, emailAddr string) (*mail.Account, error) {
	mgr := a.ensureMailManager()
	if mgr == nil {
		return nil, fmt.Errorf("mail subsystem not initialized")
	}
	return mgr.CompleteManualCallback(ctx, state, callbackURL)
}

func (a *App) MailConnectPassword(ctx context.Context, emailAddr, password, displayName, imapHost string, imapPort int, smtpHost string, smtpPort int) (*mail.Account, error) {
	mgr := a.ensureMailManager()
	if mgr == nil {
		return nil, fmt.Errorf("mail subsystem not initialized")
	}
	var custom *mail.ServerConfig
	if strings.TrimSpace(imapHost) != "" && strings.TrimSpace(smtpHost) != "" {
		if imapPort <= 0 {
			imapPort = 993
		}
		if smtpPort <= 0 {
			smtpPort = 587
		}
		custom = &mail.ServerConfig{
			Provider: mail.ProviderCustom,
			IMAPHost: strings.TrimSpace(imapHost),
			IMAPPort: imapPort,
			SMTPHost: strings.TrimSpace(smtpHost),
			SMTPPort: smtpPort,
			AuthType: mail.AuthPassword,
			Source:   "manual",
		}
	}
	return mgr.ConnectWithPassword(ctx, emailAddr, password, displayName, custom)
}

func (a *App) MailFetchInbox(ctx context.Context, accountID string, limit int) (int, []mail.MessageSummary, error) {
	mgr := a.ensureMailManager()
	if mgr == nil {
		return 0, nil, fmt.Errorf("mail subsystem not initialized")
	}
	return mgr.FetchInbox(ctx, accountID, limit)
}

func (a *App) MailFetchMessage(ctx context.Context, accountID string, seqNum uint32) (*mail.MessageDetail, error) {
	mgr := a.ensureMailManager()
	if mgr == nil {
		return nil, fmt.Errorf("mail subsystem not initialized")
	}
	return mgr.FetchMessage(ctx, accountID, seqNum)
}

func (a *App) MailFetchAttachment(ctx context.Context, accountID string, seqNum uint32, partOrName string) (*mail.AttachmentInfo, error) {
	mgr := a.ensureMailManager()
	if mgr == nil {
		return nil, fmt.Errorf("mail subsystem not initialized")
	}
	return mgr.FetchAttachment(ctx, accountID, seqNum, partOrName)
}

func (a *App) MailSearch(ctx context.Context, accountID, query string, limit int) ([]mail.MessageSummary, error) {
	mgr := a.ensureMailManager()
	if mgr == nil {
		return nil, fmt.Errorf("mail subsystem not initialized")
	}
	return mgr.Search(ctx, accountID, query, limit)
}

func (a *App) MailSend(ctx context.Context, req mail.SendRequest) error {
	mgr := a.ensureMailManager()
	if mgr == nil {
		return fmt.Errorf("mail subsystem not initialized")
	}
	if err := mgr.Send(ctx, req); err != nil {
		return err
	}
	a.mailMu.RLock()
	gk := a.mailGatekeeper
	a.mailMu.RUnlock()
	if gk != nil {
		for _, to := range req.To {
			gk.AddKnownContact(to)
		}
		for _, cc := range req.Cc {
			gk.AddKnownContact(cc)
		}
	}
	return nil
}

func (a *App) MailSaveDraft(ctx context.Context, req mail.SendRequest) (string, error) {
	mgr := a.ensureMailManager()
	if mgr == nil {
		return "", fmt.Errorf("mail subsystem not initialized")
	}
	return mgr.SaveDraft(ctx, req)
}

func (a *App) MailSyncNow(ctx context.Context) error {
	_ = a.ensureMailManager()
	a.mailMu.RLock()
	w := a.mailWatcher
	a.mailMu.RUnlock()
	if w == nil {
		return fmt.Errorf("mail watcher not initialized")
	}
	return w.SyncOnce(ctx)
}

func (a *App) MailGetWatcherStats() mail.WatcherStats {
	_ = a.ensureMailManager()
	a.mailMu.RLock()
	w := a.mailWatcher
	a.mailMu.RUnlock()
	if w == nil {
		return mail.WatcherStats{}
	}
	return w.Stats()
}

func (a *App) GetMailConfig() config.MailConfig {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	if a.cfg == nil {
		return config.Default().Mail
	}
	return a.cfg.Mail
}

func (a *App) UpdateMailConfig(cfg config.MailConfig) error {
	if cfg.PollIntervalSeconds <= 0 {
		cfg.PollIntervalSeconds = 60
	}
	if cfg.MaxInitialSync <= 0 {
		cfg.MaxInitialSync = 30
	}
	a.cfgMu.Lock()
	if a.cfg != nil {
		a.cfg.Mail = cfg
		if err := config.Save(a.cfg); err != nil {
			a.cfgMu.Unlock()
			return err
		}
	}
	a.cfgMu.Unlock()

	a.mailMu.RLock()
	w := a.mailWatcher
	a.mailMu.RUnlock()
	if w != nil {
		w.UpdateConfig(mail.WatcherConfig{
			Enabled:         cfg.Enabled,
			PollInterval:    time.Duration(cfg.PollIntervalSeconds) * time.Second,
			MaxScanPerPoll:  cfg.MaxInitialSync,
			MaxAutonomyAge:  6 * time.Hour,
			AutoIndexMemory: cfg.AutoIndexMemory,
		})
	}
	return nil
}

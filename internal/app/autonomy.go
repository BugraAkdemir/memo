package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"memo/internal/autonomy"
	"memo/internal/calendar"
	"memo/internal/logx"
	"memo/internal/mail"
	"memo/internal/provider"
	"memo/internal/twin"
	"memo/internal/whatsapp"
	netmail "net/mail"
	"strings"
	"time"
)

func (a *App) handleAutonomyIncoming(ctx context.Context, in autonomy.IncomingEvent) {
	a.twinMu.RLock()
	defer a.twinMu.RUnlock()
	if a.twinStore == nil {
		return
	}
	store := a.twinStore
	selection, err := store.ReplyModel(ctx)
	if err != nil {
		return
	}
	engine := autonomy.Engine{Store: store, Projects: a.twinProjects, Generate: func(ctx context.Context, system, user string) (string, error) {
		raw, err := a.callAutonomyReplyModel(ctx, selection, system, user)
		if err != nil {
			logx.Printf("autonomy: reply model failed provider=%s model=%s: %v", selection.Provider, selection.Model, err)
		}
		return raw, err
	}, Decide: func(ctx context.Context, system, user string) (string, error) {
		return a.callMailScorer(ctx, system, user)
	}, Shield: a.ensureMailShield(),
		SaveDraft: func(ctx context.Context, v twin.Action) error {
			_, err := a.MailSaveDraft(ctx, mail.SendRequest{AccountID: v.Account, To: []string{v.Recipient}, Subject: "Re: " + v.Subject, Body: v.Body, InReplyTo: v.InReplyTo})
			return err
		},
		Notify: func(v twin.Action) {
			raw, _ := json.Marshal(map[string]string{"id": v.ID, "state": v.State})
			if v.State == "failed" {
				logx.Printf("autonomy: action failed id=%s stage=%s reason=%s", v.ID, v.FailureStage, v.Reason)
			}
			a.emitEvent("autonomy:action_pending", string(raw))
		}}
	if a.calendarStore != nil {
		engine.Calendar = a.calendarStore
	}
	if _, err := engine.HandleIncomingEvent(ctx, in); err != nil {
		logx.Printf("autonomy: preparation failed: %v", err)
	}
}
func (a *App) autonomyMail(ctx context.Context, account mail.Account, d *mail.MessageDetail, decision mail.TriageDecision) bool {
	a.twinMu.RLock()
	s := a.twinStore
	enabled := false
	if s != nil {
		p, err := s.Preferences(ctx)
		enabled = err == nil && p.Autonomy.Enabled && p.Autonomy.Mail
	}
	a.twinMu.RUnlock()
	if !enabled {
		return false
	}
	from, err := netmail.ParseAddress(d.From)
	if err != nil {
		return true
	}
	recipients, err := netmail.ParseAddressList(d.To)
	count := len(recipients)
	if err != nil {
		count = 2
	}
	if strings.TrimSpace(d.Cc) != "" {
		count = 2
	}
	env := autonomy.Envelope{Account: account.ID, Sender: d.From, ThreadID: d.InReplyTo, Headers: d.Headers, RecipientCount: count}
	policy := autonomy.AutoReplyPolicy(env)
	noReply := policy.Blocked && policy.Code != "group_or_cc"
	in := autonomy.IncomingEvent{Key: fmt.Sprintf("INBOX:%d:%s", d.UID, d.MessageID), Channel: "mail", Account: account.ID, Sender: from.Address, Subject: d.Subject, Body: d.TextBody, At: d.Timestamp, InReplyTo: d.MessageID, Envelope: env, NoReply: noReply || strings.EqualFold(from.Address, account.Email), ForceDraft: !decision.AllowAutoReply}
	for _, v := range d.Structured.CalendarInvites {
		if strings.EqualFold(v.Method, "CANCEL") || strings.EqualFold(v.Method, "REPLY") {
			in.NoReply = true
			continue
		}
		end := v.EndTime
		if end.IsZero() {
			end = v.StartTime.Add(time.Hour)
		}
		key := ""
		if v.UID != "" {
			sum := sha256.Sum256([]byte(account.ID + "\x00" + v.UID))
			key = "mail-ics-" + hex.EncodeToString(sum[:])
		}
		title := v.Summary
		if strings.TrimSpace(title) == "" {
			title = d.Subject
		}
		in.CalendarKinds = append(in.CalendarKinds, "meeting")
		in.Calendar = append(in.Calendar, calendar.Event{ID: key, Title: title, StartTime: v.StartTime, EndTime: &end})
	}
	for _, v := range d.Structured.Flights {
		end := v.ArrivalTime
		if end.IsZero() {
			end = v.DepartureTime.Add(time.Hour)
		}
		title := v.FlightNumber
		if strings.TrimSpace(title) == "" {
			title = d.Subject
		}
		in.CalendarKinds = append(in.CalendarKinds, "flight")
		in.Calendar = append(in.Calendar, calendar.Event{Title: title, StartTime: v.DepartureTime, EndTime: &end})
	}
	for _, v := range d.Structured.Invoices {
		title := v.ProviderName
		if strings.TrimSpace(title) == "" {
			title = d.From
		}
		in.CalendarKinds = append(in.CalendarKinds, "invoice")
		in.Calendar = append(in.Calendar, calendar.Event{Title: title, StartTime: v.DueDate})
	}
	a.handleAutonomyIncoming(ctx, in)
	return true
}
func (a *App) autonomyWhatsApp(ctx context.Context, m whatsapp.Message) {
	if m.FromMe || a.waClient == nil {
		return
	}
	own := a.waClient.OwnJIDs()
	if len(own) == 0 {
		return
	}
	for _, jid := range own {
		if m.ChatJID == jid {
			return
		}
	}
	group := strings.HasSuffix(m.ChatJID, "@g.us")
	sender := m.ChatJID
	if group {
		sender = m.SenderJID
	}
	if group { // Group context is not copied into a person's private timeline.
		a.twinMu.RLock()
		s := a.twinStore
		known := false
		if s != nil {
			_, err := s.Resolve(ctx, twin.Alias{Channel: "whatsapp", Address: sender})
			known = err == nil
		}
		a.twinMu.RUnlock()
		if !known {
			return
		}
	}
	a.handleAutonomyIncoming(ctx, autonomy.IncomingEvent{Key: m.ChatJID + ":" + m.ID, Channel: "whatsapp", Account: own[0], Sender: sender, Recipient: m.ChatJID, Body: m.Text, At: m.Timestamp, Envelope: autonomy.Envelope{IsGroup: group}})
}

// Whisper only: external voice never invokes a cloud Live Mode engine.
func (a *App) transcribeExternalVoice(ctx context.Context, data []byte) (string, error) {
	if len(data) == 0 || len(data) > 20<<20 {
		return "", fmt.Errorf("voice exceeds safe limit")
	}
	a.whisperMu.RLock()
	s := a.whisperServer
	a.whisperMu.RUnlock()
	if s == nil {
		return "", fmt.Errorf("local whisper unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	wav, err := autonomy.VoiceWAV(ctx, data)
	if err != nil {
		return "", err
	}
	return s.Transcribe(ctx, wav)
}
func (a *App) autonomyVoiceEnabled(ctx context.Context) bool {
	a.twinMu.RLock()
	defer a.twinMu.RUnlock()
	if a.twinStore == nil {
		return false
	}
	p, err := a.twinStore.Preferences(ctx)
	return err == nil && p.Autonomy.Voice
}

func (a *App) autonomyWhatsAppEnabled(ctx context.Context, m whatsapp.Message) bool {
	if m.FromMe {
		return false
	}
	a.twinMu.RLock()
	defer a.twinMu.RUnlock()
	if a.twinStore == nil {
		return false
	}
	p, err := a.twinStore.Preferences(ctx)
	return err == nil && p.Autonomy.Enabled && p.Autonomy.WhatsApp
}

// Draft generation has its own budget; the five-second classifier deadline is
// too short for a complete reply. The selected API is used without fallback.
func (a *App) callAutonomyReplyModel(ctx context.Context, selection twin.ReplyModel, system, user string) (string, error) {
	selected, err := a.autonomyReplyProvider(selection)
	if err != nil {
		return "", err
	}
	p, err := provider.NewProvider(selected)
	if err != nil {
		return "", err
	}
	if len([]rune(user)) > 8000 {
		return "", fmt.Errorf("autonomy input too long")
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	start := time.Now()
	reply, err := p.ChatCompletion(ctx, provider.ChatRequest{Model: selection.Model, Messages: []provider.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}, MaxTokens: 1800, Temperature: 0, TopP: 1})
	if err != nil {
		return "", err
	}
	if reply == nil || strings.TrimSpace(reply.Content) == "" || len(reply.ToolCalls) > 0 {
		return "", fmt.Errorf("invalid autonomy response")
	}
	if reply.Usage != nil {
		a.recordCallLLMUsage(start, selection.Provider, selection.Model, categoryLearning, reply.Usage.PromptTokens, reply.Usage.CompletionTokens, usageCacheTokens(reply.Usage))
	}
	return strings.TrimSpace(reply.Content), nil
}

func (a *App) autonomyReplyProvider(selection twin.ReplyModel) (provider.ProviderConfig, error) {
	if selection.Provider == "" || selection.Model == "" {
		return provider.ProviderConfig{}, fmt.Errorf("select a mail reply model")
	}
	selected, err := a.decisionProviderConfig(selection.Provider, selection.Model)
	if err != nil {
		return selected, err
	}
	if provider.IsOptInOnly(selected.Type) {
		return selected, fmt.Errorf("web chats cannot receive background mail")
	}
	return selected, nil
}

func (a *App) callMailScorer(ctx context.Context, system, user string) (string, error) {
	cfg := a.decisionSettings()
	if !cfg.DecisionEnabled {
		return "", fmt.Errorf("decision model disabled")
	}
	selected, err := a.decisionProviderConfig(cfg.DecisionProvider, cfg.DecisionModel)
	if err != nil {
		return "", err
	}
	if provider.IsOptInOnly(selected.Type) {
		return "", fmt.Errorf("web chats cannot score background mail")
	}
	return a.callDecisionModel(ctx, system, user, 16)
}

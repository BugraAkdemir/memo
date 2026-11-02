package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"memo/internal/autonomy"
	"memo/internal/logx"
	"memo/internal/telegram"
	"memo/internal/twin"
	"memo/internal/whatsapp"
)

func (a *App) approvalBridge(s *twin.Store) *autonomy.ApprovalBridge {
	return &autonomy.ApprovalBridge{Store: s, Deliver: func(ctx context.Context, id, mode string) error {
		_, err := a.deliverTwinAction(ctx, id, "", "", mode, false)
		return err
	}, Notify: a.notifyApproval}
}
func (a *App) startApprovalBridge() {
	a.twinMu.Lock()
	if a.twinStore == nil || a.approvalCancel != nil || a.lifecycleCtx == nil {
		a.twinMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(a.lifecycleCtx)
	done := make(chan struct{})
	a.approvalCancel = cancel
	a.approvalDone = done
	s := a.twinStore
	a.twinMu.Unlock()
	// Old and interrupted schedules must be reviewed, never caught up on startup.
	actions, err := s.ApprovalActions(ctx)
	if err != nil {
		cancel()
		close(done)
		return
	}
	for _, v := range actions {
		if v.Mode == "grace_period_auto" {
			if err = s.HoldAutomatic(ctx, v.ID); err != nil {
				cancel()
				close(done)
				return
			}
		}
	}
	goRecover("approvalBridge", func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		b := a.approvalBridge(s)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := b.Tick(ctx); err != nil && ctx.Err() == nil {
					logx.Printf("autonomy: approval queue failed: %v", err)
				}
			}
		}
	})
}
func (a *App) stopApprovalBridge() {
	a.twinMu.Lock()
	cancel, done := a.approvalCancel, a.approvalDone
	a.twinMu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
	a.twinMu.Lock()
	a.approvalCancel = nil
	a.approvalDone = nil
	a.twinMu.Unlock()
}
func (a *App) publishApprovalEvent(v twin.Action, phase string) {
	// The ambient event bus never contains private recipient, subject or body.
	raw, _ := json.Marshal(map[string]any{"id": v.ID, "state": v.State, "code": v.ApprovalCode})
	a.emitEvent("autonomy:action_"+map[string]string{"pending": "pending", "executed": "executed"}[phase], string(raw))
}
func (a *App) publishApprovalOnce(ctx context.Context, s *twin.Store, v twin.Action, phase string) {
	if claimed, err := s.ClaimNotification(ctx, v.ID, phase, "ui"); err == nil && claimed {
		a.publishApprovalEvent(v, phase)
	}
}
func (a *App) queueApprovalNotification(v twin.Action, phase string) {
	if a.lifecycleCtx == nil {
		return
	}
	a.goBackground("approvalNotification", func() { a.notifyApproval(a.lifecycleCtx, v, phase) })
}
func (a *App) twinText(tr, en string) string {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	if a.cfg != nil && a.cfg.Identity.UILanguage == "tr" {
		return tr
	}
	return en
}

type approvalTarget struct {
	key  string
	send func(context.Context, string) error
}

func (a *App) approvalTargets() []approvalTarget {
	out := []approvalTarget{}
	a.tgMu.Lock()
	tg, store := a.tgClient, a.tgStore
	a.tgMu.Unlock()
	if tg != nil && store != nil {
		st := store.Get()
		if st.Enabled && st.Linked() && tg.IsRunning() {
			id := st.OwnerChatID
			out = append(out, approvalTarget{fmt.Sprintf("telegram:%d", id), func(ctx context.Context, text string) error { return tg.SendMessage(ctx, id, text) }})
		}
	}
	a.waMu.Lock()
	wa := a.waClient
	a.waMu.Unlock()
	if wa != nil && wa.IsConnected() && wa.IsLoggedIn() {
		own := wa.OwnJIDs()
		if len(own) > 0 {
			jid := own[0]
			// One physical self-chat, even when WhatsApp also exposes a LID alias.
			out = append(out, approvalTarget{"whatsapp:" + jid, func(ctx context.Context, text string) error { _, err := wa.SendMessage(ctx, jid, text); return err }})
		}
	}
	return out
}
func (a *App) notifyApproval(ctx context.Context, v twin.Action, phase string) {
	if ctx.Err() != nil {
		return
	}
	a.twinMu.RLock()
	s := a.twinStore
	a.twinMu.RUnlock()
	if s == nil {
		return
	}
	if phase == "pending" {
		current, err := s.Action(ctx, v.ID)
		if err != nil || current.State != "pending" {
			return
		}
		v = current
	}
	a.publishApprovalOnce(ctx, s, v, phase)
	status := a.twinText("Onay bekliyor", "Pending review")
	switch v.State {
	case "sent":
		status = a.twinText("Gönderildi", "Sent")
	case "cancelled":
		status = a.twinText("İptal edildi", "Cancelled")
	case "send_uncertain", "sending":
		status = a.twinText("Gönderim doğrulanamadı; tekrar göndermeyin", "Delivery unconfirmed; do not resend")
	}
	text := fmt.Sprintf(a.twinText("Memo yanıt #%d · %s\nAlıcı: %s\nKonu: %s", "Memo reply #%d · %s\nRecipient: %s\nSubject: %s"), v.ApprovalCode, status, v.Recipient, v.Subject)
	if phase == "pending" {
		text += "\n\n" + v.Body + "\n\n" + fmt.Sprintf(a.twinText("#onay %d · #iptal %d · #asla %d", "#approve %d · #cancel %d · #never %d"), v.ApprovalCode, v.ApprovalCode, v.ApprovalCode)
		if v.Mode == "grace_period_auto" && v.ExecuteAt != nil {
			text += "\n" + a.twinText("Otomatik gönderim: ", "Automatic send: ") + v.ExecuteAt.Format(time.RFC3339)
		}
	}
	if a.ensureMailShield().InspectOutbound(text, time.Now()).Blocked {
		return
	}
	// Independent delivery attempts: one unavailable service cannot suppress the other.
	var wg sync.WaitGroup
	for _, target := range a.approvalTargets() {
		claimed, err := s.ClaimNotification(ctx, v.ID, phase, target.key)
		if err != nil || !claimed {
			continue
		}
		wg.Go(func() {
			nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			if err := target.send(nctx, text); err != nil {
				logx.Printf("autonomy: notification failed action=%s target=%s", v.ID, strings.SplitN(target.key, ":", 2)[0])
			}
		})
	}
	wg.Wait()
}

// The caller must authenticate the actual owner BEFORE this method. Incoming
// message IDs are durably consumed so a duplicate bare "yes" cannot approve the
// next queued action. Old reconnect/backfill commands cannot authorize delivery.
func (a *App) controlApproval(ctx context.Context, target, eventID, text string, at time.Time) (string, bool) {
	_, _, handled := autonomy.ParseApprovalCommand(text)
	if !handled {
		return "", false
	}
	if at.IsZero() || time.Since(at) > 5*time.Minute || at.After(time.Now().Add(time.Minute)) {
		return a.twinText("Komut eski. Yanıtlar bölümünü kontrol edin.", "Command expired. Check Replies."), true
	}
	a.twinMu.RLock()
	s := a.twinStore
	a.twinMu.RUnlock()
	if s == nil {
		return "", false
	}
	if eventID == "" {
		return "", false
	}
	claimed, err := s.ClaimIncoming(ctx, "approval:"+target+":"+eventID)
	if err != nil || !claimed {
		return "", true
	}
	result, err := a.approvalBridge(s).Control(ctx, target, text)
	if result == "" {
		return "", false
	}
	if err != nil || result == "failed" {
		return a.twinText("İşlem tamamlanamadı. Yanıtlar bölümündeki durumu kontrol edin; tekrar göndermeyin.", "Action could not be completed. Check its status in Replies; do not resend."), true
	}
	switch result {
	case "approve":
		return a.twinText("Yanıt gönderildi.", "Reply sent."), true
	case "cancel":
		return a.twinText("Yanıt iptal edildi.", "Reply cancelled."), true
	case "never":
		return a.twinText("Bu kişiye yanıt kapatıldı; bekleyen yanıtları iptal edildi.", "Replies to this contact are disabled; pending replies cancelled."), true
	default:
		return a.twinText("Bildirimdeki numarayla #onay N, #iptal N veya #asla N yazın. Belirsiz işlem yapılmadı.", "Use #approve N, #cancel N or #never N with the notification number. No ambiguous action was taken."), true
	}
}

func (a *App) routeTelegramApproval(ctx context.Context, msg telegram.Message, explicitOnly bool) bool {
	if msg.ID <= 0 {
		return false
	}
	if explicitOnly && !strings.HasPrefix(strings.TrimSpace(msg.Text), "#") {
		return false
	}
	a.tgMu.Lock()
	store := a.tgStore
	a.tgMu.Unlock()
	if store == nil {
		return false
	}
	st := store.Get()
	// Only a previously linked private owner chat can authorize an operation.
	if !st.Enabled || !st.Linked() || msg.ChatID <= 0 || msg.FromID != msg.ChatID || msg.ChatID != st.OwnerChatID {
		return false
	}
	body, handled := a.controlApproval(ctx, fmt.Sprintf("telegram:%d", msg.ChatID), fmt.Sprint(msg.ID), msg.Text, msg.Timestamp)
	if handled && body != "" {
		nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_ = a.TelegramSend(nctx, msg.ChatID, body)
	}
	return handled
}
func (a *App) routeWhatsAppApproval(ctx context.Context, msg whatsapp.Message, explicitOnly bool) bool {
	if explicitOnly && !strings.HasPrefix(strings.TrimSpace(msg.Text), "#") {
		return false
	}
	a.waMu.Lock()
	wa := a.waClient
	a.waMu.Unlock()
	if wa == nil || !isSelfChatMessage(msg, wa.OwnJIDs()) || wa.IsSelfSentRecently(msg.ID) {
		return false
	}
	own := wa.OwnJIDs()
	if len(own) == 0 {
		return false
	}
	body, handled := a.controlApproval(ctx, "whatsapp:"+own[0], msg.ID, msg.Text, msg.Timestamp)
	if handled && body != "" {
		nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, _ = wa.SendMessage(nctx, msg.ChatJID, body)
	}
	return handled
}

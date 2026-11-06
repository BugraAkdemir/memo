package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"memo/internal/autonomy"
	"memo/internal/logx"
	"memo/internal/mail"
	"memo/internal/twin"
)

// briefingDeliveryWindow is how long after it was built a morning briefing may
// still be sent or retried. A briefing from yesterday is not news.
const briefingDeliveryWindow = 6 * time.Hour

// deliverMorningBriefings sends every recent morning briefing to the channels
// the owner chose. The recipient is always the owner's own account on that
// channel; no address ever comes from the settings text or from a mail.
//
// Each (briefing, channel) send is claimed before it starts, so a second tick, a
// restart or a crash cannot send it twice, and only a send known to have failed
// is retried. It returns how many sends went out.
func (a *App) deliverMorningBriefings(ctx context.Context, store *twin.Store, set twin.BriefingSettings, now time.Time) int {
	if len(set.Channels) == 0 {
		return 0
	}
	recent, err := store.Briefings(ctx, 3)
	if err != nil {
		logx.Printf("briefing: delivery could not list briefings: %v", err)
		return 0
	}
	lang := a.GetUILanguage()
	sent := 0
	for _, b := range recent {
		if b.Kind != "morning" || now.Sub(b.CreatedAt) > briefingDeliveryWindow {
			continue
		}
		subject, body := autonomy.RenderMorningBriefing(b, lang)
		for _, ch := range set.Channels {
			won, err := store.ClaimBriefingDelivery(ctx, b.Key, ch, now)
			if err != nil {
				logx.Printf("briefing: could not claim %s delivery: %v", ch, err)
				continue
			}
			if !won {
				continue
			}
			state := twin.DeliverySent
			if err := a.sendBriefing(ctx, ch, set, subject, body); err != nil {
				state = twin.DeliveryFailed
				var guard *autonomy.BlockedError
				if errors.As(err, &guard) {
					state = twin.DeliveryBlocked
				}
				// The error names the cause, never the briefing's text.
				logx.Printf("briefing: %s delivery %s: %v", ch, state, err)
			} else {
				sent++
			}
			// The send may have outlived the caller's context (shutdown); the
			// outcome is still worth recording, or it would read as uncertain.
			done, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			if err := store.FinishBriefingDelivery(done, b.Key, ch, state, now); err != nil {
				logx.Printf("briefing: could not record %s delivery: %v", ch, err)
			}
			cancel()
		}
	}
	return sent
}

// sendBriefing sends one briefing to the owner on one channel.
func (a *App) sendBriefing(ctx context.Context, channel string, set twin.BriefingSettings, subject, body string) error {
	switch channel {
	case "telegram":
		if v := autonomy.InspectOutboundDLP(body); v.Blocked {
			return &autonomy.BlockedError{Code: v.Code}
		}
		a.tgMu.Lock()
		store := a.tgStore
		a.tgMu.Unlock()
		if store == nil {
			return fmt.Errorf("telegram is not connected")
		}
		st := store.Get()
		if !st.Enabled || !st.Linked() {
			return fmt.Errorf("telegram has no linked owner chat")
		}
		return a.TelegramSend(ctx, st.OwnerChatID, body)
	case "whatsapp":
		if v := autonomy.InspectOutboundDLP(body); v.Blocked {
			return &autonomy.BlockedError{Code: v.Code}
		}
		a.waMu.Lock()
		client := a.waClient
		a.waMu.Unlock()
		if client == nil {
			return fmt.Errorf("whatsapp is not connected")
		}
		own := client.OwnJIDs()
		if len(own) == 0 {
			return fmt.Errorf("whatsapp has no own number yet")
		}
		// The owner's own chat ("message yourself"). Sent through the client, not
		// WhatsAppSend: a briefing is not a conversation with a contact.
		_, err := client.SendMessage(ctx, own[0], body)
		return err
	case "mail":
		mgr := a.ensureMailManager()
		if mgr == nil {
			return fmt.Errorf("mail is not available")
		}
		acct, err := briefingMailAccount(mgr.ListAccounts(), set.MailAccount)
		if err != nil {
			return err
		}
		req := mail.SendRequest{AccountID: acct.ID, To: []string{acct.Email}, Subject: subject, Body: body}
		// The same outbound guard as every mail: the briefing quotes other
		// people's subjects, which can hold a key or a code.
		if err := a.inspectMailOutbound(req); err != nil {
			return err
		}
		return mgr.Send(ctx, req)
	}
	return fmt.Errorf("unknown briefing channel %q", channel)
}

// briefingMailAccount picks the account the briefing is mailed from, which is
// also the address it is mailed to. An account the owner named but that is gone
// is an error: falling back to another account could mail from the wrong one.
func briefingMailAccount(accounts []mail.Account, id string) (mail.Account, error) {
	for _, acct := range accounts {
		if (id == "" || acct.ID == id) && acct.Email != "" {
			return acct, nil
		}
	}
	if id != "" {
		return mail.Account{}, fmt.Errorf("the briefing mail account is no longer connected")
	}
	return mail.Account{}, fmt.Errorf("no mail account is connected")
}

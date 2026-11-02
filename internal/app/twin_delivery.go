package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"memo/internal/autonomy"
	"strings"
	"time"

	"memo/internal/logx"
	"memo/internal/mail"
	"memo/internal/twin"
)

var ErrTwinSendUncertain = errors.New("mail delivery could not be confirmed")

// Owner edits are explicit approval. Background and remote paths use the same
// reservation with stricter current-channel checks.
func (a *App) sendTwinAction(ctx context.Context, raw json.RawMessage) (any, error) {
	var input struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := decodeTwin(raw, &input); err != nil {
		return nil, err
	}
	return a.deliverTwinAction(ctx, input.ID, input.Subject, input.Body, "owner_approved", true)
}

func (a *App) deliverTwinAction(ctx context.Context, id, subject, body, mode string, edited bool) (any, error) {
	ctx, cancelSend := context.WithTimeout(ctx, 60*time.Second)
	defer cancelSend()
	a.twinMu.RLock()
	s := a.twinStore
	if s == nil {
		a.twinMu.RUnlock()
		return nil, fmt.Errorf("twin unavailable")
	}
	action, err := s.Action(ctx, id)
	a.twinMu.RUnlock()
	if err != nil {
		return nil, err
	}
	if !edited {
		subject = action.Subject
		body = action.Body
		if action.Channel == "mail" && !strings.HasPrefix(strings.ToLower(subject), "re:") {
			subject = "Re: " + subject
		}
	}
	if action.Channel == "mail" {
		if err = a.inspectMailOutbound(mail.SendRequest{AccountID: action.Account, To: []string{action.Recipient}, Subject: subject, Body: body, InReplyTo: action.InReplyTo}); err != nil {
			return nil, err
		}
		mgr := a.ensureMailManager()
		if mgr == nil {
			return nil, fmt.Errorf("mail unavailable")
		}
		if _, ok := mgr.GetAccount(action.Account); !ok {
			return nil, twin.ErrNotFound
		}
	} else if action.Channel == "whatsapp" {
		if v := a.ensureMailShield().InspectOutbound(body, time.Now()); v.Blocked {
			return nil, &autonomy.BlockedError{Code: v.Code}
		}
		a.waMu.Lock()
		client := a.waClient
		a.waMu.Unlock()
		if client == nil || !client.IsConnected() || !client.IsLoggedIn() {
			return nil, twin.ErrDeliveryDenied
		}
		own := false
		for _, jid := range client.OwnJIDs() {
			if jid == action.Account {
				own = true
			}
		}
		if !own {
			return nil, twin.ErrDeliveryDenied
		}
	} else {
		return nil, twin.ErrInvalid
	}
	a.twinMu.RLock()
	action, err = s.BeginDelivery(ctx, id, subject, body, mode, time.Now())
	a.twinMu.RUnlock()
	if err != nil {
		return nil, err
	}
	if action.Channel == "mail" {
		err = a.MailSend(ctx, mail.SendRequest{AccountID: action.Account, To: []string{action.Recipient}, Subject: action.Subject, Body: action.Body, InReplyTo: action.InReplyTo})
	} else {
		_, err = a.WhatsAppSend(ctx, action.Recipient, action.Body)
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	a.twinMu.RLock()
	finishErr := s.FinishMailDelivery(finishCtx, action, err == nil)
	a.twinMu.RUnlock()
	action.State = "sent"
	if err != nil || finishErr != nil {
		action.State = "send_uncertain"
	}
	a.publishApprovalEvent(action, "executed")
	// The worker/remote path publishes transport notifications after reservation;
	// owner HTTP sends share exactly the same terminal-state notification.
	a.queueApprovalNotification(action, "executed")
	if err != nil || finishErr != nil {
		logx.Printf("twin: delivery uncertain action=%s transport=%v record=%v", action.ID, err, finishErr)
		return nil, ErrTwinSendUncertain
	}
	return map[string]any{"ok": true, "state": "sent", "id": action.ID}, nil
}

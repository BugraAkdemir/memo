package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"memo/internal/logx"
	"memo/internal/mail"
	"memo/internal/twin"
)

var ErrTwinSendUncertain = errors.New("mail delivery could not be confirmed")

// Only the owner-facing HTTP operation invokes this function. Preparation and
// grace-period candidates never invoke delivery on their own.
func (a *App) sendTwinAction(ctx context.Context, raw json.RawMessage) (any, error) {
	var input struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := decodeTwin(raw, &input); err != nil {
		return nil, err
	}
	a.twinMu.RLock()
	s := a.twinStore
	if s == nil {
		a.twinMu.RUnlock()
		return nil, fmt.Errorf("twin unavailable")
	}
	action, err := s.Action(ctx, input.ID)
	a.twinMu.RUnlock()
	if err != nil {
		return nil, err
	}
	req := mail.SendRequest{AccountID: action.Account, To: []string{action.Recipient}, Subject: input.Subject, Body: input.Body, InReplyTo: action.InReplyTo}
	if err = a.inspectMailOutbound(req); err != nil {
		return nil, err
	}
	mgr := a.ensureMailManager()
	if mgr == nil {
		return nil, fmt.Errorf("mail unavailable")
	}
	if _, ok := mgr.GetAccount(action.Account); !ok {
		return nil, twin.ErrNotFound
	}
	a.twinMu.RLock()
	action, err = s.BeginMailDelivery(ctx, input.ID, input.Subject, input.Body)
	a.twinMu.RUnlock()
	if err != nil {
		return nil, err
	}
	req = mail.SendRequest{AccountID: action.Account, To: []string{action.Recipient}, Subject: action.Subject, Body: action.Body, InReplyTo: action.InReplyTo}
	err = a.MailSend(ctx, req)
	// A disconnected UI must not discard the SMTP result after submission.
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	a.twinMu.RLock()
	finishErr := s.FinishMailDelivery(finishCtx, action, err == nil)
	a.twinMu.RUnlock()
	if err != nil || finishErr != nil {
		logx.Printf("mail: approved delivery uncertain action=%s transport=%v record=%v", action.ID, err, finishErr)
		return nil, ErrTwinSendUncertain
	}
	return map[string]any{"ok": true, "state": "sent", "id": action.ID}, nil
}

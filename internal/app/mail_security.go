package app

import (
	"context"
	"memo/internal/autonomy"
	"memo/internal/mail"
	"strings"
	"time"
)

func (a *App) ensureMailShield() *autonomy.Shield {
	a.mailMu.Lock()
	defer a.mailMu.Unlock()
	if a.mailShield == nil {
		a.mailShield = &autonomy.Shield{}
	}
	return a.mailShield
}
func (a *App) mailSecurityDecider() autonomy.Decider {
	if !a.decisionSettings().DecisionEnabled {
		return nil
	}
	return func(ctx context.Context, system, user string) (string, error) {
		return a.callDecisionModel(ctx, system, user, 64)
	}
}
func (a *App) inspectIncomingMail(ctx context.Context, account mail.Account, detail *mail.MessageDetail) autonomy.Verdict {
	a.ensureMailShield().ObserveOTP(detail.Structured.OTPCode, time.Now())
	text, _ := mail.FormatMailForMemory(account, detail, mail.TriageDecision{})
	return autonomy.DetectPromptInjection(ctx, text, a.mailSecurityDecider())
}
func (a *App) inspectMailOutbound(req mail.SendRequest) error {
	// Check every outward header as well as the body. Even a draft is uploaded
	// to the provider, so DLP runs BEFORE SMTP or IMAP connections are opened.
	text := strings.Join(append(append([]string{req.Subject, req.Body, req.InReplyTo}, req.To...), req.Cc...), "\n")
	v := a.ensureMailShield().InspectOutbound(text, time.Now())
	if !v.Blocked {
		return nil
	}
	a.mailMu.RLock()
	watcher := a.mailWatcher
	a.mailMu.RUnlock()
	if watcher != nil {
		watcher.RecordSecurityBlocked(v.Code)
	}
	a.emitEvent("mail:security_blocked", v.Code)
	return &autonomy.BlockedError{Code: v.Code}
}

// MailCheckSecurity previews guards without sending mail, learning an OTP,
// mutating counters or storing the submitted content.
func (a *App) MailCheckSecurity(ctx context.Context, text string) autonomy.SecurityReport {
	outbound := a.ensureMailShield().InspectOutbound(text, time.Now())
	decide := a.mailSecurityDecider()
	// Do not forward a locally detected secret to the optional review model.
	if outbound.Blocked {
		decide = nil
	}
	return autonomy.SecurityReport{
		Inbound:  autonomy.DetectPromptInjection(ctx, text, decide),
		Outbound: outbound,
	}
}

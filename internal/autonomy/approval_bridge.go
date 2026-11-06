package autonomy

import (
	"context"
	"memo/internal/twin"
	"strings"
	"time"
)

// ApprovalBridge contains no network credentials or model calls. Deliver must
// revalidate settings/DLP and atomically reserve before contacting a transport.
type ApprovalBridge struct {
	Store   *twin.Store
	Deliver func(context.Context, string, string) error
	Notify  func(context.Context, twin.Action, string)
	Now     func() time.Time
}

func (b *ApprovalBridge) Tick(ctx context.Context) error {
	actions, err := b.Store.ApprovalActions(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	if b.Now != nil {
		now = b.Now()
	}
	for _, a := range actions {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Pre-bridge records remain owner-reviewable but are never armed or pushed.
		if a.DeliveryVersion != 1 || a.Body == "" {
			continue
		}
		if b.Notify != nil {
			b.Notify(ctx, a, "pending")
		}
		if a.Mode != "grace_period_auto" || a.ExecuteAt == nil || now.Before(*a.ExecuteAt) {
			continue
		}
		if b.Deliver == nil {
			continue
		}
		err = b.Deliver(ctx, a.ID, "automatic")
		if err != nil {
			if err = b.Store.HoldAutomatic(ctx, a.ID); err != nil && err != twin.ErrDeliveryConflict {
				return err
			}
		}
	}
	return nil
}

// ParseApprovalCommand is deliberately exact. Arbitrary conversation, quoted
// commands and untrusted sender text must not become owner authorization.
func ParseApprovalCommand(text string) (op, code string, handled bool) {
	text = strings.ToLower(strings.TrimSpace(text))
	text = strings.NewReplacer("ı", "i", "ö", "o", "ü", "u", "İ", "i").Replace(text)
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", "", false
	}
	switch fields[0] {
	case "#onay", "#approve":
		op = "approve"
	case "#iptal", "#cancel":
		op = "cancel"
	case "#asla", "#never":
		op = "never"
	default:
		switch text {
		case "evet", "gonder", "yes", "send":
			return "approve", "", true
		case "iptal", "cancel":
			return "cancel", "", true
		case "bir daha bu kisiye otomatik donme", "bir daha bu kişiye otomatik donme", "never reply to this contact again":
			return "never", "", true
		}
		return "", "", false
	}
	if len(fields) != 2 {
		return "invalid", "", true
	}
	for _, r := range fields[1] {
		if r < '0' || r > '9' {
			return "invalid", "", true
		}
	}
	return op, fields[1], true
}

func (b *ApprovalBridge) Control(ctx context.Context, target, text string) (string, error) {
	op, code, handled := ParseApprovalCommand(text)
	if !handled {
		return "", nil
	}
	if op == "invalid" {
		return "choose", nil
	}
	a, err := b.Store.ApprovalForOwner(ctx, target, code, op == "never")
	if err != nil {
		if code == "" && err == twin.ErrNotFound {
			return "", nil
		}
		return "choose", nil
	}
	switch op {
	case "approve":
		if b.Deliver == nil {
			return "failed", nil
		}
		err = b.Deliver(ctx, a.ID, "bridge_approved")
	case "cancel":
		err = b.Store.CancelAction(ctx, a.ID)
	case "never":
		err = b.Store.DenyActionContact(ctx, a.ID)
	}
	if err != nil {
		return "failed", err
	}
	if op != "approve" && b.Notify != nil && a.State == "pending" {
		a.State = "cancelled"
		b.Notify(ctx, a, "executed")
	}
	return op, nil
}

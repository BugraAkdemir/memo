package twin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrDeliveryConflict = errors.New("reply is no longer pending")
var ErrDeliveryDenied = errors.New("reply contact is no longer permitted")
var ErrSentHistory = errors.New("same reply already exists in sent history")

func (s *Store) Action(ctx context.Context, id string) (Action, error) {
	var a Action
	var raw, state string
	err := s.db.QueryRowContext(ctx, "SELECT rowid,payload,state FROM pending_autonomous_actions WHERE id=?", id).Scan(&a.ApprovalCode, &raw, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, err
	}
	code := a.ApprovalCode
	if err = json.Unmarshal([]byte(raw), &a); err != nil {
		return a, err
	}
	a.ApprovalCode = code
	a.State = state
	return a, nil
}

// BeginMailDelivery durably reserves one explicit owner-approved send. A lost
// response, parallel click, restart or ambiguous SMTP failure cannot retry it.
func (s *Store) BeginMailDelivery(ctx context.Context, id, subject, body string) (Action, error) {
	return s.BeginDelivery(ctx, id, subject, body, "owner_approved", time.Now())
}

// BeginDelivery serializes consent, cancellation and the transport reservation.
// bridge_approved and automatic require current channel consent.
func (s *Store) BeginDelivery(ctx context.Context, id, subject, body, mode string, now time.Time) (Action, error) {
	var a Action
	if mode != "owner_approved" && mode != "bridge_approved" && mode != "automatic" {
		return a, ErrInvalid
	}
	if id == "" || strings.TrimSpace(body) == "" || len(body) > 16000 || len(subject) > 998 || strings.ContainsAny(subject, "\r\n") {
		return a, ErrInvalid
	}
	err := s.db.Write(ctx, func(tx *sql.Tx) error {
		var raw, state string
		var code int64
		err := tx.QueryRowContext(ctx, "SELECT rowid,payload,state FROM pending_autonomous_actions WHERE id=?", id).Scan(&code, &raw, &state)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if err = json.Unmarshal([]byte(raw), &a); err != nil {
			return err
		}
		a.ApprovalCode = code
		if state != "pending" {
			return ErrDeliveryConflict
		}
		if (a.Channel != "mail" && a.Channel != "whatsapp") || a.Account == "" || a.Recipient == "" || a.Body == "" {
			return ErrInvalid
		}
		address := a.SourceAddress
		if address == "" {
			address = a.Recipient
		}
		var policy string
		if err = tx.QueryRowContext(ctx, `SELECT c.autonomy_policy FROM contacts c JOIN contact_aliases ca ON ca.contact_id=c.id WHERE c.id=? AND ca.channel=? AND ca.address=?`, a.ContactID, a.Channel, strings.ToLower(address)).Scan(&policy); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrDeliveryDenied
			}
			return err
		}
		if policy == "never" {
			return ErrDeliveryDenied
		}
		if mode != "owner_approved" {
			var rawPrefs string
			if err = tx.QueryRowContext(ctx, "SELECT value FROM twin_preferences WHERE id=1").Scan(&rawPrefs); err != nil {
				return ErrDeliveryDenied
			}
			var p Preferences
			if err = json.Unmarshal([]byte(rawPrefs), &p); err != nil {
				return err
			}
			if !p.Autonomy.Enabled || (a.Channel == "mail" && !p.Autonomy.Mail) || (a.Channel == "whatsapp" && !p.Autonomy.WhatsApp) {
				return ErrDeliveryDenied
			}
			if mode == "automatic" {
				var reserved int
				if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM autonomy_auto_reservations WHERE event_key=? AND contact_id=?", a.ID, a.ContactID).Scan(&reserved); err != nil {
					return err
				}
				if a.DeliveryVersion != 1 || a.Mode != "grace_period_auto" || a.ExecuteAt == nil || now.Before(*a.ExecuteAt) || now.Sub(*a.ExecuteAt) > 30*time.Second || a.Group || a.Conflict || len(a.CalendarIDs) > 0 || a.ScoreStatus != "scored" || a.DecisionScore == nil || !(*a.DecisionScore >= .95 && *a.DecisionScore <= 1) || reserved != 1 || EffectiveReplyPolicy(p.Autonomy.Mode, policy) != "full_auto" {
					return ErrDeliveryDenied
				}
			}
		}
		// Older manual sends had no action id. Preserve their evidence rather than
		// offering to send an already delivered test/template a second time.
		var sent int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM contact_messages WHERE channel=? AND account=? AND address=? AND outgoing=1 AND body=? AND julianday(sent_at)>=julianday(?)`, a.Channel, a.Account, strings.ToLower(a.Recipient), a.Body, a.CreatedAt.Format(time.RFC3339Nano)).Scan(&sent); err != nil {
			return err
		}
		if sent > 0 {
			return ErrSentHistory
		}
		a.Subject = subject
		a.Body = body
		a.State = "sending"
		a.Stage = ""
		a.ExecuteAt = nil
		a.Mode = mode
		a.Reason = mode
		a.UpdatedAt = time.Now().UTC()
		rawBytes, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE pending_autonomous_actions SET payload=?,state='sending',execute_at=NULL WHERE id=? AND state='pending'", string(rawBytes), id); err != nil {
			return err
		}
		return audit(ctx, tx, "reply_send_started", a.ContactID)
	})
	return a, err
}

func (s *Store) FinishMailDelivery(ctx context.Context, a Action, delivered bool) error {
	a.UpdatedAt = time.Now().UTC()
	a.State = "send_uncertain"
	a.Reason = "send_uncertain"
	if delivered {
		a.State = "sent"
		a.Reason = "sent"
		a.SentAt = &a.UpdatedAt
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return s.db.Write(ctx, func(tx *sql.Tx) error {
		r, err := tx.ExecContext(ctx, "UPDATE pending_autonomous_actions SET payload=?,state=? WHERE id=? AND state='sending'", string(raw), a.State, a.ID)
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrDeliveryConflict
		}
		return audit(ctx, tx, "reply_"+a.State, a.ContactID)
	})
}

package twin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// ApprovalActions reads the delivery queue independently of the UI's latest-100
// history. A busy inbox must not hide an older pending approval from the worker.
func (s *Store) ApprovalActions(ctx context.Context) ([]Action, error) {
	return s.approvalActions(ctx, false)
}
func (s *Store) approvalActions(ctx context.Context, includeFinished bool) ([]Action, error) {
	query := "SELECT rowid,payload,state FROM pending_autonomous_actions"
	if !includeFinished {
		query += " WHERE state='pending'"
	}
	query += " ORDER BY rowid"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Action{}
	for rows.Next() {
		var raw, state string
		var code int64
		if err = rows.Scan(&code, &raw, &state); err != nil {
			return nil, err
		}
		var a Action
		if err = json.Unmarshal([]byte(raw), &a); err != nil {
			return nil, err
		}
		a.ApprovalCode = code
		a.State = state
		out = append(out, a)
	}
	return out, rows.Err()
}

// HoldAutomatic removes scheduling, never consent or the reviewable draft.
// Startup calls this before starting the worker: there is no offline catch-up.
func (s *Store) HoldAutomatic(ctx context.Context, id string) error {
	return s.db.Write(ctx, func(tx *sql.Tx) error {
		var raw, state string
		if err := tx.QueryRowContext(ctx, "SELECT payload,state FROM pending_autonomous_actions WHERE id=?", id).Scan(&raw, &state); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if state != "pending" {
			return ErrDeliveryConflict
		}
		var a Action
		if err := json.Unmarshal([]byte(raw), &a); err != nil {
			return err
		}
		if a.Mode != "grace_period_auto" {
			return nil
		}
		a.Mode = "awaiting_approval"
		a.ExecuteAt = nil
		a.Reason = "delivery_paused"
		b, err := json.Marshal(a)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "UPDATE pending_autonomous_actions SET payload=?,execute_at=NULL WHERE id=? AND state='pending'", string(b), id)
		return err
	})
}

// Notification attempts are claimed before the network call. An uncertain
// notification is not replayed automatically across restarts.
func (s *Store) ClaimNotification(ctx context.Context, id, phase, target string) (bool, error) {
	var claimed bool
	err := s.db.Write(ctx, func(tx *sql.Tx) error {
		r, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO autonomy_notifications(action_id,phase,target,at) VALUES(?,?,?,?)", id, phase, target, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		claimed = n == 1
		return err
	})
	return claimed, err
}

// ApprovalForOwner requires a notification addressed to this owner surface.
// Bare confirmations are accepted only when exactly one such action is pending.
func (s *Store) ApprovalForOwner(ctx context.Context, target, code string, includeFinished ...bool) (Action, error) {
	all := len(includeFinished) > 0 && includeFinished[0]
	actions, err := s.approvalActions(ctx, all)
	if err != nil {
		return Action{}, err
	}
	var found []Action
	for _, a := range actions {
		if code != "" && code != strconv.FormatInt(a.ApprovalCode, 10) {
			continue
		}
		var exists bool
		if err = s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM autonomy_notifications WHERE action_id=? AND phase='pending' AND target=?)", a.ID, target).Scan(&exists); err != nil {
			return Action{}, err
		}
		if exists {
			found = append(found, a)
		}
	}
	if len(found) == 0 {
		return Action{}, ErrNotFound
	}
	if len(found) != 1 {
		return Action{}, ErrDeliveryConflict
	}
	return found[0], nil
}

// DenyActionContact atomically changes the person's rule and cancels all their
// pending replies, racing the same writer as transport reservation.
func (s *Store) DenyActionContact(ctx context.Context, id string) error {
	return s.db.Write(ctx, func(tx *sql.Tx) error {
		var contact, state string
		if err := tx.QueryRowContext(ctx, "SELECT contact_id,state FROM pending_autonomous_actions WHERE id=?", id).Scan(&contact, &state); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if contact == "" {
			return ErrInvalid
		}
		if _, err := tx.ExecContext(ctx, "UPDATE contacts SET autonomy_policy='never' WHERE id=?", contact); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE pending_autonomous_actions SET state='cancelled',execute_at=NULL WHERE contact_id=? AND state='pending'", contact); err != nil {
			return err
		}
		return audit(ctx, tx, "owner_contact_denied", contact)
	})
}

// PauseSchedules is called inside the settings writer, so switching off and on
// cannot resurrect a previously authorized countdown.
func pauseSchedules(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT id,payload FROM pending_autonomous_actions WHERE state='pending'")
	if err != nil {
		return err
	}
	type item struct{ id, raw string }
	items := []item{}
	for rows.Next() {
		var v item
		if err = rows.Scan(&v.id, &v.raw); err != nil {
			rows.Close()
			return err
		}
		items = append(items, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range items {
		var a Action
		if err = json.Unmarshal([]byte(v.raw), &a); err != nil {
			return err
		}
		if a.Mode != "grace_period_auto" {
			continue
		}
		a.Mode = "awaiting_approval"
		a.ExecuteAt = nil
		a.Reason = "delivery_paused"
		raw, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE pending_autonomous_actions SET payload=?,execute_at=NULL WHERE id=? AND state='pending'", string(raw), v.id); err != nil {
			return err
		}
	}
	return nil
}

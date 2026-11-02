package twin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// AutonomySettings gates preparation and the separate approval delivery bridge.
type AutonomySettings struct {
	Enabled      bool   `json:"enabled"`
	Mode         string `json:"mode"`
	GraceSeconds int    `json:"grace_seconds"`
	Mail         bool   `json:"mail"`
	WhatsApp     bool   `json:"whatsapp"`
	Voice        bool   `json:"voice"`
}

// EffectiveReplyPolicy gives explicit contact choices precedence. In manual
// mode, an unconfigured contact must be chosen by the owner before preparation.
func EffectiveReplyPolicy(mode, policy string) string {
	switch policy {
	case "never", "draft_only", "full_auto":
		return policy
	case "default":
		if mode == "full_auto" || mode == "draft_only" {
			return mode
		}
	}
	return "manual"
}

type Action struct {
	DeliveryVersion int         `json:"delivery_version,omitempty"`
	ApprovalCode    int64       `json:"approval_code,omitempty"`
	SourceAddress   string      `json:"source_address,omitempty"`
	Group           bool        `json:"group,omitempty"`
	SentAt          *time.Time  `json:"sent_at,omitempty"`
	FailureStage    string      `json:"failure_stage,omitempty"`
	Stage           string      `json:"stage,omitempty"`
	UpdatedAt       time.Time   `json:"updated_at"`
	DecisionScore   *float64    `json:"decision_score,omitempty"`
	ScoreStatus     string      `json:"score_status,omitempty"`
	ID              string      `json:"id"`
	ContactID       string      `json:"contact_id"`
	Channel         string      `json:"channel"`
	Account         string      `json:"account"`
	Recipient       string      `json:"recipient"`
	Subject         string      `json:"subject"`
	Body            string      `json:"body"`
	InReplyTo       string      `json:"in_reply_to"`
	Mode            string      `json:"mode"`
	Reason          string      `json:"reason"`
	State           string      `json:"state"`
	CreatedAt       time.Time   `json:"created_at"`
	ExecuteAt       *time.Time  `json:"execute_at,omitempty"`
	Tools           []string    `json:"tools"`
	Conflict        bool        `json:"conflict"`
	Alternatives    []time.Time `json:"alternatives"`
	CalendarIDs     []string    `json:"calendar_ids"`
	DraftSaved      bool        `json:"draft_saved"`
}

// ClaimIncoming is deliberately distinct from contact-history deduplication.
// Claims survive failures/restarts: an uncertain side effect is never replayed.
func (s *Store) ClaimIncoming(ctx context.Context, key string) (bool, error) {
	return s.claimIncoming(ctx, key, nil)
}

// ClaimIncomingAction atomically pairs deduplication with a visible preparation
// record. A shutdown between the claim and the first model call must not leave
// an invisible event that can never be retried or reviewed.
func (s *Store) ClaimIncomingAction(ctx context.Context, a Action) (bool, error) {
	if a.ID == "" || a.Channel == "" || a.State != "preparing" {
		return false, ErrInvalid
	}
	return s.claimIncoming(ctx, a.ID, &a)
}

func (s *Store) claimIncoming(ctx context.Context, key string, a *Action) (bool, error) {
	var claimed bool
	err := s.db.Write(ctx, func(tx *sql.Tx) error {
		r, e := tx.ExecContext(ctx, "INSERT OR IGNORE INTO autonomy_events(event_key,at) VALUES(?,?)", key, time.Now().UTC().Format(time.RFC3339Nano))
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		if e != nil {
			return e
		}
		claimed = n == 1
		if !claimed || a == nil {
			return nil
		}
		a.UpdatedAt = time.Now().UTC()
		raw, e := json.Marshal(a)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO pending_autonomous_actions(id,contact_id,channel,payload,state,execute_at) VALUES(?,?,?,?,?,NULL)", a.ID, a.ContactID, a.Channel, string(raw), a.State)
		return e
	})
	return claimed && err == nil, err
}

// Called at store open, before workers start. Preparation is never resumed as
// an automatic delivery after a process interruption.
func interruptPreparations(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT payload FROM pending_autonomous_actions WHERE state='preparing'")
	if err != nil {
		return err
	}
	var actions []Action
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return err
		}
		var a Action
		if err = json.Unmarshal([]byte(raw), &a); err != nil {
			rows.Close()
			return err
		}
		actions = append(actions, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, a := range actions {
		a.State, a.Reason, a.Mode = "failed", "reply_cancelled", "awaiting_approval"
		a.FailureStage, a.Stage, a.ExecuteAt = a.Stage, "", nil
		a.UpdatedAt = time.Now().UTC()
		raw, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE pending_autonomous_actions SET payload=?,state='failed',execute_at=NULL WHERE id=? AND state='preparing'", string(raw), a.ID); err != nil {
			return err
		}
	}
	return nil
}
// Only CancelAction ever produces state "cancelled", so rows written before it
// recorded a reason are owner cancellations still carrying the stale
// "drafted"/empty reason of the prepared reply.
func repairCancelledReasons(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT id,payload FROM pending_autonomous_actions WHERE state='cancelled'")
	if err != nil {
		return err
	}
	fixed := map[string]string{}
	for rows.Next() {
		var id, raw string
		if err = rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return err
		}
		var a Action
		if err = json.Unmarshal([]byte(raw), &a); err != nil {
			rows.Close()
			return err
		}
		if a.Reason != "" && a.Reason != "drafted" {
			continue
		}
		a.Reason = "user_cancelled"
		out, err := json.Marshal(a)
		if err != nil {
			rows.Close()
			return err
		}
		fixed[id] = string(out)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for id, payload := range fixed {
		if _, err = tx.ExecContext(ctx, "UPDATE pending_autonomous_actions SET payload=? WHERE id=? AND state='cancelled'", payload, id); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) SaveAction(ctx context.Context, a Action) error {
	if a.ID == "" || a.Channel == "" {
		return ErrInvalid
	}
	a.UpdatedAt = time.Now().UTC()
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return s.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO pending_autonomous_actions(id,contact_id,channel,payload,state,execute_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload,state=excluded.state,execute_at=excluded.execute_at WHERE pending_autonomous_actions.state NOT IN ('sending','sent','send_uncertain','cancelled')", a.ID, a.ContactID, a.Channel, string(raw), a.State, a.ExecuteAt)
		if err != nil {
			return err
		}
		return audit(ctx, tx, a.Reason, a.ContactID)
	})
}
func (s *Store) Actions(ctx context.Context) ([]Action, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT rowid,payload,state FROM pending_autonomous_actions ORDER BY rowid DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Action{}
	for rows.Next() {
		var raw, state string
		var code int64
		if err := rows.Scan(&code, &raw, &state); err != nil {
			return nil, err
		}
		var a Action
		if err := json.Unmarshal([]byte(raw), &a); err != nil {
			return nil, err
		}
		a.State = state
		a.ApprovalCode = code
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) CancelAction(ctx context.Context, id string) error {
	return s.db.Write(ctx, func(tx *sql.Tx) error {
		// The payload carries the user-visible reason; flipping only the state
		// column left "drafted" behind and the UI called a cancelled reply
		// "prepared for review".
		var raw string
		if e := tx.QueryRowContext(ctx, "SELECT payload FROM pending_autonomous_actions WHERE id=? AND state='pending'", id).Scan(&raw); e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return ErrNotFound
			}
			return e
		}
		var a Action
		if e := json.Unmarshal([]byte(raw), &a); e != nil {
			return e
		}
		a.State, a.Reason, a.ExecuteAt = "cancelled", "user_cancelled", nil
		a.UpdatedAt = time.Now().UTC()
		updated, e := json.Marshal(a)
		if e != nil {
			return e
		}
		r, e := tx.ExecContext(ctx, "UPDATE pending_autonomous_actions SET payload=?,state='cancelled',execute_at=NULL WHERE id=? AND state='pending'", string(updated), id)
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		if e != nil {
			return e
		}
		if n == 0 {
			return ErrNotFound
		}
		return audit(ctx, tx, "action_cancelled", "")
	})
}

// ReserveCandidate persists attempts, including failed uploads and cancellations.
// A restart or another channel cannot reset the contact's reply budget.
func (s *Store) ReserveCandidate(ctx context.Context, contact, thread, key string, now time.Time, hasThread bool) (bool, error) {
	allowed := false
	err := s.db.Write(ctx, func(tx *sql.Tx) error {
		var hour, day int
		err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(at>?),0),COUNT(*) FROM autonomy_auto_reservations WHERE at>? AND (contact_id=? OR (? AND thread_key=?))`, now.Add(-time.Hour).Unix(), now.Add(-24*time.Hour).Unix(), contact, hasThread, thread).Scan(&hour, &day)
		if err != nil {
			return err
		}
		if hour >= 1 || day >= 3 {
			return nil
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO autonomy_auto_reservations(event_key,contact_id,thread_key,at) VALUES(?,?,?,?)", key, contact, thread, now.Unix())
		allowed = err == nil
		return err
	})
	return allowed, err
}

package twin

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// AutonomySettings prepares decisions only. Delivery belongs to the separate
// approval bridge; preparing a full-auto candidate never sends a message.
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
	Stage         string      `json:"stage,omitempty"`
	UpdatedAt     time.Time   `json:"updated_at"`
	DecisionScore *float64    `json:"decision_score,omitempty"`
	ScoreStatus   string      `json:"score_status,omitempty"`
	ID            string      `json:"id"`
	ContactID     string      `json:"contact_id"`
	Channel       string      `json:"channel"`
	Account       string      `json:"account"`
	Recipient     string      `json:"recipient"`
	Subject       string      `json:"subject"`
	Body          string      `json:"body"`
	InReplyTo     string      `json:"in_reply_to"`
	Mode          string      `json:"mode"`
	Reason        string      `json:"reason"`
	State         string      `json:"state"`
	CreatedAt     time.Time   `json:"created_at"`
	ExecuteAt     *time.Time  `json:"execute_at,omitempty"`
	Tools         []string    `json:"tools"`
	Conflict      bool        `json:"conflict"`
	Alternatives  []time.Time `json:"alternatives"`
	CalendarIDs   []string    `json:"calendar_ids"`
	DraftSaved    bool        `json:"draft_saved"`
}

// ClaimIncoming is deliberately distinct from contact-history deduplication.
// Claims survive failures/restarts: an uncertain side effect is never replayed.
func (s *Store) ClaimIncoming(ctx context.Context, key string) (bool, error) {
	var claimed bool
	err := s.db.Write(ctx, func(tx *sql.Tx) error {
		r, e := tx.ExecContext(ctx, "INSERT OR IGNORE INTO autonomy_events(event_key,at) VALUES(?,?)", key, time.Now().UTC().Format(time.RFC3339Nano))
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		claimed = n == 1
		return e
	})
	return claimed, err
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
		_, err := tx.ExecContext(ctx, "INSERT INTO pending_autonomous_actions(id,contact_id,channel,payload,state,execute_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload,state=excluded.state,execute_at=excluded.execute_at", a.ID, a.ContactID, a.Channel, string(raw), a.State, a.ExecuteAt)
		if err != nil {
			return err
		}
		return audit(ctx, tx, a.Reason, a.ContactID)
	})
}
func (s *Store) Actions(ctx context.Context) ([]Action, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT payload,state FROM pending_autonomous_actions ORDER BY rowid DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Action{}
	for rows.Next() {
		var raw, state string
		if err := rows.Scan(&raw, &state); err != nil {
			return nil, err
		}
		var a Action
		if err := json.Unmarshal([]byte(raw), &a); err != nil {
			return nil, err
		}
		a.State = state
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) CancelAction(ctx context.Context, id string) error {
	return s.db.Write(ctx, func(tx *sql.Tx) error {
		r, e := tx.ExecContext(ctx, "UPDATE pending_autonomous_actions SET state='cancelled' WHERE id=? AND state='pending'", id)
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

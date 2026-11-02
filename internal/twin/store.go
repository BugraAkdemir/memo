// Package twin stores explicitly linked identities and source-dated conversations.
package twin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"
	_ "time/tzdata"

	"github.com/google/uuid"
	"memo/internal/database"
)

var ErrInvalid = errors.New("invalid twin input")
var ErrNotFound = errors.New("twin contact not found")
var ErrAliasConflict = errors.New("address already belongs to another contact")

const schema = `
CREATE TABLE IF NOT EXISTS autonomy_notifications(action_id TEXT NOT NULL, phase TEXT NOT NULL, target TEXT NOT NULL, at TEXT NOT NULL, PRIMARY KEY(action_id,phase,target));
CREATE TABLE IF NOT EXISTS twin_reply_model(id INTEGER PRIMARY KEY CHECK(id=1), provider TEXT NOT NULL, model TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS autonomy_auto_reservations(event_key TEXT PRIMARY KEY, contact_id TEXT NOT NULL, thread_key TEXT NOT NULL, at INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS autonomy_reservations_contact ON autonomy_auto_reservations(contact_id,at);
CREATE TABLE IF NOT EXISTS autonomy_events(event_key TEXT PRIMARY KEY, at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS contacts(id TEXT PRIMARY KEY, name TEXT NOT NULL, autonomy_policy TEXT NOT NULL DEFAULT 'draft_only');
CREATE TABLE IF NOT EXISTS contact_aliases(channel TEXT NOT NULL, address TEXT NOT NULL, contact_id TEXT NOT NULL, PRIMARY KEY(channel,address));
CREATE INDEX IF NOT EXISTS aliases_contact ON contact_aliases(contact_id);
CREATE TABLE IF NOT EXISTS twin_preferences(id INTEGER PRIMARY KEY CHECK(id=1), value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS commitments(id TEXT PRIMARY KEY, contact_id TEXT NOT NULL, kind TEXT NOT NULL, body TEXT NOT NULL, due_at TEXT, status TEXT NOT NULL DEFAULT 'open');
CREATE TABLE IF NOT EXISTS processed_external_events(channel TEXT NOT NULL, account TEXT NOT NULL, event_id TEXT NOT NULL, processed_at TEXT NOT NULL, PRIMARY KEY(channel,account,event_id));
CREATE TABLE IF NOT EXISTS pending_autonomous_actions(id TEXT PRIMARY KEY, contact_id TEXT NOT NULL, channel TEXT NOT NULL, payload TEXT NOT NULL, state TEXT NOT NULL DEFAULT 'pending', execute_at TEXT);
CREATE TABLE IF NOT EXISTS twin_audit_log(id INTEGER PRIMARY KEY AUTOINCREMENT, at TEXT NOT NULL, action TEXT NOT NULL, contact_id TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS contact_messages(channel TEXT NOT NULL, account TEXT NOT NULL, event_id TEXT NOT NULL, address TEXT NOT NULL, outgoing INTEGER NOT NULL, body TEXT NOT NULL, sent_at TEXT NOT NULL, PRIMARY KEY(channel,account,event_id,address));
CREATE INDEX IF NOT EXISTS messages_alias ON contact_messages(channel,address,sent_at);
CREATE TABLE IF NOT EXISTS briefings(key TEXT PRIMARY KEY, kind TEXT NOT NULL, created_at TEXT NOT NULL, payload TEXT NOT NULL, read INTEGER NOT NULL DEFAULT 0);
`

type Store struct{ db *database.DB }
type Alias struct {
	Channel string `json:"channel"`
	Address string `json:"address"`
}
type Contact struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	AutonomyPolicy string       `json:"autonomy_policy"`
	Aliases        []Alias      `json:"aliases"`
	Style          StyleProfile `json:"style"`
}
type Preferences struct {
	Autonomy               AutonomySettings `json:"autonomy"`
	Timezone               string           `json:"timezone"`
	WorkStart              string           `json:"work_start"`
	WorkEnd                string           `json:"work_end"`
	AllowProjectInspection bool             `json:"allow_project_inspection"`
	Radar                  RadarSettings    `json:"radar"`
	Briefing               BriefingSettings `json:"briefing"`
}
type Message struct {
	Channel  string    `json:"channel"`
	Account  string    `json:"account"`
	EventID  string    `json:"event_id"`
	Address  string    `json:"address"`
	Outgoing bool      `json:"outgoing"`
	Body     string    `json:"body"`
	SentAt   time.Time `json:"sent_at"`
	// Optional context used only by the follow-up radar.
	Subject   string `json:"subject,omitempty"`
	InReplyTo string `json:"in_reply_to,omitempty"`
	Group     bool   `json:"group,omitempty"`
	// NoTrack keeps a sent message from opening a waiting record (a reminder).
	NoTrack bool `json:"no_track,omitempty"`
}

func NewStore(dir string) (*Store, error) {
	db, err := database.Open(database.Config{Path: filepath.Join(dir, "twin.db"), MaxPool: 1})
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	err = db.Write(context.Background(), func(tx *sql.Tx) error {
		if _, e := tx.Exec(schema); e != nil {
			return e
		}
		if err := interruptPreparations(context.Background(), tx); err != nil {
			return err
		}
		if err := repairCancelledReasons(context.Background(), tx); err != nil {
			return err
		}
		if err := migrateCommitments(tx); err != nil {
			return err
		}
		_, e := tx.Exec("UPDATE pending_autonomous_actions SET state='send_uncertain' WHERE state='sending'")
		return e
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func audit(ctx context.Context, tx *sql.Tx, action, id string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO twin_audit_log(at,action,contact_id) VALUES(?,?,?)", time.Now().UTC().Format(time.RFC3339Nano), action, id)
	return err
}
func (s *Store) Preferences(ctx context.Context) (Preferences, error) {
	p := Preferences{Timezone: "UTC", WorkStart: "09:00", WorkEnd: "18:00"}
	var raw string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM twin_preferences WHERE id=1").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	err = json.Unmarshal([]byte(raw), &p)
	return p, err
}
func (s *Store) SavePreferences(ctx context.Context, p Preferences) error {
	if _, err := time.LoadLocation(p.Timezone); err != nil {
		return fmt.Errorf("%w: timezone", ErrInvalid)
	}
	start, e1 := time.Parse("15:04", p.WorkStart)
	end, e2 := time.Parse("15:04", p.WorkEnd)
	if e1 != nil || e2 != nil || !end.After(start) {
		return fmt.Errorf("%w: working hours", ErrInvalid)
	}
	if p.Autonomy.Mode == "" {
		p.Autonomy.Mode = "draft_only"
	}
	if p.Autonomy.GraceSeconds == 0 {
		p.Autonomy.GraceSeconds = 60
	}
	if (p.Autonomy.Mode != "draft_only" && p.Autonomy.Mode != "full_auto" && p.Autonomy.Mode != "manual") || p.Autonomy.GraceSeconds < 10 || p.Autonomy.GraceSeconds > 3600 {
		return ErrInvalid
	}
	if p.Radar.FollowUpHours == 0 {
		p.Radar.FollowUpHours = 48
	}
	if p.Radar.FollowUpHours < 1 || p.Radar.FollowUpHours > 720 {
		return fmt.Errorf("%w: follow-up hours", ErrInvalid)
	}
	if err := validBriefingSettings(&p.Briefing); err != nil {
		return err
	}
	b, _ := json.Marshal(p)
	return s.db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO twin_preferences(id,value) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET value=excluded.value", string(b)); err != nil {
			return err
		}
		if err := pauseSchedules(ctx, tx); err != nil {
			return err
		}
		return audit(ctx, tx, "preferences_updated", "")
	})
}

// RecordMessage atomically deduplicates a source event and stores its original
// time. Names never link identities; a fresh address gets its own contact.
func (s *Store) RecordMessage(ctx context.Context, m Message, name string) error {
	a, err := NormalizeAlias(Alias{m.Channel, m.Address})
	if err != nil {
		return err
	}
	if m.EventID == "" || len(m.EventID) > 512 || m.Account == "" || m.SentAt.IsZero() || len(m.Body) > 128*1024 {
		return ErrInvalid
	}
	return s.db.Write(ctx, func(tx *sql.Tx) error {
		// Address participates in the event key: an outgoing CC event can belong
		// to several recipients without silently losing all but the first.
		res, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO contact_messages(channel,account,event_id,address,outgoing,body,sent_at) VALUES(?,?,?,?,?,?,?)", a.Channel, m.Account, m.EventID, a.Address, m.Outgoing, m.Body, m.SentAt.Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return nil
		}
		var id string
		err = tx.QueryRowContext(ctx, "SELECT contact_id FROM contact_aliases WHERE channel=? AND address=?", a.Channel, a.Address).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			id = uuid.NewString()
			if name == "" {
				name = a.Address
			}
			if len(name) > 300 {
				name = a.Address
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO contacts(id,name,autonomy_policy) VALUES(?,?,'default')", id, name); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO contact_aliases(channel,address,contact_id) VALUES(?,?,?)", a.Channel, a.Address, id); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if err = trackCommitments(ctx, tx, a, m, id); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO processed_external_events(channel,account,event_id,processed_at) VALUES(?,?,?,?)", a.Channel, m.Account, m.EventID, time.Now().UTC().Format(time.RFC3339Nano))
		return err
	})
}

func (s *Store) Checkpoint(ctx context.Context) error { return s.db.CheckpointTruncate(ctx) }

// Active avoids converting every fresh-install plain chat into a tool turn.
// Contacts or explicitly saved preferences make the read-only twin available;
// this flag never starts automatic replies.
func (s *Store) Active(ctx context.Context) bool {
	var active bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM contacts) OR EXISTS(SELECT 1 FROM twin_preferences)").Scan(&active)
	return err == nil && active
}

package twin

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
)

// RadarSettings controls the follow-up radar. It only ever prepares a reminder
// draft for owner review; nothing is sent by the radar itself.
type RadarSettings struct {
	Enabled       bool `json:"enabled"`
	FollowUpHours int  `json:"follow_up_hours"`
	// TrackPromises additionally reads the owner's own sent messages for
	// promises with a clear date. Off by default; the reply model sees the text.
	TrackPromises bool `json:"track_promises"`
}

// Commitment is an awaited reply ("waiting_on_other") or something the owner
// promised someone ("my_promise").
type Commitment struct {
	ID         string     `json:"id"`
	ContactID  string     `json:"contact_id"`
	Kind       string     `json:"kind"`
	Channel    string     `json:"channel"`
	Account    string     `json:"account"`
	Address    string     `json:"address"`
	Subject    string     `json:"subject"`
	InReplyTo  string     `json:"in_reply_to"`
	SourceRef  string     `json:"source_ref"`
	Body       string     `json:"body"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	DueAt      time.Time  `json:"due_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// Fixed-width UTC so lexicographic comparison in SQL equals time order
// (RFC3339Nano trims zeros, which breaks string ordering).
const commitmentTime = "2006-01-02T15:04:05.000000000Z"

func ts(t time.Time) string { return t.UTC().Format(commitmentTime) }

func parseTS(s string) time.Time {
	t, err := time.Parse(commitmentTime, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// The first schema only had id/contact_id/kind/body/due_at/status and nothing
// wrote to it; add the columns the radar needs without touching existing rows.
func migrateCommitments(tx *sql.Tx) error {
	rows, err := tx.Query("PRAGMA table_info(commitments)")
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err = rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		have[name] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, c := range []struct{ name, def string }{
		{"channel", "TEXT NOT NULL DEFAULT ''"},
		{"account", "TEXT NOT NULL DEFAULT ''"},
		{"address", "TEXT NOT NULL DEFAULT ''"},
		{"subject", "TEXT NOT NULL DEFAULT ''"},
		{"in_reply_to", "TEXT NOT NULL DEFAULT ''"},
		{"source_ref", "TEXT NOT NULL DEFAULT ''"},
		{"created_at", "TEXT NOT NULL DEFAULT ''"},
		{"resolved_at", "TEXT"},
	} {
		if have[c.name] {
			continue
		}
		if _, err = tx.Exec("ALTER TABLE commitments ADD COLUMN " + c.name + " " + c.def); err != nil {
			return err
		}
	}
	_, err = tx.Exec("CREATE INDEX IF NOT EXISTS commitments_due ON commitments(kind,status,due_at)")
	return err
}

// expectsReply is a deliberately simple, language-independent test: a real
// question mark of any common script. URL query strings ("?a=1") and quoted
// earlier mail ("> ...") do not count.
func expectsReply(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), ">") {
			continue
		}
		r := []rune(line)
		for i, c := range r {
			switch c {
			case '？', '؟', ';':
				return true
			case ';':
				// Only the Greek question mark, never an ordinary semicolon.
				if i > 0 && unicode.Is(unicode.Greek, r[i-1]) {
					return true
				}
			case '?':
				if i+1 == len(r) || unicode.IsSpace(r[i+1]) || strings.ContainsRune(`)"'»”’]`, r[i+1]) {
					return true
				}
			}
		}
	}
	return false
}

func commitmentID(channel, account, event, address string) string {
	sum := sha256.Sum256([]byte("commitment\x00" + channel + "\x00" + account + "\x00" + event + "\x00" + address))
	return hex.EncodeToString(sum[:16])
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// trackCommitments runs inside RecordMessage's transaction, so a message and
// the commitment it opens or closes can never disagree.
func trackCommitments(ctx context.Context, tx *sql.Tx, a Alias, m Message, contactID string) error {
	if !m.Outgoing {
		// A reply on ANY channel of the same contact answers the question, but
		// only one that arrived after it (a backfilled older message must not).
		_, err := tx.ExecContext(ctx, "UPDATE commitments SET status='resolved',resolved_at=? WHERE contact_id=? AND kind='waiting_on_other' AND status IN ('open','reminded') AND created_at<?", ts(time.Now()), contactID, ts(m.SentAt))
		return err
	}
	// Delivery of a reminder exists for mail and WhatsApp only, and a group or
	// CC thread never gets an automatic nudge.
	if m.Group || (a.Channel != "mail" && a.Channel != "whatsapp") || !expectsReply(m.Body) {
		return nil
	}
	var raw string
	err := tx.QueryRowContext(ctx, "SELECT value FROM twin_preferences WHERE id=1").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var p Preferences
	if err = json.Unmarshal([]byte(raw), &p); err != nil {
		return err
	}
	if !p.Radar.Enabled {
		return nil
	}
	hours := p.Radar.FollowUpHours
	if hours <= 0 {
		hours = 48
	}
	due := m.SentAt.Add(time.Duration(hours) * time.Hour)
	// Never open an already overdue commitment: importing old history must not
	// produce a burst of reminders for conversations that ended long ago.
	if !due.After(time.Now()) {
		return nil
	}
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO commitments(id,contact_id,kind,body,due_at,status,channel,account,address,subject,in_reply_to,source_ref,created_at)
VALUES(?,?,'waiting_on_other',?,?,'open',?,?,?,?,?,?,?)`,
		commitmentID(a.Channel, m.Account, m.EventID, a.Address), contactID, truncateRunes(m.Body, 2000), ts(due),
		a.Channel, m.Account, a.Address, truncateRunes(m.Subject, 500), truncateRunes(m.InReplyTo, 500), truncateRunes(m.EventID, 500), ts(m.SentAt))
	return err
}

const commitmentColumns = "id,contact_id,kind,channel,account,address,subject,in_reply_to,source_ref,body,status,created_at,COALESCE(due_at,''),COALESCE(resolved_at,'')"

func scanCommitments(rows *sql.Rows) ([]Commitment, error) {
	defer rows.Close()
	out := []Commitment{}
	for rows.Next() {
		var c Commitment
		var created, due, resolved string
		if err := rows.Scan(&c.ID, &c.ContactID, &c.Kind, &c.Channel, &c.Account, &c.Address, &c.Subject, &c.InReplyTo, &c.SourceRef, &c.Body, &c.Status, &created, &due, &resolved); err != nil {
			return nil, err
		}
		c.CreatedAt, c.DueAt = parseTS(created), parseTS(due)
		if resolved != "" {
			t := parseTS(resolved)
			c.ResolvedAt = &t
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Commitments lists what is still open for the owner, awaited replies and own
// promises alike (open or already reminded), soonest due first.
func (s *Store) Commitments(ctx context.Context) ([]Commitment, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+commitmentColumns+" FROM commitments WHERE kind IN ('waiting_on_other','my_promise') AND status IN ('open','reminded') ORDER BY due_at LIMIT 200")
	if err != nil {
		return nil, err
	}
	return scanCommitments(rows)
}

// CommitmentHistory lists the most recent commitments with one status.
func (s *Store) CommitmentHistory(ctx context.Context, status string) ([]Commitment, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+commitmentColumns+" FROM commitments WHERE kind='waiting_on_other' AND status=? ORDER BY created_at DESC LIMIT 100", status)
	if err != nil {
		return nil, err
	}
	return scanCommitments(rows)
}

// DueCommitments returns open commitments whose waiting time has run out.
func (s *Store) DueCommitments(ctx context.Context, now time.Time, limit int) ([]Commitment, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+commitmentColumns+" FROM commitments WHERE kind='waiting_on_other' AND status='open' AND due_at<=? ORDER BY due_at LIMIT ?", ts(now), limit)
	if err != nil {
		return nil, err
	}
	return scanCommitments(rows)
}

// SetCommitmentStatus moves a commitment between states atomically; only one
// caller can win a given from→to transition.
func (s *Store) SetCommitmentStatus(ctx context.Context, id, from, to string) (bool, error) {
	switch to {
	case "open", "reminded", "resolved", "cancelled":
	default:
		return false, ErrInvalid
	}
	won := false
	err := s.db.Write(ctx, func(tx *sql.Tx) error {
		resolved := any(nil)
		if to == "resolved" || to == "cancelled" {
			resolved = ts(time.Now())
		}
		r, err := tx.ExecContext(ctx, "UPDATE commitments SET status=?,resolved_at=COALESCE(?,resolved_at) WHERE id=? AND status=?", to, resolved, id, from)
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		won = err == nil && n == 1
		return err
	})
	return won, err
}

// Promise is something the owner said they would do for someone, with a date.
type Promise struct {
	Channel   string
	Account   string
	Address   string
	Summary   string
	SourceRef string
	DueAt     time.Time
}

// AddPromise stores a promise once. A promise needs a real, future, bounded
// date: guessing one would make the reminder noise. It reports false when the
// same promise from the same message already exists.
func (s *Store) AddPromise(ctx context.Context, p Promise) (bool, error) {
	summary := strings.TrimSpace(p.Summary)
	if summary == "" || len([]rune(summary)) > 300 || p.DueAt.IsZero() || !p.DueAt.After(time.Now()) || p.DueAt.After(time.Now().AddDate(1, 0, 1)) || p.Account == "" || p.SourceRef == "" {
		return false, ErrInvalid
	}
	if p.Channel != "mail" && p.Channel != "whatsapp" {
		return false, ErrInvalid
	}
	a, err := NormalizeAlias(Alias{Channel: p.Channel, Address: p.Address})
	if err != nil {
		return false, err
	}
	added := false
	err = s.db.Write(ctx, func(tx *sql.Tx) error {
		var contact string
		if err := tx.QueryRowContext(ctx, "SELECT contact_id FROM contact_aliases WHERE channel=? AND address=?", a.Channel, a.Address).Scan(&contact); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		r, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO commitments(id,contact_id,kind,body,due_at,status,channel,account,address,subject,in_reply_to,source_ref,created_at)
VALUES(?,?,'my_promise',?,?,'open',?,?,?,'','',?,?)`,
			commitmentID("promise:"+a.Channel, p.Account, p.SourceRef+"\x00"+summary, a.Address), contact, summary, ts(p.DueAt), a.Channel, p.Account, a.Address, truncateRunes(p.SourceRef, 500), ts(time.Now()))
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		added = err == nil && n == 1
		return err
	})
	return added, err
}

// DuePromises returns open promises whose date has arrived.
func (s *Store) DuePromises(ctx context.Context, now time.Time, limit int) ([]Commitment, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+commitmentColumns+" FROM commitments WHERE kind='my_promise' AND status='open' AND due_at<=? ORDER BY due_at LIMIT ?", ts(now), limit)
	if err != nil {
		return nil, err
	}
	return scanCommitments(rows)
}

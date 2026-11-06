package twin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
)

var waAddress = regexp.MustCompile(`^[0-9]+@(s\.whatsapp\.net|lid)$`)

func NormalizeAlias(a Alias) (Alias, error) {
	a.Channel = strings.ToLower(strings.TrimSpace(a.Channel))
	a.Address = strings.TrimSpace(a.Address)
	if len(a.Address) > 320 || strings.ContainsAny(a.Address, "\r\n\x00") {
		return a, ErrInvalid
	}
	switch a.Channel {
	case "mail":
		v, e := mail.ParseAddress(a.Address)
		if e != nil {
			return a, ErrInvalid
		}
		a.Address = strings.ToLower(v.Address)
	case "whatsapp":
		a.Address = strings.TrimPrefix(a.Address, "+")
		if !strings.Contains(a.Address, "@") {
			a.Address += "@s.whatsapp.net"
		}
		if !waAddress.MatchString(a.Address) {
			return a, ErrInvalid
		}
	case "telegram":
		n, e := strconv.ParseInt(a.Address, 10, 64)
		if e != nil || n <= 0 {
			return a, ErrInvalid
		}
		a.Address = strconv.FormatInt(n, 10)
	default:
		return a, ErrInvalid
	}
	return a, nil
}
func (s *Store) Resolve(ctx context.Context, a Alias) (string, error) {
	a, e := NormalizeAlias(a)
	if e != nil {
		return "", e
	}
	var id string
	e = s.db.QueryRowContext(ctx, "SELECT contact_id FROM contact_aliases WHERE channel=? AND address=?", a.Channel, a.Address).Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, e
}
func (s *Store) SaveContact(ctx context.Context, c Contact) (string, error) {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" || len(c.Name) > 300 || len(c.Aliases) == 0 || len(c.Aliases) > 30 {
		return "", ErrInvalid
	}
	if c.AutonomyPolicy == "" {
		c.AutonomyPolicy = "default"
	}
	if c.AutonomyPolicy != "draft_only" && c.AutonomyPolicy != "never" && c.AutonomyPolicy != "default" && c.AutonomyPolicy != "full_auto" {
		return "", ErrInvalid
	}
	aliases := []Alias{}
	seen := map[Alias]bool{}
	for _, a := range c.Aliases {
		a, e := NormalizeAlias(a)
		if e != nil {
			return "", e
		}
		if seen[a] {
			return "", ErrInvalid
		}
		seen[a] = true
		aliases = append(aliases, a)
	}
	fresh := c.ID == ""
	if fresh {
		c.ID = newID()
	}
	err := s.db.Write(ctx, func(tx *sql.Tx) error {
		if !fresh {
			var id string
			if e := tx.QueryRowContext(ctx, "SELECT id FROM contacts WHERE id=?", c.ID).Scan(&id); errors.Is(e, sql.ErrNoRows) {
				return ErrNotFound
			} else if e != nil {
				return e
			}
		}
		for _, a := range aliases {
			var owner string
			e := tx.QueryRowContext(ctx, "SELECT contact_id FROM contact_aliases WHERE channel=? AND address=?", a.Channel, a.Address).Scan(&owner)
			if e == nil && owner != c.ID {
				return ErrAliasConflict
			}
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return e
			}
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO contacts(id,name,autonomy_policy) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,autonomy_policy=excluded.autonomy_policy", c.ID, c.Name, c.AutonomyPolicy); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, "DELETE FROM contact_aliases WHERE contact_id=?", c.ID); e != nil {
			return e
		}
		for _, a := range aliases {
			if _, e := tx.ExecContext(ctx, "INSERT INTO contact_aliases(channel,address,contact_id) VALUES(?,?,?)", a.Channel, a.Address, c.ID); e != nil {
				return e
			}
		}
		return audit(ctx, tx, "contact_saved", c.ID)
	})
	return c.ID, err
}

// Merge is an explicit owner operation. It cannot be invoked by the external
// executor. Source history follows its aliases; conflicting policies stay safe.
func (s *Store) Merge(ctx context.Context, target, source string) error {
	if target == source || target == "" || source == "" {
		return ErrInvalid
	}
	return s.db.Write(ctx, func(tx *sql.Tx) error {
		policies := []string{}
		for _, id := range []string{target, source} {
			var p string
			e := tx.QueryRowContext(ctx, "SELECT autonomy_policy FROM contacts WHERE id=?", id).Scan(&p)
			if errors.Is(e, sql.ErrNoRows) {
				return ErrNotFound
			}
			if e != nil {
				return e
			}
			policies = append(policies, p)
		}
		if _, e := tx.ExecContext(ctx, "UPDATE contact_aliases SET contact_id=? WHERE contact_id=?", target, source); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, "UPDATE commitments SET contact_id=? WHERE contact_id=?", target, source); e != nil {
			return e
		}
		// Future pending actions cannot silently inherit a new recipient identity.
		if _, e := tx.ExecContext(ctx, "UPDATE pending_autonomous_actions SET state='cancelled' WHERE contact_id IN (?,?) AND state='pending'", target, source); e != nil {
			return e
		}
		if policies[0] == "default" || policies[1] == "default" {
			if _, e := tx.ExecContext(ctx, "UPDATE contacts SET autonomy_policy='default' WHERE id=?", target); e != nil {
				return e
			}
		}
		if policies[0] == "draft_only" || policies[1] == "draft_only" {
			if _, e := tx.ExecContext(ctx, "UPDATE contacts SET autonomy_policy='draft_only' WHERE id=?", target); e != nil {
				return e
			}
		}
		if policies[0] == "never" || policies[1] == "never" {
			if _, e := tx.ExecContext(ctx, "UPDATE contacts SET autonomy_policy='never' WHERE id=?", target); e != nil {
				return e
			}
		}
		if _, e := tx.ExecContext(ctx, "DELETE FROM contacts WHERE id=?", source); e != nil {
			return e
		}
		return audit(ctx, tx, "contact_merged", target)
	})
}
func (s *Store) Contacts(ctx context.Context) ([]Contact, error) {
	out := []Contact{}
	rows, e := s.db.QueryContext(ctx, "SELECT id,name,autonomy_policy FROM contacts ORDER BY name,id")
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var c Contact
		if e = rows.Scan(&c.ID, &c.Name, &c.AutonomyPolicy); e != nil {
			rows.Close()
			return nil, e
		}
		out = append(out, c)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	// Release rows before nested queries: the DB deliberately has one connection.
	for i := range out {
		out[i].Aliases = []Alias{}
		rows, e = s.db.QueryContext(ctx, "SELECT channel,address FROM contact_aliases WHERE contact_id=? ORDER BY channel,address", out[i].ID)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var a Alias
			if e = rows.Scan(&a.Channel, &a.Address); e != nil {
				rows.Close()
				return nil, e
			}
			out[i].Aliases = append(out[i].Aliases, a)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
		msgs, e := s.timeline(ctx, out[i].ID, 100, true)
		if e != nil {
			return nil, e
		}
		out[i].Style = InferStyle(msgs)
	}
	return out, nil
}
func (s *Store) Timeline(ctx context.Context, id string, limit int) ([]Message, error) {
	return s.timeline(ctx, id, limit, false)
}
func (s *Store) timeline(ctx context.Context, id string, limit int, outgoingOnly bool) ([]Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	var found string
	if e := s.db.QueryRowContext(ctx, "SELECT id FROM contacts WHERE id=?", id).Scan(&found); errors.Is(e, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if e != nil {
		return nil, e
	}
	rows, e := s.db.QueryContext(ctx, `SELECT m.channel,m.account,m.event_id,MIN(m.address),m.outgoing,m.body,m.sent_at FROM contact_messages m JOIN contact_aliases a ON a.channel=m.channel AND a.address=m.address WHERE a.contact_id=? AND (?=0 OR m.outgoing=1) GROUP BY m.channel,m.account,m.event_id,m.outgoing,m.body,m.sent_at ORDER BY julianday(m.sent_at) DESC,m.event_id DESC LIMIT ?`, id, outgoingOnly, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var at string
		if e = rows.Scan(&m.Channel, &m.Account, &m.EventID, &m.Address, &m.Outgoing, &m.Body, &at); e != nil {
			return nil, e
		}
		m.SentAt, e = parseTime(at)
		if e != nil {
			return nil, fmt.Errorf("twin: invalid stored timestamp: %w", e)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

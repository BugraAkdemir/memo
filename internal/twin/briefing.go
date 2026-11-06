package twin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// BriefingSettings controls the morning briefing and the pre-meeting note.
// Both are off by default and are built without any model call.
type BriefingSettings struct {
	Morning           bool   `json:"morning"`
	MorningTime       string `json:"morning_time"`
	PreMeeting        bool   `json:"pre_meeting"`
	PreMeetingMinutes int    `json:"pre_meeting_minutes"`
	// Sections lists the built-in sections of the morning briefing. A missing
	// list (nil) means all of them, as before this field existed; an explicit
	// empty list means none (custom items only).
	Sections []string `json:"sections"`
	// Custom are the owner's own items: a title and an instruction.
	Custom []BriefingCustom `json:"custom"`
	// Channels are the places, besides the app itself, the morning briefing is
	// sent to: always the owner's own account on each.
	Channels []string `json:"channels"`
	// MailAccount is the connected mail account the briefing is mailed from, and
	// to (the account's own address). Empty means the first connected account.
	MailAccount string `json:"mail_account"`
	// Model optionally writes the briefing. Empty means no model is called.
	Model BriefingModel `json:"model"`
}

// BriefingCustom is one owner-defined briefing item.
type BriefingCustom struct {
	Title  string `json:"title"`
	Prompt string `json:"prompt"`
}

// BriefingModel is the provider/model pair that writes the briefing.
type BriefingModel struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// Built-in morning sections and delivery channels the owner can pick from.
var (
	BriefingSectionIDs = []string{"meetings", "overnight", "drafts", "waiting", "promises", "code"}
	BriefingChannelIDs = []string{"telegram", "whatsapp", "mail"}
)

const (
	briefingMaxCustom   = 5
	briefingTitleRunes  = 60
	briefingPromptRunes = 500
)

// SectionEnabled reports whether a built-in section belongs in the briefing.
func (b BriefingSettings) SectionEnabled(id string) bool {
	if b.Sections == nil {
		return true
	}
	return oneOf(b.Sections, id)
}

func oneOf(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// pickKnown validates ids against the allowed list and drops duplicates,
// keeping the owner's order. A nil list stays nil.
func pickKnown(in, allowed []string, what string) ([]string, error) {
	if in == nil {
		return nil, nil
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if !oneOf(allowed, v) {
			return nil, fmt.Errorf("%w: %s", ErrInvalid, what)
		}
		if !oneOf(out, v) {
			out = append(out, v)
		}
	}
	return out, nil
}

// BriefingSection is one headed list of short lines.
//
// Heading is a code the UI translates; Items are language-neutral data (times,
// names, addresses) and Counts maps a state code to a number.
type BriefingSection struct {
	Heading string         `json:"heading"`
	Items   []string       `json:"items"`
	Counts  map[string]int `json:"counts,omitempty"`
}

// Briefing is a generated card for the owner: kind "morning" or "meeting".
type Briefing struct {
	Key       string            `json:"key"`
	Kind      string            `json:"kind"`
	Title     string            `json:"title"`
	CreatedAt time.Time         `json:"created_at"`
	Read      bool              `json:"read"`
	Sections  []BriefingSection `json:"sections"`
}

const (
	briefingKeep     = 30
	briefingMaxItems = 20
	briefingMaxLines = 12
)

func validBriefingSettings(b *BriefingSettings) error {
	if b.MorningTime == "" {
		b.MorningTime = "09:00"
	}
	if b.PreMeetingMinutes == 0 {
		b.PreMeetingMinutes = 15
	}
	// time.Parse("15:04") accepts only exactly hh:mm, so "9am" and "09:00:30" fail.
	if _, err := time.Parse("15:04", b.MorningTime); err != nil {
		return fmt.Errorf("%w: morning time", ErrInvalid)
	}
	if b.PreMeetingMinutes < 5 || b.PreMeetingMinutes > 120 {
		return fmt.Errorf("%w: pre-meeting minutes", ErrInvalid)
	}
	var err error
	if b.Sections, err = pickKnown(b.Sections, BriefingSectionIDs, "briefing section"); err != nil {
		return err
	}
	if b.Channels, err = pickKnown(b.Channels, BriefingChannelIDs, "briefing channel"); err != nil {
		return err
	}
	if len(b.Custom) > briefingMaxCustom {
		return fmt.Errorf("%w: too many custom items", ErrInvalid)
	}
	custom := make([]BriefingCustom, 0, len(b.Custom))
	for _, c := range b.Custom {
		c.Title = strings.TrimSpace(c.Title)
		c.Prompt = strings.TrimSpace(c.Prompt)
		if c.Title == "" && c.Prompt == "" {
			continue // an empty row the editor left behind
		}
		if c.Title == "" || c.Prompt == "" ||
			len([]rune(c.Title)) > briefingTitleRunes || len([]rune(c.Prompt)) > briefingPromptRunes {
			return fmt.Errorf("%w: custom item", ErrInvalid)
		}
		custom = append(custom, c)
	}
	b.Custom = custom
	b.MailAccount = strings.TrimSpace(b.MailAccount)
	if len(b.MailAccount) > 200 {
		return fmt.Errorf("%w: mail account", ErrInvalid)
	}
	b.Model.Provider = strings.TrimSpace(b.Model.Provider)
	b.Model.Model = strings.TrimSpace(b.Model.Model)
	if (b.Model.Provider == "") != (b.Model.Model == "") ||
		len(b.Model.Provider) > 200 || len(b.Model.Model) > 200 {
		return fmt.Errorf("%w: briefing model", ErrInvalid)
	}
	return nil
}

func clampBriefing(b Briefing) Briefing {
	b.Title = truncateRunes(strings.TrimSpace(b.Title), 200)
	secs := make([]BriefingSection, 0, briefingMaxLines)
	for _, sec := range b.Sections {
		if len(secs) >= briefingMaxLines {
			break
		}
		items := make([]string, 0, len(sec.Items))
		for _, it := range sec.Items {
			if len(items) >= briefingMaxItems {
				break
			}
			items = append(items, truncateRunes(strings.TrimSpace(it), 400))
		}
		secs = append(secs, BriefingSection{Heading: truncateRunes(strings.TrimSpace(sec.Heading), 120), Items: items, Counts: sec.Counts})
	}
	b.Sections = secs
	return b
}

// SaveBriefing stores a briefing once per key and reports whether this call
// created it: a restart or a second scan can never brief the same morning or
// meeting twice. Only the most recent briefingKeep are retained.
func (s *Store) SaveBriefing(ctx context.Context, b Briefing) (bool, error) {
	if strings.TrimSpace(b.Key) == "" || len(b.Key) > 200 || (b.Kind != "morning" && b.Kind != "meeting") {
		return false, ErrInvalid
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	b = clampBriefing(b)
	raw, err := json.Marshal(b)
	if err != nil {
		return false, err
	}
	created := false
	err = s.db.Write(ctx, func(tx *sql.Tx) error {
		r, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO briefings(key,kind,created_at,payload,read) VALUES(?,?,?,?,0)", b.Key, b.Kind, ts(b.CreatedAt), string(raw))
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		if err != nil {
			return err
		}
		created = n == 1
		if !created {
			return nil
		}
		_, err = tx.ExecContext(ctx, "DELETE FROM briefings WHERE key NOT IN (SELECT key FROM briefings ORDER BY created_at DESC, key DESC LIMIT ?)", briefingKeep)
		return err
	})
	return created, err
}

// HasBriefing tells whether a key was already briefed.
func (s *Store) HasBriefing(ctx context.Context, key string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM briefings WHERE key=?", key).Scan(&n)
	return n > 0, err
}

// Briefings lists the newest briefings first.
func (s *Store) Briefings(ctx context.Context, limit int) ([]Briefing, error) {
	if limit <= 0 || limit > briefingKeep {
		limit = briefingKeep
	}
	rows, err := s.db.QueryContext(ctx, "SELECT payload,read FROM briefings ORDER BY created_at DESC, key DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Briefing{}
	for rows.Next() {
		var raw string
		var read bool
		if err := rows.Scan(&raw, &read); err != nil {
			return nil, err
		}
		var b Briefing
		if err := json.Unmarshal([]byte(raw), &b); err != nil {
			return nil, err
		}
		b.Read = read
		if b.Sections == nil {
			b.Sections = []BriefingSection{}
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// MarkBriefingRead records that the owner has seen a briefing.
func (s *Store) MarkBriefingRead(ctx context.Context, key string) error {
	return s.db.Write(ctx, func(tx *sql.Tx) error {
		r, err := tx.ExecContext(ctx, "UPDATE briefings SET read=1 WHERE key=?", key)
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

package whatsapp

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store handles message persistence for WhatsApp.
type Store struct {
	db *sql.DB
}

// NewStore opens or creates the WhatsApp message store.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("whatsapp store: open: %w", err)
	}
	db.SetMaxOpenConns(4) // WAL mode allows concurrent readers; 4 conns avoids read starvation during history sync

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("whatsapp store: ping: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("whatsapp store: migrate: %w", err)
	}

	log.Printf("WhatsApp message store ready: %s", dbPath)
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate creates tables if they don't exist.
func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			chat_jid TEXT NOT NULL,
			sender_jid TEXT NOT NULL,
			sender_name TEXT DEFAULT '',
			text TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			from_me INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_chat ON messages(chat_jid)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_ts ON messages(timestamp)`,
		`CREATE TABLE IF NOT EXISTS contacts (
			jid TEXT PRIMARY KEY,
			name TEXT DEFAULT '',
			push_name TEXT DEFAULT '',
			notify_name TEXT DEFAULT '',
			updated_at INTEGER NOT NULL
		)`,
	}
	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// SaveMessage stores an incoming or outgoing message.
func (s *Store) SaveMessage(msg Message) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO messages (id, chat_jid, sender_jid, sender_name, text, timestamp, from_me)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ChatJID, msg.SenderJID, msg.SenderName, msg.Text,
		msg.Timestamp.Unix(), boolToInt(msg.FromMe),
	)
	return err
}

// SaveContact stores or updates a contact name mapping.
func (s *Store) SaveContact(jid, name string) error {
	_, err := s.db.Exec(
		`INSERT INTO contacts (jid, name, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(jid) DO UPDATE SET name = excluded.name, updated_at = excluded.updated_at`,
		jid, name, time.Now().Unix(),
	)
	return err
}

// GetContactName retrieves a contact's display name from the local store.
func (s *Store) GetContactName(jid string) string {
	var name string
	err := s.db.QueryRow(`SELECT name FROM contacts WHERE jid = ?`, jid).Scan(&name)
	if err != nil {
		return ""
	}
	return name
}

// SearchMessages searches messages by text content.
func (s *Store) SearchMessages(query string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, chat_jid, sender_jid, sender_name, text, timestamp, from_me
		 FROM messages WHERE text LIKE ? ORDER BY timestamp DESC LIMIT ?`,
		"%"+query+"%", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		var unixTS int64
		if err := rows.Scan(&m.ID, &m.ChatJID, &m.SenderJID, &m.SenderName, &m.Text, &unixTS, &m.FromMe); err != nil {
			return nil, err
		}
		m.Timestamp = time.Unix(unixTS, 0)
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if msgs == nil {
		return []Message{}, nil
	}
	return msgs, nil
}

// GetMessagesSince retrieves messages after a given timestamp.
func (s *Store) GetMessagesSince(since time.Time, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, chat_jid, sender_jid, sender_name, text, timestamp, from_me
		 FROM messages WHERE timestamp > ? ORDER BY timestamp ASC LIMIT ?`,
		since.Unix(), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		var unixTS int64
		if err := rows.Scan(&m.ID, &m.ChatJID, &m.SenderJID, &m.SenderName, &m.Text, &unixTS, &m.FromMe); err != nil {
			return nil, err
		}
		m.Timestamp = time.Unix(unixTS, 0)
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if msgs == nil {
		return []Message{}, nil
	}
	return msgs, nil
}

// GetChatMessages retrieves messages for a specific chat.
func (s *Store) GetChatMessages(chatJID string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, chat_jid, sender_jid, sender_name, text, timestamp, from_me
		 FROM messages WHERE chat_jid = ? ORDER BY timestamp DESC LIMIT ?`,
		chatJID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		var unixTS int64
		if err := rows.Scan(&m.ID, &m.ChatJID, &m.SenderJID, &m.SenderName, &m.Text, &unixTS, &m.FromMe); err != nil {
			return nil, err
		}
		m.Timestamp = time.Unix(unixTS, 0)
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if msgs == nil {
		return []Message{}, nil
	}
	return msgs, nil
}

// ChatSummary represents a chat overview with the latest message.
type ChatSummary struct {
	JID          string    `json:"jid"`
	DisplayName  string    `json:"display_name"`
	LastMessage  string    `json:"last_message"`
	LastTime     time.Time `json:"last_time"`
	Unread       int       `json:"unread"`
}

// GetChatList returns the list of unique chats with the latest message.
func (s *Store) GetChatList() ([]ChatSummary, error) {
	rows, err := s.db.Query(
		`SELECT m.chat_jid,
			(SELECT m2.text FROM messages m2 WHERE m2.chat_jid = m.chat_jid ORDER BY m2.timestamp DESC LIMIT 1) as last_msg,
			MAX(m.timestamp) as last_ts,
			SUM(CASE WHEN m.from_me = 0 THEN 1 ELSE 0 END) as total
		 FROM messages m
		 GROUP BY m.chat_jid
		 ORDER BY last_ts DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []ChatSummary
	for rows.Next() {
		var c ChatSummary
		var unixTS int64
		if err := rows.Scan(&c.JID, &c.LastMessage, &unixTS, &c.Unread); err != nil {
			return nil, err
		}
		c.LastTime = time.Unix(unixTS, 0)
		// Try to resolve display name from contacts table
		c.DisplayName = s.GetContactName(c.JID)
		if c.DisplayName == "" {
			c.DisplayName = partsBeforeAt(c.JID)
		}
		chats = append(chats, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if chats == nil {
		return []ChatSummary{}, nil
	}
	return chats, nil
}

// Stats returns message statistics.
func (s *Store) Stats() (total int, last24h int, err error) {
	err = s.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&total)
	if err != nil {
		return
	}
	err = s.db.QueryRow(
		`SELECT COUNT(*) FROM messages WHERE timestamp > ?`,
		time.Now().Add(-24*time.Hour).Unix(),
	).Scan(&last24h)
	return
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func partsBeforeAt(s string) string {
	if parts := strings.SplitN(s, "@", 2); len(parts) > 0 {
		return parts[0]
	}
	return s
}

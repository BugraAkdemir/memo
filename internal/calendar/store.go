// SPDX-License-Identifier: AGPL-3.0-or-later

package calendar

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

const schema = `
CREATE TABLE IF NOT EXISTS events (
    id           TEXT    PRIMARY KEY,
    title        TEXT    NOT NULL,
    start_time   INTEGER NOT NULL,
    end_time     INTEGER,
    description  TEXT    NOT NULL DEFAULT '',
    source       TEXT    NOT NULL DEFAULT 'manual',
    contact_name TEXT    NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    reminder_sent INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_events_start ON events(start_time);
`

// Store persists calendar events in a dedicated SQLite database.
type Store struct {
	db *sql.DB
}

// NewStore opens (or creates) the calendar database at dir/events.db.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("calendar: mkdir: %w", err)
	}
	path := filepath.Join(dir, "events.db")
	db, err := sql.Open("sqlite3", path+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("calendar: open db: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("calendar: schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the database connection.
func (s *Store) Close() error { return s.db.Close() }

// Add inserts a new event. If e.ID is empty a UUID is generated.
func (s *Store) Add(ctx context.Context, e Event) (Event, error) {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	if e.Source == "" {
		e.Source = SourceManual
	}

	var endUnix *int64
	if e.EndTime != nil {
		v := e.EndTime.Unix()
		endUnix = &v
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO events (id, title, start_time, end_time, description, source, contact_name, created_at, reminder_sent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		e.ID, e.Title, e.StartTime.Unix(), endUnix,
		e.Description, string(e.Source), e.ContactName, e.CreatedAt.Unix(),
	)
	if err != nil {
		return Event{}, fmt.Errorf("calendar: add: %w", err)
	}
	return e, nil
}

// List returns events whose StartTime falls within [from, to].
func (s *Store) List(ctx context.Context, from, to time.Time) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, start_time, end_time, description, source, contact_name, created_at, reminder_sent
		FROM events
		WHERE start_time BETWEEN ? AND ?
		ORDER BY start_time ASC`,
		from.Unix(), to.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("calendar: list: %w", err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

// Delete removes the event with the given ID. No error if not found.
func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("calendar: delete: %w", err)
	}
	return nil
}

// ListUpcoming returns all events whose StartTime is after `from`, ordered by
// start time ascending. Used for title-based cancellation lookup when the
// intent did not include a time reference.
func (s *Store) ListUpcoming(ctx context.Context, from time.Time, limit int) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, start_time, end_time, description, source, contact_name, created_at, reminder_sent
		FROM events
		WHERE start_time > ?
		ORDER BY start_time ASC
		LIMIT ?`,
		from.Unix(), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("calendar: list upcoming: %w", err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

// MarkReminderSent sets reminder_sent=1 for the given event ID.
func (s *Store) MarkReminderSent(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE events SET reminder_sent=1 WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("calendar: mark reminder: %w", err)
	}
	return nil
}

// PendingReminders returns unsent events whose StartTime is within the next
// leadMinutes minutes (i.e. reminder should fire now).
func (s *Store) PendingReminders(ctx context.Context, now time.Time, leadMinutes int) ([]Event, error) {
	deadline := now.Add(time.Duration(leadMinutes) * time.Minute).Unix()
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, start_time, end_time, description, source, contact_name, created_at, reminder_sent
		FROM events
		WHERE reminder_sent = 0
		  AND start_time > ?
		  AND start_time <= ?
		ORDER BY start_time ASC`,
		now.Unix(), deadline,
	)
	if err != nil {
		return nil, fmt.Errorf("calendar: pending reminders: %w", err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

func scanEvents(rows *sql.Rows) ([]Event, error) {
	var events []Event
	for rows.Next() {
		var e Event
		var startUnix, createdUnix int64
		var endUnix sql.NullInt64
		var src string
		var reminderSent int
		if err := rows.Scan(
			&e.ID, &e.Title, &startUnix, &endUnix,
			&e.Description, &src, &e.ContactName, &createdUnix, &reminderSent,
		); err != nil {
			return nil, fmt.Errorf("calendar: scan: %w", err)
		}
		e.StartTime = time.Unix(startUnix, 0)
		e.CreatedAt = time.Unix(createdUnix, 0)
		e.Source = Source(src)
		e.ReminderSent = reminderSent != 0
		if endUnix.Valid {
			t := time.Unix(endUnix.Int64, 0)
			e.EndTime = &t
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

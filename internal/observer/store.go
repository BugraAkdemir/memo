// SPDX-License-Identifier: AGPL-3.0-or-later

// Package observer implements the observation layer of the Memo Learning
// System (see docs/learning-system/README.md). It silently records what the
// user does — when they chat, run agents, or trigger the orchestra — into a
// local SQLite database. Nothing leaves the device; later phases analyse these
// observations to detect habitual patterns.
package observer

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"memo/internal/database"
)

// Activity types categorise an observation. New sources (Home Assistant, IoT,
// calendar, …) add new constants here without touching the storage layer.
const (
	ActivityChat      = "chat"
	ActivityAgent     = "agent"
	ActivityOrchestra = "orchestra"
	ActivitySession   = "session"
	ActivityIntent    = "intent"    // declared intent or habit from chat/WhatsApp
	ActivityWhatsApp  = "whatsapp" // message received/sent via WhatsApp
)

// Observation is a single recorded event. It is intentionally low-level and
// extensible: anything that does not fit a column goes into Metadata as a JSON
// blob, so the schema stays stable as new recorders are added.
type Observation struct {
	ID               int64     `json:"id"`
	Timestamp        time.Time `json:"timestamp"`
	DayOfWeek        int       `json:"day_of_week"`         // 0=Sunday .. 6=Saturday (time.Weekday)
	TimeOfDaySeconds int       `json:"time_of_day_seconds"` // seconds since local midnight [0,86400)
	SessionLengthMin int       `json:"session_length_min"`
	Topic            string    `json:"topic"`
	ActivityType     string    `json:"activity_type"`
	WordCount        int       `json:"word_count"`
	Metadata         string    `json:"metadata"` // JSON blob — extensible
}

// Store persists observations in a dedicated SQLite database
// (data/profile/observations.db by default).
type Store struct {
	db     *database.DB
	dir    string
	dbPath string
}

// StoreConfig configures a Store. Dir is the directory holding the database
// file; it is created by the caller (or by Open via the database package).
type StoreConfig struct {
	Dir string
}

func (c StoreConfig) dbPath() string {
	return filepath.Join(c.Dir, "observations.db")
}

// NewStore opens (creating if needed) the observations database and ensures the
// schema exists.
func NewStore(cfg StoreConfig) (*Store, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("observer.NewStore: dir is required")
	}

	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, fmt.Errorf("observer.NewStore: mkdir: %w", err)
	}

	dbPath := cfg.dbPath()
	db, err := database.Open(database.Config{Path: dbPath, MaxPool: 1})
	if err != nil {
		return nil, fmt.Errorf("observer.NewStore: %w", err)
	}

	s := &Store{db: db, dir: cfg.Dir, dbPath: dbPath}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("observer.NewStore: schema: %w", err)
	}
	return s, nil
}

func (s *Store) initSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS observations (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp           TEXT    NOT NULL,
			day_of_week         INTEGER NOT NULL,
			time_of_day_seconds INTEGER NOT NULL,
			session_length_min  INTEGER NOT NULL DEFAULT 0,
			topic               TEXT    NOT NULL DEFAULT '',
			activity_type       TEXT    NOT NULL DEFAULT '',
			word_count          INTEGER NOT NULL DEFAULT 0,
			metadata            TEXT    NOT NULL DEFAULT ''
		)
	`); err != nil {
		return err
	}

	// Pattern detection queries filter by activity type and recent time window,
	// so index both.
	if _, err := s.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_obs_activity ON observations(activity_type)
	`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_obs_timestamp ON observations(timestamp)
	`); err != nil {
		return err
	}
	return nil
}

// Record persists a single observation. The timestamp-derived fields (day of
// week, seconds since midnight) are always computed from obs.Timestamp's local
// time, so callers only need to set Timestamp (defaulting to now if zero).
func (s *Store) Record(obs Observation) (int64, error) {
	if obs.Timestamp.IsZero() {
		obs.Timestamp = time.Now()
	}
	t := obs.Timestamp.Local()
	obs.DayOfWeek = int(t.Weekday())
	obs.TimeOfDaySeconds = t.Hour()*3600 + t.Minute()*60 + t.Second()

	var id int64
	err := s.db.Write(context.Background(), func(tx *sql.Tx) error {
		res, err := tx.Exec(`
			INSERT INTO observations
				(timestamp, day_of_week, time_of_day_seconds, session_length_min, topic, activity_type, word_count, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`,
			obs.Timestamp.UTC().Format(time.RFC3339Nano),
			obs.DayOfWeek,
			obs.TimeOfDaySeconds,
			obs.SessionLengthMin,
			obs.Topic,
			obs.ActivityType,
			obs.WordCount,
			obs.Metadata,
		)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("observer.Record: %w", err)
	}
	return id, nil
}

// QueryFilter narrows a Query. Zero values mean "no constraint".
type QueryFilter struct {
	ActivityType string    // exact match if set
	Since        time.Time // only observations at/after this time
	Limit        int       // max rows (0 = no limit)
}

// Query returns observations matching the filter, newest first.
func (s *Store) Query(ctx context.Context, f QueryFilter) ([]Observation, error) {
	q := `SELECT id, timestamp, day_of_week, time_of_day_seconds, session_length_min,
	             topic, activity_type, word_count, metadata
	      FROM observations WHERE 1=1`
	var args []any
	if f.ActivityType != "" {
		q += " AND activity_type = ?"
		args = append(args, f.ActivityType)
	}
	if !f.Since.IsZero() {
		q += " AND timestamp >= ?"
		args = append(args, f.Since.UTC().Format(time.RFC3339Nano))
	}
	q += " ORDER BY timestamp DESC"
	if f.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, f.Limit)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("observer.Query: %w", err)
	}
	defer rows.Close()

	var out []Observation
	for rows.Next() {
		var obs Observation
		var ts string
		if err := rows.Scan(&obs.ID, &ts, &obs.DayOfWeek, &obs.TimeOfDaySeconds,
			&obs.SessionLengthMin, &obs.Topic, &obs.ActivityType, &obs.WordCount, &obs.Metadata); err != nil {
			return nil, fmt.Errorf("observer.Query: scan: %w", err)
		}
		if parsed, perr := time.Parse(time.RFC3339Nano, ts); perr == nil {
			obs.Timestamp = parsed
		}
		out = append(out, obs)
	}
	return out, rows.Err()
}

// Count returns the total number of stored observations.
func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM observations`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("observer.Count: %w", err)
	}
	return n, nil
}

// Prune deletes observations older than the cutoff. The learning system only
// analyses a sliding window (see README §5), so old rows are dropped to keep
// the database small and the user's data minimal.
func (s *Store) Prune(before time.Time) (int64, error) {
	var affected int64
	err := s.db.Write(context.Background(), func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM observations WHERE timestamp < ?`,
			before.UTC().Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		affected, err = res.RowsAffected()
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("observer.Prune: %w", err)
	}
	return affected, nil
}

// ClearAll removes every observation — backs the "delete all my data" control.
func (s *Store) ClearAll() error {
	return s.db.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM observations`)
		return err
	})
}

// Close releases the underlying database.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

package mood

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"memo/internal/logx"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const schema = `
CREATE TABLE IF NOT EXISTS mood_state (
	id         INTEGER PRIMARY KEY CHECK (id = 1),
	score      REAL    NOT NULL DEFAULT 0.0,
	updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS mood_history (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	score       REAL    NOT NULL,
	i_anlik     REAL    NOT NULL,
	recorded_at INTEGER NOT NULL
);`

type Store struct {
	db *sql.DB
}

func openStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("mood.Store: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", path))
	if err != nil {
		return nil, fmt.Errorf("mood.Store: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("mood.Store: schema: %w", err)
	}
	// sqlite3 creates the file itself at the process umask (typically 0644,
	// world-readable) — harden it now that the schema Exec guarantees it
	// exists. Best-effort: shouldn't block startup if it fails.
	if err := os.Chmod(path, 0o600); err != nil {
		logx.Printf("mood.Store: chmod %q: %v", path, err)
	}
	return &Store{db: db}, nil
}

// loadScore SQLite'dan mevcut skoru okur. Hiç kayıt yoksa 0.0 döner.
func (s *Store) loadScore(ctx context.Context) (float64, error) {
	var score float64
	err := s.db.QueryRowContext(ctx, `SELECT score FROM mood_state WHERE id = 1`).Scan(&score)
	if errors.Is(err, sql.ErrNoRows) {
		return 0.0, nil
	}
	return score, err
}

// saveScore yeni skoru kalıcı olarak yazar (INSERT OR REPLACE — tek satır garantisi).
func (s *Store) saveScore(ctx context.Context, score, iAnlik float64) error {
	now := time.Now().Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO mood_state (id, score, updated_at) VALUES (1, ?, ?)`,
		score, now,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO mood_history (score, i_anlik, recorded_at) VALUES (?, ?, ?)`,
		score, iAnlik, now,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// historySince returns mood_history rows recorded at or after since, oldest
// first — a time-window trend read, separate from loadScore's single
// "current value" read.
func (s *Store) historySince(ctx context.Context, since time.Time) ([]HistoryPoint, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT score, i_anlik, recorded_at FROM mood_history WHERE recorded_at >= ? ORDER BY recorded_at ASC`,
		since.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HistoryPoint
	for rows.Next() {
		var p HistoryPoint
		var recordedAt int64
		if err := rows.Scan(&p.Score, &p.IAnlik, &recordedAt); err != nil {
			return nil, err
		}
		p.RecordedAt = time.Unix(recordedAt, 0)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) close() error { return s.db.Close() }

// checkpoint forces a WAL checkpoint (mode TRUNCATE) so committed-but-WAL-only
// mood records are flushed into mood.db's main file before it's archived.
// Runs directly on s.db (not a second connection) — SetMaxOpenConns(1)
// above means database/sql's own pool already serializes this against any
// other query this Store issues; a second sql.Open to the same path (what
// cloudsync used to do) would not share that serialization at all. See
// BUG_REPORT.md P1-4.
func (s *Store) checkpoint(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

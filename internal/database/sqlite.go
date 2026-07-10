package database

import (
	"context"
	"database/sql"
	"fmt"
	"memo/internal/logx"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const defaultDriver = "sqlite3"

type Config struct {
	Path    string
	MaxPool int
}

type DB struct {
	sql     *sql.DB
	writeCh chan writeTask
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc

	// closeMu/closed coordinate Write() against Close() to close a real
	// deadlock: without them, a Write() call racing Close() could have its
	// task admitted into writeCh's buffer *after* writeLoop already saw
	// ctx.Done(), drained whatever was queued at that instant, and returned
	// — orphaning the task forever with nobody left to send to its done
	// channel, so the caller blocks on <-done indefinitely. Write() holds a
	// read lock for its entire body (send + wait for done); Close() takes
	// the write lock before cancelling, which can only succeed once every
	// in-flight Write() has fully finished, so writeLoop never exits out
	// from under a task that's still being submitted.
	closeMu sync.RWMutex
	closed  bool
}

type writeTask struct {
	ctx  context.Context
	fn   func(tx *sql.Tx) error
	done chan error
}

func Open(cfg Config) (*DB, error) {
	if cfg.MaxPool <= 0 {
		cfg.MaxPool = 1
	}
	if cfg.Path == "" {
		return nil, fmt.Errorf("database.Open: path is required")
	}

	// Ensure the parent directory exists before opening. SQLite will not
	// create missing directories and fails to open the file otherwise. This
	// matters after a full data wipe, which removes the DB's directory: the
	// subsequent re-open must be able to recreate it.
	if dir := filepath.Dir(cfg.Path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("database.Open: create dir %q: %w", dir, err)
		}
	}

	connStr := buildDSN(cfg.Path)

	driver := DriverName()
	pool, err := sql.Open(driver, connStr)
	if err != nil {
		return nil, fmt.Errorf("database.Open: %w", err)
	}

	pool.SetMaxOpenConns(cfg.MaxPool)
	pool.SetMaxIdleConns(cfg.MaxPool)
	pool.SetConnMaxLifetime(0)
	pool.SetConnMaxIdleTime(30 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	db := &DB{
		sql:     pool,
		writeCh: make(chan writeTask, 64),
		ctx:     ctx,
		cancel:  cancel,
	}

	if err := db.Ping(); err != nil {
		pool.Close()
		cancel()
		return nil, fmt.Errorf("database.Open: ping: %w", err)
	}

	// mattn/go-sqlite3 creates the file itself (no Go-level perm parameter to
	// pass in, unlike os.WriteFile elsewhere in this codebase), so it lands
	// at whatever the process umask allows — typically 0644, world-readable.
	// These databases hold chat history, memory, calendar, and (for
	// whatsapp/mood, opened separately) session data; harden after the fact.
	// Ping guarantees the file now exists. Best-effort: a failure here (e.g.
	// read-only filesystem quirks) shouldn't block the app from starting.
	if err := os.Chmod(cfg.Path, 0o600); err != nil {
		logx.Printf("database.Open: chmod %q: %v", cfg.Path, err)
	}

	db.wg.Add(1)
	go db.writeLoop()

	return db, nil
}

func buildDSN(path string) string {
	q := url.Values{}
	q.Set("_journal_mode", "WAL")
	q.Set("_busy_timeout", "5000")
	q.Set("_synchronous", "NORMAL")
	q.Set("_txlock", "immediate")
	// SQLite URI parser treats backslashes as path separators only on Windows
	// native builds — but the go-sqlite3 driver passes the URI through the C
	// layer which requires forward-slash paths. Absolute Windows paths
	// (C:\...) must also get a leading slash to form a valid file:/// URI.
	uriPath := filepath.ToSlash(path)
	if filepath.IsAbs(path) && len(uriPath) > 0 && uriPath[0] != '/' {
		uriPath = "/" + uriPath
	}
	return fmt.Sprintf("file:%s?%s", uriPath, q.Encode())
}

func (db *DB) writeLoop() {
	defer db.wg.Done()
	for {
		select {
		case task := <-db.writeCh:
			err := db.execWrite(task.ctx, task.fn)
			select {
			case task.done <- err:
			default:
				if err != nil {
					logx.Printf("database: write error dropped (caller gone): %v", err)
				}
			}
		case <-db.ctx.Done():
			// Drain queued tasks so Close() callers don't silently lose writes.
			for {
				select {
				case task := <-db.writeCh:
					err := db.execWrite(task.ctx, task.fn)
					select {
					case task.done <- err:
					default:
					}
				default:
					return
				}
			}
		}
	}
}

func (db *DB) execWrite(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	committed = true
	return nil
}

func (db *DB) Write(ctx context.Context, fn func(tx *sql.Tx) error) error {
	// Held for the whole call (see the closeMu field doc) so Close() cannot
	// tear down writeLoop while this task is still being submitted/awaited.
	db.closeMu.RLock()
	defer db.closeMu.RUnlock()
	if db.closed {
		return fmt.Errorf("database: closed")
	}

	done := make(chan error, 1)
	task := writeTask{ctx: ctx, fn: fn, done: done}

	select {
	case db.writeCh <- task:
	case <-ctx.Done():
		return ctx.Err()
	case <-db.ctx.Done():
		return fmt.Errorf("database: closed")
	}

	// Wait on caller's context only — the write is already serialised in
	// writeLoop and will complete regardless of DB shutdown, so we must not
	// use db.ctx here or we'd return a false error while the write commits.
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ExecContext executes a write query through the serialised write loop so
// that all DDL and DML writes are funneled through a single goroutine,
// preventing "database is locked" errors under concurrent access.
func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	var result sql.Result
	err := db.Write(ctx, func(tx *sql.Tx) error {
		var err error
		result, err = tx.ExecContext(ctx, query, args...)
		return err
	})
	return result, err
}

func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.sql.QueryContext(ctx, query, args...)
}

func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return db.sql.QueryRowContext(ctx, query, args...)
}

func (db *DB) Close() error {
	// Blocks until every in-flight Write() has released its read lock (i.e.
	// fully submitted its task and received its result), so writeLoop below
	// can never be cancelled out from under a task that's still mid-send —
	// see the closeMu field doc for the deadlock this closes.
	db.closeMu.Lock()
	db.closed = true
	db.closeMu.Unlock()

	db.cancel()
	db.wg.Wait()
	return db.sql.Close()
}

func (db *DB) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.sql.PingContext(ctx)
}

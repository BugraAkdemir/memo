package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
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
}

type writeTask struct {
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
	return fmt.Sprintf("file:%s?%s", path, q.Encode())
}

func (db *DB) writeLoop() {
	defer db.wg.Done()
	for {
		select {
		case task := <-db.writeCh:
			err := db.execWrite(task.fn)
			select {
			case task.done <- err:
			default:
			}
		case <-db.ctx.Done():
			return
		}
	}
}

func (db *DB) execWrite(fn func(tx *sql.Tx) error) error {
	tx, err := db.sql.BeginTx(db.ctx, nil)
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

func (db *DB) Write(fn func(tx *sql.Tx) error) error {
	done := make(chan error, 1)
	task := writeTask{fn: fn, done: done}

	select {
	case db.writeCh <- task:
	case <-db.ctx.Done():
		return db.ctx.Err()
	}

	select {
	case err := <-done:
		return err
	case <-db.ctx.Done():
		return db.ctx.Err()
	}
}

func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.sql.ExecContext(ctx, query, args...)
}

func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.sql.QueryContext(ctx, query, args...)
}

func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return db.sql.QueryRowContext(ctx, query, args...)
}

func (db *DB) Close() error {
	db.cancel()
	db.wg.Wait()
	return db.sql.Close()
}

func (db *DB) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.sql.PingContext(ctx)
}

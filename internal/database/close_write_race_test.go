package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
)

// TestWriteAfterCloseNeverHangs is a regression test for a real deadlock:
// Write()'s first select could pick "task admitted into writeCh" even
// though db.ctx was already Done (Go picks pseudo-randomly between ready
// select cases), racing against writeLoop's drain-and-exit on ctx.Done().
// If writeLoop had already drained-and-returned by the time the task landed
// in the channel, nobody would ever read it or signal its done channel, and
// the caller blocked on <-done forever — reproduced by simply calling
// Write() right after Close() returns, which used to hang non-deterministically
// (roughly 1 in a handful of tries under -count).
//
// Fixed by having Write() hold closeMu for its entire body and Close() take
// the write lock before cancelling, so writeLoop can never be torn down
// while a Write() call is still submitting/awaiting its task.
func TestWriteAfterCloseNeverHangs(t *testing.T) {
	for i := range 20 {
		db, err := Open(Config{Path: filepath.Join(t.TempDir(), "test.db"), MaxPool: 1})
		if err != nil {
			t.Fatalf("iteration %d: Open() error = %v", i, err)
		}
		db.Close()

		err = db.Write(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.Exec("SELECT 1")
			return err
		})
		if err == nil {
			t.Fatalf("iteration %d: Write() after Close() should return an error, got nil", i)
		}
	}
}

// TestConcurrentWriteRacingClose exercises the original, tighter race: a
// Write() call in flight exactly as Close() runs, rather than strictly
// after it. Must complete promptly and never hang (relies on the test
// binary's own -timeout to fail loudly if this regresses).
func TestConcurrentWriteRacingClose(t *testing.T) {
	db, err := Open(Config{Path: filepath.Join(t.TempDir(), "test.db"), MaxPool: 1})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		for range 50 {
			_ = db.Write(context.Background(), func(tx *sql.Tx) error {
				_, err := tx.Exec("SELECT 1")
				return err
			})
		}
	})

	db.Close()
	wg.Wait()
}

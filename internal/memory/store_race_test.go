package memory

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// TestCloseKeepsDBReferenceUsable is a regression test for a nil-pointer
// panic observed in practice: Close() used to set s.db = nil after closing
// the underlying database.DB, while background goroutines (e.g. the FTS/vec
// migration goroutines started in initSchema, which run for up to 60-120s)
// hold onto s and call s.db.Write/QueryContext without re-checking s.closed.
// If Close() raced in between, the next s.db access dereferenced a nil
// pointer and panicked.
//
// The fix stops nulling s.db — database.DB's own methods already handle
// being called after their internal Close() gracefully (they return an
// error, e.g. "sql: database is closed", instead of panicking), so keeping
// the outer pointer valid removes the race entirely rather than requiring
// every call site to snapshot-and-check.
//
// This deliberately calls Close() synchronously first, then uses s.db,
// rather than racing a live Write() against Close() — database.DB has a
// separate, still-open shutdown race of its own (a Write() already admitted
// into writeCh exactly as writeLoop drains-and-exits on ctx.Done() can block
// forever on its done channel) that is out of scope here and must not be
// exercised by this test.
func TestCloseKeepsDBReferenceUsable(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	s.Close()

	// Must not panic — this is exactly what a lingering background migration
	// goroutine would do after Close() completes.
	err = s.db.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec("SELECT 1")
		return err
	})
	if err == nil {
		t.Error("Write() after Close() should return an error, got nil")
	}
}

// TestEnsureVecMetadataResetsMigrationFlagOnDimensionChange is a regression
// test for BUG: switching the embedding model (and therefore its dimension)
// drops and recreates vec_memories empty, but the old vec_migration_done
// flag survived, so initSchema's "already migrated, skip" check silently
// left the new table permanently empty — vector search would return zero
// results forever with no error.
func TestEnsureVecMetadataResetsMigrationFlagOnDimensionChange(t *testing.T) {
	dir := t.TempDir()

	s1, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore(dim=3) error = %v", err)
	}
	if !s1.useVec {
		t.Skip("vec0 extension not available in this environment")
	}
	// Simulate a completed migration for the original dimension.
	if err := s1.db.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT OR REPLACE INTO _metadata(key, value) VALUES ('vec_migration_done', '1')")
		return err
	}); err != nil {
		t.Fatalf("seed vec_migration_done: %v", err)
	}
	s1.Close()

	// Reopen against the same DB file with a different dimension — this is
	// exactly what happens when the user switches embedding models.
	s2, err := NewStore(StoreConfig{Dir: dir, Dimension: 5, EmbeddingFunc: func(ctx context.Context, text string) ([]float32, error) {
		return []float32{1, 2, 3, 4, 5}, nil
	}})
	if err != nil {
		t.Fatalf("NewStore(dim=5) error = %v", err)
	}
	defer s2.Close()

	var migrated sql.NullString
	err = s2.db.QueryRowContext(context.Background(),
		"SELECT value FROM _metadata WHERE key = 'vec_migration_done'").Scan(&migrated)
	if err == nil && migrated.String == "1" {
		t.Fatal("vec_migration_done should have been cleared after a dimension change, but is still \"1\"")
	}
}

// TestMigrateEmbeddingsToVecSkipsDimensionMismatch is a regression test for
// BUG: migrateEmbeddingsToVec inserted every pending row in a single
// transaction with no per-row dimension check. vec0 rejects a vector whose
// width doesn't match the column's declared width, so one stale-dimension
// row (left over from a prior embedding model) aborted the whole batch —
// and because vec_migration_done never gets set on failure, every restart
// retried and failed identically, forever.
func TestMigrateEmbeddingsToVecSkipsDimensionMismatch(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer s.Close()
	if !s.useVec {
		t.Skip("vec0 extension not available in this environment")
	}

	ctx := context.Background()

	// A well-formed dim=3 row that should migrate successfully.
	if err := s.SaveInteraction(ctx, "coffee beans", "arabica"); err != nil {
		t.Fatalf("SaveInteraction: %v", err)
	}

	// A stale dim=5 row inserted directly (simulating leftover data from a
	// previous, differently-dimensioned embedding model) — must not be able
	// to poison the rest of the batch.
	staleBlob := floatsToBlob([]float32{1, 2, 3, 4, 5})
	if err := s.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO memories(uuid, role, content, timestamp, user_msg, assist_msg, embedding)
			 VALUES ('stale-dim-row', 'user', 'stale', ?, 'stale', 'stale', ?)`,
			time.Now().Format(time.RFC3339), staleBlob,
		)
		return err
	}); err != nil {
		t.Fatalf("insert stale row: %v", err)
	}

	if err := s.migrateEmbeddingsToVec(ctx); err != nil {
		t.Fatalf("migrateEmbeddingsToVec() error = %v, want nil (mismatched rows must be skipped, not fatal)", err)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM vec_memories").Scan(&count); err != nil {
		t.Fatalf("count vec_memories: %v", err)
	}
	if count != 1 {
		t.Errorf("vec_memories count = %d, want 1 (only the dim=3 row should have migrated)", count)
	}
}

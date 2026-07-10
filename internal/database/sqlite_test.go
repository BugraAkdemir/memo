package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestOpenCreatesMissingDir verifies that Open creates the DB's parent
// directory when it does not exist. This is the regression guard for the
// "store not initialized" bug after a full data wipe, which deletes the
// memory directory before the store is re-opened.
func TestOpenCreatesMissingDir(t *testing.T) {
	tests := []struct {
		name string
		rel  string // path relative to a fresh temp dir
	}{
		{name: "one level missing", rel: "memory/memo.db"},
		{name: "nested missing", rel: "a/b/c/memo.db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tt.rel)
			if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
				t.Fatalf("precondition: parent dir should not exist, stat err = %v", err)
			}

			db, err := Open(Config{Path: path, MaxPool: 1})
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			defer db.Close()

			if err := db.Ping(); err != nil {
				t.Fatalf("Ping() error = %v", err)
			}
		})
	}
}

// TestOpenHardensFilePermissions guards BUG-H1: mattn/go-sqlite3 creates the
// DB file itself with no way to pass a Go-level perm, so it lands at
// whatever the process umask allows (typically 0644, world-readable). Open
// must chmod it to 0600 after creation.
func TestOpenHardensFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memo.db")

	db, err := Open(Config{Path: path, MaxPool: 1})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if mode := info.Mode() & os.ModePerm; mode != 0o600 {
		t.Errorf("mode = %o, want 0600", mode)
	}
}

func TestVec0Available(t *testing.T) {
	db, err := Open(Config{
		Path:    filepath.Join(t.TempDir(), "test.db"),
		MaxPool: 1,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Check via module list (vec0 registers as a virtual table module, not a function)
	var modCount int
	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pragma_module_list WHERE name = 'vec0'",
	).Scan(&modCount)
	if err == nil && modCount > 0 {
		t.Logf("vec0 module found (%d entries)", modCount)
	} else {
		// Fallback: check function list
		var funcCount int
		err2 := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM pragma_function_list WHERE name LIKE '%vec%'",
		).Scan(&funcCount)
		if err2 != nil {
			t.Fatalf("query pragma_function_list: %v", err2)
		}
		if funcCount == 0 {
			t.Skip("vec0 extension not available in this environment")
		}
		t.Logf("vec0 functions found: %d", funcCount)
	}

	_, err = db.ExecContext(ctx,
		"CREATE VIRTUAL TABLE IF NOT EXISTS test_vec USING vec0(embedding FLOAT[3] distance_metric=cosine)",
	)
	if err != nil {
		t.Fatalf("create vec0 table: %v", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO test_vec(rowid, embedding) VALUES (1, '[1.0, 0.0, 0.0]')`,
	)
	if err != nil {
		t.Fatalf("insert into vec0: %v", err)
	}

	rows, err := db.QueryContext(ctx,
		`SELECT rowid, distance FROM test_vec WHERE embedding MATCH '[0.9, 0.1, 0.0]' AND k = 5`,
	)
	if err != nil {
		t.Fatalf("search vec0: %v", err)
	}
	defer rows.Close()

	var found bool
	for rows.Next() {
		var rowid int64
		var dist float64
		rows.Scan(&rowid, &dist)
		if rowid == 1 {
			found = true
			t.Logf("Found rowid=1 with distance=%f", dist)
		}
	}
	if !found {
		t.Error("expected to find rowid=1 in search results")
	}
}

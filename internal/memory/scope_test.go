// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// scopeEmbedding gives every "coffee ..." text its own direction (cosine
// between any two of them < 0.92, so the save-time duplicate check keeps them
// all) while still being close to a bare "coffee" query.
func scopeEmbedding(_ context.Context, text string) ([]float32, error) {
	t := strings.ToLower(text)
	if !strings.Contains(t, "coffee") {
		return []float32{0, 0, 1}, nil
	}
	switch {
	case strings.Contains(t, "machine"), strings.Contains(t, "note a"):
		return []float32{1, 0.5, 0}, nil
	case strings.Contains(t, "cup"), strings.Contains(t, "note b"):
		return []float32{1, 0, 0.5}, nil
	case strings.Contains(t, "beans"):
		return []float32{1, 0.3, 0.3}, nil
	case strings.Contains(t, "grinder"):
		return []float32{1, 0.3, -0.3}, nil
	case strings.Contains(t, "note c"):
		return []float32{1, -0.5, 0}, nil
	}
	return []float32{1, 0, 0}, nil
}

// scopeStore opens a store; forceGo exercises the no-sqlite-vec path CI runners
// without the extension always take, !forceGo uses sqlite-vec when available.
func scopeStore(t *testing.T, forceGo bool) *Store {
	t.Helper()
	s, err := NewStore(StoreConfig{
		Dir: t.TempDir(), Dimension: 3, EmbeddingFunc: scopeEmbedding, ForceGoFallback: forceGo,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func eachMode(t *testing.T, f func(t *testing.T, forceGo bool)) {
	t.Run("vec-if-available", func(t *testing.T) { f(t, false) })
	t.Run("go-fallback", func(t *testing.T) { f(t, true) })
}

func contents(rs []MemoryResult) string {
	var parts []string
	for _, r := range rs {
		parts = append(parts, r.UserMsg)
	}
	return strings.Join(parts, " | ")
}

// seedPools puts one coffee memory into project A, project B and the general
// pool, plus a second general one, all matching the same query.
func seedPools(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	for _, w := range []struct {
		sc  Scope
		msg string
	}{
		{ProjectScope("p_a"), "coffee beans for project A"},
		{ProjectScope("p_b"), "coffee grinder for project B"},
		{AllProjects, "coffee machine in general chat"},
		{ProjectScope(""), "coffee cup in general pool"},
	} {
		if err := s.SaveInteraction(ctx, w.sc, w.msg, "ok"); err != nil {
			t.Fatalf("SaveInteraction(%q): %v", w.msg, err)
		}
	}
}

func TestScope_ProjectChatsOnlySeeTheirOwnPool(t *testing.T) {
	eachMode(t, func(t *testing.T, forceGo bool) {
		s := scopeStore(t, forceGo)
		seedPools(t, s)
		ctx := context.Background()

		cases := []struct {
			name    string
			sc      Scope
			want    []string
			notWant []string
		}{
			{"project A", ProjectScope("p_a"), []string{"project A"}, []string{"project B", "general"}},
			{"project B", ProjectScope("p_b"), []string{"project B"}, []string{"project A", "general"}},
			{"general pool only", ProjectScope(""), []string{"general chat", "general pool"}, []string{"project A", "project B"}},
			{"zero value fails closed to the general pool", Scope{}, []string{"general chat"}, []string{"project A", "project B"}},
			{"all projects", AllProjects, []string{"project A", "project B", "general chat", "general pool"}, nil},
		}
		for _, c := range cases {
			got, err := s.RetrieveContext(ctx, c.sc, "coffee", 10, 0)
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			all := contents(got)
			for _, w := range c.want {
				if !strings.Contains(all, w) {
					t.Errorf("%s: missing %q in %q", c.name, w, all)
				}
			}
			for _, n := range c.notWant {
				if strings.Contains(all, n) {
					t.Errorf("%s: LEAK: %q appeared in %q", c.name, n, all)
				}
			}
		}
	})
}

func TestScope_LexicalSearchIsScopedToo(t *testing.T) {
	s := scopeStore(t, true)
	if !s.useFTS {
		t.Skip("FTS5 unavailable in this build")
	}
	seedPools(t, s)
	ctx := context.Background()

	got, err := s.RetrieveLexicalContext(ctx, ProjectScope("p_a"), "coffee", 10)
	if err != nil {
		t.Fatal(err)
	}
	all := contents(got)
	if !strings.Contains(all, "project A") || strings.Contains(all, "project B") || strings.Contains(all, "general") {
		t.Fatalf("lexical search leaked across pools: %q", all)
	}
	if got, _ := s.RetrieveLexicalContext(ctx, AllProjects, "coffee", 10); len(got) != 4 {
		t.Fatalf("AllProjects lexical = %d rows, want 4", len(got))
	}
}

func TestScope_PinnedFactsNeverCrossIntoAProject(t *testing.T) {
	eachMode(t, func(t *testing.T, forceGo bool) {
		s := scopeStore(t, forceGo)
		ctx := context.Background()
		for _, w := range []struct {
			sc      Scope
			content string
		}{
			{AllProjects, "User's name is Ada"}, // personal fact, general pool
			{ProjectScope("p_a"), "Project A uses tabs"},
			{ProjectScope("p_b"), "Project B uses spaces"},
		} {
			if err := s.SaveExplicit(ctx, w.sc, w.content, ""); err != nil {
				t.Fatal(err)
			}
		}

		inA, err := s.GetPinnedFacts(ctx, ProjectScope("p_a"))
		if err != nil {
			t.Fatal(err)
		}
		if len(inA) != 1 || !strings.Contains(inA[0].Content, "Project A") {
			t.Fatalf("project A pinned = %+v; the user's personal fact must not be here", inA)
		}
		ranked, err := s.GetPinnedFactsRanked(ctx, ProjectScope("p_a"), "name", 5)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range ranked {
			if strings.Contains(r.Content, "Ada") || strings.Contains(r.Content, "Project B") {
				t.Fatalf("ranked pinned leaked %q into project A", r.Content)
			}
		}
		if all, _ := s.GetPinnedFacts(ctx, AllProjects); len(all) != 3 {
			t.Fatalf("AllProjects pinned = %d, want 3", len(all))
		}
		if gen, _ := s.GetPinnedFacts(ctx, Scope{}); len(gen) != 1 || !strings.Contains(gen[0].Content, "Ada") {
			t.Fatalf("zero-value scope pinned = %+v, want only the general fact", gen)
		}
	})
}

func TestScope_DuplicateCheckStaysInsideItsOwnPool(t *testing.T) {
	eachMode(t, func(t *testing.T, forceGo bool) {
		s := scopeStore(t, forceGo)
		ctx := context.Background()
		save := func(sc Scope) {
			t.Helper()
			if err := s.SaveInteraction(ctx, sc, "coffee question", "coffee answer"); err != nil {
				t.Fatal(err)
			}
		}
		save(ProjectScope("p_a"))
		save(ProjectScope("p_b"))
		save(AllProjects)
		if n := s.Count(); n != 3 {
			t.Fatalf("the same turn in three pools stored %d rows, want 3 (a duplicate in another pool must not suppress it)", n)
		}
		save(ProjectScope("p_a")) // a real repeat inside one pool is still dropped
		if n := s.Count(); n != 3 {
			t.Fatalf("repeat inside the same pool was stored: %d rows", n)
		}
	})
}

func TestScope_ConsolidationNeverMergesAcrossPools(t *testing.T) {
	s := scopeStore(t, true)
	ctx := context.Background()
	// Import skips the save-time duplicate check, so near-identical rows can
	// exist in both pools — exactly what the merge pass looks for.
	payload := ExportPayload{Version: 2, Memories: []ExportMemory{
		{UUID: "g1", Content: "[t] User: coffee one general", UserMsg: "coffee one general", Importance: 3, Source: "conversation", Timestamp: "2026-01-01T00:00:00Z"},
		{UUID: "g2", Content: "[t] User: coffee two general", UserMsg: "coffee two general", Importance: 3, Source: "conversation", Timestamp: "2026-01-02T00:00:00Z"},
		{UUID: "a1", Content: "[t] User: coffee one projecta", UserMsg: "coffee one projecta", Importance: 3, Source: "conversation", Timestamp: "2026-01-03T00:00:00Z", ProjectID: "p_a"},
		{UUID: "a2", Content: "[t] User: coffee two projecta", UserMsg: "coffee two projecta", Importance: 3, Source: "conversation", Timestamp: "2026-01-04T00:00:00Z", ProjectID: "p_a"},
	}}
	data, _ := json.Marshal(payload)
	if n, err := s.Import(ctx, data); err != nil || n != 4 {
		t.Fatalf("Import = %d, %v", n, err)
	}

	pairs, err := s.FindMergeCandidates(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) == 0 {
		t.Fatal("expected the two general rows to be merge candidates")
	}
	for _, p := range pairs {
		if strings.Contains(p.Content1, "projecta") || strings.Contains(p.Content2, "projecta") {
			t.Fatalf("a project row entered the general consolidation pass: %+v", p)
		}
	}
}

func TestScope_ExportImportKeepsTheProject(t *testing.T) {
	src := scopeStore(t, true)
	seedPools(t, src)
	data, err := src.Export(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	dst := scopeStore(t, true)
	if n, err := dst.Import(context.Background(), data); err != nil || n != 4 {
		t.Fatalf("Import = %d, %v", n, err)
	}
	got, _ := dst.RetrieveContext(context.Background(), ProjectScope("p_a"), "coffee", 10, 0)
	if all := contents(got); !strings.Contains(all, "project A") || strings.Contains(all, "project B") || strings.Contains(all, "general") {
		t.Fatalf("project A after import = %q", all)
	}
}

// ---- one-time schema migration ------------------------------------------

// legacyStoreDir builds a directory holding a memory.db as the previous release
// wrote it: the current schema with memories.project_id removed.
func legacyStoreDir(t *testing.T, rows int) string {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: scopeEmbedding, ForceGoFallback: true})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < rows; i++ {
		if err := s.SaveInteraction(context.Background(), AllProjects, "coffee note "+string(rune('a'+i)), "reply"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(dir, "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		"DROP INDEX IF EXISTS idx_memories_project",
		"ALTER TABLE memories DROP COLUMN project_id",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("building legacy db (%s): %v", stmt, err)
		}
	}
	return dir
}

func hasProjectColumn(t *testing.T, dbPath string) bool {
	t.Helper()
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('memories') WHERE name='project_id'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func rowCount(t *testing.T, dbPath string) int {
	t.Helper()
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM memories").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMigration_SnapshotsThenAddsTheColumnAndKeepsEveryRow(t *testing.T) {
	dir := legacyStoreDir(t, 3)
	dbPath := filepath.Join(dir, "memory.db")
	if hasProjectColumn(t, dbPath) {
		t.Fatal("test setup: legacy db already has project_id")
	}
	snaps := t.TempDir()

	s, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: scopeEmbedding, ForceGoFallback: true, SnapshotDir: snaps})
	if err != nil {
		t.Fatalf("NewStore on legacy db: %v", err)
	}
	if !s.ProjectScopeAvailable() {
		t.Fatal("migration did not enable project scope")
	}
	if n := s.Count(); n != 3 {
		t.Fatalf("rows after migration = %d, want 3", n)
	}
	got, err := s.RetrieveContext(context.Background(), AllProjects, "coffee", 10, 0)
	if err != nil || len(got) != 3 {
		t.Fatalf("old rows must stay readable as the general pool: %d, %v", len(got), err)
	}
	if only, _ := s.RetrieveContext(context.Background(), ProjectScope("p_new"), "coffee", 10, 0); len(only) != 0 {
		t.Fatalf("a brand-new project saw %d legacy rows", len(only))
	}
	s.Close()
	if !hasProjectColumn(t, dbPath) {
		t.Fatal("column missing after migration")
	}

	entries, _ := os.ReadDir(snaps)
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "memory-pre-project-id-") {
		t.Fatalf("snapshot dir = %v, want exactly one pre-migration snapshot", entries)
	}
	snap := filepath.Join(snaps, entries[0].Name())
	if hasProjectColumn(t, snap) || rowCount(t, snap) != 3 {
		t.Fatalf("the snapshot must be the pre-migration database (no column, 3 rows)")
	}

	// Idempotent: reopening neither snapshots again nor changes anything.
	s2, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: scopeEmbedding, ForceGoFallback: true, SnapshotDir: snaps})
	if err != nil {
		t.Fatal(err)
	}
	s2.Close()
	if again, _ := os.ReadDir(snaps); len(again) != 1 {
		t.Fatalf("second open created another snapshot: %v", again)
	}
}

func TestMigration_ABrandNewDatabaseNeedsNoSnapshot(t *testing.T) {
	snaps := t.TempDir()
	s, err := NewStore(StoreConfig{Dir: t.TempDir(), Dimension: 3, EmbeddingFunc: scopeEmbedding, ForceGoFallback: true, SnapshotDir: snaps})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !s.ProjectScopeAvailable() {
		t.Fatal("new database must have project scope")
	}
	if entries, _ := os.ReadDir(snaps); len(entries) != 0 {
		t.Fatalf("an empty database was snapshotted: %v", entries)
	}
}

func TestMigration_IfTheSnapshotCannotBeWrittenNothingIsMigrated(t *testing.T) {
	dir := legacyStoreDir(t, 2)
	dbPath := filepath.Join(dir, "memory.db")

	// A snapshot "directory" that is really a file: MkdirAll must fail.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: scopeEmbedding, ForceGoFallback: true, SnapshotDir: filepath.Join(blocker, "snaps")})
	if err != nil {
		t.Fatalf("a failed snapshot must not stop memory from opening: %v", err)
	}
	defer s.Close()
	if s.ProjectScopeAvailable() {
		t.Fatal("project scope enabled without a safety snapshot")
	}

	// Legacy behaviour is fully intact for ordinary chats…
	ctx := context.Background()
	if n := s.Count(); n != 2 {
		t.Fatalf("rows = %d, want the 2 originals untouched", n)
	}
	if got, err := s.RetrieveContext(ctx, AllProjects, "coffee", 10, 0); err != nil || len(got) != 2 {
		t.Fatalf("general retrieval = %d, %v", len(got), err)
	}
	if err := s.SaveInteraction(ctx, AllProjects, "coffee again later", "ok"); err != nil {
		t.Fatalf("general save: %v", err)
	}
	// …and a project chat gets nothing rather than the general pool.
	if got, _ := s.RetrieveContext(ctx, ProjectScope("p_a"), "coffee", 10, 0); len(got) != 0 {
		t.Fatalf("project scope returned %d rows from a store that cannot partition", len(got))
	}
	if err := s.SaveInteraction(ctx, ProjectScope("p_a"), "project secret coffee", "ok"); err != nil {
		t.Fatal(err)
	}
	if n := s.Count(); n != 3 {
		t.Fatalf("a project save landed in the general pool: rows = %d, want 3", n)
	}
	s.Close()
	if hasProjectColumn(t, dbPath) {
		t.Fatal("the column was added even though the snapshot failed")
	}
}

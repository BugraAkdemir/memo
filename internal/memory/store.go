package memory

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"memo/internal/logx"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"memo/internal/database"
	"memo/internal/models"
)

type EmbeddingFunc func(ctx context.Context, text string) ([]float32, error)

// ConsolidationFunc merges two similar memory entries into one.
// Implementations call an LLM and return the merged text.
type ConsolidationFunc func(ctx context.Context, content1, content2 string) (string, error)

type MemoryResult = models.MemoryResult
type MemoryFileInfo = models.MemoryFileInfo
type GobFileInfo = models.MemoryFileInfo

type Store struct {
	db            *database.DB
	embed         EmbeddingFunc
	consolidateFn ConsolidationFunc
	dim           int
	dir           string
	dbPath        string
	mu            sync.RWMutex
	closed        bool
	useVec        bool
	useFTS        bool
	stopCh        chan struct{}

	// forceGoFallback skips sqlite-vec entirely during initSchema, as if
	// the extension weren't compiled in — test-only (see StoreConfig's doc
	// comment). Read exactly once, synchronously, inside initSchema before
	// NewStore returns; never touched again afterward, unlike an earlier
	// attempt at this that mutated useVec directly from a test helper
	// *after* construction — initSchema's own background vec-migration
	// goroutine (started for exactly this Store) was already reading
	// useVec concurrently by then, a genuine data race caught by -race.
	forceGoFallback bool
}

// SetConsolidationFunc registers the LLM-backed merge function used by the
// daily consolidation pass. Pass nil to disable.
func (s *Store) SetConsolidationFunc(fn ConsolidationFunc) {
	s.mu.Lock()
	s.consolidateFn = fn
	s.mu.Unlock()
}

type StoreConfig struct {
	Dir           string
	Dimension     int
	EmbeddingFunc EmbeddingFunc

	// ForceGoFallback makes NewStore skip sqlite-vec entirely, as if the
	// extension weren't compiled in — for tests that need to exercise
	// goSearch specifically (the code path a CI runner without sqlite-vec
	// always takes) regardless of whether the extension happens to be
	// available on the machine actually running the test. Production
	// callers should never set this.
	ForceGoFallback bool
}

func (c StoreConfig) dbPath() string {
	return filepath.Join(c.Dir, "memory.db")
}

func NewStore(cfg StoreConfig) (*Store, error) {
	if cfg.Dimension <= 0 {
		cfg.Dimension = 768
	}
	if cfg.EmbeddingFunc == nil {
		return nil, fmt.Errorf("memory.NewStore: embeddingFunc is nil")
	}

	dbPath := cfg.dbPath()

	dbCfg := database.Config{
		Path:    dbPath,
		MaxPool: 1,
	}

	db, err := database.Open(dbCfg)
	if err != nil {
		return nil, fmt.Errorf("memory.NewStore: %w", err)
	}

	s := &Store{
		db:              db,
		embed:           cfg.EmbeddingFunc,
		dim:             cfg.Dimension,
		dir:             cfg.Dir,
		dbPath:          dbPath,
		forceGoFallback: cfg.ForceGoFallback,
	}

	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("memory.NewStore: schema: %w", err)
	}

	s.stopCh = make(chan struct{})
	logx.GoRecover("memory.Store.runImportanceDecay", s.runImportanceDecay)

	return s, nil
}

func (s *Store) initSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS memories (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid        TEXT NOT NULL UNIQUE,
			role        TEXT NOT NULL DEFAULT 'user',
			content     TEXT NOT NULL,
			timestamp   TEXT NOT NULL,
			user_msg    TEXT NOT NULL DEFAULT '',
			assist_msg  TEXT NOT NULL DEFAULT '',
			type        TEXT NOT NULL DEFAULT 'conversation',
			embedding   BLOB DEFAULT NULL,
			chunk_index  INTEGER NOT NULL DEFAULT 0,
			parent_uuid  TEXT NOT NULL DEFAULT '',
			total_chunks INTEGER NOT NULL DEFAULT 1
		)
	`); err != nil {
		return err
	}

	// Migrate: add columns to existing tables that predate this schema.
	for _, col := range []struct{ name, def string }{
		{"chunk_index", "INTEGER NOT NULL DEFAULT 0"},
		{"parent_uuid", "TEXT NOT NULL DEFAULT ''"},
		{"total_chunks", "INTEGER NOT NULL DEFAULT 1"},
		{"session_id", "TEXT NOT NULL DEFAULT ''"},
		{"importance", "INTEGER NOT NULL DEFAULT 3"},
		{"tags", "TEXT NOT NULL DEFAULT ''"},
		{"source", "TEXT NOT NULL DEFAULT 'conversation'"},
		{"retrieve_count", "INTEGER NOT NULL DEFAULT 0"},
		{"pending_deletion", "INTEGER NOT NULL DEFAULT 0"},
	} {
		var count int
		_ = s.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM pragma_table_info('memories') WHERE name = ?", col.name,
		).Scan(&count)
		if count == 0 {
			if _, err := s.db.ExecContext(ctx,
				"ALTER TABLE memories ADD COLUMN "+col.name+" "+col.def,
			); err != nil {
				return fmt.Errorf("alter memories add %s: %w", col.name, err)
			}
			logx.Printf("MEMORY: migrated column memories.%s", col.name)
		}
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`); err != nil {
		return err
	}

	for _, idx := range []string{
		"CREATE INDEX IF NOT EXISTS idx_memories_timestamp ON memories(timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_memories_importance ON memories(importance)",
		"CREATE INDEX IF NOT EXISTS idx_memories_source ON memories(source)",
		"CREATE INDEX IF NOT EXISTS idx_memories_retrieve_count ON memories(retrieve_count)",
		"CREATE INDEX IF NOT EXISTS idx_memories_pending ON memories(pending_deletion)",
	} {
		if _, err := s.db.ExecContext(ctx, idx); err != nil {
			logx.Printf("MEMORY: index warning: %v", err)
		}
	}

	if ftsErr := s.tryCreateFTSTable(ctx); ftsErr == nil {
		s.useFTS = true
		var ftsMigrated string
		if err := s.db.QueryRowContext(ctx,
			"SELECT value FROM _metadata WHERE key = 'fts_migration_done'",
		).Scan(&ftsMigrated); err != nil || ftsMigrated != "1" {
			stopCh := s.stopCh
			go func() {
				defer logx.Recover("memory.Store/FTS migration")
				migCtx, migCancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer migCancel()
				// Abort early if the store is closed during migration.
				go func() {
					defer logx.Recover("memory.Store/FTS migration watchdog")
					select {
					case <-stopCh:
						migCancel()
					case <-migCtx.Done():
					}
				}()
				if err := s.migrateFTS(migCtx); err != nil {
					logx.Printf("MEMORY: FTS migrate: %v", err)
					return
				}
				_ = s.db.Write(migCtx, func(tx *sql.Tx) error {
					_, err := tx.Exec(
						"INSERT OR REPLACE INTO _metadata(key, value) VALUES ('fts_migration_done', '1')",
					)
					return err
				})
				logx.Printf("MEMORY: FTS migration complete")
			}()
		}
	} else {
		s.useFTS = false
		logx.Printf("MEMORY: fts5 not available (%v), keyword search disabled", ftsErr)
	}

	vecErr := fmt.Errorf("forced Go fallback for testing")
	if !s.forceGoFallback {
		vecErr = s.tryCreateVecTable(ctx)
	}
	if vecErr == nil {
		s.useVec = true
		if err := s.ensureVecMetadata(ctx); err != nil {
			logx.Printf("MEMORY: metadata init: %v", err)
		}

		var migrated string
		if err := s.db.QueryRowContext(ctx,
			"SELECT value FROM _metadata WHERE key = 'vec_migration_done'",
		).Scan(&migrated); err == nil && migrated == "1" {
			logx.Printf("MEMORY: vec migration already complete, skipping")
		} else {
			// Run migration in the background so NewStore returns immediately
			// and the store is usable right away. Writes are still serialised
			// through the DB write-loop, so there is no race with live saves.
			stopCh2 := s.stopCh
			go func() {
				defer logx.Recover("memory.Store/vec migration")
				migCtx, migCancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer migCancel()
				// Abort early if the store is closed during migration.
				go func() {
					defer logx.Recover("memory.Store/vec migration watchdog")
					select {
					case <-stopCh2:
						migCancel()
					case <-migCtx.Done():
					}
				}()
				if err := s.migrateEmbeddingsToVec(migCtx); err != nil {
					logx.Printf("MEMORY: migrate to vec: %v", err)
					return
				}
				_ = s.db.Write(migCtx, func(tx *sql.Tx) error {
					_, err := tx.Exec(
						"INSERT OR REPLACE INTO _metadata(key, value) VALUES ('vec_migration_done', '1')",
					)
					return err
				})
				logx.Printf("MEMORY: vec migration complete")
			}()
		}
	} else {
		s.useVec = false
		logx.Printf("MEMORY: vec0 not available (%v), using Go fallback", vecErr)
	}

	var existingDim int
	err := s.db.QueryRowContext(ctx,
		"SELECT value FROM _metadata WHERE key = 'embedding_dimension'",
	).Scan(&existingDim)
	if err != nil {
		_ = s.db.Write(ctx, func(tx *sql.Tx) error {
			_, err := tx.Exec(
				"INSERT OR REPLACE INTO _metadata(key, value) VALUES ('embedding_dimension', ?)",
				fmt.Sprintf("%d", s.dim),
			)
			return err
		})
	}

	return nil
}

func (s *Store) tryCreateFTSTable(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
			content,
			user_msg,
			assist_msg,
			tokenize='unicode61'
		)
	`)
	return err
}

func (s *Store) tryCreateVecTable(ctx context.Context) error {
	q := fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS vec_memories USING vec0(
		embedding FLOAT[%d] distance_metric=cosine
	)`, s.dim)
	_, err := s.db.ExecContext(ctx, q)
	return err
}

func (s *Store) ensureVecMetadata(ctx context.Context) error {
	var existingDim int
	err := s.db.QueryRowContext(ctx,
		"SELECT value FROM _metadata WHERE key = 'embedding_dimension'",
	).Scan(&existingDim)

	if err != nil {
		return s.db.Write(ctx, func(tx *sql.Tx) error {
			_, err := tx.Exec(
				"INSERT OR REPLACE INTO _metadata(key, value) VALUES ('embedding_dimension', ?)",
				fmt.Sprintf("%d", s.dim),
			)
			return err
		})
	}

	if existingDim != s.dim {
		logx.Printf("MEMORY: embedding dimension changed from %d to %d, recreating vec index", existingDim, s.dim)
		return s.db.Write(ctx, func(tx *sql.Tx) error {
			if _, err := tx.Exec("DROP TABLE IF EXISTS vec_memories"); err != nil {
				return err
			}
			if _, err := tx.Exec(fmt.Sprintf(
				"CREATE VIRTUAL TABLE IF NOT EXISTS vec_memories USING vec0(embedding FLOAT[%d] distance_metric=cosine)",
				s.dim,
			)); err != nil {
				return err
			}
			// The vec index was just dropped and recreated empty, so the old
			// 'vec_migration_done' flag (if any) is now stale — without
			// clearing it, initSchema's migration-done check would see "1"
			// from the previous dimension and skip re-running the backfill
			// forever, leaving vec_memories permanently empty and vector
			// search silently returning zero results.
			if _, err := tx.Exec("DELETE FROM _metadata WHERE key = 'vec_migration_done'"); err != nil {
				return err
			}
			_, err := tx.Exec(
				"INSERT OR REPLACE INTO _metadata(key, value) VALUES ('embedding_dimension', ?)",
				fmt.Sprintf("%d", s.dim),
			)
			return err
		})
	}

	return nil
}

func (s *Store) migrateFTS(ctx context.Context) error {
	type row struct {
		id        int64
		content   string
		userMsg   string
		assistMsg string
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, content, user_msg, assist_msg FROM memories
		 WHERE id NOT IN (SELECT rowid FROM memories_fts)`)
	if err != nil {
		return err
	}
	var pending []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.content, &r.userMsg, &r.assistMsg); err != nil {
			continue
		}
		pending = append(pending, r)
	}
	rows.Close()

	if len(pending) == 0 {
		return nil
	}

	return s.db.Write(ctx, func(tx *sql.Tx) error {
		for _, r := range pending {
			if _, err := tx.Exec(
				"INSERT INTO memories_fts(rowid, content, user_msg, assist_msg) VALUES (?, ?, ?, ?)",
				r.id, r.content, r.userMsg, r.assistMsg,
			); err != nil {
				return fmt.Errorf("fts insert row %d: %w", r.id, err)
			}
		}
		return nil
	})
}

func (s *Store) migrateEmbeddingsToVec(ctx context.Context) error {
	if !s.useVec {
		return nil
	}

	// The DB runs on a single-connection pool (MaxOpenConns=1). An open rows
	// cursor holds that one connection, so issuing a Write() while the cursor
	// is still open deadlocks: the write's BeginTx can never acquire a
	// connection until the cursor closes, and the cursor doesn't close until
	// the write finishes. The deadlock only resolves when the context deadline
	// fires (~120s), which is exactly the "migrate to vec: context deadline
	// exceeded" stall. Fix: drain all rows into memory and close the cursor
	// BEFORE writing, so the write transaction can grab the connection freely.
	type pendingVec struct {
		id  int64
		vec []float32
	}

	// Only migrate rows not already present in vec_memories. This makes the
	// migration idempotent across restarts (the previous deadlock left the
	// vec_migration_done flag unset, so it re-runs and would otherwise hit a
	// UNIQUE constraint on rows a partial run or live save already inserted).
	// vec0 virtual tables do not honour INSERT OR IGNORE, so we must filter
	// here rather than rely on a conflict clause.
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, embedding FROM memories
		 WHERE embedding IS NOT NULL
		   AND id NOT IN (SELECT rowid FROM vec_memories)`)
	if err != nil {
		return err
	}

	var pending []pendingVec
	var skippedDim int
	for rows.Next() {
		var id int64
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			continue
		}
		vec := blobToFloats(blob)
		if len(vec) == 0 {
			continue
		}
		// A dimension mismatch means this row's embedding predates a later
		// embedding-model switch (vec_memories is a fixed-width vec0 table
		// created for the *current* s.dim). vec0 rejects an INSERT whose
		// vector width doesn't match the declared column, and since every
		// row here is written in a single transaction, one such row would
		// abort the whole batch and — because vec_migration_done never gets
		// set on failure — retry and fail identically forever, leaving
		// vec_memories permanently empty. Skip it instead; it's still
		// reachable via FTS keyword search and goSearch's fallback path.
		if len(vec) != s.dim {
			skippedDim++
			continue
		}
		pending = append(pending, pendingVec{id: id, vec: vec})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("memory.migrateEmbeddingsToVec: scan: %w", err)
	}
	rows.Close() // release the single connection before the write

	if skippedDim > 0 {
		logx.Printf("MEMORY: vec migration skipped %d row(s) with a stale embedding dimension", skippedDim)
	}

	if len(pending) == 0 {
		return nil
	}

	return s.db.Write(ctx, func(tx *sql.Tx) error {
		for _, p := range pending {
			jsonVec, err := json.Marshal(p.vec)
			if err != nil {
				logx.Printf("MEMORY: vec migration: marshal row %d: %v", p.id, err)
				continue
			}
			// vec0 ignores INSERT OR IGNORE, so delete any stale row first to
			// stay idempotent even if one slipped past the NOT IN filter.
			if _, err := tx.Exec("DELETE FROM vec_memories WHERE rowid = ?", p.id); err != nil {
				logx.Printf("MEMORY: vec migration: delete stale row %d: %v", p.id, err)
				continue
			}
			if _, err := tx.Exec(
				"INSERT INTO vec_memories(rowid, embedding) VALUES (?, ?)",
				p.id, string(jsonVec),
			); err != nil {
				// A single row failing to insert must not roll back every
				// other row already staged in this transaction.
				logx.Printf("MEMORY: vec migration: insert row %d: %v", p.id, err)
				continue
			}
		}
		return nil
	})
}

const chunkMaxTokens = 300
const chunkOverlapTokens = 50

// duplicateInteractionSimilarity is the cosine-similarity floor above which
// a new conversation turn is treated as a near-repeat of an existing one
// (e.g. another "selam" exchange) and skipped instead of inserted, so
// trivial repeated turns don't pile up and crowd out genuinely distinct
// memories during retrieval. Chosen empirically; revisit if real usage shows
// it merging turns that are actually distinct, or failing to catch repeats.
const duplicateInteractionSimilarity = 0.92

// capForEmbedding returns text truncated to a single safe-sized piece for
// one s.embed call, reusing splitLongWord's conservative byte-per-token
// margin (see BUG-H3). For callers that need one bounded string rather than
// chunkText's full multi-chunk split: saveChunk's embedText still has the
// full assistant reply appended unconditionally with no size bound of its
// own (chunkText only ever chunks the user's side), and RetrieveContext
// embeds the raw query/expanded-query/compound-segment text directly with
// no chunking at all — both were left able to overflow the embedding
// server's batch-size limit even after chunkText itself was fixed.
func capForEmbedding(text string, maxTokens int) string {
	return splitLongWord(text, maxTokens)[0]
}

func (s *Store) SaveInteraction(ctx context.Context, userMsg, assistantMsg string) error {
	chunks := chunkText(userMsg, chunkMaxTokens, chunkOverlapTokens)
	parentUUID := fmt.Sprintf("mem_%d", time.Now().UnixNano())
	totalChunks := len(chunks)

	for i, chunk := range chunks {
		if err := s.saveChunk(ctx, chunk, assistantMsg, parentUUID, i, totalChunks); err != nil {
			return fmt.Errorf("memory.SaveInteraction chunk[%d]: %w", i, err)
		}
	}
	return nil
}

func (s *Store) saveChunk(ctx context.Context, userChunk, assistantMsg, parentUUID string, chunkIndex, totalChunks int) error {
	embedStart := time.Now()
	embedText := userChunk
	if assistantMsg != "" {
		embedText = userChunk + "\n" + assistantMsg
	}
	embedding, err := s.embed(ctx, capForEmbedding(embedText, chunkMaxTokens))
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	if len(embedding) != s.dim {
		return fmt.Errorf("embedding dimension %d != expected %d", len(embedding), s.dim)
	}
	embedDur := time.Since(embedStart)

	// Only single-chunk saves are eligible for dedup. A long message split
	// into multiple chunks (see chunkText) deliberately produces several
	// rows sharing one parentUUID, often with heavy word overlap between
	// consecutive chunks by design (chunkOverlapTokens) — treating those as
	// duplicates of each other would silently drop pieces of a single long
	// message instead of catching a genuinely repeated short turn.
	if totalChunks == 1 {
		if dupUUID, err := s.findDuplicateInteraction(ctx, embedding); err != nil {
			logx.Printf("memory: duplicate check failed, saving anyway: %v", err)
		} else if dupUUID != "" {
			logx.Printf("MEMORY SAVE SKIPPED: near-duplicate of %s (similarity >= %.2f)", dupUUID, duplicateInteractionSimilarity)
			return nil
		}
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	uuid := parentUUID
	if totalChunks > 1 {
		uuid = fmt.Sprintf("%s_%d", parentUUID, chunkIndex)
	}

	storedAssist := assistantMsg
	content := fmt.Sprintf("[%s] User: %s", timestamp, userChunk)
	if storedAssist != "" {
		content += "\nAssistant: " + storedAssist
	}

	writeStart := time.Now()
	err = s.db.Write(ctx, func(tx *sql.Tx) error {
		embedBlob := floatsToBlob(embedding)

		res, err := tx.Exec(
			`INSERT INTO memories(uuid, role, content, timestamp, user_msg, assist_msg, embedding,
			                      chunk_index, parent_uuid, total_chunks,
			                      session_id, importance, tags, source, retrieve_count)
             VALUES (?, 'user', ?, ?, ?, ?, ?, ?, ?, ?, '', 3, '', 'conversation', 0)`,
			uuid, content, timestamp, userChunk, storedAssist, embedBlob, chunkIndex, parentUUID, totalChunks,
		)
		if err != nil {
			return fmt.Errorf("insert memory: %w", err)
		}

		rowID, err := res.LastInsertId()
		if err != nil {
			return err
		}

		if s.useVec {
			vecJSON, _ := json.Marshal(embedding)
			if _, err := tx.Exec(
				"INSERT INTO vec_memories(rowid, embedding) VALUES (?, ?)",
				rowID, string(vecJSON),
			); err != nil {
				return fmt.Errorf("insert vector: %w", err)
			}
		}

		if s.useFTS {
			if _, err := tx.Exec(
				"INSERT INTO memories_fts(rowid, content, user_msg, assist_msg) VALUES (?, ?, ?, ?)",
				rowID, content, userChunk, storedAssist,
			); err != nil {
				return fmt.Errorf("insert fts: %w", err)
			}
		}

		return nil
	})
	writeDur := time.Since(writeStart)

	logx.Printf("LATENCY memory.save total_ms=%d embed_ms=%d write_ms=%d dim=%d vec=%v fts=%v chunk=%d/%d",
		(embedDur + writeDur).Milliseconds(),
		embedDur.Milliseconds(),
		writeDur.Milliseconds(),
		s.dim,
		s.useVec,
		s.useFTS,
		chunkIndex+1,
		totalChunks,
	)

	return err
}

// findDuplicateInteraction looks for an existing conversation-sourced memory
// whose embedding is a near-duplicate of embedding (see
// duplicateInteractionSimilarity), returning its UUID if found. Pinned/
// explicit facts (source == "explicit", written by SaveExplicit) are never
// treated as a duplicate target here — SaveExplicit has its own insert path
// entirely and is never skipped or merged by this check, only ordinary chat
// turns saved via SaveInteraction are.
func (s *Store) findDuplicateInteraction(ctx context.Context, embedding []float32) (string, error) {
	searchFn := s.goSearch
	if s.useVec {
		searchFn = s.vecSearch
	}
	candidates, err := searchFn(ctx, embedding, 5, duplicateInteractionSimilarity)
	if err != nil {
		return "", err
	}
	for _, c := range candidates {
		if c.Source == "conversation" {
			return c.ID, nil
		}
	}
	return "", nil
}

// escapeFTSQuery turns a raw natural-language query into an FTS5 MATCH
// expression that finds a row containing ANY of its words, ranked by bm25
// (rarer words score higher, so common filler words don't dominate).
//
// Joining with plain spaces would use FTS5's default AND operator, requiring
// every single word — including "ve"/"biliyor"/"musun" filler — to appear in
// the same row. A multi-topic recall question ("adımı ve doğum günümü ve
// favori rengimi biliyor musun") then matches nothing at all, because no
// memory row contains that literal combination of words, and the FTS
// supplement silently contributes zero candidates to the hybrid RRF merge.
// OR makes each word an independent candidate match instead, which is what a
// keyword-recall pass over a multi-topic question needs.
func escapeFTSQuery(q string) string {
	words := strings.Fields(q)
	if len(words) == 0 {
		return q
	}
	parts := make([]string, len(words))
	for i, w := range words {
		w = strings.ReplaceAll(w, `"`, `""`)
		parts[i] = `"` + w + `"`
	}
	return strings.Join(parts, " OR ")
}

func (s *Store) ftsSearch(ctx context.Context, query string, topK int) ([]MemoryResult, error) {
	// m.pending_deletion = 0 excludes a consolidation merge's two originals
	// — see vecSearch's identical fix for the full explanation (BUG-H5).
	// Unlike vecSearch's sqlite-vec KNN query, FTS5's MATCH has no fragile
	// virtual-table shape to preserve, so this filters directly in SQL.
	rows, err := s.db.QueryContext(ctx, `
        SELECT m.uuid, m.content, m.timestamp, m.user_msg, m.assist_msg,
               memories_fts.rank,
               m.importance, m.source, m.tags, m.session_id, m.retrieve_count
        FROM memories_fts
        JOIN memories m ON m.id = memories_fts.rowid
        WHERE memories_fts MATCH ? AND m.pending_deletion = 0
        ORDER BY memories_fts.rank, m.uuid
        LIMIT ?
    `, escapeFTSQuery(query), topK)
	if err != nil {
		return nil, fmt.Errorf("memory.ftsSearch: %w", err)
	}
	defer rows.Close()

	var results []MemoryResult
	for rows.Next() {
		var uuid, content, timestamp, userMsg, assistMsg string
		var rank float64
		var importance, retrieveCount int
		var source, tags, sessionID string
		if err := rows.Scan(&uuid, &content, &timestamp, &userMsg, &assistMsg, &rank,
			&importance, &source, &tags, &sessionID, &retrieveCount); err != nil {
			continue
		}
		results = append(results, MemoryResult{
			ID:            uuid,
			Content:       content,
			Timestamp:     timestamp,
			UserMsg:       userMsg,
			AssistMsg:     assistMsg,
			MatchType:     "fts",
			Importance:    importance,
			Source:        source,
			Tags:          tags,
			SessionID:     sessionID,
			RetrieveCount: retrieveCount,
		})
	}
	return results, rows.Err()
}

func reciprocalRankFusion(vecResults, ftsResults []MemoryResult, topK int) []MemoryResult {
	const k = 60.0
	scores := make(map[string]float64)
	best := make(map[string]MemoryResult)

	applyRank := func(results []MemoryResult, matchType string) {
		for rank, r := range results {
			scores[r.ID] += 1.0 / (k + float64(rank+1))
			if _, seen := best[r.ID]; !seen {
				r.MatchType = matchType
				best[r.ID] = r
			} else if matchType != best[r.ID].MatchType {
				m := best[r.ID]
				m.MatchType = "hybrid"
				best[r.ID] = m
			}
		}
	}

	applyRank(vecResults, "vector")
	applyRank(ftsResults, "fts")

	return topScoredByRRF(scores, best, topK)
}

// mergeVectorCandidates RRF-fuses any number of vector-search result lists
// (the whole-query search, the optional expand-query variant, and one per
// splitCompoundQuery segment) into a single ranked candidate pool in one
// pass. This deliberately replaces folding the lists together pairwise via
// repeated reciprocalRankFusion calls: each intermediate
// reciprocalRankFusion call truncates its own output back down to topK, so
// a candidate absent from the whole-query search AND an earlier segment
// (e.g. a fact sharing zero words with either) could already be evicted
// before a *later* segment — the one it actually matches well — ever got a
// chance to add it back in.
//
// Each candidate's score is its BEST (max) rank contribution across every
// list, not the sum — deliberately different from reciprocalRankFusion's
// own vec+fts merge, where summing is the correct way to reward a hybrid
// (both-vector-and-keyword) match. Here, summing across segments actively
// works against splitCompoundQuery's whole stated purpose ("a fact only
// needs to beat noise on its own specific topic, not the whole sentence,"
// see that function's doc comment): a candidate that ranks decently in
// *multiple* segments (routine conversational noise sharing common words
// with several segments at once) would out-accumulate a candidate that
// ranks #1 in exactly the one segment it's actually relevant to. Confirmed
// directly: a compound-question fact ranked #1 in its own topic segment's
// search still lost to routine noise under sum-based fusion, because the
// noise had summed contributions from two segments the fact was never
// competitive in at all — switching to max-per-candidate fixed it (see
// TestRecall_CasualFactNotCrowdedOutByRoutineNoise_GoFallback).
func mergeVectorCandidates(topK int, lists ...[]MemoryResult) []MemoryResult {
	const k = 60.0
	scores := make(map[string]float64)
	best := make(map[string]MemoryResult)

	for _, results := range lists {
		for rank, r := range results {
			if s := 1.0 / (k + float64(rank+1)); s > scores[r.ID] {
				scores[r.ID] = s
			}
			if _, seen := best[r.ID]; !seen {
				r.MatchType = "vector"
				best[r.ID] = r
			}
		}
	}

	return topScoredByRRF(scores, best, topK)
}

// topScoredByRRF sorts scores descending and returns the top topK entries
// from best, shared by reciprocalRankFusion and mergeVectorCandidates.
//
// Tiebreaks by ID when scores are exactly equal (common: two candidates
// landing at the identical rank position in the same set of merged lists
// get an identical summed score) — without this, sort.Slice's relative
// order for tied entries falls back to their position in the slice built
// from iterating the scores map, and Go's map iteration order is
// deliberately randomized per process, not per call. The same tied pair
// could then sort either way from one run to the next, silently flipping
// whether a given memory survives a topK cutoff at the tie boundary — a
// real, intermittent CI failure (TestRecall_CasualFactNotCrowdedOutByRoutineNoise),
// not a data race. The tiebreaker makes the result fully deterministic for
// a given input regardless of map iteration order.
func topScoredByRRF(scores map[string]float64, best map[string]MemoryResult, topK int) []MemoryResult {
	type scored struct {
		id    string
		score float64
	}
	ranked := make([]scored, 0, len(scores))
	for id, score := range scores {
		ranked = append(ranked, scored{id, score})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].id < ranked[j].id
	})

	if topK > len(ranked) {
		topK = len(ranked)
	}
	out := make([]MemoryResult, 0, topK)
	for _, s := range ranked[:topK] {
		r := best[s.id]
		r.Similarity = float32(s.score)
		out = append(out, r)
	}
	return out
}

// expandQuery generates a secondary search query for multi-query retrieval.
// For long queries (>7 words), it extracts the first 5 words as a topic anchor.
// Returns "" when no useful expansion is possible.
func expandQuery(q string) string {
	words := strings.Fields(q)
	if len(words) <= 7 {
		return ""
	}
	// Take only the first 5 content words — captures topic without trailing question noise
	return strings.Join(words[:5], " ")
}

// splitCompoundQuery breaks a multi-topic question into individual topic
// segments, splitting on conjunctions and clause punctuation ("ve"/"ile"/
// "and"/","). A single embedding vector for a whole compound sentence like
// "adımı ve doğum günümü ve en sevdiğim rengimi biliyor musun" blends three
// unrelated topics together — against a memory store that has accumulated
// many superficially-similar routine conversational turns (every chat turn
// is saved by default, see internal/app/memory.go's saveMemoryAsync), a
// short, specific fact about only ONE of those topics can rank below far
// more numerous "reasonably close to the whole blended question" noise
// entries and never even make the candidate pool. Splitting gives each
// topic ("adımı", "doğum günümü", "en sevdiğim rengimi") its own query, so a
// fact only needs to beat noise on its own specific topic, not the whole
// sentence. Returns nil when the query doesn't look compound (0 or 1
// segment after splitting) — callers should treat that as "no decomposition
// available" and fall back to the single whole-query search only.
func splitCompoundQuery(q string) []string {
	replacer := strings.NewReplacer(",", " , ", "?", " ", ".", " ", "!", " ")
	normalized := replacer.Replace(q)

	isSeparator := func(w string) bool {
		switch strings.ToLower(w) {
		case ",", "ve", "ile", "and", "&":
			return true
		}
		return false
	}

	var segments []string
	var current []string
	flush := func() {
		if len(current) > 0 {
			segments = append(segments, strings.Join(current, " "))
			current = nil
		}
	}
	for w := range strings.FieldsSeq(normalized) {
		if isSeparator(w) {
			flush()
			continue
		}
		current = append(current, w)
	}
	flush()

	if len(segments) <= 1 {
		return nil
	}
	return segments
}

func (s *Store) RetrieveContext(ctx context.Context, query string, topK int, minSimilarity float32) ([]MemoryResult, error) {
	start := time.Now()

	embedStart := time.Now()
	queryEmbedding, err := s.embed(ctx, capForEmbedding(query, chunkMaxTokens))
	if err != nil {
		return nil, fmt.Errorf("memory.RetrieveContext: embed: %w", err)
	}
	embedDur := time.Since(embedStart)

	if len(queryEmbedding) != s.dim {
		return nil, fmt.Errorf("memory.RetrieveContext: embedding dimension %d != expected %d", len(queryEmbedding), s.dim)
	}

	if topK <= 0 {
		return nil, nil
	}

	candidateK := min(max(topK*5, 20), 100)

	vecSearchFn := s.goSearch
	if s.useVec {
		vecSearchFn = s.vecSearch
	}

	wholeQueryResults, err := vecSearchFn(ctx, queryEmbedding, candidateK, minSimilarity)
	if err != nil {
		return nil, fmt.Errorf("memory.RetrieveContext: vector search: %w", err)
	}

	// Collect every vector-origin candidate list — the whole-query search,
	// the optional expand-query variant, and one per splitCompoundQuery
	// segment — and fuse them in a single pass at the end (mergeVectorCandidates)
	// instead of folding them together pairwise via reciprocalRankFusion.
	// Sequential pairwise fold-and-truncate used to genuinely lose
	// candidates: each intermediate reciprocalRankFusion call truncates its
	// output back down to candidateK, so a candidate excluded from the
	// whole-query search AND an earlier segment (a fact sharing zero words
	// with either) could already be evicted before a *later* segment —
	// the one it actually matches well — ever got a chance to add it back
	// in. Confirmed directly: a compound-question fact ranked #1 in its own
	// topic segment's search still lost to routine noise in the final
	// result, because the noise had already accumulated RRF score across
	// two earlier merge rounds the fact was never part of (see
	// TestRecall_CasualFactNotCrowdedOutByRoutineNoise_GoFallback).
	vectorLists := [][]MemoryResult{wholeQueryResults}

	// Multi-query: if query is long, do a second search with a topic-focused variant.
	if expanded := expandQuery(query); expanded != "" {
		if expEmb, expErr := s.embed(ctx, capForEmbedding(expanded, chunkMaxTokens)); expErr == nil && len(expEmb) == s.dim {
			if expResults, _ := vecSearchFn(ctx, expEmb, candidateK/2, minSimilarity); len(expResults) > 0 {
				vectorLists = append(vectorLists, expResults)
			}
		}
	}

	// Compound-question decomposition: each topic segment gets its own
	// full-budget vector search, so a fact about one topic doesn't have to
	// survive being blended into a single averaged vector for the whole
	// sentence — see splitCompoundQuery's doc comment.
	for _, segment := range splitCompoundQuery(query) {
		segEmb, segErr := s.embed(ctx, capForEmbedding(segment, chunkMaxTokens))
		if segErr != nil || len(segEmb) != s.dim {
			continue
		}
		if segResults, _ := vecSearchFn(ctx, segEmb, candidateK, minSimilarity); len(segResults) > 0 {
			vectorLists = append(vectorLists, segResults)
		}
	}

	vecMemories := mergeVectorCandidates(candidateK, vectorLists...)

	var memories []MemoryResult
	if s.useFTS {
		ftsMemories, ftsErr := s.ftsSearch(ctx, query, candidateK)
		if ftsErr != nil {
			logx.Printf("MEMORY: ftsSearch: %v", ftsErr)
		} else if len(ftsMemories) > 0 {
			memories = reciprocalRankFusion(vecMemories, ftsMemories, topK)
			// RRF scores range ~0.008–0.016 (k=60); filter low-confidence matches.
			const minRRFScore = float32(0.008)
			filtered := memories[:0]
			for _, m := range memories {
				if m.Similarity >= minRRFScore {
					filtered = append(filtered, m)
				}
			}
			memories = filtered
		}
	}
	if len(memories) == 0 {
		memories = vecMemories
		if topK < len(memories) {
			memories = memories[:topK]
		}
	}

	// Boost similarity scores based on importance (1–5).
	// importance=1 → ×0.90, importance=3 → ×1.10, importance=5 → ×1.30
	// Re-sort after boost since relative order may change.
	for i := range memories {
		imp := memories[i].Importance
		if imp < 1 {
			imp = 1
		} else if imp > 5 {
			imp = 5
		}
		memories[i].Similarity *= float32(0.8 + float64(imp)*0.1)
	}
	// Same ID tiebreaker as reciprocalRankFusion, and for the same reason —
	// this slice can carry ties (e.g. two candidates with the exact same
	// RRF score and the exact same importance) whose relative order would
	// otherwise depend on whatever order they happened to arrive in from
	// upstream map-iteration-order-sensitive merges.
	sort.Slice(memories, func(i, j int) bool {
		if memories[i].Similarity != memories[j].Similarity {
			return memories[i].Similarity > memories[j].Similarity
		}
		return memories[i].ID < memories[j].ID
	})

	if len(memories) > 0 {
		ids := make([]string, len(memories))
		for i, m := range memories {
			ids[i] = m.ID
		}
		logx.GoRecover("memory.Store.incrementRetrieveCounts", func() { s.incrementRetrieveCounts(ids) })
	}

	logx.Printf("LATENCY memory.retrieve total_ms=%d embed_ms=%d top_k=%d returned=%d vec=%v fts=%v",
		time.Since(start).Milliseconds(),
		embedDur.Milliseconds(),
		topK,
		len(memories),
		s.useVec,
		s.useFTS,
	)

	return memories, nil
}

func (s *Store) vecSearch(ctx context.Context, queryEmbedding []float32, topK int, minSimilarity float32) ([]MemoryResult, error) {
	vecJSON, err := json.Marshal(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("memory.vecSearch: marshal: %w", err)
	}

	// Deliberately no secondary sort key here (unlike ftsSearch's `ORDER BY
	// memories_fts.rank, m.uuid` a few lines up): `WHERE embedding MATCH ?
	// AND k = ? ORDER BY distance` is the exact shape sqlite-vec's virtual
	// table pattern-matches to run its optimized KNN scan — confirmed
	// empirically that appending `, m.uuid` here breaks that recognition
	// and silently changes which rows come back (TestRecall_CasualFactNotCrowdedOutByRoutineNoise
	// went from consistently passing to consistently failing locally with
	// vec0 available). Any remaining tie-order non-determinism from this
	// query is handled downstream instead, in reciprocalRankFusion's own ID
	// tiebreaker.
	// pending_deletion is selected (not filtered in the WHERE clause — see
	// the comment above about not touching this query's shape) and checked
	// in the Go loop below, same way minSimilarity already is: consolidation
	// (saveMergedAs) marks a merged pair's two originals pending_deletion=1
	// but leaves their rows/vectors fully intact until PurgePendingDeletions
	// eventually deletes them (up to ~187 days later) — without this check
	// those originals kept resurfacing as near-duplicates of the very
	// merged row that replaced them (BUG-H5).
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.uuid, m.content, m.timestamp, m.user_msg, m.assist_msg, v.distance,
		       m.importance, m.source, m.tags, m.session_id, m.retrieve_count, m.pending_deletion
		FROM vec_memories v
		JOIN memories m ON m.id = v.rowid
		WHERE v.embedding MATCH ?
		  AND k = ?
		ORDER BY v.distance
	`, string(vecJSON), topK)
	if err != nil {
		return nil, fmt.Errorf("memory.vecSearch: %w", err)
	}
	defer rows.Close()

	var results []MemoryResult
	for rows.Next() {
		var (
			uuid, content, timestamp  string
			userMsg, assistMsg        string
			distance                  float64
			importance, retrieveCount int
			source, tags, sessionID   string
			pendingDeletion           int
		)
		if err := rows.Scan(&uuid, &content, &timestamp, &userMsg, &assistMsg, &distance,
			&importance, &source, &tags, &sessionID, &retrieveCount, &pendingDeletion); err != nil {
			return nil, fmt.Errorf("memory.vecSearch: scan: %w", err)
		}
		if pendingDeletion != 0 {
			continue
		}

		sim := float32(1.0 - math.Max(0, math.Min(distance, 2))/2)
		if sim < minSimilarity {
			continue
		}

		results = append(results, MemoryResult{
			ID:            uuid,
			Content:       content,
			Similarity:    sim,
			Timestamp:     timestamp,
			UserMsg:       userMsg,
			AssistMsg:     assistMsg,
			Importance:    importance,
			Source:        source,
			Tags:          tags,
			SessionID:     sessionID,
			RetrieveCount: retrieveCount,
		})
	}

	return results, rows.Err()
}

func (s *Store) goSearch(ctx context.Context, queryEmbedding []float32, topK int, minSimilarity float32) ([]MemoryResult, error) {
	// pending_deletion = 0 excludes a consolidation merge's two originals
	// (saveMergedAs marks them pending_deletion=1 but leaves the rows
	// intact until PurgePendingDeletions eventually deletes them) — see
	// vecSearch's identical fix for the full explanation (BUG-H5). This is
	// the no-vec0 Go fallback path (unlike vecSearch, no fragile virtual
	// table query shape to worry about), so the filter goes directly in SQL.
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, uuid, content, timestamp, user_msg, assist_msg, embedding,
		        importance, source, tags, session_id, retrieve_count
		 FROM memories WHERE embedding IS NOT NULL AND pending_deletion = 0`)
	if err != nil {
		return nil, fmt.Errorf("memory.goSearch: %w", err)
	}
	defer rows.Close()

	type scored struct {
		uuid          string
		content       string
		ts            string
		userMsg       string
		assistMsg     string
		sim           float32
		importance    int
		source        string
		tags          string
		sessionID     string
		retrieveCount int
	}

	queryNorm := vectorNorm(queryEmbedding)

	var all []scored
	for rows.Next() {
		var id int64
		var uuid, content, timestamp, userMsg, assistMsg string
		var blob []byte
		var importance, retrieveCount int
		var source, tags, sessionID string
		if err := rows.Scan(&id, &uuid, &content, &timestamp, &userMsg, &assistMsg, &blob,
			&importance, &source, &tags, &sessionID, &retrieveCount); err != nil {
			continue
		}

		vec := blobToFloats(blob)
		if len(vec) != s.dim {
			continue
		}

		sim := cosineSimilarityFast(queryEmbedding, queryNorm, vec, vectorNorm(vec))
		if sim < minSimilarity {
			continue
		}

		all = append(all, scored{
			uuid:          uuid,
			content:       content,
			ts:            timestamp,
			userMsg:       userMsg,
			assistMsg:     assistMsg,
			sim:           sim,
			importance:    importance,
			source:        source,
			tags:          tags,
			sessionID:     sessionID,
			retrieveCount: retrieveCount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// ID tiebreak for the same reason as reciprocalRankFusion's (see its
	// comment): a plain `sim >` comparator leaves the relative order of
	// exactly-tied entries dependent on their pre-sort order in `all`,
	// which in turn depends on SQLite's row order for this ORDER-BY-less
	// scan — not something this function should rely on staying stable.
	sort.Slice(all, func(i, j int) bool {
		if all[i].sim != all[j].sim {
			return all[i].sim > all[j].sim
		}
		return all[i].uuid < all[j].uuid
	})

	if topK > len(all) {
		topK = len(all)
	}

	results := make([]MemoryResult, 0, topK)
	for _, s := range all[:topK] {
		results = append(results, MemoryResult{
			ID:            s.uuid,
			Content:       s.content,
			Similarity:    s.sim,
			Timestamp:     s.ts,
			UserMsg:       s.userMsg,
			AssistMsg:     s.assistMsg,
			Importance:    s.importance,
			Source:        s.source,
			Tags:          s.tags,
			SessionID:     s.sessionID,
			RetrieveCount: s.retrieveCount,
		})
	}

	return results, nil
}

// pinnedFactsLimit bounds how many explicit facts GetPinnedFacts returns.
// Pinned facts are injected into every system prompt in full, unconditionally
// — unlike RAG results they are never trimmed by relevance — so this cap is
// what keeps prompt size bounded as the set grows over weeks of use.
//
// Raised from 50 to 75 (2026-07-21): the general consolidation pass
// (FindMergeCandidates) deliberately excludes source='explicit' rows (see its
// doc comment — merging two pinned facts together used to silently un-pin the
// result), so contrary to what this comment previously claimed, nothing was
// actually shrinking the pinned set over time; the cap was a bare
// most-recent-75-win truncation with no size-limiting mechanism behind it at
// all. FindPinnedMergeCandidates/runPinnedConsolidation (below) now cover
// that gap directly, so the higher cap has some real backing — but the two
// changes both help independently: the higher number buys more headroom even
// with duplicates, and dedup slows how fast that headroom fills.
//
// BuildSystemPrompt (internal/identity) separately caps the *entire* memory
// block (pinned + RAG results combined) at ~16K tokens regardless of this
// constant, so raising it cannot blow past the model's actual context window
// on its own — worst case, more of that existing budget goes to pinned facts
// and less to RAG results, since pinned facts are prepended first.
const pinnedFactsLimit = 75

// GetPinnedFacts returns explicit (importance=5, source="explicit") memories,
// most recent first. Callers are expected to inject the full result into
// every system prompt unconditionally, bypassing RetrieveContext's RAG
// ranking entirely — a core personal fact must not depend on winning a
// similarity contest against routine conversational noise (see AGENTS.md's
// Known Pitfalls, Memory / Vector Store section, 2026-07-15 entries, for the
// bug class this exists to sidestep).
func (s *Store) GetPinnedFacts(ctx context.Context) ([]MemoryResult, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT uuid, content, timestamp, user_msg, assist_msg,
		       importance, source, tags, session_id, retrieve_count
		FROM memories
		WHERE source = 'explicit' AND importance = 5 AND pending_deletion = 0
		ORDER BY timestamp DESC
		LIMIT ?
	`, pinnedFactsLimit)
	if err != nil {
		return nil, fmt.Errorf("memory.GetPinnedFacts: %w", err)
	}
	defer rows.Close()

	var out []MemoryResult
	for rows.Next() {
		var r MemoryResult
		if err := rows.Scan(&r.ID, &r.Content, &r.Timestamp, &r.UserMsg, &r.AssistMsg,
			&r.Importance, &r.Source, &r.Tags, &r.SessionID, &r.RetrieveCount); err != nil {
			continue
		}
		r.MatchType = "pinned"
		r.Similarity = 1.0
		out = append(out, r)
	}
	return out, rows.Err()
}

// incrementRetrieveCounts increments retrieve_count for the given memory UUIDs.
// Called asynchronously after a successful retrieval to track usage frequency.
func (s *Store) incrementRetrieveCounts(ids []string) {
	if len(ids) == 0 {
		return
	}
	s.mu.RLock()
	closed := s.closed
	db := s.db
	s.mu.RUnlock()
	if closed || db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	if err := db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(
			"UPDATE memories SET retrieve_count = retrieve_count + 1 WHERE uuid IN ("+placeholders+")",
			args...,
		)
		return err
	}); err != nil {
		logx.Printf("memory: update retrieve_count: %v", err)
	}
}

func (s *Store) DebugSearch(ctx context.Context, query string, topK int) []MemoryResult {
	results, err := s.RetrieveContext(ctx, query, topK, 0)
	if err != nil {
		logx.Printf("DEBUG SEARCH ERROR: %v", err)
		return nil
	}
	return results
}

func (s *Store) Count() int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memories").Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

func (s *Store) ClearAll() error {
	return s.db.Write(context.Background(), func(tx *sql.Tx) error {
		if s.useVec {
			if _, err := tx.Exec("DELETE FROM vec_memories"); err != nil {
				return err
			}
		}
		if s.useFTS {
			if _, err := tx.Exec("DELETE FROM memories_fts"); err != nil {
				return err
			}
		}
		if _, err := tx.Exec("DELETE FROM memories"); err != nil {
			return err
		}
		return nil
	})
}

func (s *Store) ListGobFiles() []MemoryFileInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx,
		"SELECT uuid, timestamp, LENGTH(user_msg) + LENGTH(assist_msg) FROM memories ORDER BY id DESC LIMIT 100",
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var files []MemoryFileInfo
	for rows.Next() {
		var uuid, ts string
		var sizeBytes int64
		if err := rows.Scan(&uuid, &ts, &sizeBytes); err != nil {
			continue
		}
		files = append(files, MemoryFileInfo{
			Path:     uuid,
			Name:     uuid,
			SizeKB:   (sizeBytes + 1023) / 1024,
			Modified: ts,
		})
	}
	return files
}

func (s *Store) DeleteGobFile(relPath string) error {
	return s.db.Write(context.Background(), func(tx *sql.Tx) error {
		var id int64
		err := tx.QueryRow("SELECT id FROM memories WHERE uuid = ?", relPath).Scan(&id)
		if err != nil {
			return fmt.Errorf("memory not found: %s", relPath)
		}

		if s.useVec {
			if _, err := tx.Exec("DELETE FROM vec_memories WHERE rowid = ?", id); err != nil {
				return err
			}
		}
		if s.useFTS {
			if _, err := tx.Exec("DELETE FROM memories_fts WHERE rowid = ?", id); err != nil {
				return err
			}
		}
		if _, err := tx.Exec("DELETE FROM memories WHERE id = ?", id); err != nil {
			return err
		}
		return nil
	})
}

func (s *Store) Stats() models.MemoryStats {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var stats models.MemoryStats
	stats.Dimension = s.dim

	// Single-pass count query — one table scan instead of four round-trips.
	var total, explicit, thisWeek, pending int
	scanErr := s.db.QueryRowContext(ctx, `
		SELECT
			SUM(CASE WHEN pending_deletion = 0 THEN 1 ELSE 0 END),
			SUM(CASE WHEN pending_deletion = 0 AND source = 'explicit' THEN 1 ELSE 0 END),
			SUM(CASE WHEN pending_deletion = 0 AND julianday('now') - julianday(timestamp) <= 7 THEN 1 ELSE 0 END),
			SUM(CASE WHEN pending_deletion = 1 THEN 1 ELSE 0 END)
		FROM memories
	`).Scan(&total, &explicit, &thisWeek, &pending)
	if scanErr != nil {
		logx.Printf("MEMORY: stats count query: %v", scanErr)
	}
	stats.Count = total
	stats.ExplicitCount = explicit
	stats.AddedThisWeek = thisWeek
	stats.PendingDeletion = pending
	stats.VecCount = stats.Count

	// Top 5 most-retrieved memories for the analytics panel
	rows, err := s.db.QueryContext(ctx, `
		SELECT uuid, content, timestamp, importance, source, retrieve_count
		FROM memories
		WHERE pending_deletion = 0 AND retrieve_count > 0
		ORDER BY retrieve_count DESC
		LIMIT 5
	`)
	if err != nil {
		logx.Printf("MEMORY: stats top-retrieved query: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var m models.MemoryResult
			if rErr := rows.Scan(&m.ID, &m.Content, &m.Timestamp, &m.Importance, &m.Source, &m.RetrieveCount); rErr != nil {
				logx.Printf("MEMORY: stats top-retrieved scan: %v", rErr)
			} else {
				stats.TopRetrieved = append(stats.TopRetrieved, m)
			}
		}
		if rErr := rows.Err(); rErr != nil {
			logx.Printf("MEMORY: stats top-retrieved iteration: %v", rErr)
		}
	}

	return stats
}

// FilteredSearch returns memories matching a query with optional date and tag filters.
// since: earliest timestamp (zero = no filter). tag: substring match on tags field (empty = no filter).
//
// Strategy: semantic retrieval first, post-filter by date/tag. If the semantic
// pass returns fewer than topK results after filtering (because top-ranked memories
// are all outside the filter window), fall back to a direct SQL-ordered query that
// respects the filters — ensuring recent/tagged memories are not silently dropped.
func (s *Store) FilteredSearch(ctx context.Context, query string, topK int, minSimilarity float32, since time.Time, tag string) ([]MemoryResult, error) {
	results, err := s.RetrieveContext(ctx, query, topK*3, minSimilarity)
	if err != nil {
		return nil, err
	}
	if since.IsZero() && tag == "" {
		if topK < len(results) {
			return results[:topK], nil
		}
		return results, nil
	}

	filtered := make([]MemoryResult, 0, topK)
	for _, r := range results {
		if !since.IsZero() {
			t, parseErr := time.Parse(time.RFC3339, r.Timestamp)
			if parseErr != nil || t.Before(since) {
				continue
			}
		}
		if tag != "" && !strings.Contains(strings.ToLower(r.Tags), strings.ToLower(tag)) {
			continue
		}
		filtered = append(filtered, r)
		if len(filtered) >= topK {
			break
		}
	}

	// Fallback: semantic results were all outside the filter window.
	// Query DB directly so that recent/tagged memories are not silently dropped.
	if len(filtered) < topK {
		sqlResults, sqlErr := s.sqlFilteredFallback(ctx, topK-len(filtered), since, tag)
		if sqlErr != nil {
			logx.Printf("MEMORY: FilteredSearch fallback: %v", sqlErr)
		} else {
			// Append SQL results, deduplicating by UUID.
			seen := make(map[string]struct{}, len(filtered))
			for _, r := range filtered {
				seen[r.ID] = struct{}{}
			}
			for _, r := range sqlResults {
				if _, dup := seen[r.ID]; !dup {
					filtered = append(filtered, r)
				}
			}
		}
	}

	return filtered, nil
}

// RecentSince returns up to limit memories recorded at or after since, most
// recent first — a pure time-window fetch with no query/semantic ranking,
// for callers that want everything in a window (e.g. a periodic self-insight
// digest) rather than top-K by similarity to a search query. Reuses
// FilteredSearch's own SQL fallback path instead of duplicating the query.
func (s *Store) RecentSince(ctx context.Context, since time.Time, limit int) ([]MemoryResult, error) {
	return s.sqlFilteredFallback(ctx, limit, since, "")
}

// sqlFilteredFallback returns up to limit memories that satisfy the since/tag constraints,
// ordered by timestamp descending (most recent first). Used when semantic retrieval
// returns no results within the filter window.
func (s *Store) sqlFilteredFallback(ctx context.Context, limit int, since time.Time, tag string) ([]MemoryResult, error) {
	var args []any
	where := "pending_deletion = 0"
	if !since.IsZero() {
		where += " AND timestamp >= ?"
		args = append(args, since.UTC().Format(time.RFC3339))
	}
	if tag != "" {
		where += " AND LOWER(tags) LIKE ?"
		args = append(args, "%"+strings.ToLower(tag)+"%")
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx,
		"SELECT uuid, content, timestamp, user_msg, assist_msg, importance, source, tags, retrieve_count FROM memories WHERE "+where+" ORDER BY timestamp DESC LIMIT ?",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryResult
	for rows.Next() {
		var r MemoryResult
		if sErr := rows.Scan(&r.ID, &r.Content, &r.Timestamp, &r.UserMsg, &r.AssistMsg, &r.Importance, &r.Source, &r.Tags, &r.RetrieveCount); sErr == nil {
			r.MatchType = "filter"
			r.Similarity = 0.5
			out = append(out, r)
		}
	}
	return out, rows.Err()
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.stopCh != nil {
		close(s.stopCh)
	}
	// Deliberately not nulling s.db here: database.DB's methods (Write/
	// QueryContext/etc.) already handle the closed state gracefully,
	// returning an error instead of panicking. Nulling it previously left a
	// window where a background goroutine (e.g. the FTS/vec migration
	// goroutines started in initSchema, which can run for up to 60-120s)
	// could read s.db as nil if it raced with Close(), causing a genuine
	// nil-pointer panic — observed in practice via `go test -race`.
	return s.db.Close()
}

func formatMemoryAge(timestamp string) string {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < 24*time.Hour:
		return "today"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d weeks ago", int(d.Hours()/(24*7)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%d months ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%d years ago", int(d.Hours()/(24*365)))
	}
}

func importanceLabel(imp int) string {
	switch {
	case imp >= 5:
		return "pinned"
	case imp >= 4:
		return "high"
	case imp >= 3:
		return "normal"
	default:
		return "low"
	}
}

func FormatMemoriesForPrompt(memories []MemoryResult) string {
	if len(memories) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n--- RELEVANT MEMORIES ---\n")
	for i, m := range memories {
		age := formatMemoryAge(m.Timestamp)
		imp := importanceLabel(m.Importance)
		src := m.Source
		if src == "" {
			src = "conversation"
		}
		meta := fmt.Sprintf("relevance=%.0f%% | importance=%s | source=%s", m.Similarity*100, imp, src)
		if age != "" {
			meta += " | " + age
		}
		fmt.Fprintf(&sb, "[Memory %d | %s]\n%s\n\n", i+1, meta, m.Content)
	}
	sb.WriteString("--- END MEMORIES ---\n")
	return sb.String()
}

func FormatMemoriesUserOnly(memories []MemoryResult) string {
	if len(memories) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n--- RELEVANT MEMORIES ---\n")
	for i, m := range memories {
		age := formatMemoryAge(m.Timestamp)
		imp := importanceLabel(m.Importance)
		src := m.Source
		if src == "" {
			src = "conversation"
		}
		meta := fmt.Sprintf("relevance=%.0f%% | importance=%s | source=%s", m.Similarity*100, imp, src)
		if age != "" {
			meta += " | " + age
		}
		fmt.Fprintf(&sb, "[Memory %d | %s]\n%s\n\n", i+1, meta, stripAssistantReply(m.Content))
	}
	sb.WriteString("--- END MEMORIES ---\n")
	return sb.String()
}

func (s *Store) SaveExplicit(ctx context.Context, content, tags string) error {
	embedText := content
	embedding, err := s.embed(ctx, embedText)
	if err != nil {
		return fmt.Errorf("memory.SaveExplicit: embed: %w", err)
	}
	if len(embedding) != s.dim {
		return fmt.Errorf("memory.SaveExplicit: dimension mismatch: got %d want %d", len(embedding), s.dim)
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	uuid := fmt.Sprintf("explicit_%d", time.Now().UnixNano())
	return s.db.Write(ctx, func(tx *sql.Tx) error {
		embedBlob := floatsToBlob(embedding)
		res, err := tx.Exec(
			`INSERT INTO memories(uuid, role, content, timestamp, user_msg, assist_msg, embedding,
			                      chunk_index, parent_uuid, total_chunks,
			                      session_id, importance, tags, source, retrieve_count)
			 VALUES (?, 'user', ?, ?, ?, '', ?, 0, ?, 1, '', 5, ?, 'explicit', 0)`,
			uuid, content, timestamp, content, embedBlob, uuid, tags,
		)
		if err != nil {
			return fmt.Errorf("insert explicit memory: %w", err)
		}
		rowID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if s.useVec {
			vecJSON, _ := json.Marshal(embedding)
			if _, err := tx.Exec("INSERT INTO vec_memories(rowid, embedding) VALUES (?, ?)", rowID, string(vecJSON)); err != nil {
				return fmt.Errorf("insert vec: %w", err)
			}
		}
		if s.useFTS {
			if _, err := tx.Exec("INSERT INTO memories_fts(rowid, content, user_msg, assist_msg) VALUES (?, ?, ?, '')", rowID, content, content); err != nil {
				return fmt.Errorf("insert fts: %w", err)
			}
		}
		return nil
	})
}

func (s *Store) DeleteByContent(ctx context.Context, pattern string) (int, error) {
	type entry struct{ id int64 }
	rows, err := s.db.QueryContext(ctx,
		"SELECT id FROM memories WHERE content LIKE ? OR user_msg LIKE ?",
		"%"+pattern+"%", "%"+pattern+"%",
	)
	if err != nil {
		return 0, fmt.Errorf("memory.DeleteByContent: %w", err)
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if scanErr := rows.Scan(&e.id); scanErr == nil {
			entries = append(entries, e)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return 0, nil
	}
	deleted := 0
	writeErr := s.db.Write(ctx, func(tx *sql.Tx) error {
		for _, e := range entries {
			if _, err := tx.Exec("DELETE FROM memories WHERE id = ?", e.id); err != nil {
				return err
			}
			if s.useVec {
				if _, vecErr := tx.Exec("DELETE FROM vec_memories WHERE rowid = ?", e.id); vecErr != nil {
					logx.Printf("MEMORY: DeleteByContent vec cascade id=%d: %v", e.id, vecErr)
				}
			}
			if s.useFTS {
				if _, ftsErr := tx.Exec("DELETE FROM memories_fts WHERE rowid = ?", e.id); ftsErr != nil {
					logx.Printf("MEMORY: DeleteByContent fts cascade id=%d: %v", e.id, ftsErr)
				}
			}
			deleted++
		}
		return nil
	})
	return deleted, writeErr
}

func (s *Store) MarkStaleForDeletion(ctx context.Context) (int, error) {
	var marked int
	err := s.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.Exec(`
			UPDATE memories
			SET pending_deletion = 1
			WHERE pending_deletion = 0
			  AND source != 'explicit'
			  AND importance <= 2
			  AND retrieve_count = 0
			  AND julianday('now') - julianday(timestamp) > 180
		`)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		marked = int(n)
		return nil
	})
	return marked, err
}

func (s *Store) PurgePendingDeletions(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM memories
		WHERE pending_deletion = 1
		  AND julianday('now') - julianday(timestamp) > 187
	`)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if scanErr := rows.Scan(&id); scanErr == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	if len(ids) == 0 {
		return 0, nil
	}
	deleted := 0
	writeErr := s.db.Write(ctx, func(tx *sql.Tx) error {
		for _, id := range ids {
			if _, err := tx.Exec("DELETE FROM memories WHERE id = ?", id); err != nil {
				return err
			}
			if s.useVec {
				_, _ = tx.Exec("DELETE FROM vec_memories WHERE rowid = ?", id)
			}
			if s.useFTS {
				_, _ = tx.Exec("DELETE FROM memories_fts WHERE rowid = ?", id)
			}
			deleted++
		}
		return nil
	})
	return deleted, writeErr
}

func (s *Store) applyImportanceRules() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if marked, err := s.MarkStaleForDeletion(ctx); err != nil {
		logx.Printf("MEMORY: MarkStaleForDeletion: %v", err)
	} else if marked > 0 {
		logx.Printf("MEMORY: marked %d stale memories for deletion", marked)
	}
	if purged, err := s.PurgePendingDeletions(ctx); err != nil {
		logx.Printf("MEMORY: PurgePendingDeletions: %v", err)
	} else if purged > 0 {
		logx.Printf("MEMORY: purged %d pending-deletion memories", purged)
	}

	s.mu.RLock()
	fn := s.consolidateFn
	s.mu.RUnlock()
	if fn != nil {
		cCtx, cCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cCancel()
		s.runConsolidation(cCtx, fn)

		pCtx, pCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer pCancel()
		s.runPinnedConsolidation(pCtx, fn)
	}
}

// MergeCandidate holds a pair of memories that are similar enough to merge.
type MergeCandidate struct {
	ID1        int64
	ID2        int64
	Content1   string
	Content2   string
	Similarity float32
}

const (
	consolidateSimilarityThreshold = float32(0.92)
	consolidateSampleSize          = 200
	consolidateMaxPairs            = 5

	// pinnedConsolidateMaxPairs is deliberately smaller than
	// consolidateMaxPairs: each pair costs one mergeMemoriesLLM call, which
	// on a local-model setup competes with real chat for llama-server's
	// single inference slot (--parallel 1, see AGENTS.md's Memory / Vector
	// Store notes on extractAndPinFacts) — keeping this small bounds how
	// much extra background load runPinnedConsolidation can add per day.
	pinnedConsolidateMaxPairs = 3
)

// FindMergeCandidates returns up to limit pairs of memories whose embeddings
// have cosine similarity ≥ 0.92. It samples the consolidateSampleSize most
// recent non-deleted memories to keep the O(n²) scan fast.
//
// source='explicit' (pinned facts, see GetPinnedFacts) is excluded from the
// candidate pool, not just 'merged': saveMerged writes the merge result with
// source='merged'/importance=4, so merging two pinned facts together would
// silently un-pin the result — GetPinnedFacts' WHERE clause would no longer
// match it. Pinned facts are meant to be a small, deliberately-curated set;
// this consolidation pass exists for the much larger general conversational
// pool, not for that set — see FindPinnedMergeCandidates for the pinned set's
// own, separate dedup path that keeps the merge result pinned.
func (s *Store) FindMergeCandidates(ctx context.Context, limit int) ([]MergeCandidate, error) {
	return s.findMergeCandidates(ctx, `
		SELECT id, content, embedding
		FROM memories
		WHERE embedding IS NOT NULL
		  AND pending_deletion = 0
		  AND source NOT IN ('merged', 'explicit')
		ORDER BY timestamp DESC
		LIMIT ?
	`, consolidateSampleSize, limit)
}

// FindPinnedMergeCandidates is FindMergeCandidates' counterpart for the
// pinned-facts pool (source='explicit', importance=5) — the set
// FindMergeCandidates itself deliberately excludes. Its sample size is
// pinnedFactsLimit itself rather than consolidateSampleSize: the pinned pool
// is already capped that small, so scanning all of it is cheap, and there is
// no reason to look at only a fraction of a set this size. Results must be
// saved via savePinnedMerged, not saveMerged, or the merge would silently
// un-pin the fact (the exact bug this split was built to keep impossible).
func (s *Store) FindPinnedMergeCandidates(ctx context.Context, limit int) ([]MergeCandidate, error) {
	return s.findMergeCandidates(ctx, `
		SELECT id, content, embedding
		FROM memories
		WHERE embedding IS NOT NULL
		  AND pending_deletion = 0
		  AND source = 'explicit'
		  AND importance = 5
		ORDER BY timestamp DESC
		LIMIT ?
	`, pinnedFactsLimit, limit)
}

// findMergeCandidates runs query (expected to select id, content, embedding
// and take a single sample-size LIMIT parameter), then pairs up results
// whose embeddings have cosine similarity ≥ consolidateSimilarityThreshold,
// returning up to limit pairs ranked by similarity descending. Shared by
// FindMergeCandidates and FindPinnedMergeCandidates — the two differ only in
// which rows are eligible, never in how pairing/ranking works.
func (s *Store) findMergeCandidates(ctx context.Context, query string, sampleSize, limit int) ([]MergeCandidate, error) {
	rows, err := s.db.QueryContext(ctx, query, sampleSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type item struct {
		id      int64
		content string
		vec     []float32
		norm    float64
	}

	var items []item
	for rows.Next() {
		var it item
		var blob []byte
		if err := rows.Scan(&it.id, &it.content, &blob); err != nil {
			continue
		}
		it.vec = blobToFloats(blob)
		if len(it.vec) == 0 {
			continue
		}
		var n float64
		for _, v := range it.vec {
			n += float64(v) * float64(v)
		}
		it.norm = math.Sqrt(n)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	type pair struct {
		i, j int
		sim  float32
	}
	var pairs []pair
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			sim := cosineSimilarityFast(items[i].vec, items[i].norm, items[j].vec, items[j].norm)
			if sim >= consolidateSimilarityThreshold {
				pairs = append(pairs, pair{i, j, sim})
			}
		}
	}
	sort.Slice(pairs, func(a, b int) bool { return pairs[a].sim > pairs[b].sim })

	used := make(map[int]bool)
	var result []MergeCandidate
	for _, p := range pairs {
		if len(result) >= limit {
			break
		}
		if used[p.i] || used[p.j] {
			continue
		}
		used[p.i] = true
		used[p.j] = true
		result = append(result, MergeCandidate{
			ID1:        items[p.i].id,
			ID2:        items[p.j].id,
			Content1:   items[p.i].content,
			Content2:   items[p.j].content,
			Similarity: p.sim,
		})
	}
	return result, nil
}

// saveMerged inserts the merged content as a new memory (source='merged') and
// marks the two originals as pending_deletion in a single transaction.
func (s *Store) saveMerged(ctx context.Context, content string, id1, id2 int64) error {
	return s.saveMergedAs(ctx, content, id1, id2, "merged", "merged", 4)
}

// savePinnedMerged is saveMerged's counterpart for FindPinnedMergeCandidates'
// results: the merge result keeps source='explicit'/importance=5, so it
// still matches GetPinnedFacts' WHERE clause — a merge here must never
// silently un-pin a fact the way running it through plain saveMerged would.
func (s *Store) savePinnedMerged(ctx context.Context, content string, id1, id2 int64) error {
	return s.saveMergedAs(ctx, content, id1, id2, "pinned_merged", "explicit", 5)
}

// saveMergedAs is saveMerged/savePinnedMerged's shared implementation —
// identical insert/embed/link-cleanup logic, differing only in the new row's
// uuid prefix, source, and importance.
func (s *Store) saveMergedAs(ctx context.Context, content string, id1, id2 int64, uuidPrefix, source string, importance int) error {
	uuid := fmt.Sprintf("%s_%d", uuidPrefix, time.Now().UnixNano())
	timestamp := time.Now().UTC().Format(time.RFC3339)

	var embedBlob []byte
	var embedding []float32
	if emb, err := s.embed(ctx, content); err != nil {
		logx.Printf("MEMORY: saveMergedAs embed failed (uuid=%s): %v — merged memory will be saved without a vector and will not surface in similarity search", uuid, err)
	} else if len(emb) != s.dim {
		logx.Printf("MEMORY: saveMergedAs embed dimension mismatch (uuid=%s): got %d, want %d — merged memory will be saved without a vector and will not surface in similarity search", uuid, len(emb), s.dim)
	} else {
		embedding = emb
		embedBlob = floatsToBlob(emb)
	}

	return s.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`INSERT INTO memories(uuid, role, content, timestamp, user_msg, assist_msg, embedding,
			                      chunk_index, parent_uuid, total_chunks,
			                      session_id, importance, tags, source, retrieve_count)
			 VALUES (?, 'user', ?, ?, ?, '', ?, 0, ?, 1, '', ?, ?, ?, 0)`,
			uuid, content, timestamp, content, embedBlob, uuid, importance, source, source,
		)
		if err != nil {
			return err
		}
		rowID, _ := res.LastInsertId()
		if embedding != nil && s.useVec {
			vecJSON, _ := json.Marshal(embedding)
			if _, err := tx.Exec("INSERT INTO vec_memories(rowid, embedding) VALUES (?, ?)", rowID, string(vecJSON)); err != nil {
				logx.Printf("MEMORY: saveMergedAs vec insert: %v", err)
			}
		}
		if s.useFTS {
			if _, err := tx.Exec(
				"INSERT INTO memories_fts(rowid, content, user_msg, assist_msg) VALUES (?, ?, ?, '')",
				rowID, content, content,
			); err != nil {
				logx.Printf("MEMORY: saveMergedAs fts insert: %v", err)
			}
		}
		_, err = tx.Exec("UPDATE memories SET pending_deletion = 1 WHERE id IN (?, ?)", id1, id2)
		return err
	})
}

func (s *Store) runConsolidation(ctx context.Context, fn ConsolidationFunc) {
	s.runConsolidationWith(ctx, fn, s.FindMergeCandidates, s.saveMerged, consolidateMaxPairs, "consolidation")
}

// runPinnedConsolidation is runConsolidation's counterpart for the pinned
// facts pool — dedups near-duplicate pinned facts (e.g. auto-extraction
// pinning the same durable fact twice, worded differently, on separate
// occasions) while keeping the merge result pinned, so pinnedFactsLimit's
// recency cap has fewer near-duplicates competing for its slots over weeks
// of use. See pinnedFactsLimit's doc comment for the gap this closes.
func (s *Store) runPinnedConsolidation(ctx context.Context, fn ConsolidationFunc) {
	s.runConsolidationWith(ctx, fn, s.FindPinnedMergeCandidates, s.savePinnedMerged, pinnedConsolidateMaxPairs, "pinned consolidation")
}

// runConsolidationWith is runConsolidation/runPinnedConsolidation's shared
// implementation: find candidates, ask fn to merge each pair's content, and
// save. find and save are swapped between the two pools so a pinned-fact
// merge can never take the general pool's un-pinning save path.
func (s *Store) runConsolidationWith(
	ctx context.Context,
	fn ConsolidationFunc,
	find func(context.Context, int) ([]MergeCandidate, error),
	save func(context.Context, string, int64, int64) error,
	maxPairs int,
	label string,
) {
	candidates, err := find(ctx, maxPairs)
	if err != nil {
		logx.Printf("MEMORY: %s scan: %v", label, err)
		return
	}
	if len(candidates) == 0 {
		return
	}
	logx.Printf("MEMORY: %s: %d pair(s)", label, len(candidates))
	for _, c := range candidates {
		mergeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		merged, err := fn(mergeCtx, c.Content1, c.Content2)
		cancel()
		if err != nil {
			logx.Printf("MEMORY: %s LLM: %v", label, err)
			continue
		}
		merged = strings.TrimSpace(merged)
		if merged == "" {
			continue
		}
		saveCtx, saveCancel := context.WithTimeout(ctx, 15*time.Second)
		if err := save(saveCtx, merged, c.ID1, c.ID2); err != nil {
			logx.Printf("MEMORY: %s save: %v", label, err)
		} else {
			logx.Printf("MEMORY: %s merged pair (sim=%.2f)", label, c.Similarity)
		}
		saveCancel()
	}
}

func (s *Store) runImportanceDecay() {
	// Recovered per-call, not once around the whole loop: a panic on one
	// day's pass must not permanently disable stale-memory cleanup and
	// consolidation for the rest of the process's life (this only runs once
	// every 24h, so losing the loop silently could go unnoticed a long time).
	runOnce := func() {
		defer logx.Recover("memory.Store.runImportanceDecay/applyImportanceRules")
		s.applyImportanceRules()
	}

	select {
	case <-time.After(5 * time.Minute):
	case <-s.stopCh:
		return
	}
	runOnce()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			runOnce()
		case <-s.stopCh:
			return
		}
	}
}

// ExportMemory is the JSON schema for a single exported memory entry.
type ExportMemory struct {
	UUID       string `json:"uuid"`
	Content    string `json:"content"`
	Timestamp  string `json:"timestamp"`
	UserMsg    string `json:"user_msg"`
	AssistMsg  string `json:"assist_msg"`
	Importance int    `json:"importance"`
	Tags       string `json:"tags"`
	Source     string `json:"source"`
}

// ExportPayload is the top-level envelope for memory export/import.
type ExportPayload struct {
	Version    int            `json:"version"`
	ExportedAt string         `json:"exported_at"`
	Memories   []ExportMemory `json:"memories"`
}

func (s *Store) Export(ctx context.Context) ([]byte, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT uuid, content, timestamp, user_msg, assist_msg, importance, tags, source
		FROM memories
		WHERE pending_deletion = 0
		ORDER BY timestamp
	`)
	if err != nil {
		return nil, fmt.Errorf("memory.Export: query: %w", err)
	}
	defer rows.Close()
	var memories []ExportMemory
	for rows.Next() {
		var m ExportMemory
		if err := rows.Scan(&m.UUID, &m.Content, &m.Timestamp, &m.UserMsg, &m.AssistMsg, &m.Importance, &m.Tags, &m.Source); err != nil {
			continue
		}
		memories = append(memories, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	payload := ExportPayload{
		Version:    2,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Memories:   memories,
	}
	return json.Marshal(payload)
}

func (s *Store) Import(ctx context.Context, data []byte) (int, error) {
	var payload ExportPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return 0, fmt.Errorf("memory.Import: parse: %w", err)
	}
	imported := 0
	for _, m := range payload.Memories {
		embedding, err := s.embed(ctx, m.Content)
		if err != nil || len(embedding) != s.dim {
			logx.Printf("MEMORY: import embed failed for %s, inserting without vector: %v", m.UUID, err)
			embedding = nil
		}
		imp := m.Importance
		if imp < 1 || imp > 5 {
			imp = 3
		}
		src := m.Source
		if src == "" {
			src = "conversation"
		}
		var embedBlob []byte
		if embedding != nil {
			embedBlob = floatsToBlob(embedding)
		}
		writeErr := s.db.Write(ctx, func(tx *sql.Tx) error {
			// INSERT OR IGNORE handles the UUID-already-exists case atomically,
			// eliminating the race between a prior SELECT and this INSERT.
			res, err := tx.Exec(
				`INSERT OR IGNORE INTO memories(uuid, role, content, timestamp, user_msg, assist_msg, embedding,
				                      chunk_index, parent_uuid, total_chunks,
				                      session_id, importance, tags, source, retrieve_count, pending_deletion)
				 VALUES (?, 'user', ?, ?, ?, ?, ?, 0, ?, 1, '', ?, ?, ?, 0, 0)`,
				m.UUID, m.Content, m.Timestamp, m.UserMsg, m.AssistMsg, embedBlob, m.UUID, imp, m.Tags, src,
			)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return nil // UUID already existed — skip vec/FTS insert
			}
			if embedding == nil {
				return nil
			}
			rowID, err := res.LastInsertId()
			if err != nil {
				return err
			}
			if s.useVec {
				vecJSON, _ := json.Marshal(embedding)
				_, _ = tx.Exec("INSERT OR IGNORE INTO vec_memories(rowid, embedding) VALUES (?, ?)", rowID, string(vecJSON))
			}
			if s.useFTS {
				_, _ = tx.Exec("INSERT OR IGNORE INTO memories_fts(rowid, content, user_msg, assist_msg) VALUES (?, ?, ?, ?)",
					rowID, m.Content, m.UserMsg, m.AssistMsg)
			}
			return nil
		})
		if writeErr == nil {
			imported++
		}
	}
	return imported, nil
}

func stripAssistantReply(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Assistant:") || strings.HasPrefix(trimmed, "Asistan:") {
			break
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func vectorNorm(v []float32) float64 {
	var sum float64
	for _, val := range v {
		sum += float64(val) * float64(val)
	}
	return math.Sqrt(sum)
}

func cosineSimilarityFast(a []float32, aNorm float64, b []float32, bNorm float64) float32 {
	if len(a) == 0 || len(a) != len(b) || aNorm == 0 || bNorm == 0 {
		return 0
	}
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	return float32(dot / (aNorm * bNorm))
}

func normalizeVector(v []float32) []float32 {
	if len(v) == 0 {
		return v
	}
	var norm float64
	for _, val := range v {
		norm += float64(val * val)
	}
	if norm == 0 {
		return v
	}
	scale := float32(1 / math.Sqrt(norm))
	out := make([]float32, len(v))
	for i, val := range v {
		out[i] = val * scale
	}
	return out
}

func floatsToBlob(v []float32) []byte {
	b := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

func blobToFloats(b []byte) []float32 {
	if len(b) == 0 || len(b)%4 != 0 {
		return nil
	}
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

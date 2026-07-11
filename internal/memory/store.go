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
		db:     db,
		embed:  cfg.EmbeddingFunc,
		dim:    cfg.Dimension,
		dir:    cfg.Dir,
		dbPath: dbPath,
	}

	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("memory.NewStore: schema: %w", err)
	}

	s.stopCh = make(chan struct{})
	go s.runImportanceDecay()

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
				migCtx, migCancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer migCancel()
				// Abort early if the store is closed during migration.
				go func() {
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

	vecErr := s.tryCreateVecTable(ctx)
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
				migCtx, migCancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer migCancel()
				// Abort early if the store is closed during migration.
				go func() {
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
	embedding, err := s.embed(ctx, embedText)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	if len(embedding) != s.dim {
		return fmt.Errorf("embedding dimension %d != expected %d", len(embedding), s.dim)
	}
	embedDur := time.Since(embedStart)

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
	return strings.Join(parts, " ")
}

func (s *Store) ftsSearch(ctx context.Context, query string, topK int) ([]MemoryResult, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT m.uuid, m.content, m.timestamp, m.user_msg, m.assist_msg,
               memories_fts.rank,
               m.importance, m.source, m.tags, m.session_id, m.retrieve_count
        FROM memories_fts
        JOIN memories m ON m.id = memories_fts.rowid
        WHERE memories_fts MATCH ?
        ORDER BY memories_fts.rank
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

	type scored struct {
		id    string
		score float64
	}
	var ranked []scored
	for id, score := range scores {
		ranked = append(ranked, scored{id, score})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
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

func (s *Store) RetrieveContext(ctx context.Context, query string, topK int, minSimilarity float32) ([]MemoryResult, error) {
	start := time.Now()

	embedStart := time.Now()
	queryEmbedding, err := s.embed(ctx, query)
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

	var vecMemories []MemoryResult
	if s.useVec {
		vecMemories, err = s.vecSearch(ctx, queryEmbedding, candidateK, minSimilarity)
	} else {
		vecMemories, err = s.goSearch(ctx, queryEmbedding, candidateK, minSimilarity)
	}
	if err != nil {
		return nil, fmt.Errorf("memory.RetrieveContext: vector search: %w", err)
	}
	for i := range vecMemories {
		vecMemories[i].MatchType = "vector"
	}

	// Multi-query: if query is long, do a second search with a topic-focused variant
	// and merge with the primary results via RRF for better recall.
	if expanded := expandQuery(query); expanded != "" {
		if expEmb, expErr := s.embed(ctx, expanded); expErr == nil && len(expEmb) == s.dim {
			var expResults []MemoryResult
			if s.useVec {
				expResults, _ = s.vecSearch(ctx, expEmb, candidateK/2, minSimilarity)
			} else {
				expResults, _ = s.goSearch(ctx, expEmb, candidateK/2, minSimilarity)
			}
			if len(expResults) > 0 {
				vecMemories = reciprocalRankFusion(vecMemories, expResults, candidateK)
			}
		}
	}

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
	sort.Slice(memories, func(i, j int) bool {
		return memories[i].Similarity > memories[j].Similarity
	})

	if len(memories) > 0 {
		ids := make([]string, len(memories))
		for i, m := range memories {
			ids[i] = m.ID
		}
		go s.incrementRetrieveCounts(ids)
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

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.uuid, m.content, m.timestamp, m.user_msg, m.assist_msg, v.distance,
		       m.importance, m.source, m.tags, m.session_id, m.retrieve_count
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
		)
		if err := rows.Scan(&uuid, &content, &timestamp, &userMsg, &assistMsg, &distance,
			&importance, &source, &tags, &sessionID, &retrieveCount); err != nil {
			return nil, fmt.Errorf("memory.vecSearch: scan: %w", err)
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
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, uuid, content, timestamp, user_msg, assist_msg, embedding,
		        importance, source, tags, session_id, retrieve_count
		 FROM memories WHERE embedding IS NOT NULL`)
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

	sort.Slice(all, func(i, j int) bool {
		return all[i].sim > all[j].sim
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
			 VALUES (?, 'user', ?, ?, ?, '', ?, ?, 0, ?, 1, '', 5, ?, 'explicit', 0)`,
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
)

// FindMergeCandidates returns up to limit pairs of memories whose embeddings
// have cosine similarity ≥ 0.92. It samples the consolidateSampleSize most
// recent non-deleted memories to keep the O(n²) scan fast.
func (s *Store) FindMergeCandidates(ctx context.Context, limit int) ([]MergeCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, content, embedding
		FROM memories
		WHERE embedding IS NOT NULL
		  AND pending_deletion = 0
		  AND source NOT IN ('merged')
		ORDER BY timestamp DESC
		LIMIT ?
	`, consolidateSampleSize)
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
	uuid := fmt.Sprintf("merged_%d", time.Now().UnixNano())
	timestamp := time.Now().UTC().Format(time.RFC3339)

	var embedBlob []byte
	var embedding []float32
	if emb, err := s.embed(ctx, content); err != nil {
		logx.Printf("MEMORY: saveMerged embed failed (uuid=%s): %v — merged memory will be saved without a vector and will not surface in similarity search", uuid, err)
	} else if len(emb) != s.dim {
		logx.Printf("MEMORY: saveMerged embed dimension mismatch (uuid=%s): got %d, want %d — merged memory will be saved without a vector and will not surface in similarity search", uuid, len(emb), s.dim)
	} else {
		embedding = emb
		embedBlob = floatsToBlob(emb)
	}

	return s.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`INSERT INTO memories(uuid, role, content, timestamp, user_msg, assist_msg, embedding,
			                      chunk_index, parent_uuid, total_chunks,
			                      session_id, importance, tags, source, retrieve_count)
			 VALUES (?, 'user', ?, ?, ?, '', ?, 0, ?, 1, '', 4, 'merged', 'merged', 0)`,
			uuid, content, timestamp, content, embedBlob, uuid,
		)
		if err != nil {
			return err
		}
		rowID, _ := res.LastInsertId()
		if embedding != nil && s.useVec {
			vecJSON, _ := json.Marshal(embedding)
			if _, err := tx.Exec("INSERT INTO vec_memories(rowid, embedding) VALUES (?, ?)", rowID, string(vecJSON)); err != nil {
				logx.Printf("MEMORY: saveMerged vec insert: %v", err)
			}
		}
		if s.useFTS {
			if _, err := tx.Exec(
				"INSERT INTO memories_fts(rowid, content, user_msg, assist_msg) VALUES (?, ?, ?, '')",
				rowID, content, content,
			); err != nil {
				logx.Printf("MEMORY: saveMerged fts insert: %v", err)
			}
		}
		_, err = tx.Exec("UPDATE memories SET pending_deletion = 1 WHERE id IN (?, ?)", id1, id2)
		return err
	})
}

func (s *Store) runConsolidation(ctx context.Context, fn ConsolidationFunc) {
	candidates, err := s.FindMergeCandidates(ctx, consolidateMaxPairs)
	if err != nil {
		logx.Printf("MEMORY: consolidation scan: %v", err)
		return
	}
	if len(candidates) == 0 {
		return
	}
	logx.Printf("MEMORY: consolidating %d memory pair(s)", len(candidates))
	for _, c := range candidates {
		mergeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		merged, err := fn(mergeCtx, c.Content1, c.Content2)
		cancel()
		if err != nil {
			logx.Printf("MEMORY: consolidation LLM: %v", err)
			continue
		}
		merged = strings.TrimSpace(merged)
		if merged == "" {
			continue
		}
		saveCtx, saveCancel := context.WithTimeout(ctx, 15*time.Second)
		if err := s.saveMerged(saveCtx, merged, c.ID1, c.ID2); err != nil {
			logx.Printf("MEMORY: consolidation save: %v", err)
		} else {
			logx.Printf("MEMORY: merged pair (sim=%.2f)", c.Similarity)
		}
		saveCancel()
	}
}

func (s *Store) runImportanceDecay() {
	select {
	case <-time.After(5 * time.Minute):
	case <-s.stopCh:
		return
	}
	s.applyImportanceRules()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.applyImportanceRules()
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

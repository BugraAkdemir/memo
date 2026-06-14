package memory

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
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

type MemoryResult = models.MemoryResult
type MemoryFileInfo = models.MemoryFileInfo
type GobFileInfo = models.MemoryFileInfo

type Store struct {
	db     *database.DB
	embed  EmbeddingFunc
	dim    int
	dir    string
	dbPath string
	mu     sync.RWMutex
	closed bool
	useVec bool
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

	return s, nil
}

func (s *Store) initSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL UNIQUE,
			role TEXT NOT NULL DEFAULT 'user',
			content TEXT NOT NULL,
			timestamp TEXT NOT NULL,
			user_msg TEXT NOT NULL DEFAULT '',
			assist_msg TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT 'conversation',
			embedding BLOB DEFAULT NULL
		)
	`); err != nil {
		return err
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`); err != nil {
		return err
	}

	vecErr := s.tryCreateVecTable(ctx)
	if vecErr == nil {
		s.useVec = true
		if err := s.ensureVecMetadata(ctx); err != nil {
			log.Printf("MEMORY: metadata init: %v", err)
		}

		if err := s.migrateEmbeddingsToVec(ctx); err != nil {
			log.Printf("MEMORY: migrate to vec: %v", err)
		}
	} else {
		s.useVec = false
		log.Printf("MEMORY: vec0 not available (%v), using Go fallback", vecErr)
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
		log.Printf("MEMORY: embedding dimension changed from %d to %d, recreating vec index", existingDim, s.dim)
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
			_, err := tx.Exec(
				"INSERT OR REPLACE INTO _metadata(key, value) VALUES ('embedding_dimension', ?)",
				fmt.Sprintf("%d", s.dim),
			)
			return err
		})
	}

	return nil
}

func (s *Store) migrateEmbeddingsToVec(ctx context.Context) error {
	if !s.useVec {
		return nil
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT id, embedding FROM memories WHERE embedding IS NOT NULL")
	if err != nil {
		return err
	}
	defer rows.Close()

	return s.db.Write(ctx, func(tx *sql.Tx) error {
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
			jsonVec, _ := json.Marshal(vec)
			tx.Exec("INSERT OR IGNORE INTO vec_memories(rowid, embedding) VALUES (?, ?)", id, string(jsonVec))
		}
		return nil
	})
}

func (s *Store) SaveInteraction(ctx context.Context, userMsg, assistantMsg string) error {
	embedStart := time.Now()
	embedding, err := s.embed(ctx, userMsg)
	if err != nil {
		return fmt.Errorf("memory.SaveInteraction: embed: %w", err)
	}
	if len(embedding) != s.dim {
		return fmt.Errorf("memory.SaveInteraction: embedding dimension %d != expected %d", len(embedding), s.dim)
	}
	embedDur := time.Since(embedStart)

	timestamp := time.Now().UTC().Format(time.RFC3339)
	uuid := fmt.Sprintf("mem_%d", time.Now().UnixNano())
	content := fmt.Sprintf("[%s] User: %s\nAssistant: %s", timestamp, userMsg, assistantMsg)

	writeStart := time.Now()
	err = s.db.Write(ctx, func(tx *sql.Tx) error {
		embedBlob := floatsToBlob(embedding)

		res, err := tx.Exec(
			`INSERT INTO memories(uuid, role, content, timestamp, user_msg, assist_msg, embedding)
			 VALUES (?, 'user', ?, ?, ?, ?, ?)`,
			uuid, content, timestamp, userMsg, assistantMsg, embedBlob,
		)
		if err != nil {
			return fmt.Errorf("insert memory: %w", err)
		}

		if s.useVec {
			rowID, err := res.LastInsertId()
			if err != nil {
				return err
			}
			vecJSON, _ := json.Marshal(embedding)
			_, err = tx.Exec(
				"INSERT INTO vec_memories(rowid, embedding) VALUES (?, ?)",
				rowID, string(vecJSON),
			)
			if err != nil {
				return fmt.Errorf("insert vector: %w", err)
			}
		}

		return nil
	})
	writeDur := time.Since(writeStart)

	log.Printf("LATENCY memory.save total_ms=%d embed_ms=%d write_ms=%d dim=%d vec=%v",
		time.Since(embedStart.Add(writeDur)).Milliseconds(),
		embedDur.Milliseconds(),
		writeDur.Milliseconds(),
		s.dim,
		s.useVec,
	)

	return err
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

	var memories []MemoryResult

	if s.useVec {
		memories, err = s.vecSearch(ctx, queryEmbedding, topK, minSimilarity)
	} else {
		memories, err = s.goSearch(ctx, queryEmbedding, topK, minSimilarity)
	}

	log.Printf("LATENCY memory.retrieve total_ms=%d embed_ms=%d top_k=%d returned=%d vec=%v",
		time.Since(start).Milliseconds(),
		embedDur.Milliseconds(),
		topK,
		len(memories),
		s.useVec,
	)

	return memories, err
}

func (s *Store) vecSearch(ctx context.Context, queryEmbedding []float32, topK int, minSimilarity float32) ([]MemoryResult, error) {
	vecJSON, err := json.Marshal(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("memory.vecSearch: marshal: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.uuid, m.content, m.timestamp, m.user_msg, m.assist_msg, v.distance
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
			uuid, content, timestamp string
			userMsg, assistMsg       string
			distance                 float64
		)
		if err := rows.Scan(&uuid, &content, &timestamp, &userMsg, &assistMsg, &distance); err != nil {
			return nil, fmt.Errorf("memory.vecSearch: scan: %w", err)
		}

		sim := float32(1.0 - math.Max(0, math.Min(distance, 2))/2)
		if sim < minSimilarity {
			continue
		}

		results = append(results, MemoryResult{
			ID:         uuid,
			Content:    content,
			Similarity: sim,
			Timestamp:  timestamp,
			UserMsg:    userMsg,
			AssistMsg:  assistMsg,
		})
	}

	return results, rows.Err()
}

func (s *Store) goSearch(ctx context.Context, queryEmbedding []float32, topK int, minSimilarity float32) ([]MemoryResult, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, uuid, content, timestamp, user_msg, assist_msg, embedding FROM memories WHERE embedding IS NOT NULL")
	if err != nil {
		return nil, fmt.Errorf("memory.goSearch: %w", err)
	}
	defer rows.Close()

	type scored struct {
		uuid     string
		content  string
		ts       string
		userMsg  string
		assistMsg string
		sim      float32
	}

	queryNorm := vectorNorm(queryEmbedding)

	var all []scored
	for rows.Next() {
		var id int64
		var uuid, content, timestamp, userMsg, assistMsg string
		var blob []byte
		if err := rows.Scan(&id, &uuid, &content, &timestamp, &userMsg, &assistMsg, &blob); err != nil {
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
			uuid:      uuid,
			content:   content,
			ts:        timestamp,
			userMsg:   userMsg,
			assistMsg: assistMsg,
			sim:       sim,
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
			ID:         s.uuid,
			Content:    s.content,
			Similarity: s.sim,
			Timestamp:  s.ts,
			UserMsg:    s.userMsg,
			AssistMsg:  s.assistMsg,
		})
	}

	return results, nil
}

func (s *Store) DebugSearch(ctx context.Context, query string, topK int) []MemoryResult {
	results, err := s.RetrieveContext(ctx, query, topK, 0)
	if err != nil {
		log.Printf("DEBUG SEARCH ERROR: %v", err)
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
		if _, err := tx.Exec("DELETE FROM memories WHERE id = ?", id); err != nil {
			return err
		}
		return nil
	})
}

func (s *Store) Stats() models.MemoryStats {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var stats models.MemoryStats
	stats.Dimension = s.dim

	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memories").Scan(&stats.Count); err != nil {
		log.Printf("MEMORY: stats count query: %v", err)
	}
	stats.VecCount = stats.Count

	return stats
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.db.Close()
}

func FormatMemoriesForPrompt(memories []MemoryResult) string {
	if len(memories) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n--- RELEVANT MEMORIES ---\n")
	for i, m := range memories {
		sb.WriteString(fmt.Sprintf("[Memory %d | Relevance: %.0f%%]\n%s\n\n", i+1, m.Similarity*100, m.Content))
	}
	sb.WriteString("--- END MEMORIES ---\n")
	return sb.String()
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

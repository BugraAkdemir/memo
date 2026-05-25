package memory

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryResult struct {
	Content    string  `json:"content"`
	Similarity float32 `json:"similarity"`
	Timestamp  string  `json:"timestamp"`
	ID         string  `json:"id"`
}

type scoredMemory struct {
	ID         string
	Similarity float32
}

func (s *Store) RetrieveContext(ctx context.Context, query string, topK int, minSimilarity float32) ([]MemoryResult, error) {
	totalStart := time.Now()
	actualCount := s.Count()
	if actualCount == 0 {
		log.Printf("Memory retrieve: cache is empty")
		return nil, nil
	}
	if topK <= 0 {
		return nil, nil
	}

	embedStart := time.Now()
	queryEmbedding, err := s.embeddingFunc(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("memory.RetrieveContext: embed: %w", err)
	}
	embedDuration := time.Since(embedStart)

	nResults := topK
	if nResults > actualCount {
		nResults = actualCount
	}

	log.Printf("Memory query: %q (topK=%d, minSim=%.2f, cache=%d)", truncateForLog(query, 80), nResults, minSimilarity, actualCount)

	searchStart := time.Now()
	ranked, err := s.searchIndex(ctx, queryEmbedding, nResults)
	if err != nil {
		return nil, fmt.Errorf("memory.RetrieveContext: search cache: %w", err)
	}
	searchDuration := time.Since(searchStart)

	var memories []MemoryResult
	var diskDuration time.Duration
	diskReads := 0
	for _, r := range ranked {
		if r.Similarity < minSimilarity {
			continue
		}

		diskStart := time.Now()
		doc, err := s.getDocumentByID(r.ID)
		diskDuration += time.Since(diskStart)
		diskReads++
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				log.Printf("Memory cache invalidation: %s exists in RAM but not on disk", r.ID)
				s.removeIndexByID(r.ID)
				continue
			}
			return nil, fmt.Errorf("memory.RetrieveContext: read %s: %w", r.ID, err)
		}

		log.Printf("  -> [%.0f%%] %s: %s", r.Similarity*100, r.ID, truncateForLog(doc.Content, 100))
		memories = append(memories, MemoryResult{
			Content:    doc.Content,
			Similarity: r.Similarity,
			Timestamp:  doc.Metadata["timestamp"],
			ID:         doc.ID,
		})
	}

	log.Printf("Memory result: %d/%d passed filter (min=%.0f%%)", len(memories), len(ranked), minSimilarity*100)
	log.Printf(
		"LATENCY memory.retrieve total_ms=%d embed_ms=%d search_ms=%d disk_ms=%d disk_reads=%d cache_docs=%d top_k=%d returned=%d",
		time.Since(totalStart).Milliseconds(),
		embedDuration.Milliseconds(),
		searchDuration.Milliseconds(),
		diskDuration.Milliseconds(),
		diskReads,
		actualCount,
		nResults,
		len(memories),
	)
	return memories, nil
}

// DebugSearch runs a query with NO similarity filter and returns all results.
// This is for diagnosing memory issues.
func (s *Store) DebugSearch(ctx context.Context, query string, topK int) []MemoryResult {
	actualCount := s.Count()
	log.Printf("DEBUG SEARCH: query=%q, cache_count=%d", query, actualCount)

	if actualCount == 0 || topK <= 0 {
		return nil
	}

	queryEmbedding, err := s.embeddingFunc(ctx, query)
	if err != nil {
		log.Printf("DEBUG SEARCH EMBED ERROR: %v", err)
		return nil
	}

	nResults := topK
	if nResults > actualCount {
		nResults = actualCount
	}

	ranked, err := s.searchIndex(ctx, queryEmbedding, nResults)
	if err != nil {
		log.Printf("DEBUG SEARCH ERROR: %v", err)
		return nil
	}

	var memories []MemoryResult
	for _, r := range ranked {
		doc, err := s.getDocumentByID(r.ID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				log.Printf("DEBUG SEARCH MISSING DISK FILE: %s", r.ID)
				s.removeIndexByID(r.ID)
			} else {
				log.Printf("DEBUG SEARCH READ ERROR: %v", err)
			}
			continue
		}

		log.Printf("  DEBUG [%.1f%%] %s: %s", r.Similarity*100, r.ID, truncateForLog(doc.Content, 120))
		memories = append(memories, MemoryResult{
			Content:    doc.Content,
			Similarity: r.Similarity,
			Timestamp:  doc.Metadata["timestamp"],
			ID:         doc.ID,
		})
	}
	return memories
}

func (s *Store) searchIndex(ctx context.Context, queryEmbedding []float32, topK int) ([]scoredMemory, error) {
	queryEmbedding = normalizeVector(queryEmbedding)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.index) == 0 {
		return nil, nil
	}
	if topK > len(s.index) {
		topK = len(s.index)
	}

	workers := runtime.NumCPU()
	if workers > len(s.index) {
		workers = len(s.index)
	}
	if workers < 1 {
		workers = 1
	}

	results := make([]scoredMemory, 0, len(s.index))
	resultsMu := sync.Mutex{}
	var wg sync.WaitGroup

	chunkSize := (len(s.index) + workers - 1) / workers
	for worker := 0; worker < workers; worker++ {
		start := worker * chunkSize
		if start >= len(s.index) {
			break
		}
		end := start + chunkSize
		if end > len(s.index) {
			end = len(s.index)
		}

		wg.Add(1)
		go func(items []MemoryIndex) {
			defer wg.Done()

			local := make([]scoredMemory, 0, len(items))
			for _, item := range items {
				select {
				case <-ctx.Done():
					return
				default:
				}
				local = append(local, scoredMemory{
					ID:         item.ID,
					Similarity: cosineSimilarity(queryEmbedding, item.Vector),
				})
			}

			resultsMu.Lock()
			results = append(results, local...)
			resultsMu.Unlock()
		}(s.index[start:end])
	}

	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	return results[:topK], nil
}

func (s *Store) getDocumentByID(id string) (docResult, error) {
	doc, err := readDocument(s.documentPath(id))
	if err != nil {
		return docResult{}, err
	}
	return docResult{
		ID:       doc.ID,
		Content:  doc.Content,
		Metadata: doc.Metadata,
	}, nil
}

func (s *Store) removeIndexByID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeIndexLocked(id)
}

type docResult struct {
	ID       string
	Content  string
	Metadata map[string]string
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

func cosineSimilarity(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

func normalizeVector(v []float32) []float32 {
	if len(v) == 0 {
		return v
	}

	var norm float64
	for _, value := range v {
		norm += float64(value * value)
	}
	if norm == 0 {
		return v
	}

	scale := float32(1 / math.Sqrt(norm))
	out := make([]float32, len(v))
	for i, value := range v {
		out[i] = value * scale
	}
	return out
}

func truncateForLog(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

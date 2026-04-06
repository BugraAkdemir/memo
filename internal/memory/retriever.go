package memory

import (
	"context"
	"fmt"
	"log"
	"strings"
)

type MemoryResult struct {
	Content    string  `json:"content"`
	Similarity float32 `json:"similarity"`
	Timestamp  string  `json:"timestamp"`
	ID         string  `json:"id"`
}

func (s *Store) RetrieveContext(ctx context.Context, query string, topK int, minSimilarity float32) ([]MemoryResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Use actual collection count, not our tracked docCount (which can be stale after restart)
	actualCount := s.collection.Count()
	if actualCount == 0 {
		log.Printf("Memory retrieve: collection is empty (tracked=%d, actual=%d)", s.docCount, actualCount)
		return nil, nil
	}

	nResults := topK
	if nResults > actualCount {
		nResults = actualCount
	}

	log.Printf("Memory query: %q (topK=%d, minSim=%.2f, docs=%d)", truncateForLog(query, 80), nResults, minSimilarity, actualCount)

	results, err := s.collection.Query(ctx, query, nResults, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("memory.RetrieveContext: %w", err)
	}

	var memories []MemoryResult
	for _, r := range results {
		log.Printf("  → [%.0f%%] %s: %s", r.Similarity*100, r.ID, truncateForLog(r.Content, 100))
		if r.Similarity < minSimilarity {
			continue
		}
		memories = append(memories, MemoryResult{
			Content:    r.Content,
			Similarity: r.Similarity,
			Timestamp:  r.Metadata["timestamp"],
			ID:         r.ID,
		})
	}

	log.Printf("Memory result: %d/%d passed filter (min=%.0f%%)", len(memories), len(results), minSimilarity*100)
	return memories, nil
}

// DebugSearch runs a query with NO similarity filter and returns all results.
// This is for diagnosing memory issues.
func (s *Store) DebugSearch(ctx context.Context, query string, topK int) []MemoryResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	actualCount := s.collection.Count()
	log.Printf("DEBUG SEARCH: query=%q, collection_count=%d, tracked_count=%d", query, actualCount, s.docCount)

	if actualCount == 0 {
		return nil
	}

	nResults := topK
	if nResults > actualCount {
		nResults = actualCount
	}

	results, err := s.collection.Query(ctx, query, nResults, nil, nil)
	if err != nil {
		log.Printf("DEBUG SEARCH ERROR: %v", err)
		return nil
	}

	var memories []MemoryResult
	for _, r := range results {
		log.Printf("  DEBUG [%.1f%%] %s: %s", r.Similarity*100, r.ID, truncateForLog(r.Content, 120))
		memories = append(memories, MemoryResult{
			Content:    r.Content,
			Similarity: r.Similarity,
			Timestamp:  r.Metadata["timestamp"],
			ID:         r.ID,
		})
	}
	return memories
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

func truncateForLog(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

package memory

import (
	"context"
	"fmt"
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

	if s.docCount == 0 {
		return nil, nil
	}

	nResults := topK
	if nResults > s.docCount {
		nResults = s.docCount
	}

	results, err := s.collection.Query(ctx, query, nResults, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("memory.RetrieveContext: %w", err)
	}

	var memories []MemoryResult
	for _, r := range results {
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

	return memories, nil
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

package memory

import (
	"strings"

	"memo/internal/truncate"
)

// chunkText splits text into overlapping chunks sized by estimated token
// count (truncate.EstimateTokens), not word count. Word count is a poor
// proxy for token count — a word can tokenize to anywhere from a fraction
// of a token to several (long identifiers, URLs, agglutinative Turkish) —
// so a chunk that looked safely under budget by word count alone could
// still overflow the embedding model's context.
// maxTokens: estimated token budget per chunk; overlapTokens: estimated
// tokens shared between consecutive chunks.
// Returns the original text as a single chunk if it fits within maxTokens.
func chunkText(text string, maxTokens, overlapTokens int) []string {
	if truncate.EstimateTokens(text) <= maxTokens {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	// prefix[i] = estimated tokens of words[0:i] joined by single spaces.
	prefix := make([]int, len(words)+1)
	for i, w := range words {
		prefix[i+1] = prefix[i] + truncate.EstimateTokens(w) + 1 // +1 for the joining space
	}
	total := prefix[len(words)]

	minChunkTokens := min(overlapTokens*2, maxTokens/2)

	var chunks []string
	for start := 0; start < len(words); {
		end := start + 1
		for end < len(words) && prefix[end+1]-prefix[start] <= maxTokens {
			end++
		}

		remaining := total - prefix[end]
		if remaining > 0 && remaining < minChunkTokens && len(chunks) > 0 {
			chunks[len(chunks)-1] += " " + strings.Join(words[start:], " ")
			break
		}

		chunks = append(chunks, strings.Join(words[start:end], " "))
		if end == len(words) {
			break
		}

		// Back up from `end` by roughly overlapTokens worth of words so the
		// next chunk repeats that tail — guaranteed forward progress since
		// next defaults to end (> start) if the whole span is needed.
		next := end
		for next > start && prefix[end]-prefix[next-1] < overlapTokens {
			next--
		}
		if next <= start {
			next = end
		}
		start = next
	}
	return chunks
}

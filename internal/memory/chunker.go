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

	// A single space-free "word" (long URL, base64 blob, minified code, a
	// hash) can itself exceed maxTokens — the grouping loop below always
	// takes at least one word per chunk, so without this fallback that one
	// word is emitted as its own oversized chunk, overflowing the embedding
	// server's batch-size limit on both save and retrieve.
	expanded := make([]string, 0, len(words))
	for _, w := range words {
		expanded = append(expanded, splitLongWord(w, maxTokens)...)
	}
	words = expanded

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

// splitLongWord force-splits a single space-free "word" that alone exceeds
// maxTokens into rune-safe pieces that each fit the budget. Returns the word
// unchanged (as a single-element slice) if it's already within budget.
func splitLongWord(word string, maxTokens int) []string {
	if truncate.EstimateTokens(word) <= maxTokens {
		return []string{word}
	}

	maxBytes := max(maxTokens*3, 1)

	var pieces []string
	start := 0
	for i := range word {
		if i-start >= maxBytes {
			pieces = append(pieces, word[start:i])
			start = i
		}
	}
	pieces = append(pieces, word[start:])
	return pieces
}

package memory

import (
	"strings"

	"memo/internal/truncate"
)

func chunkText(text string, maxTokens, overlapTokens int) []string {
	if truncate.EstimateTokens(text) <= maxTokens {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

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

func maxBytesForTokenBudget(maxTokens int) int {
	return max(maxTokens, 1)
}

func splitLongWord(word string, maxTokens int) []string {
	if truncate.EstimateTokens(word) <= maxTokens {
		return []string{word}
	}

	maxBytes := maxBytesForTokenBudget(maxTokens)

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

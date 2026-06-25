package memory

import "strings"

// chunkText splits text into overlapping word-based chunks.
// maxWords: maximum words per chunk; overlapWords: words shared between consecutive chunks.
// Returns the original text as a single chunk if it fits within maxWords.
func chunkText(text string, maxWords, overlapWords int) []string {
	words := strings.Fields(text)
	if len(words) <= maxWords {
		return []string{text}
	}

	step := maxWords - overlapWords
	if step <= 0 {
		step = maxWords
	}

	var chunks []string
	for start := 0; start < len(words); start += step {
		end := start + maxWords
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[start:end], " "))
		if end == len(words) {
			break
		}
	}
	return chunks
}

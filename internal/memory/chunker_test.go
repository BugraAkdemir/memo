package memory

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"memo/internal/truncate"
)

func TestChunkText_ShortText(t *testing.T) {
	text := "bu kısa bir mesajdır yani bölünmemeli hiç"
	chunks := chunkText(text, 300, 50)
	if len(chunks) != 1 {
		t.Fatalf("short text: got %d chunks, want 1", len(chunks))
	}
	if chunks[0] != text {
		t.Fatalf("short text chunk = %q, want original", chunks[0])
	}
}

// TestChunkText_LongWordsExceedBudget reproduces the original bug: text
// short by word count can still be long by token count. 300 words of a
// long identifier-like token (12 chars ≈ 5 estimated tokens) is ~1500
// estimated tokens — well over a 300-token budget — even though the old
// word-count-only chunker would have called it a single, unsplit chunk.
func TestChunkText_LongWordsExceedBudget(t *testing.T) {
	words := make([]string, 300)
	for i := range words {
		words[i] = "abcdefghijkl" // 12 chars ≈ 5 estimated tokens
	}
	text := strings.Join(words, " ")

	if len(strings.Fields(text)) > 300 {
		t.Fatalf("test setup: expected exactly 300 words")
	}

	chunks := chunkText(text, 300, 50)
	if len(chunks) < 2 {
		t.Fatalf("long-token text: got %d chunks, want >=2 (word count alone would wrongly say 1)", len(chunks))
	}
	for i, c := range chunks {
		if tok := truncate.EstimateTokens(c); tok > 300+50 { // allow the trailing small-remainder merge
			t.Fatalf("chunk[%d] has ~%d estimated tokens, want roughly <= 300", i, tok)
		}
	}
}

func TestChunkText_LongText(t *testing.T) {
	words := make([]string, 600)
	for i := range words {
		words[i] = "kelime"
	}
	text := strings.Join(words, " ")
	chunks := chunkText(text, 300, 50)
	if len(chunks) < 2 {
		t.Fatalf("long text: got %d chunks, want >=2", len(chunks))
	}
	for i, c := range chunks {
		if tok := truncate.EstimateTokens(c); tok > 300+50 {
			t.Fatalf("chunk[%d] has ~%d estimated tokens, want roughly <= 300", i, tok)
		}
	}
}

func TestChunkText_OverlapContent(t *testing.T) {
	words := make([]string, 400)
	for i := range words {
		words[i] = fmt.Sprintf("word%d", i)
	}
	text := strings.Join(words, " ")
	chunks := chunkText(text, 200, 50)
	if len(chunks) < 2 {
		t.Skip("not enough chunks to test overlap")
	}
	chunk0Words := strings.Fields(chunks[0])
	chunk1Words := strings.Fields(chunks[1])

	// The tail of chunk0 must reappear verbatim at the head of chunk1 (the
	// exact word count depends on token accounting, not a fixed count).
	last := chunk0Words[len(chunk0Words)-1]
	if !slices.Contains(chunk1Words, last) {
		t.Fatalf("chunk1 does not contain chunk0's last word %q — no overlap produced", last)
	}
}

func TestChunkText_NoDataLoss(t *testing.T) {
	words := make([]string, 500)
	for i := range words {
		words[i] = fmt.Sprintf("w%d", i)
	}
	text := strings.Join(words, " ")
	chunks := chunkText(text, 150, 30)

	joined := strings.Join(chunks, " ")
	for _, w := range words {
		if !strings.Contains(joined, w) {
			t.Fatalf("word %q lost from chunked output", w)
		}
	}
}

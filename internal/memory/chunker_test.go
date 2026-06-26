package memory

import (
	"fmt"
	"strings"
	"testing"
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
		wc := len(strings.Fields(c))
		if wc > 400 {
			t.Fatalf("chunk[%d] has %d words, want <= 400", i, wc)
		}
	}
}

func TestChunkText_OverlapContent(t *testing.T) {
	words := make([]string, 400)
	for i := range words {
		words[i] = fmt.Sprintf("w%d", i)
	}
	text := strings.Join(words, " ")
	chunks := chunkText(text, 200, 50)
	if len(chunks) < 2 {
		t.Skip("not enough chunks to test overlap")
	}
	chunk0Words := strings.Fields(chunks[0])
	chunk1Words := strings.Fields(chunks[1])
	last50of0 := strings.Join(chunk0Words[len(chunk0Words)-50:], " ")
	first50of1 := strings.Join(chunk1Words[:50], " ")
	if last50of0 != first50of1 {
		t.Fatalf("overlap mismatch:\nlast50_of_chunk0 = %q\nfirst50_of_chunk1 = %q", last50of0, first50of1)
	}
}

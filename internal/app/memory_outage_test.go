package app

import (
	"context"
	"errors"
	"memo/internal/config"
	"memo/internal/memory"
	"sync/atomic"
	"testing"
)

func TestMemoryOutageRetainsPinnedAndLexicalRecall(t *testing.T) {
	var down atomic.Bool
	var calls atomic.Int32
	store, err := memory.NewStore(memory.StoreConfig{Dir: t.TempDir(), Dimension: 4, EmbeddingFunc: func(context.Context, string) ([]float32, error) {
		calls.Add(1)
		if down.Load() {
			return nil, errors.New("connection refused")
		}
		return []float32{1, 0, 0, 0}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, fact := range []string{"My name is Ada", "I prefer Turkish replies"} {
		if err := store.SaveExplicit(context.Background(), memory.AllProjects, fact, "profile"); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SaveInteraction(context.Background(), memory.AllProjects, "The telescope project uses a silver mirror", "That telescope specification is recorded"); err != nil {
		t.Fatal(err)
	}
	down.Store(true)
	before := calls.Load()
	a := &App{store: store, cfg: &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: true, TopK: 5, PinnedFactsPerTurn: 1}}}
	got := a.retrieveMemory(context.Background(), memory.AllProjects, "telescope")
	pinned, lexical := 0, 0
	for _, r := range got {
		if r.MatchType == "pinned" {
			pinned++
		}
		if r.MatchType == "fts" {
			lexical++
		}
	}
	if pinned != 1 || lexical == 0 {
		t.Fatalf("pinned=%d lexical=%d results=%+v", pinned, lexical, got)
	}
	if calls.Load()-before != 1 {
		t.Fatal("retried failed embeddings for the pinned fallback")
	}
	if got[0].MatchType != "pinned" {
		t.Fatal("lexical recall crowded out pinned facts")
	}
}
func TestMemoryOutageRespectsCancelledCaller(t *testing.T) {
	store, err := memory.NewStore(memory.StoreConfig{Dir: t.TempDir(), Dimension: 4, EmbeddingFunc: func(ctx context.Context, _ string) ([]float32, error) { return nil, ctx.Err() }})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := &App{store: store, cfg: &config.AppConfig{Memory: config.MemoryConfig{TopK: 5, PinnedFactsPerTurn: 1}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := a.retrieveMemory(ctx, memory.AllProjects, "anything"); len(got) != 0 {
		t.Fatal("cancelled caller recalled memory")
	}
}

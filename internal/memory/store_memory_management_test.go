package memory

import (
	"context"
	"testing"
)

// TestDeleteByUUIDs_DeletesExactRowsOnly is the core safety property the
// Settings > Memory tab's checkbox-delete UI depends on: two pinned facts
// whose content deliberately overlaps as a substring ("User likes cats" is
// a substring of "User likes cats and dogs too") — exactly the shape that
// would over-match under DeleteByContent's LIKE %pattern% search. Deleting
// only the first one's uuid must leave the second untouched.
func TestDeleteByUUIDs_DeletesExactRowsOnly(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveExplicit(ctx, "User likes cats", ""); err != nil {
		t.Fatalf("SaveExplicit(cats) error = %v", err)
	}
	if err := store.SaveExplicit(ctx, "User likes cats and dogs too", ""); err != nil {
		t.Fatalf("SaveExplicit(cats and dogs) error = %v", err)
	}

	facts, err := store.GetPinnedFacts(ctx)
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("seeded 2 pinned facts, GetPinnedFacts returned %d", len(facts))
	}
	var targetID, keepID, keepContent string
	for _, f := range facts {
		if f.Content == "User likes cats" {
			targetID = f.ID
		} else {
			keepID = f.ID
			keepContent = f.Content
		}
	}
	if targetID == "" || keepID == "" {
		t.Fatalf("could not locate both seeded facts by content, got %+v", facts)
	}

	deleted, err := store.DeleteByUUIDs(ctx, []string{targetID})
	if err != nil {
		t.Fatalf("DeleteByUUIDs() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("DeleteByUUIDs() deleted = %d, want 1", deleted)
	}

	after, err := store.GetPinnedFacts(ctx)
	if err != nil {
		t.Fatalf("GetPinnedFacts() after delete error = %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("pinned facts after delete = %d, want 1 (deleted the wrong row(s)?): %+v", len(after), after)
	}
	if after[0].ID != keepID || after[0].Content != keepContent {
		t.Fatalf("surviving fact = %+v, want id=%q content=%q — the untouched row changed", after[0], keepID, keepContent)
	}
}

// TestDeleteByUUIDs_UnknownIDSkippedWithoutError covers a stale selection
// (the UI's list was refreshed elsewhere, or Dream/consolidation already
// retired the row) — must not error, must not touch anything else.
func TestDeleteByUUIDs_UnknownIDSkippedWithoutError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveExplicit(ctx, "User likes cats", ""); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}

	deleted, err := store.DeleteByUUIDs(ctx, []string{"explicit_does_not_exist"})
	if err != nil {
		t.Fatalf("DeleteByUUIDs(unknown id) error = %v, want nil", err)
	}
	if deleted != 0 {
		t.Fatalf("DeleteByUUIDs(unknown id) deleted = %d, want 0", deleted)
	}

	facts, err := store.GetPinnedFacts(ctx)
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("pinned facts after no-op delete = %d, want 1 (untouched)", len(facts))
	}
}

// TestDeleteByUUIDs_EmptyInputIsNoOp guards the checkbox UI's "nothing
// selected" state — must not, say, fall through to deleting everything.
func TestDeleteByUUIDs_EmptyInputIsNoOp(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveExplicit(ctx, "User likes cats", ""); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}

	deleted, err := store.DeleteByUUIDs(ctx, nil)
	if err != nil || deleted != 0 {
		t.Fatalf("DeleteByUUIDs(nil) = (%d, %v), want (0, nil)", deleted, err)
	}

	facts, _ := store.GetPinnedFacts(ctx)
	if len(facts) != 1 {
		t.Fatalf("pinned facts after empty-input delete = %d, want 1 (untouched)", len(facts))
	}
}

// TestListConversationMemories_ExcludesPinnedFacts is
// GetPinnedFacts'/DeleteByUUIDs' counterpart correctness check: the
// conversation-history browse list must show only source != 'explicit'
// rows, and totals must reflect the true matching count regardless of
// limit/offset paging.
func TestListConversationMemories_ExcludesPinnedFacts(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveExplicit(ctx, "User's dog is named Zeytin", ""); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}
	// Each topic must land in a different testEmbedding() bucket — same-
	// bucket content is silently deduped by findDuplicateInteraction (near-
	// duplicate guard), which would undercount this test's seed data
	// rather than exercise ListConversationMemories itself.
	topics := []string{"coffee beans", "mountain hiking", "a test query"}
	for i, topic := range topics {
		if err := store.SaveInteraction(ctx, topic, "reply about "+topic); err != nil {
			t.Fatalf("SaveInteraction(%d) error = %v", i, err)
		}
	}

	results, total, err := store.ListConversationMemories(ctx, 50, 0)
	if err != nil {
		t.Fatalf("ListConversationMemories() error = %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3 (the pinned fact must not be counted)", total)
	}
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	for _, r := range results {
		if r.Source == "explicit" {
			t.Fatalf("ListConversationMemories returned a pinned (source=explicit) row: %+v", r)
		}
	}

	// Pagination: same total, a smaller page.
	page, total2, err := store.ListConversationMemories(ctx, 2, 0)
	if err != nil {
		t.Fatalf("ListConversationMemories(limit=2) error = %v", err)
	}
	if total2 != 3 {
		t.Fatalf("total on page 1 = %d, want 3", total2)
	}
	if len(page) != 2 {
		t.Fatalf("page 1 results = %d, want 2", len(page))
	}
}

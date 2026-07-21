// SPDX-License-Identifier: AGPL-3.0-or-later

package routine

import (
	"os"
	"testing"
)

func TestStoreCreateGetListUpdateDelete(t *testing.T) {
	dir := t.TempDir()
	st, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	created, err := st.Create(Routine{Prompt: "günaydın", Schedule: Schedule{TimeOfDay: "08:00"}, Enabled: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected an assigned ID")
	}

	got, err := st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Prompt != "günaydın" {
		t.Errorf("got Prompt=%q, want %q", got.Prompt, "günaydın")
	}

	list := st.List()
	if len(list) != 1 {
		t.Fatalf("List: got %d routines, want 1", len(list))
	}

	got.Prompt = "iyi akşamlar"
	updated, err := st.Update(*got)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Prompt != "iyi akşamlar" {
		t.Errorf("after Update, Prompt=%q, want %q", updated.Prompt, "iyi akşamlar")
	}

	// A second Store instance over the same dir must see the persisted update.
	st2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore (reload): %v", err)
	}
	reloaded, err := st2.Get(created.ID)
	if err != nil {
		t.Fatalf("Get (reload): %v", err)
	}
	if reloaded.Prompt != "iyi akşamlar" {
		t.Errorf("reloaded Prompt=%q, want %q", reloaded.Prompt, "iyi akşamlar")
	}

	if err := st.Delete(created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := st.Get(created.ID); err == nil {
		t.Error("expected Get to fail after Delete")
	}
}

// TestStoreUpdate_AfterDelete_DoesNotResurrect covers the narrowed-lock-scope
// fix for BUG-L6: Update's disk write happens under a separate writeMu, not
// the map's mu, so a concurrent Delete can complete in between. Update
// re-checks existence right before applying its result to the in-memory map
// (under the same mu.Lock() as the mutation) so it can't resurrect a
// concurrently-deleted routine, and cleans up its own stray file write.
func TestStoreUpdate_AfterDelete_DoesNotResurrect(t *testing.T) {
	dir := t.TempDir()
	st, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	created, err := st.Create(Routine{Prompt: "x"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := st.Delete(created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := st.Update(*created); err == nil {
		t.Error("expected Update on a deleted routine to fail")
	}
	if _, err := st.Get(created.ID); err == nil {
		t.Error("Update should not have resurrected a deleted routine into the store")
	}
	if _, err := os.Stat(st.path(created.ID)); err == nil {
		t.Error("Update should have cleaned up its own stray file write for a deleted routine")
	}
}

// TestSyncUTCOffset_UpdatesNilAndDifferingOffsets covers both cases
// SyncUTCOffset needs to correct: a routine created before UTCOffsetMinutes
// existed (nil, falling back to host-local time), and one whose stored
// offset no longer matches the client's current wall-clock offset (a DST
// transition or relocation since it was created/last synced).
func TestSyncUTCOffset_UpdatesNilAndDifferingOffsets(t *testing.T) {
	dir := t.TempDir()
	st, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	noOffset, err := st.Create(Routine{Prompt: "a", Schedule: Schedule{TimeOfDay: "08:00"}})
	if err != nil {
		t.Fatalf("Create(noOffset): %v", err)
	}
	staleMinutes := 60
	stale, err := st.Create(Routine{Prompt: "b", Schedule: Schedule{TimeOfDay: "09:00", UTCOffsetMinutes: &staleMinutes}})
	if err != nil {
		t.Fatalf("Create(stale): %v", err)
	}

	changed, err := st.SyncUTCOffset(180)
	if err != nil {
		t.Fatalf("SyncUTCOffset: %v", err)
	}
	if changed != 2 {
		t.Fatalf("SyncUTCOffset() changed = %d, want 2", changed)
	}

	for _, id := range []string{noOffset.ID, stale.ID} {
		got, err := st.Get(id)
		if err != nil {
			t.Fatalf("Get(%s): %v", id, err)
		}
		if got.Schedule.UTCOffsetMinutes == nil || *got.Schedule.UTCOffsetMinutes != 180 {
			t.Errorf("routine %s: UTCOffsetMinutes = %v, want 180", id, got.Schedule.UTCOffsetMinutes)
		}
	}
}

// TestSyncUTCOffset_NoOpWhenAlreadyMatching confirms a routine whose stored
// offset already matches the given value isn't rewritten — this keeps
// calling SyncUTCOffset on every client reconnect cheap in the common case
// (no DST transition, no relocation since the last sync).
func TestSyncUTCOffset_NoOpWhenAlreadyMatching(t *testing.T) {
	dir := t.TempDir()
	st, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	minutes := 180
	created, err := st.Create(Routine{Prompt: "a", Schedule: Schedule{TimeOfDay: "08:00", UTCOffsetMinutes: &minutes}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	beforeUpdatedAt := created.UpdatedAt

	changed, err := st.SyncUTCOffset(180)
	if err != nil {
		t.Fatalf("SyncUTCOffset: %v", err)
	}
	if changed != 0 {
		t.Fatalf("SyncUTCOffset() changed = %d, want 0 (offset already matches)", changed)
	}

	got, err := st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.UpdatedAt.Equal(beforeUpdatedAt) {
		t.Errorf("UpdatedAt changed even though SyncUTCOffset should have been a no-op for this routine")
	}
}

// TestSyncUTCOffset_PersistsAcrossReload confirms the correction survives a
// process restart (a fresh Store instance over the same directory), not just
// the in-memory map — SyncUTCOffset's whole point is to fix the value a
// later routine-loop tick would otherwise read back from disk.
func TestSyncUTCOffset_PersistsAcrossReload(t *testing.T) {
	dir := t.TempDir()
	st, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	created, err := st.Create(Routine{Prompt: "a", Schedule: Schedule{TimeOfDay: "08:00"}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := st.SyncUTCOffset(-300); err != nil {
		t.Fatalf("SyncUTCOffset: %v", err)
	}

	st2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore (reload): %v", err)
	}
	reloaded, err := st2.Get(created.ID)
	if err != nil {
		t.Fatalf("Get (reload): %v", err)
	}
	if reloaded.Schedule.UTCOffsetMinutes == nil || *reloaded.Schedule.UTCOffsetMinutes != -300 {
		t.Errorf("reloaded UTCOffsetMinutes = %v, want -300", reloaded.Schedule.UTCOffsetMinutes)
	}
}

func TestStoreGetDeleteUnknownID(t *testing.T) {
	st, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if _, err := st.Get("does-not-exist"); err == nil {
		t.Error("expected error for unknown ID")
	}
	if err := st.Delete("does-not-exist"); err == nil {
		t.Error("expected error deleting unknown ID")
	}
}

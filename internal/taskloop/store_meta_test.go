package taskloop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_SetStatus_RejectsUnknown(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("chat1", "Test", []string{"item"})

	if err := store.SetStatus(tl.ID, "banana"); err == nil {
		t.Fatal("SetStatus accepted an invalid status")
	}
	// A valid new phase must be accepted.
	if err := store.SetStatus(tl.ID, taskListExecuting); err != nil {
		t.Fatalf("SetStatus(executing): %v", err)
	}
	got, _ := store.Get(tl.ID)
	if got.Status != taskListExecuting {
		t.Fatalf("Status = %q, want %q", got.Status, taskListExecuting)
	}
}

func TestStore_CreateWithMeta_PersistsNotifyAndPath(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)

	tl, err := store.CreateWithMeta("chat1", "T", []string{"a", "b"}, NotifyEverything, "/repo/Task.md")
	if err != nil {
		t.Fatalf("CreateWithMeta: %v", err)
	}
	if tl.NotifyLevel != NotifyEverything || tl.TaskMdPath != "/repo/Task.md" {
		t.Fatalf("meta not set: %+v", tl)
	}

	// Survives a reload from disk.
	store2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got, _ := store2.Get(tl.ID)
	if got.NotifyLevel != NotifyEverything || got.TaskMdPath != "/repo/Task.md" {
		t.Fatalf("meta not persisted: %+v", got)
	}
}

func TestStore_Create_DefaultsNotifyImportant(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("chat1", "T", []string{"a"})
	if tl.NotifyLevel != NotifyImportant {
		t.Fatalf("NotifyLevel = %q, want %q", tl.NotifyLevel, NotifyImportant)
	}
}

func TestStore_LoadAll_RecoversActivePhases(t *testing.T) {
	for _, phase := range []string{taskListRunning, taskListPlanning, taskListExecuting} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			store, _ := NewStore(dir)
			tl, _ := store.Create("chat1", "T", []string{"a", "b"})
			_ = store.SetStatus(tl.ID, phase)
			_ = store.SetItemRunning(tl.ID, tl.Items[0].ID)

			store2, err := NewStore(dir)
			if err != nil {
				t.Fatalf("reload: %v", err)
			}
			got, _ := store2.Get(tl.ID)
			if got.Status != taskListPaused {
				t.Fatalf("Status = %q, want %q", got.Status, taskListPaused)
			}
			if got.Items[0].Status != "pending" || got.Items[0].StartedAt != "" {
				t.Fatalf("half-run item not reset: %+v", got.Items[0])
			}
		})
	}
}

func TestStore_LoadAll_KeepsWaitingStates(t *testing.T) {
	for _, phase := range []string{taskListWaitingLimit, taskListWaitingUser} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			store, _ := NewStore(dir)
			tl, _ := store.Create("chat1", "T", []string{"a"})
			_ = store.SetStatus(tl.ID, phase)

			store2, err := NewStore(dir)
			if err != nil {
				t.Fatalf("reload: %v", err)
			}
			got, _ := store2.Get(tl.ID)
			if got.Status != phase {
				t.Fatalf("Status = %q, want kept as %q", got.Status, phase)
			}
		})
	}
}

func TestStore_LoadAll_LegacyRunningJSON(t *testing.T) {
	dir := t.TempDir()
	// Simulate a pre-v4.4.0 file with the old "running" status and no meta.
	legacy := `{"id":"abc","chat_id":"c","title":"old","items":[{"id":"i1","text":"x","status":"running"}],"status":"running","created_at":"2026-01-01 00:00","updated_at":"2026-01-01 00:00"}`
	if err := os.WriteFile(filepath.Join(dir, "abc.json"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	got, err := store.Get("abc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != taskListPaused {
		t.Fatalf("legacy running not recovered: %q", got.Status)
	}
	if got.Items[0].Status != "pending" {
		t.Fatalf("legacy running item not reset: %q", got.Items[0].Status)
	}
	if got.NotifyLevel != "" {
		t.Fatalf("legacy file gained a notify level: %q", got.NotifyLevel)
	}
}

func TestStore_SetStatus_AllNewPhases(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("chat1", "T", []string{"a"})
	for _, s := range []string{
		taskListPlanning, taskListExecuting, taskListWaitingLimit,
		taskListWaitingUser, taskListPaused, taskListDone, taskListFailed, taskListCancelled,
	} {
		if err := store.SetStatus(tl.ID, s); err != nil {
			t.Fatalf("SetStatus(%q): %v", s, err)
		}
	}
}

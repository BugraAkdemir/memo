package taskloop

import (
	"path/filepath"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "tasklists"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestStore_PlanPersistenceAndMode(t *testing.T) {
	s := newStore(t)
	tl, err := s.Create("chat1", "t", []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}

	if s.HasPlan(tl.ID) {
		t.Fatal("HasPlan true before any SavePlan")
	}
	if err := s.SetMode(tl.ID, ModePlanner); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	if got, _ := s.Get(tl.ID); got.Mode != ModePlanner {
		t.Fatalf("Mode = %q, want %q", got.Mode, ModePlanner)
	}

	plan := Plan{Steps: []PlanStep{
		{ID: "S1", ItemID: "1", Text: "do a"},
		{ID: "S2", ItemID: "2", Text: "do b", DependsOn: []string{"S1"}},
	}}
	if err := s.SavePlan(tl.ID, plan); err != nil {
		t.Fatalf("SavePlan: %v", err)
	}
	if !s.HasPlan(tl.ID) {
		t.Fatal("HasPlan false after SavePlan")
	}

	got, err := s.GetPlan(tl.ID)
	if err != nil {
		t.Fatalf("GetPlan: %v", err)
	}
	if got.ListID != tl.ID || len(got.Steps) != 2 {
		t.Fatalf("GetPlan = %+v", got)
	}

	if err := s.SetStepStatus(tl.ID, "S1", "done", ""); err != nil {
		t.Fatalf("SetStepStatus: %v", err)
	}
	n, err := s.IncrementStepAttempts(tl.ID, "S2")
	if err != nil || n != 1 {
		t.Fatalf("IncrementStepAttempts = %d, %v", n, err)
	}
	got, _ = s.GetPlan(tl.ID)
	if got.Steps[0].Status != "done" || got.Steps[1].Attempts != 1 {
		t.Fatalf("mutations not persisted: %+v", got.Steps)
	}

	// Delete removes the plan file too; a fresh store must not see it.
	if err := s.Delete(tl.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	s2, err := NewStore(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if s2.HasPlan(tl.ID) {
		t.Fatal("plan file survived Delete")
	}
	if _, err := s2.Get(tl.ID); err == nil {
		t.Fatal("list survived Delete")
	}
}

func TestStore_LoadAllIgnoresPlanFiles(t *testing.T) {
	s := newStore(t)
	tl, _ := s.Create("c", "t", []string{"x"})
	if err := s.SavePlan(tl.ID, Plan{Steps: []PlanStep{{ID: "S1", Text: "x"}}}); err != nil {
		t.Fatal(err)
	}
	// Reload: the .plan.json sibling must not become a phantom TaskList.
	s2, err := NewStore(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.List(); len(got) != 1 || got[0].ID != tl.ID {
		t.Fatalf("List after reload = %+v, want exactly the one real list", got)
	}
}

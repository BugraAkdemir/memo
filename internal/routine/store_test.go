// SPDX-License-Identifier: AGPL-3.0-or-later

package routine

import "testing"

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

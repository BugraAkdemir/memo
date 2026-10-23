package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasPersistedCompletionDoesNotTrustFailedSave(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	id := m.NewChat()
	events := []interface{}{map[string]interface{}{"type": "subagent_completion", "completion_id": uint64(7)}}
	// Force the session save to fail without relying on Unix permissions/root.
	target := filepath.Join(dir, id+".json")
	if err = os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	m.AddMessageToSession(id, "assistant", "accepted", "", "", events)
	if m.HasPersistedCompletion(id, 7) {
		t.Fatal("acknowledged memory-only completion")
	}
	if err = os.Remove(target); err != nil {
		t.Fatal(err)
	}
	m.AddMessageToSession(id, "assistant", "saved", "", "", events)
	if !m.HasPersistedCompletion(id, 7) {
		t.Fatal("saved marker not found")
	}
	if m.HasPersistedCompletion(id, 8) {
		t.Fatal("wrong completion matched")
	}
}

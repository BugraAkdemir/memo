package e2e

import (
	"io"
	"memo/internal/cloudsync"
	"testing"
)

func TestEncryptedBackupRESTUsesPOSTAndRejectsWrongPassword(t *testing.T) {
	h := NewHarness(t)
	resp := h.postJSON("/api/export", map[string]any{"password": "portable-backup-password", "include_models": false})
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 200 || !cloudsync.IsEncryptedBackup(data) {
		t.Fatalf("encrypted export status=%d err=%v", resp.StatusCode, err)
	}
	bad := h.postJSON("/api/import", map[string]any{"data": data, "password": "wrong-password"})
	bad.Body.Close()
	if bad.StatusCode < 400 {
		t.Fatal("wrong password accepted")
	}
	// Successful import/reopen is covered by independent device-key app tests.
}

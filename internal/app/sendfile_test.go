package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDeliverFile_SelfChatSourceForcesThatChannel mirrors
// TestResolveRoutineDeliveryTarget_SelfChatSourceIsAlwaysForced's reasoning
// (routine_test.go): a self-chat source on ctx must route there, never fall
// through to the outbox/frontend path — confirmed here with waClient/
// tgClient both left nil, which the not-connected branches must handle
// without touching the underlying client value.
//
// consumed must be true even though these are error paths: there is no
// retry mechanism, so a caller-owned temp zip has nothing left to be used
// for regardless of whether the send actually succeeded — this was a real
// bug (found on review, not by a user report) where consumed=false here
// silently leaked a temp zip on every failed/not-connected send.
func TestDeliverFile_SelfChatSourceForcesThatChannel(t *testing.T) {
	a := &App{} // waClient, tgClient both nil

	t.Run("whatsapp_source_not_connected", func(t *testing.T) {
		ctx := withSelfChatSource(context.Background(), SelfChatSource{WhatsApp: true, WhatsAppJID: "905551234567@s.whatsapp.net"})
		msg, consumed, err := a.DeliverFile(ctx, "/tmp/whatever.zip", "whatever.zip", true)
		if err == nil {
			t.Fatal("DeliverFile() with nil waClient should error, got nil")
		}
		if !consumed {
			t.Error("consumed should be true even on error — no retry path exists, a temp zip has nothing left to wait for")
		}
		if msg != "" {
			t.Errorf("message should be empty on error, got %q", msg)
		}
	})

	t.Run("telegram_source_not_connected", func(t *testing.T) {
		ctx := withSelfChatSource(context.Background(), SelfChatSource{Telegram: true, TelegramChatID: 555})
		msg, consumed, err := a.DeliverFile(ctx, "/tmp/whatever.zip", "whatever.zip", true)
		if err == nil {
			t.Fatal("DeliverFile() with nil tgClient should error, got nil")
		}
		if !consumed {
			t.Error("consumed should be true even on error — no retry path exists, a temp zip has nothing left to wait for")
		}
		if msg != "" {
			t.Errorf("message should be empty on error, got %q", msg)
		}
	})
}

// TestDeliverFile_NoSource_RegistersOutboxLink confirms the normal/frontend
// chat path (no self-chat source on ctx) never touches WhatsApp/Telegram at
// all — it stages the file in the outbox and returns a clickable markdown
// link whose href round-trips through GetOutboxFile back to the exact
// path/filename that was staged.
func TestDeliverFile_NoSource_RegistersOutboxLink(t *testing.T) {
	a := &App{}
	msg, consumed, err := a.DeliverFile(context.Background(), "/tmp/report.zip", "report.zip", true)
	if err != nil {
		t.Fatalf("DeliverFile() error = %v", err)
	}
	if consumed {
		t.Error("consumed should be false — the outbox still needs the file on disk")
	}
	if !strings.Contains(msg, "report.zip") {
		t.Errorf("message %q should mention the display filename", msg)
	}
	if !strings.Contains(msg, "/api/files/outbox/") {
		t.Errorf("message %q should contain a relative outbox download link", msg)
	}

	start := strings.Index(msg, "/api/files/outbox/")
	token := strings.TrimSuffix(msg[start+len("/api/files/outbox/"):], ")")
	path, filename, ok := a.GetOutboxFile(token)
	if !ok {
		t.Fatalf("GetOutboxFile(%q) not found, want the staged entry", token)
	}
	if path != "/tmp/report.zip" || filename != "report.zip" {
		t.Errorf("GetOutboxFile(%q) = (%q, %q), want (/tmp/report.zip, report.zip)", token, path, filename)
	}
}

// TestGetOutboxFile_UnknownTokenNotFound confirms a token that was never
// registered (guessed, or from a different backend process) is reported as
// not found rather than panicking on a nil map.
func TestGetOutboxFile_UnknownTokenNotFound(t *testing.T) {
	a := &App{}
	if _, _, ok := a.GetOutboxFile("nonexistent"); ok {
		t.Error("GetOutboxFile() for an unregistered token should return ok=false")
	}
}

// TestGetOutboxFile_ExpiredTempFileIsDeletedFromDisk confirms expiry cleanup
// actually reclaims a temp zip nobody ever downloaded — not just the map
// entry, but the file itself, which was a real bug: the original
// implementation only ever did delete(a.outbox, token) on expiry, leaking
// every unclaimed temp zip in os.TempDir() permanently.
func TestGetOutboxFile_ExpiredTempFileIsDeletedFromDisk(t *testing.T) {
	a := &App{}
	tmp, err := os.CreateTemp(t.TempDir(), "expired-*.zip")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	tmp.Close()
	a.outbox = map[string]outboxEntry{
		"stale-token": {path: tmp.Name(), filename: "old.zip", expiresAt: time.Now().Add(-time.Minute), isTempFile: true},
	}
	if _, _, ok := a.GetOutboxFile("stale-token"); ok {
		t.Error("GetOutboxFile() for an expired token should return ok=false")
	}
	a.outboxMu.Lock()
	_, stillThere := a.outbox["stale-token"]
	a.outboxMu.Unlock()
	if stillThere {
		t.Error("expired entry should be pruned from the map on lookup, not just reported as not-found")
	}
	if _, err := os.Stat(tmp.Name()); !os.IsNotExist(err) {
		t.Errorf("expired temp file should be deleted from disk, stat err = %v", err)
	}
}

// TestGetOutboxFile_ExpiredRealUserFileIsNeverDeleted is the critical
// counterpart to the test above: a single-file share (isTempFile=false)
// points directly at the user's own real file — GetOutboxFile's expiry
// cleanup must NEVER delete it, no matter how long ago the download link
// expired. Deleting a temp zip on expiry is a cleanup; deleting a user's
// real file because a link expired would be a serious correctness bug.
func TestGetOutboxFile_ExpiredRealUserFileIsNeverDeleted(t *testing.T) {
	a := &App{}
	realFile := filepath.Join(t.TempDir(), "my-real-document.txt")
	if err := os.WriteFile(realFile, []byte("do not delete me"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	a.outbox = map[string]outboxEntry{
		"stale-token": {path: realFile, filename: "my-real-document.txt", expiresAt: time.Now().Add(-time.Minute), isTempFile: false},
	}
	if _, _, ok := a.GetOutboxFile("stale-token"); ok {
		t.Error("GetOutboxFile() for an expired token should return ok=false")
	}
	if _, err := os.Stat(realFile); err != nil {
		t.Fatalf("the user's real file must survive link expiry, stat err = %v", err)
	}
}

// TestRegisterOutboxFile_PrunesExpiredEntriesOnNewRegistration confirms the
// outbox doesn't grow unbounded over a long backend uptime — an old,
// already-expired temp entry gets swept out (map entry AND underlying
// file) the next time any new file is staged, not only when someone
// happens to look it up directly.
func TestRegisterOutboxFile_PrunesExpiredEntriesOnNewRegistration(t *testing.T) {
	a := &App{}
	tmp, err := os.CreateTemp(t.TempDir(), "expired-*.zip")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	tmp.Close()
	a.outbox = map[string]outboxEntry{
		"stale-token": {path: tmp.Name(), filename: "old.zip", expiresAt: time.Now().Add(-time.Minute), isTempFile: true},
	}
	a.registerOutboxFile("/tmp/new.zip", "new.zip", true)

	a.outboxMu.Lock()
	defer a.outboxMu.Unlock()
	if _, stillThere := a.outbox["stale-token"]; stillThere {
		t.Error("registerOutboxFile() should prune already-expired entries, not just add the new one")
	}
	if len(a.outbox) != 1 {
		t.Errorf("outbox should contain exactly the one fresh entry, got %d entries", len(a.outbox))
	}
	if _, err := os.Stat(tmp.Name()); !os.IsNotExist(err) {
		t.Errorf("pruned temp file should be deleted from disk, stat err = %v", err)
	}
}

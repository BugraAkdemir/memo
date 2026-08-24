package app

import (
	"context"
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
func TestDeliverFile_SelfChatSourceForcesThatChannel(t *testing.T) {
	a := &App{} // waClient, tgClient both nil

	t.Run("whatsapp_source_not_connected", func(t *testing.T) {
		ctx := withSelfChatSource(context.Background(), SelfChatSource{WhatsApp: true, WhatsAppJID: "905551234567@s.whatsapp.net"})
		msg, consumed, err := a.DeliverFile(ctx, "/tmp/whatever.zip", "whatever.zip")
		if err == nil {
			t.Fatal("DeliverFile() with nil waClient should error, got nil")
		}
		if consumed {
			t.Error("consumed should be false on error")
		}
		if msg != "" {
			t.Errorf("message should be empty on error, got %q", msg)
		}
	})

	t.Run("telegram_source_not_connected", func(t *testing.T) {
		ctx := withSelfChatSource(context.Background(), SelfChatSource{Telegram: true, TelegramChatID: 555})
		msg, consumed, err := a.DeliverFile(ctx, "/tmp/whatever.zip", "whatever.zip")
		if err == nil {
			t.Fatal("DeliverFile() with nil tgClient should error, got nil")
		}
		if consumed {
			t.Error("consumed should be false on error")
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
	msg, consumed, err := a.DeliverFile(context.Background(), "/tmp/report.zip", "report.zip")
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

// TestGetOutboxFile_ExpiredTokenNotFound confirms outboxTTL is actually
// enforced, not just recorded — an entry older than the TTL must read back
// as not-found (and get pruned), not silently keep serving forever.
func TestGetOutboxFile_ExpiredTokenNotFound(t *testing.T) {
	a := &App{}
	a.outbox = map[string]outboxEntry{
		"stale-token": {path: "/tmp/old.zip", filename: "old.zip", expiresAt: time.Now().Add(-time.Minute)},
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
}

// TestRegisterOutboxFile_PrunesExpiredEntriesOnNewRegistration confirms the
// outbox doesn't grow unbounded over a long backend uptime — an old,
// already-expired entry gets swept out the next time any new file is
// staged, not only when someone happens to look it up directly.
func TestRegisterOutboxFile_PrunesExpiredEntriesOnNewRegistration(t *testing.T) {
	a := &App{}
	a.outbox = map[string]outboxEntry{
		"stale-token": {path: "/tmp/old.zip", filename: "old.zip", expiresAt: time.Now().Add(-time.Minute)},
	}
	a.registerOutboxFile("/tmp/new.zip", "new.zip")

	a.outboxMu.Lock()
	defer a.outboxMu.Unlock()
	if _, stillThere := a.outbox["stale-token"]; stillThere {
		t.Error("registerOutboxFile() should prune already-expired entries, not just add the new one")
	}
	if len(a.outbox) != 1 {
		t.Errorf("outbox should contain exactly the one fresh entry, got %d entries", len(a.outbox))
	}
}

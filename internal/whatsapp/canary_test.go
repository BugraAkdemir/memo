//go:build canary

package whatsapp

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
)

// TestCanary_WhatsAppHandshake opens a fresh (unpaired) whatsmeow session
// and calls ConnectContext. With Store.ID == nil, whatsmeow uses the
// pre-login path and completes the Noise handshake against WhatsApp's
// servers without any phone number or existing session — the same path
// that produces a QR code during real pairing.
//
// This only catches "whatsmeow can no longer talk to WhatsApp at all"
// (protocol/endpoint breakage). It does not cover messaging or account-
// specific bans. Only runs under -tags canary (see .github/workflows/canary.yml).
func TestCanary_WhatsAppHandshake(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sessionDB := filepath.Join(t.TempDir(), "session.db")
	storeDB, err := sqlstore.New(ctx, "sqlite3",
		"file:"+sessionDB+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", nil)
	if err != nil {
		t.Fatalf("sqlstore.New: %v", err)
	}
	defer storeDB.Close()

	deviceStore, err := storeDB.GetFirstDevice(ctx)
	if err != nil {
		t.Fatalf("GetFirstDevice: %v", err)
	}
	if deviceStore == nil {
		t.Fatal("GetFirstDevice returned nil device")
	}
	// Fresh store: no paired session → ID must be nil so Connect uses preLoginHTTP.
	if deviceStore.ID != nil {
		t.Fatal("expected unregistered device (ID == nil) for a fresh session store")
	}

	client := whatsmeow.NewClient(deviceStore, nil)
	if client == nil {
		t.Fatal("whatsmeow.NewClient returned nil")
	}
	// Avoid a background reconnect loop holding the process if handshake fails.
	client.EnableAutoReconnect = false

	if err := client.ConnectContext(ctx); err != nil {
		t.Fatalf("ConnectContext (Noise handshake without session): %v", err)
	}
	defer client.Disconnect()

	if !client.IsConnected() {
		t.Fatal("ConnectContext succeeded but IsConnected() is false")
	}
	t.Log("ok: WhatsApp Noise handshake completed without a paired session")
}

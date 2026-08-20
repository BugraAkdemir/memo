package whatsapp

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

// TestOwnJID_StripsDeviceSuffix is the regression test for the AD-vs-non-AD
// JID mixup this whole feature hinges on: wa.Store.ID is this device's own
// JID *with* its device suffix (e.g. "9055...:5@s.whatsapp.net"), but a
// message's ChatJID for the "Message Yourself" self-chat never carries one
// — comparing against Store.ID.String() directly would never match, so
// self-chat detection would silently never fire. OwnJID must return the
// stripped (non-AD) form.
func TestOwnJID_StripsDeviceSuffix(t *testing.T) {
	jid := types.JID{User: "905555555555", Device: 5, Server: types.DefaultUserServer}
	c := &Client{waClient: &whatsmeow.Client{Store: &store.Device{ID: &jid}}}

	got := c.OwnJID()
	want := "905555555555@s.whatsapp.net"
	if got != want {
		t.Errorf("OwnJID() = %q, want %q (device suffix must be stripped)", got, want)
	}
}

// TestOwnJID_NotConnectedReturnsEmpty guards the pre-pairing/pre-connect
// state (nil waClient, or a waClient with no Store yet) — must return "",
// not panic, so callers checking self-chat before WhatsApp is even
// connected fail safe.
func TestOwnJID_NotConnectedReturnsEmpty(t *testing.T) {
	if got := (&Client{}).OwnJID(); got != "" {
		t.Errorf("OwnJID() on a nil waClient = %q, want empty", got)
	}
	if got := (&Client{waClient: &whatsmeow.Client{}}).OwnJID(); got != "" {
		t.Errorf("OwnJID() with a nil Store = %q, want empty", got)
	}
}

// TestMarkSelfSent_IsSelfSentRecently covers the loop-prevention guard: a
// message ID this client just sent via SendMessage must be recognized as
// "our own" so an incoming-message handler doesn't treat it as a fresh
// command and reply to its own reply.
func TestMarkSelfSent_IsSelfSentRecently(t *testing.T) {
	c := &Client{}

	if c.IsSelfSentRecently("msg-1") {
		t.Fatal("IsSelfSentRecently() = true before markSelfSent was ever called")
	}

	c.markSelfSent("msg-1")
	if !c.IsSelfSentRecently("msg-1") {
		t.Fatal("IsSelfSentRecently(\"msg-1\") = false right after markSelfSent")
	}
	if c.IsSelfSentRecently("msg-2") {
		t.Fatal("IsSelfSentRecently(\"msg-2\") = true, want false — a different ID was never marked")
	}

	// Empty IDs must be a no-op, not a wildcard match.
	c.markSelfSent("")
	if c.IsSelfSentRecently("") {
		t.Fatal("IsSelfSentRecently(\"\") = true — markSelfSent(\"\") should have been a no-op")
	}
}

// TestMarkSelfSent_CleansUpStaleEntriesPastCap confirms the map doesn't
// grow unbounded across a long session: once past the 64-entry cap, an
// entry older than 5 minutes is swept on the next markSelfSent call.
func TestMarkSelfSent_CleansUpStaleEntriesPastCap(t *testing.T) {
	c := &Client{selfSentIDs: make(map[string]time.Time)}

	stale := "stale-id"
	c.selfSentIDs[stale] = time.Now().Add(-10 * time.Minute)
	for i := 0; i < 64; i++ {
		c.selfSentIDs[string(rune('a'+i%26))+string(rune(i))] = time.Now()
	}

	c.markSelfSent("trigger-cleanup")

	if c.IsSelfSentRecently(stale) {
		t.Error("stale entry (>5min old) should have been swept once the map passed the 64-entry cap")
	}
	if !c.IsSelfSentRecently("trigger-cleanup") {
		t.Error("the entry that triggered cleanup should itself still be recorded")
	}
}

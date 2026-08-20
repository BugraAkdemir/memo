package whatsapp

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

// TestOwnJIDs_StripsDeviceSuffix is the regression test for the AD-vs-non-AD
// JID mixup this whole feature hinges on: wa.Store.ID is this device's own
// JID *with* its device suffix (e.g. "9055...:5@s.whatsapp.net"), but a
// message's ChatJID for the "Message Yourself" self-chat never carries one
// — comparing against Store.ID.String() directly would never match, so
// self-chat detection would silently never fire. OwnJIDs must return the
// stripped (non-AD) form.
func TestOwnJIDs_StripsDeviceSuffix(t *testing.T) {
	jid := types.JID{User: "905555555555", Device: 5, Server: types.DefaultUserServer}
	c := &Client{waClient: &whatsmeow.Client{Store: &store.Device{ID: &jid}}}

	got := c.OwnJIDs()
	want := "905555555555@s.whatsapp.net"
	if len(got) != 1 || got[0] != want {
		t.Errorf("OwnJIDs() = %v, want [%q] (device suffix must be stripped)", got, want)
	}
}

// TestOwnJIDs_IncludesLID is the regression test for the actual reported
// bug: confirmed live against a real WhatsApp account, a genuine self-chat
// message's ChatJID arrived as "<n>@lid" — WhatsApp's Linked-ID form — not
// "<phone>@s.whatsapp.net" like Store.ID. Checking only Store.ID (the
// original, pre-fix behavior) meant self-chat was silently never
// recognized: the self-chat assistant just never replied to anything, with
// no error anywhere. OwnJIDs must include both forms.
func TestOwnJIDs_IncludesLID(t *testing.T) {
	id := types.JID{User: "905555555555", Device: 5, Server: types.DefaultUserServer}
	lid := types.JID{User: "110874714980365", Server: types.HiddenUserServer}
	c := &Client{waClient: &whatsmeow.Client{Store: &store.Device{ID: &id, LID: lid}}}

	got := c.OwnJIDs()
	wantPhone := "905555555555@s.whatsapp.net"
	wantLID := "110874714980365@lid"
	if len(got) != 2 || got[0] != wantPhone || got[1] != wantLID {
		t.Errorf("OwnJIDs() = %v, want [%q, %q]", got, wantPhone, wantLID)
	}
}

// TestOwnJIDs_NotConnectedReturnsEmpty guards the pre-pairing/pre-connect
// state (nil waClient, or a waClient with no Store yet) — must return nil,
// not panic, so callers checking self-chat before WhatsApp is even
// connected fail safe.
func TestOwnJIDs_NotConnectedReturnsEmpty(t *testing.T) {
	if got := (&Client{}).OwnJIDs(); len(got) != 0 {
		t.Errorf("OwnJIDs() on a nil waClient = %v, want empty", got)
	}
	if got := (&Client{waClient: &whatsmeow.Client{}}).OwnJIDs(); len(got) != 0 {
		t.Errorf("OwnJIDs() with a nil Store = %v, want empty", got)
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

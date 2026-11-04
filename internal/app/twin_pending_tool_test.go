package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/twin"
)

// Found live: with no way to list drafts the chat model answered "no pending
// drafts" while four existed. The tool must show only drafts still waiting for
// the owner, with a bounded preview, never finished or cancelled ones.
func TestTwinPendingActionsToolListsOnlyOpenDraftsWithBoundedPreview(t *testing.T) {
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	ctx := context.Background()
	a, s, _ := radarApp(t, 48)
	long := "Merhaba, " + strings.Repeat("uzun metin ", 80) + "ARKA-SIR-SONDA"
	now := time.Now()
	for _, v := range []twin.Action{
		{ID: "open1", Channel: "mail", Account: "acc", Recipient: "ali@example.com", Subject: "ACIK-TASLAK", Body: long, Mode: "awaiting_approval", State: "pending", Reason: "drafted", CreatedAt: now},
		{ID: "auto1", Channel: "mail", Account: "acc", Recipient: "veli@example.com", Subject: "OTOMATIK-ADAY", Body: "kisa", Mode: "grace_period_auto", State: "pending", Reason: "drafted", CreatedAt: now},
		{ID: "gone1", Channel: "mail", Account: "acc", Recipient: "x@example.com", Subject: "IPTAL-EDILDI", Body: "x", Mode: "awaiting_approval", State: "cancelled", Reason: "user_cancelled", CreatedAt: now},
		{ID: "done1", Channel: "mail", Account: "acc", Recipient: "y@example.com", Subject: "GONDERILDI", Body: "y", Mode: "awaiting_approval", State: "sent", Reason: "sent", CreatedAt: now},
	} {
		v.DeliveryVersion = 1
		if err := s.SaveAction(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	out, err := a.CallCommunicationTool(ctx, "twin_pending_actions", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ACIK-TASLAK") || !strings.Contains(out, "OTOMATIK-ADAY") {
		t.Fatalf("open drafts missing: %s", out)
	}
	if strings.Contains(out, "IPTAL-EDILDI") || strings.Contains(out, "GONDERILDI") {
		t.Fatalf("finished drafts must not be listed: %s", out)
	}
	if strings.Contains(out, "ARKA-SIR-SONDA") {
		t.Fatalf("preview must be bounded: %s", out)
	}
	if !strings.Contains(out, "grace_period_auto") || !strings.Contains(out, "awaiting_approval") {
		t.Fatalf("the owner must see which draft would send itself: %s", out)
	}
}

package app

import (
	"context"
	"testing"
)

func TestTaskMemoryDisabledCtx(t *testing.T) {
	base := context.Background()
	if taskMemoryDisabled(base) {
		t.Fatal("plain context should not be marked")
	}
	if !taskMemoryDisabled(withTaskMemoryDisabled(base)) {
		t.Fatal("withTaskMemoryDisabled did not take")
	}
	// Independent of the current-chat-id key.
	ctx := withCurrentChatID(withTaskMemoryDisabled(base), "c1")
	if !taskMemoryDisabled(ctx) || currentChatIDFromContext(ctx) != "c1" {
		t.Fatalf("keys interfere: disabled=%v chat=%q", taskMemoryDisabled(ctx), currentChatIDFromContext(ctx))
	}
}

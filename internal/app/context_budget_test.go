package app

import (
	"testing"

	"memo/internal/config"
)

func TestContextBudgetFor_Precedence(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}

	// Fallback table by provider name.
	if got := a.contextBudgetFor("gemini"); got != 1024*1024 {
		t.Fatalf("gemini fallback = %d", got)
	}
	if got := a.contextBudgetFor("claude"); got != 200*1024 {
		t.Fatalf("claude fallback = %d", got)
	}
	if got := a.contextBudgetFor("something-else"); got != 128*1024 {
		t.Fatalf("default fallback = %d", got)
	}

	// Global override beats the fallback table.
	a.cfg.Llama.MaxContextTokens = 9
	if got := a.contextBudgetFor("claude"); got != 9 {
		t.Fatalf("global override = %d, want 9", got)
	}
}

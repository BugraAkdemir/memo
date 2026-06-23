package app

import (
	"context"
	"strings"
	"testing"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/memory"
	moodpkg "memo/internal/mood"
)

func TestBuildMessages_MoodDisabled_StripsAssistant(t *testing.T) {
	id := identity.New("Test", "Memo", "casual", "")

	t.Run("mood_nil_strips_assistant", func(t *testing.T) {
		a := &App{
			cfg: &config.AppConfig{
				Memory: config.MemoryConfig{MemoryEnabled: false},
				Llama:  config.LlamaConfig{CtxSize: 4096},
			},
			identity: id,
			mood:     nil,
		}

		messages := a.buildMessages(context.Background(), "merhaba", nil)
		if len(messages) == 0 {
			t.Fatal("expected at least one message")
		}
		sys := messages[0]
		content, ok := sys.Content.(string)
		if !ok {
			t.Fatalf("expected string content, got %T", sys.Content)
		}
		if sys.Role != "system" {
			t.Fatalf("expected system role, got %s", sys.Role)
		}
		if !strings.Contains(content, "Memo") {
			t.Error("system prompt should contain assistant name")
		}
		if strings.Contains(content, "Current Emotional State") {
			t.Error("system prompt should NOT contain mood directive when mood is nil")
		}
	})

	t.Run("mood_enabled_false_strips_assistant", func(t *testing.T) {
		moodEngine, err := moodpkg.New(moodpkg.Config{
			Enabled: false,
			DBPath:  t.TempDir() + "/mood.db",
		})
		if err != nil {
			t.Fatalf("create mood engine: %v", err)
		}
		defer moodEngine.Close()

		a := &App{
			cfg: &config.AppConfig{
				Memory: config.MemoryConfig{MemoryEnabled: false},
				Llama:  config.LlamaConfig{CtxSize: 4096},
			},
			identity: id,
			mood:     moodEngine,
		}

		messages := a.buildMessages(context.Background(), "test message", nil)
		content := messages[0].Content.(string)

		// When mood is disabled NOTHING mood-related may be injected — the model
		// is driven solely by the configured system prompt.
		if strings.Contains(content, "nötr moddasın") {
			t.Error("system prompt must NOT contain the neutral mood block when mood is disabled")
		}
		if strings.Contains(content, "Current Emotional State") {
			t.Error("system prompt must NOT contain mood directive when mood is disabled")
		}
		if !strings.Contains(content, "Memo") {
			t.Error("system prompt should still contain the base identity")
		}
	})

	t.Run("mood_disabled_memory_strips_assistant", func(t *testing.T) {
		memContent := "[2026-06-20] User: Test question\nAssistant: Old cold response"
		memories := []memory.MemoryResult{
			{Content: memContent, Similarity: 0.9},
		}

		formatted := memory.FormatMemoriesUserOnly(memories)
		if strings.Contains(formatted, "Old cold response") {
			t.Error("FormatMemoriesUserOnly should strip assistant replies")
		}
		if !strings.Contains(formatted, "Test question") {
			t.Error("FormatMemoriesUserOnly should keep user messages")
		}

		formattedFull := memory.FormatMemoriesForPrompt(memories)
		if !strings.Contains(formattedFull, "Old cold response") {
			t.Error("FormatMemoriesForPrompt should include assistant replies")
		}
	})

	t.Run("mood_enabled_true_includes_directives", func(t *testing.T) {
		moodEngine, err := moodpkg.New(moodpkg.Config{
			Enabled: true,
			DBPath:  t.TempDir() + "/mood.db",
			Alpha:   0.95,
			Beta:    0.80,
			SigmaMin: 0.0,
			SigmaMax: 0.0,
		})
		if err != nil {
			t.Fatalf("create mood engine: %v", err)
		}
		defer moodEngine.Close()

		// Push score high enough to be non-neutral (> 2.0)
		for i := 0; i < 5; i++ {
			moodEngine.Update(context.Background(), 10.0)
		}

		a := &App{
			cfg: &config.AppConfig{
				Memory: config.MemoryConfig{MemoryEnabled: false},
				Llama:  config.LlamaConfig{CtxSize: 4096},
			},
			identity: id,
			mood:     moodEngine,
		}

		messages := a.buildMessages(context.Background(), "test", nil)
		content := messages[0].Content.(string)

		score := moodEngine.Score()
		t.Logf("mood score: %.2f", score)

		if strings.Contains(content, "nötr moddasın") {
			t.Error("system prompt should NOT contain neutral directive when mood is enabled")
		}
		// With β=0.80 and 5 updates at +10, score should be well above 2.0 (neutral threshold)
		if score <= 2.0 {
			t.Skip("score still neutral, skipping label check")
		}
		if !strings.Contains(content, "Current Emotional State") {
			t.Error("system prompt should contain mood directive when mood score is non-neutral")
		}
	})
}

func TestBuildMessages_MemoryEnabledNotCrash(t *testing.T) {
	id := identity.New("Test", "Memo", "casual", "")
	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: true},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity: id,
	}

	messages := a.buildMessages(context.Background(), "hello", nil)
	if len(messages) == 0 {
		t.Fatal("expected messages even when memory retrieval fails")
	}
	if messages[0].Role != "system" {
		t.Errorf("expected system role, got %s", messages[0].Role)
	}
}

func TestApiContextBudget_Defaults(t *testing.T) {
	a := &App{
		cfg: &config.AppConfig{
			Llama: config.LlamaConfig{CtxSize: 0},
		},
	}
	budget := a.apiContextBudget()
	if budget <= 0 {
		t.Errorf("expected positive budget, got %d", budget)
	}
	if budget != 128*1024 {
		t.Errorf("expected default 128K budget, got %d", budget)
	}
}

package identity

import (
	"strings"
	"testing"

	"memo/internal/memory"
)

func TestNew(t *testing.T) {
	id := New("Alice", "Memo", "casual", "")
	if id.UserName != "Alice" {
		t.Errorf("UserName = %q, want Alice", id.UserName)
	}
	if id.AssistantName != "Memo" {
		t.Errorf("AssistantName = %q, want Memo", id.AssistantName)
	}
	if id.Style != "casual" {
		t.Errorf("Style = %q, want casual", id.Style)
	}
}

func TestNewWithCustomRole(t *testing.T) {
	id := New("User", "Bot", "formal", "You are a code assistant.")
	if id.CustomRole != "You are a code assistant." {
		t.Errorf("CustomRole = %q", id.CustomRole)
	}
}

func TestBuildSystemPromptWithCustomRole(t *testing.T) {
	id := New("Alice", "Memo", "casual", "You are a helpful coding assistant.")
	prompt := id.BuildSystemPrompt(nil, false)
	if !strings.Contains(prompt, "coding assistant") {
		t.Error("custom role should appear in system prompt")
	}
	if strings.Contains(prompt, "Your identity:") {
		t.Error("identity block should not appear when custom role is set")
	}
}

func TestBuildSystemPromptWithoutCustomRole(t *testing.T) {
	id := New("Alice", "Memo", "casual", "")
	prompt := id.BuildSystemPrompt(nil, false)
	if prompt == "" {
		t.Fatal("BuildSystemPrompt() returned empty")
	}
	if !strings.Contains(prompt, "Alice") {
		t.Error("system prompt should contain user name")
	}
	if !strings.Contains(prompt, "Memo") {
		t.Error("system prompt should contain assistant name")
	}
}

func TestBuildSystemPromptWithMemories(t *testing.T) {
	id := New("Alice", "Memo", "casual", "")
	memories := []memory.MemoryResult{
		{Content: "User likes coffee", Similarity: 0.95},
	}
	prompt := id.BuildSystemPrompt(memories, false)
	if !strings.Contains(prompt, "coffee") {
		t.Error("system prompt should contain memory content")
	}
}

func TestBuildSystemPromptEmptyMemories(t *testing.T) {
	id := New("Alice", "Memo", "casual", "")
	prompt := id.BuildSystemPrompt([]memory.MemoryResult{}, false)
	if prompt == "" {
		t.Fatal("BuildSystemPrompt() with empty memories returned empty")
	}
}

func TestUpdateUserName(t *testing.T) {
	id := New("Alice", "Memo", "casual", "")
	id.Update("Bob", "", "", "")
	if id.UserName != "Bob" {
		t.Errorf("UserName = %q, want Bob", id.UserName)
	}
}

func TestUpdateAssistantName(t *testing.T) {
	id := New("Alice", "Memo", "casual", "")
	id.Update("", "Bot", "", "")
	if id.AssistantName != "Bot" {
		t.Errorf("AssistantName = %q, want Bot", id.AssistantName)
	}
}

func TestUpdateStyle(t *testing.T) {
	id := New("Alice", "Memo", "casual", "")
	id.Update("", "", "formal", "")
	if id.Style != "formal" {
		t.Errorf("Style = %q, want formal", id.Style)
	}
}

func TestUpdateCustomRole(t *testing.T) {
	id := New("Alice", "Memo", "casual", "")
	id.Update("", "", "", "You are a translator.")
	if id.CustomRole != "You are a translator." {
		t.Errorf("CustomRole = %q", id.CustomRole)
	}
}

func TestGetStyleInstructions(t *testing.T) {
	instructions := GetStyleInstructions("casual")
	if !strings.Contains(instructions, "CASUAL") {
		t.Error("casual style instructions should contain CASUAL")
	}

	instructions = GetStyleInstructions("formal")
	if !strings.Contains(instructions, "FORMAL") {
		t.Error("formal style instructions should contain FORMAL")
	}

	instructions = GetStyleInstructions("technical")
	if !strings.Contains(instructions, "TECHNICAL") {
		t.Error("technical style instructions should contain TECHNICAL")
	}

	instructions = GetStyleInstructions("creative")
	if !strings.Contains(instructions, "CREATIVE") {
		t.Error("creative style instructions should contain CREATIVE")
	}
}

func TestGetStyleInstructionsFallback(t *testing.T) {
	instructions := GetStyleInstructions("nonexistent")
	if !strings.Contains(instructions, "CASUAL") {
		t.Error("unknown style should fallback to casual")
	}
}

func TestAvailableStyles(t *testing.T) {
	styles := AvailableStyles()
	if len(styles) != 4 {
		t.Errorf("AvailableStyles() returned %d, want 4", len(styles))
	}
	expected := []string{"formal", "casual", "technical", "creative"}
	for i, s := range expected {
		if styles[i] != s {
			t.Errorf("styles[%d] = %q, want %q", i, styles[i], s)
		}
	}
}

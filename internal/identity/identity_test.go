package identity

import (
	"strings"
	"testing"

	"memo/internal/memory"
)

func TestNew(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
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
	id := New("User", "Bot", "formal", "You are a code assistant.", false)
	if id.CustomRole != "You are a code assistant." {
		t.Errorf("CustomRole = %q", id.CustomRole)
	}
}

func TestBuildSystemPromptWithCustomRole(t *testing.T) {
	id := New("Alice", "Memo", "casual", "You are a helpful coding assistant.", false)
	prompt := id.BuildSystemPrompt(nil, false, true, true)
	if !strings.Contains(prompt, "coding assistant") {
		t.Error("custom role should appear in system prompt")
	}
	if strings.Contains(prompt, "Your identity:") {
		t.Error("identity block should not appear when custom role is set")
	}
}

// TestBuildSystemPromptWithCustomRole_StillHasOriginBlock locks in the fix
// for a wizard persona (or any hand-written custom prompt) silently
// dropping "who made you" grounding — CustomRole replaces buildIdentityBlock
// entirely, so the origin facts must be appended independently of it, not
// live inside it.
func TestBuildSystemPromptWithCustomRole_StillHasOriginBlock(t *testing.T) {
	id := New("Alice", "Memo", "casual", "You are a formal, professional assistant.", false)
	prompt := id.BuildSystemPrompt(nil, false, true, true)
	if !strings.Contains(prompt, "Buğra Akdemir") {
		t.Error("origin block (who built Memo) should be present even when a custom role/persona is set")
	}
}

func TestBuildSystemPrompt_MinimalMode_StripsIdentityAndOrigin(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", true)
	prompt := id.BuildSystemPrompt(nil, false, true, true)
	if prompt != "" {
		t.Errorf("MinimalMode with no memories should produce an empty prompt, got %q", prompt)
	}
}

func TestBuildSystemPrompt_MinimalMode_IgnoresCustomRoleToo(t *testing.T) {
	// MinimalMode is a stronger override than CustomRole — even a
	// wizard-picked persona or hand-written prompt is stripped.
	id := New("Alice", "Memo", "casual", "You are a pirate.", true)
	prompt := id.BuildSystemPrompt(nil, false, true, true)
	if strings.Contains(prompt, "pirate") {
		t.Error("MinimalMode should strip CustomRole too, not just the default identity block")
	}
}

func TestBuildSystemPrompt_MinimalMode_StillIncludesMemory(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", true)
	memories := []memory.MemoryResult{{Content: "User likes coffee", Similarity: 0.95}}
	prompt := id.BuildSystemPrompt(memories, false, true, true)
	if !strings.Contains(prompt, "coffee") {
		t.Error("MinimalMode should still include memory context — it only strips identity/persona injection")
	}
	if strings.Contains(prompt, "Buğra Akdemir") {
		t.Error("MinimalMode should not include the origin block")
	}
}

func TestBuildSystemPromptWithoutCustomRole(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	prompt := id.BuildSystemPrompt(nil, false, true, true)
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
	id := New("Alice", "Memo", "casual", "", false)
	memories := []memory.MemoryResult{
		{Content: "User likes coffee", Similarity: 0.95},
	}
	prompt := id.BuildSystemPrompt(memories, false, true, true)
	if !strings.Contains(prompt, "coffee") {
		t.Error("system prompt should contain memory content")
	}
}

func TestBuildSystemPromptEmptyMemories(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	prompt := id.BuildSystemPrompt([]memory.MemoryResult{}, false, true, true)
	if prompt == "" {
		t.Fatal("BuildSystemPrompt() with empty memories returned empty")
	}
}

// TestBuildSystemPrompt_MentionsOffCapabilities is the fix for a user
// complaint: asked in a plain (non-agent, no-web-search) chat to create a
// file and to search the web, the model flatly said it doesn't have those
// abilities at all — because the system prompt never mentioned them when
// off, it had no way to say "not turned on right now" instead. Both
// features off must each be named as a toggle, not silently absent.
func TestBuildSystemPrompt_MentionsOffCapabilities(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	prompt := id.BuildSystemPrompt(nil, false, false, false)
	if !strings.Contains(prompt, "Agent mode") {
		t.Error("prompt should mention agent mode is off and toggleable when agentEnabled=false")
	}
	if !strings.Contains(prompt, "Web search is off") {
		t.Error("prompt should mention web search is off and toggleable when webSearchEnabled=false")
	}
}

// TestBuildSystemPrompt_OmitsOnCapabilities confirms the block only
// mentions what's OFF — when a feature is on, its own instructions are
// injected elsewhere (buildAgentSystemPrompt in chat.go, live search results
// in helpers.go), so restating "agent mode is on" here would just be
// redundant token spend.
func TestBuildSystemPrompt_OmitsOnCapabilities(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	prompt := id.BuildSystemPrompt(nil, false, true, true)
	if strings.Contains(prompt, "Agent mode") {
		t.Error("prompt should not mention agent mode at all when it's already on")
	}
	if strings.Contains(prompt, "Web search is off") {
		t.Error("prompt should not mention web search being off when it's already on")
	}
}

// TestBuildSystemPrompt_MinimalMode_OmitsCapabilitiesBlock confirms the
// capabilities block is treated like the rest of the identity/persona
// injection MinimalMode strips — not an unconditional addition that would
// break MinimalMode's "zero extra tokens beyond memory" contract.
func TestBuildSystemPrompt_MinimalMode_OmitsCapabilitiesBlock(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", true)
	prompt := id.BuildSystemPrompt(nil, false, false, false)
	if strings.Contains(prompt, "Agent mode") || strings.Contains(prompt, "Web search") {
		t.Error("MinimalMode should strip the capabilities block too, not just identity/origin/style")
	}
}

func TestUpdateUserName(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	id.Update("Bob", "", "", "")
	if id.UserName != "Bob" {
		t.Errorf("UserName = %q, want Bob", id.UserName)
	}
}

func TestUpdateAssistantName(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	id.Update("", "Bot", "", "")
	if id.AssistantName != "Bot" {
		t.Errorf("AssistantName = %q, want Bot", id.AssistantName)
	}
}

func TestUpdateStyle(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	id.Update("", "", "formal", "")
	if id.Style != "formal" {
		t.Errorf("Style = %q, want formal", id.Style)
	}
}

func TestUpdateCustomRole(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
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

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
	prompt := id.BuildSystemPrompt(nil, false, true, true, false, false)
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
	prompt := id.BuildSystemPrompt(nil, false, true, true, false, false)
	if !strings.Contains(prompt, "Buğra Akdemir") {
		t.Error("origin block (who built Memo) should be present even when a custom role/persona is set")
	}
}

func TestBuildSystemPrompt_MinimalMode_StripsIdentityAndOrigin(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", true)
	prompt := id.BuildSystemPrompt(nil, false, true, true, false, false)
	if prompt != "" {
		t.Errorf("MinimalMode with no memories should produce an empty prompt, got %q", prompt)
	}
}

func TestBuildSystemPrompt_MinimalMode_IgnoresCustomRoleToo(t *testing.T) {
	// MinimalMode is a stronger override than CustomRole — even a
	// wizard-picked persona or hand-written prompt is stripped.
	id := New("Alice", "Memo", "casual", "You are a pirate.", true)
	prompt := id.BuildSystemPrompt(nil, false, true, true, false, false)
	if strings.Contains(prompt, "pirate") {
		t.Error("MinimalMode should strip CustomRole too, not just the default identity block")
	}
}

func TestLearnedStyleNotesAppearsInSystemPrompt(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	id.SetLearnedStyleNotes("Alice prefers short, blunt answers with no filler.")
	prompt := id.BuildSystemPrompt(nil, false, true, true, false, false)
	if !strings.Contains(prompt, "Alice prefers short, blunt answers with no filler.") {
		t.Error("learned style notes should be injected into the system prompt")
	}
}

func TestLearnedStyleNotesEmptyByDefault(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	if id.GetLearnedStyleNotes() != "" {
		t.Errorf("GetLearnedStyleNotes() = %q, want empty by default", id.GetLearnedStyleNotes())
	}
}

func TestLearnedStyleNotesAppliesUnderCustomRoleToo(t *testing.T) {
	// Additive, unlike CustomRole itself — should still apply under a
	// wizard persona or fully custom prompt, same reasoning as the origin
	// block above.
	id := New("Alice", "Memo", "casual", "You are a pirate.", false)
	id.SetLearnedStyleNotes("Alice likes emoji.")
	prompt := id.BuildSystemPrompt(nil, false, true, true, false, false)
	if !strings.Contains(prompt, "Alice likes emoji.") {
		t.Error("learned style notes should still be injected even when CustomRole is set")
	}
}

func TestLearnedStyleNotesStrippedByMinimalMode(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", true)
	id.SetLearnedStyleNotes("Alice likes emoji.")
	prompt := id.BuildSystemPrompt(nil, false, true, true, false, false)
	if strings.Contains(prompt, "Alice likes emoji.") {
		t.Error("MinimalMode should strip learned style notes too")
	}
}

func TestBuildSystemPrompt_MinimalMode_StillIncludesMemory(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", true)
	memories := []memory.MemoryResult{{Content: "User likes coffee", Similarity: 0.95}}
	prompt := id.BuildSystemPrompt(memories, false, true, true, false, false)
	if !strings.Contains(prompt, "coffee") {
		t.Error("MinimalMode should still include memory context — it only strips identity/persona injection")
	}
	if strings.Contains(prompt, "Buğra Akdemir") {
		t.Error("MinimalMode should not include the origin block")
	}
}

func TestBuildSystemPromptWithoutCustomRole(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	prompt := id.BuildSystemPrompt(nil, false, true, true, false, false)
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
	prompt := id.BuildSystemPrompt(memories, false, true, true, false, false)
	if !strings.Contains(prompt, "coffee") {
		t.Error("system prompt should contain memory content")
	}
}

// TestBuildSystemPromptWithMemories_WarnsAgainstVerbatimReuse is the
// regression test for a real, live-reproduced bug: repeating a generic
// message like "selam" causes each turn to be saved to memory, and the
// *next* "selam" retrieves the model's own past reply as a "relevant
// memory" — with nothing telling the model not to just copy it, a weak
// model converges onto repeating the exact same reply verbatim after a few
// turns (confirmed directly: 4 identical replies in a row against a real
// backend + real provider). The system prompt must explicitly instruct the
// model that memories are background context, never a template to copy.
func TestBuildSystemPromptWithMemories_WarnsAgainstVerbatimReuse(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	memories := []memory.MemoryResult{
		{Content: "User: selam\nAssistant: Selam! Ne var ne yok?", Similarity: 0.5},
	}
	prompt := id.BuildSystemPrompt(memories, true, true, true, false, false)
	if !strings.Contains(prompt, "fresh reply to the current message") {
		t.Error("system prompt should still tell the model to write a fresh reply rather than lean on recalled memories")
	}
	if strings.Contains(prompt, "Assistant: Selam! Ne var ne yok?") {
		t.Error("chat path is stripAssistant=true: the memory block must not carry Memo's own past reply text")
	}
}

func TestBuildSystemPromptEmptyMemories(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	prompt := id.BuildSystemPrompt([]memory.MemoryResult{}, false, true, true, false, false)
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
	prompt := id.BuildSystemPrompt(nil, false, false, false, false, false)
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
	prompt := id.BuildSystemPrompt(nil, false, true, true, false, false)
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
	prompt := id.BuildSystemPrompt(nil, false, false, false, false, false)
	if strings.Contains(prompt, "Agent mode") || strings.Contains(prompt, "Web search") {
		t.Error("MinimalMode should strip the capabilities block too, not just identity/origin/style")
	}
}

// TestBuildSystemPrompt_MentionsPassiveReminderFeature is the fix for a
// related user complaint: asked directly "can you set a reminder for me",
// the model said it had no automatic reminder system at all — at the exact
// moment processMessageIntent (chat.go) was silently scanning that same
// message and about to create exactly such a reminder in the background.
// Unlike agent mode/web search, this capability has no on/off toggle, so it
// must always be mentioned (not gated the way buildCapabilitiesBlock only
// mentions what's currently off).
func TestBuildSystemPrompt_MentionsPassiveReminderFeature(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	prompt := id.BuildSystemPrompt(nil, false, false, false, false, false)
	if !strings.Contains(prompt, "calendar-worthy plans") {
		t.Error("prompt should mention the always-on passive calendar/reminder feature")
	}
}

// TestBuildSystemPrompt_MinimalMode_OmitsPassiveFeaturesBlock confirms this
// new block is treated like the rest of the identity/persona injection
// MinimalMode strips, same as the capabilities block above.
func TestBuildSystemPrompt_MinimalMode_OmitsPassiveFeaturesBlock(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", true)
	prompt := id.BuildSystemPrompt(nil, false, false, false, false, false)
	if strings.Contains(prompt, "calendar-worthy plans") {
		t.Error("MinimalMode should strip the passive-features block too, not just identity/origin/style")
	}
}

// TestBuildSystemPrompt_MentionsReachableChannels is the fix for a user
// complaint: Memo assumed it could only be talked to through its own
// desktop/web app, even when the WhatsApp and/or Telegram bridges were
// actually connected and already relaying messages to this same
// identity/memory — same affirm-instead-of-deny reasoning as the passive
// calendar feature above, but conditional on what's genuinely live rather
// than always-on.
func TestBuildSystemPrompt_MentionsReachableChannels(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	prompt := id.BuildSystemPrompt(nil, false, false, false, true, true)
	if !strings.Contains(prompt, "WhatsApp") || !strings.Contains(prompt, "Telegram") {
		t.Error("prompt should name both channels when both are reachable")
	}
}

// TestBuildSystemPrompt_OmitsUnreachableChannels confirms the block never
// claims a channel that isn't actually connected right now — a fresh
// install with neither bridge set up must not have the model tell the user
// it's reachable somewhere it isn't.
func TestBuildSystemPrompt_OmitsUnreachableChannels(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	prompt := id.BuildSystemPrompt(nil, false, false, false, false, false)
	if strings.Contains(prompt, "WhatsApp") || strings.Contains(prompt, "Telegram") {
		t.Error("prompt should not mention WhatsApp/Telegram when neither is reachable")
	}
}

// TestBuildSystemPrompt_MentionsOnlyTheReachableChannel confirms only the
// live channel is named, not both, when just one bridge is connected.
func TestBuildSystemPrompt_MentionsOnlyTheReachableChannel(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	prompt := id.BuildSystemPrompt(nil, false, false, false, true, false)
	if !strings.Contains(prompt, "WhatsApp") {
		t.Error("prompt should mention WhatsApp when it's the reachable channel")
	}
	if strings.Contains(prompt, "Telegram") {
		t.Error("prompt should not mention Telegram when it isn't reachable")
	}
}

// TestBuildSystemPrompt_MinimalMode_OmitsChannelAwarenessBlock confirms the
// channel-awareness sentence is treated like the rest of the passive
// features block MinimalMode strips.
func TestBuildSystemPrompt_MinimalMode_OmitsChannelAwarenessBlock(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", true)
	prompt := id.BuildSystemPrompt(nil, false, false, false, true, true)
	if strings.Contains(prompt, "WhatsApp") || strings.Contains(prompt, "Telegram") {
		t.Error("MinimalMode should strip the channel-awareness block too, not just identity/origin/style")
	}
}

// TestBuildSystemPrompt_MinimalMode_KeepPersona_OnlyRestoresPersona checks
// the granular override: with MinimalMode on and only KeepPersona set, the
// identity/origin/style block comes back but the passive-features and
// capabilities blocks stay stripped — the user's own scenario ("turn
// learning off, but not the system prompt") depends on this independence.
func TestBuildSystemPrompt_MinimalMode_KeepPersona_OnlyRestoresPersona(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", true)
	id.SetMinimalModeOverrides(true, false, false, false)
	prompt := id.BuildSystemPrompt(nil, false, false, false, false, false)

	if !strings.Contains(prompt, "Buğra Akdemir") {
		t.Error("KeepPersona=true should restore the origin block")
	}
	if strings.Contains(prompt, "calendar-worthy plans") {
		t.Error("KeepPersona alone should not restore the passive-features block")
	}
	if strings.Contains(prompt, "Agent mode") {
		t.Error("KeepPersona alone should not restore the capabilities block")
	}
}

// TestBuildSystemPrompt_MinimalMode_KeepCapabilitiesOnly is the inverse:
// only the capabilities block comes back, persona/passive stay stripped.
func TestBuildSystemPrompt_MinimalMode_KeepCapabilitiesOnly(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", true)
	id.SetMinimalModeOverrides(false, true, false, false)
	prompt := id.BuildSystemPrompt(nil, false, false, false, false, false)

	if !strings.Contains(prompt, "Agent mode") {
		t.Error("KeepCapabilities=true should restore the capabilities block")
	}
	if strings.Contains(prompt, "Buğra Akdemir") {
		t.Error("KeepCapabilities alone should not restore the persona/origin block")
	}
	if strings.Contains(prompt, "calendar-worthy plans") {
		t.Error("KeepCapabilities alone should not restore the passive-features block")
	}
}

// TestBuildSystemPrompt_MinimalMode_KeepPassiveOnly is the third
// combination: only the passive-features block comes back.
func TestBuildSystemPrompt_MinimalMode_KeepPassiveOnly(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", true)
	id.SetMinimalModeOverrides(false, false, true, false)
	prompt := id.BuildSystemPrompt(nil, false, false, false, false, false)

	if !strings.Contains(prompt, "calendar-worthy plans") {
		t.Error("KeepPassive=true should restore the passive-features block")
	}
	if strings.Contains(prompt, "Buğra Akdemir") {
		t.Error("KeepPassive alone should not restore the persona/origin block")
	}
	if strings.Contains(prompt, "Agent mode") {
		t.Error("KeepPassive alone should not restore the capabilities block")
	}
}

// TestBuildSystemPrompt_MinimalMode_OverridesAreNoOpWhenNotMinimal confirms
// the overrides only matter while MinimalMode is actually on — setting them
// with MinimalMode off must not change anything (everything's already on).
func TestBuildSystemPrompt_MinimalMode_OverridesAreNoOpWhenNotMinimal(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", false)
	id.SetMinimalModeOverrides(false, false, false, false)
	prompt := id.BuildSystemPrompt(nil, false, false, false, false, false)

	if !strings.Contains(prompt, "Buğra Akdemir") || !strings.Contains(prompt, "Agent mode") || !strings.Contains(prompt, "calendar-worthy plans") {
		t.Error("overrides should have no effect when MinimalMode is off — everything should already be present")
	}
}

// TestGetMinimalModeOverrides_DefaultFalse checks the zero-value default —
// a fresh Identity (or MinimalMode toggled on without ever touching the
// dropdown) behaves exactly like the original all-off Minimal Mode.
func TestGetMinimalModeOverrides_DefaultFalse(t *testing.T) {
	id := New("Alice", "Memo", "casual", "", true)
	p, c, pa, pr := id.GetMinimalModeOverrides()
	if p || c || pa || pr {
		t.Errorf("GetMinimalModeOverrides() = (%v,%v,%v,%v), want all false by default", p, c, pa, pr)
	}
	if id.GetMinimalModeKeepProactive() {
		t.Error("GetMinimalModeKeepProactive() should default to false")
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

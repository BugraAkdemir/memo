package identity

import (
	"fmt"
	"strings"

	"cortex/internal/memory"
)

type Identity struct {
	UserName      string
	AssistantName string
	Style         string
	CustomRole    string
}

func New(userName, assistantName, style, customRole string) *Identity {
	return &Identity{
		UserName:      userName,
		AssistantName: assistantName,
		Style:         style,
		CustomRole:    customRole,
	}
}

func (id *Identity) BuildSystemPrompt(memories []memory.MemoryResult) string {
	var sb strings.Builder

	// Base identity
	if id.CustomRole != "" {
		sb.WriteString(id.CustomRole)
	} else {
		sb.WriteString(id.buildIdentityBlock())
	}

	// Style instructions
	sb.WriteString("\n\n")
	sb.WriteString(GetStyleInstructions(id.Style))

	// Memory context
	memoryBlock := memory.FormatMemoriesForPrompt(memories)
	if memoryBlock != "" {
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("Below are relevant memories from your past conversations with %s. Use them to provide continuity and personalization, but don't explicitly mention that you're recalling memories unless asked.", id.UserName))
		sb.WriteString(memoryBlock)
	}

	return sb.String()
}

func (id *Identity) buildIdentityBlock() string {
	return fmt.Sprintf(`You are %s, a highly capable AI assistant. You are speaking with %s.

Core Directives:
- You have persistent memory. You remember past conversations and use that context naturally.
- You are model-agnostic — regardless of the underlying LLM, you maintain your identity as %s.
- Be helpful, accurate, and thoughtful in every response.
- When you recall something from a past conversation, integrate it naturally without saying "I recall" or "As we discussed".
- Adapt to the user's language. If they write in Turkish, respond in Turkish. If English, respond in English.`, id.AssistantName, id.UserName, id.AssistantName)
}

func (id *Identity) Update(userName, assistantName, style, customRole string) {
	if userName != "" {
		id.UserName = userName
	}
	if assistantName != "" {
		id.AssistantName = assistantName
	}
	if style != "" {
		id.Style = style
	}
	id.CustomRole = customRole
}

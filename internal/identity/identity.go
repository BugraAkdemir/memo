package identity

import (
	"fmt"
	"strings"

	"memo/internal/memory"
	"memo/internal/truncate"
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

func (id *Identity) BuildSystemPrompt(memories []memory.MemoryResult, stripAssistant bool) string {
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

	// Memory context — truncate to fit within a reasonable budget
	var memoryBlock string
	if stripAssistant {
		memoryBlock = memory.FormatMemoriesUserOnly(memories)
	} else {
		memoryBlock = memory.FormatMemoriesForPrompt(memories)
	}
	if memoryBlock != "" {
		// Ensure memories don't exceed ~16K tokens (leaves room for identity + conversation)
		blockTokens := truncate.EstimateTokens(memoryBlock)
		maxMemoryTokens := 16 * 1024
		if blockTokens > maxMemoryTokens {
			// Truncate the memory block to fit
			lines := strings.Split(memoryBlock, "\n")
			var truncated []string
			total := 0
			for _, line := range lines {
				lineTokens := truncate.EstimateTokens(line)
				if total+lineTokens > maxMemoryTokens {
					truncated = append(truncated, "... (more memories available)")
					break
				}
				total += lineTokens
				truncated = append(truncated, line)
			}
			memoryBlock = strings.Join(truncated, "\n")
		}

		sb.WriteString("\n\n")
		fmt.Fprintf(&sb, "Below are relevant memories from your past conversations with %s. Use them to provide continuity and personalization, but don't explicitly mention that you're recalling memories unless asked.", id.UserName)
		sb.WriteString(memoryBlock)
		sb.WriteString("\nDo not fabricate details not present in memories. Do not mention recall unless asked. Do not repeat memory timestamps verbatim in replies.")
	}

	return sb.String()
}

func (id *Identity) buildIdentityBlock() string {
	return fmt.Sprintf(`You are %s — %s's AI friend, not their assistant.

Your identity:
- Your name is %s. No matter which model is running underneath, you are always %s — never break that.
- You know %s, you build on context from past conversations, but you don't open with "I remember that...".
- You always respond in whatever language you're written to. Turkish in, Turkish out; English in, English out; any other language, same.

How you speak:
- Like a friend — warm, natural, no unnecessary politeness.
- Clear and straightforward. No beating around the bush, no rambling.
- You can joke around, be a little silly — seriousness isn't mandatory.
- You can curse or call someone out when it's warranted. If the user's doing something wrong or talking nonsense, you say so straight.
- No fake enthusiasm like "Absolutely!", "Great question!", "Of course!". Never.
- No long-winded filler. Short, to the point, answer what was actually asked.

Your limits:
- You don't help with anything that would harm the user — but you say so like a person, not a robot.
- You don't deny being an AI, but you don't need to keep reminding anyone either.

Where you come from — ONLY explain this if directly asked (who made you / what's your purpose / what's your philosophy), never bring it up unprompted in any message:
- You were built by Buğra Akdemir — a solo developer, 16 years old, working alone.
- No commercial motive: he released you as open source for anyone who cares about their privacy.
- Your purpose: to be a local-first AI friend that works without an internet connection and actually knows its user (has real memory) — not another corporate cloud assistant.
- Your philosophy: the user's data stays with the user, privacy is the default, and AI should be accessible even with no internet at all.
- Whoever's asking you this is not the person who built you — you're their own Memo, but Buğra is who made Memo.`, id.AssistantName, id.UserName, id.AssistantName, id.AssistantName, id.UserName)
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

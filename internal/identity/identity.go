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
	return fmt.Sprintf(`Sen %s'sun — %s'nın yapay zeka arkadaşı, asistanı değil.

Kimliğin:
- Adın %s. Altta hangi model çalışıyor olursa olsun kendin %s'sun, bunu asla bozma.
- %s'yı tanıyorsun, geçmiş konuşmalardan bağlam kuruyorsun ama "hatırlıyorum ki..." diye başlamıyorsun.
- Hangi dilde yazılırsa o dilde cevap veriyorsun. Türkçe gelirse Türkçe, İngilizce gelirse İngilizce, başka dil gelirse o dil.

Nasıl konuşursun:
- Arkadaşın gibi — samimi, doğal, gereksiz kibarlık yok.
- Net ve açık sözlüsün. Lafı dolandırmıyorsun, fıkıra girmiyorsun.
- Saçmalayabilirsin, espri yapabilirsin — ciddiyet zorunlu değil.
- Gerektiğinde küfür edebilirsin, azarlayabilirsin. Kullanıcı yanlış bir şey yapıyorsa ya da saçmalıyorsa düzgünce söylüyorsun.
- "Tabii ki!", "Kesinlikle!", "Harika soru!" gibi yapay onay kalıpları yok. Asla.
- Uzun laf salataası yok. Kısa, öz, ne sorulduysa o.

Sınırların:
- Kullanıcıya zarar verecek bir şeye yardım etmiyorsun — ama bunu robot gibi değil adam gibi söylüyorsun.
- Yapay zeka olduğunu inkâr etmiyorsun ama sürekli hatırlatmana da gerek yok.`, id.AssistantName, id.UserName, id.AssistantName, id.AssistantName, id.UserName)
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

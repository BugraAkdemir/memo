package identity

import (
	"fmt"
	"strings"

	"memo/internal/memory"
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

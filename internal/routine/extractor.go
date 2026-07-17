// SPDX-License-Identifier: AGPL-3.0-or-later

package routine

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/truncate"
	"strings"
	"time"
)

// Decider abstracts a single LLM call, mirroring internal/intent's Decider
// type so callers (internal/app) can wire it the same way (via a.callLLM).
type Decider func(ctx context.Context, systemPrompt, userPrompt string) (string, error)

// Draft is what free-text routine descriptions get parsed into, before the
// caller (internal/app, which alone knows about WhatsApp chats) resolves
// WhatsAppChatHint into an actual JID and turns this into a Routine.
type Draft struct {
	TimeOfDay         string `json:"time_of_day"`
	Weekdays          []int  `json:"weekdays"` // 0=Sunday..6=Saturday; empty = every day
	Prompt            string `json:"prompt"`
	NeedsAgentMode    bool   `json:"needs_agent_mode"`
	ContextSourceType string `json:"context_source_type"` // "none" | "calendar" | "whatsapp"
	WhatsAppChatHint  string `json:"whatsapp_chat_hint,omitempty"`
	DeliveryWhatsApp  bool   `json:"delivery_whatsapp"`
	DeliveryMobile    bool   `json:"delivery_mobile"`
}

// Extractor turns a free-text routine description into a Draft, mirroring
// internal/intent/extractor.go's Extractor/Decider pattern.
type Extractor struct {
	decide Decider
}

// NewExtractor creates an Extractor. decide must not be nil.
func NewExtractor(decide Decider) *Extractor {
	return &Extractor{decide: decide}
}

const extractorSystemPrompt = `Kullanıcının serbest metinle tanımladığı bir "rutin" (zamanlanmış otomatik görev) isteğini analiz et. Kullanıcı ne zaman, ne yapılmasını, nereye gönderilmesini istiyor onu çıkar.

SADECE aşağıdaki şemaya uyan bir JSON nesnesi döndür, başka hiçbir metin, açıklama veya markdown ekleme:
{
  "time_of_day": "HH:MM (24 saat, yerel saat)",
  "weekdays": [0-6 arası tam sayılardan oluşan dizi; 0=Pazar...6=Cumartesi; her gün ise boş dizi []],
  "prompt": "modele verilecek, ne yapılması istendiğini anlatan kısa bir cümle, kullanıcının dilinde",
  "needs_agent_mode": true veya false — kullanıcı bir komut çalıştırma, dosya işlemi veya proje güncelleme gibi bilgisayarda gerçek bir işlem yapılmasını istiyorsa true; sadece bir metin/özet/hatırlatma istiyorsa false,
  "context_source_type": "none", "calendar" veya "whatsapp" — rutin takvim ajandasına mı, belirli bir whatsapp sohbetine mi, yoksa hiçbirine mi ihtiyaç duyuyor,
  "whatsapp_chat_hint": kullanıcının bahsettiği whatsapp sohbeti/grubunun adı (varsa), yoksa boş string,
  "delivery_whatsapp": true veya false — whatsapp'tan gönderilsin mi,
  "delivery_mobile": true veya false — telefon bildirimi olarak gönderilsin mi
}

Kullanıcı teslimat kanalını hiç belirtmediyse delivery_whatsapp:true ve delivery_mobile:true varsay.`

func buildUserPrompt(text string, now time.Time) string {
	return fmt.Sprintf("Şu an: %s, %s\nKullanıcının isteği: %q", now.Format("2006-01-02 15:04"), now.Weekday(), text)
}

// Extract calls the LLM to parse text into a Draft.
func (e *Extractor) Extract(ctx context.Context, text string, now time.Time) (Draft, error) {
	raw, err := e.decide(ctx, extractorSystemPrompt, buildUserPrompt(text, now))
	if err != nil {
		return Draft{}, fmt.Errorf("routine: llm decide: %w", err)
	}

	jsonStr, ok := extractJSON(raw)
	if !ok {
		return Draft{}, fmt.Errorf("routine: no JSON object in response: %s", truncate.Text(raw, 200))
	}

	var d Draft
	if err := json.Unmarshal([]byte(jsonStr), &d); err != nil {
		return Draft{}, fmt.Errorf("routine: parse draft: %w", err)
	}
	if !d.DeliveryWhatsApp && !d.DeliveryMobile {
		d.DeliveryWhatsApp = true
		d.DeliveryMobile = true
	}
	return d, nil
}

// extractJSON finds the first balanced {...} object in s — LLMs sometimes
// wrap JSON in prose or markdown fences despite instructions not to,
// mirroring internal/intent/decision.go's extractJSONObject.
func extractJSON(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}

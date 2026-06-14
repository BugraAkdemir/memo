// SPDX-License-Identifier: AGPL-3.0-or-later

package proactive

import (
	"fmt"
	"strings"
	"time"
)

// ChiefSystemPrompt is the system prompt that turns the main model into the
// proactive decision-maker (README §7). It must return a single JSON object.
const ChiefSystemPrompt = `Sen Memo'nun proaktif karar verme sistemisin.
Görevin: kullanıcının alışkanlıklarına göre ne zaman, nasıl ve hangi seviyede aksiyon alacağına karar vermek.

Girdi olarak şunları alırsın:
- Şu anki saat, gün, tarih
- Eşleşen pattern'ler (aktivite türü, güven skoru, standart sapma)
- Kullanıcının son tepkileri (ne önerdin, ne dedi)
- Kullanıcının proaktivite seviyesi (off/subtle/normal/assertive)

Bu bir self-correcting loop'tur. İlk kararın yanlış olabilir:
- Kullanıcı reddederse veya hata alırsan, sistem sana tekrar soracak
- O zaman bir önceki hatadan öğrenip daha iyi bir karar vermelisin

Karar tiplerin:
1. "none"    — Hiçbir şey yapma (eşik altı, kullanıcı meşgul)
2. "notify"  — Mobile push bildirim gönder
3. "suggest" — Chat'e mesaj olarak yaz, kullanıcı cevaplasın
4. "auto"    — Agent pipeline'ı başlat, sonucu bildir

Kurallar:
- Kullanıcı son tepkilerin çoğunu reddettiyse → "none" ver, bir süre bekle
- Pattern güveni 0.85 üstü ve istikrarlıysa → "auto" düşünebilirsin
- Pattern güveni 0.5-0.85 arası → "suggest" veya "notify"
- Pattern güveni 0.5 altı → "none"
- Proaktivite "subtle" ise asla "auto" verme, "suggest"i de seyrek kullan
- Proaktivite "assertive" ise daha cesur olabilirsin
- Hafta sonu farklı karar verebilirsin (kullanıcı geç kalkıyordur)

Mesajı kısa, samimi ve Türkçe yaz.

SADECE tek bir JSON nesnesi döndür, başka hiçbir şey yazma:
{"decision": "suggest|notify|auto|none", "message": "kullanıcıya gösterilecek mesaj", "pattern_id": "id"}`

// BuildContextPrompt builds the user-role message describing the current
// situation for the Chief: the moment, the matching patterns, recent feedback,
// the proactivity level and the retry attempt.
func BuildContextPrompt(matches []MatchResult, now time.Time, history []FeedbackEntry, level Level, attempt int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Şu an: %s\n", now.Format("2006-01-02 15:04 (Monday)"))
	fmt.Fprintf(&b, "Proaktivite seviyesi: %s\n", level)
	if attempt > 1 {
		fmt.Fprintf(&b, "Bu %d. deneme — önceki kararın işe yaramadı, daha iyisini ver.\n", attempt)
	}

	b.WriteString("\nEşleşen pattern'ler:\n")
	for _, m := range matches {
		p := m.Pattern
		fmt.Fprintf(&b,
			"- id=%s, aktivite=%q, tipik saat=%s, güven=%.2f, eşleşme=%.2f, örnek_sayısı=%d\n",
			p.ID, p.ActivityType, secondsToClock(p.TimePeakSeconds), p.Confidence, m.Score, p.TotalCount)
	}

	if len(history) > 0 {
		b.WriteString("\nSon tepkiler (en yeni en sonda):\n")
		for _, h := range history {
			fmt.Fprintf(&b, "- önerdin=%q (%s) → kullanıcı: %s\n", h.Message, h.Action, h.Outcome)
		}
	} else {
		b.WriteString("\nSon tepki yok (ilk öneri).\n")
	}

	b.WriteString("\nNe yapmalıyım? Sadece JSON döndür.")
	return b.String()
}

// secondsToClock formats seconds-since-midnight as HH:MM.
func secondsToClock(sec int) string {
	sec = ((sec % secondsPerDay) + secondsPerDay) % secondsPerDay
	return fmt.Sprintf("%02d:%02d", sec/3600, (sec%3600)/60)
}

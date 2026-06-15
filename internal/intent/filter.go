// SPDX-License-Identifier: AGPL-3.0-or-later

package intent

import (
	"regexp"
	"strings"
)

// intentKeywords is a conservative list of Turkish and English words that
// suggest a message might contain a time-bound intent or habit declaration.
// False-negative rate is acceptable here — the LLM handles edge cases.
// False-positive rate must be low to avoid unnecessary LLM calls.
var intentKeywords = []string{
	// Zaman
	"yarın", "bugün", "öbürgün", "bu akşam", "bu sabah", "bu hafta",
	"pazartesi", "salı", "çarşamba", "perşembe", "cuma", "cumartesi", "pazar",
	"tomorrow", "tonight", "today", "monday", "tuesday", "wednesday",
	"thursday", "friday", "saturday", "sunday",
	"saat ", "hafta", "ay ", "next week",
	// Niyet
	"başlayacağım", "yapacağım", "gideceğim", "gelecek", "olacak",
	"planlıyorum", "düşünüyorum", "istiyorum", "buluşalım", "gidelim",
	"yapalım", "randevu", "toplantı", "antrenman", "spor", "gym",
	"i will", "i'm going to", "let's", "planning to", "going to",
	"meeting", "appointment", "schedule", "remind",
	// Alışkanlık
	"her gün", "her sabah", "her akşam", "düzenli", "artık", "bundan sonra",
	"every day", "every morning", "every evening", "from now on", "regularly",
	// İptal
	"iptal", "vazgeç", "gitmeyeceğim", "yapmayacağım", "ertele",
	"cancel", "called off", "postpone", "not going",
}

// timePattern matches clock-like time expressions that appear in virtually any
// language (digits are universal): "21:00", "21.00", "9pm", "9 pm", "9am",
// "9 o'clock". This lets the filter catch calendar intents written in languages
// whose words are not in intentKeywords (German, Spanish, French, …).
var timePattern = regexp.MustCompile(`(?i)\b\d{1,2}[:.h]\d{2}\b|\b\d{1,2}\s?(am|pm)\b|\b\d{1,2}\s?o['’]?clock\b`)

// MightHaveIntent returns true when text contains an intent keyword or a
// time-like expression. It is a fast pre-filter; the LLM makes the definitive
// call.
func MightHaveIntent(text string) bool {
	lower := strings.ToLower(text)
	for _, kw := range intentKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return timePattern.MatchString(text)
}

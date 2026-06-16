// SPDX-License-Identifier: AGPL-3.0-or-later

package intent

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// ============================================================================
// Universal time/date patterns — language-agnostic.
// ============================================================================

// universalTime matches clock-like expressions that use digits in virtually any
// language: "21:00", "21.00", "9pm", "9 am", "9 o'clock", "14h", "14h30",
// "5'de", "3'te", "um 3", "à 5", "a las 3", etc.
var universalTime = regexp.MustCompile(`(?i)` +
	// Standard time formats
	`\b\d{1,2}[:.h]\d{2}\b|` +
	// AM/PM
	`\b\d{1,2}\s?(am|pm)\b|` +
	// O'clock variants
	`\b\d{1,2}\s?o['\x{2019}]?clock\b|` +
	// Hour-only with time suffix / preposition (Turkish 'de/'te/'da/'ta, French à, Spanish a, Italian alle, etc.)
	// Matches "5'de", "5'te", "3'da", "3'te" — Turkish possessive/time suffix
	`\b\d{1,2}['\x{2019}](?:de|te|da|ta)\b|` +
	// "saat X" (hour X in Turkish)
	`\bsaat\s+\d{1,2}\b|` +
	// "um X", "à X", "a las X", "alle X", "à Xh" (common in many European languages)
	`\b(?:um|à|a(?:\s+las?)?|alle|ora)\s+\d{1,2}\b|` +
	// "X Uhr" (German)
	`\b\d{1,2}\s+Uhr\b`,
)

// universalDate matches date expressions that appear in many writing systems.
var universalDate = regexp.MustCompile(
	`\b\d{4}[-/]\d{1,2}[-/]\d{1,2}\b|` + // ISO-ish: 2026-06-16
		`\b\d{1,2}/\d{1,2}(?:/\d{2,4})?\b|` + // 6/16, 16/06/2026
		`\b\d{1,2}\.\d{1,2}\.\d{2,4}\b`, // 16.06.2026
)

// relativeTime matches short relative-time phrases that appear in many
// languages: "in 5 min", "next week", "within 2 hours".
var relativeTime = regexp.MustCompile(`(?i)` +
	`\b(?:in|within)\s+\d+\s+(?:min|sec|hour|day|week|month|year)s?\b|` +
	`\bnext\s+(?:week|month|year)\b|` +
	`\btomorrow\b|\btonight\b|\btoday\b|` +
	// Turkish relative
	`\byar[ıi]n\b|\bbug[üu]n\b|\b[öo]b[üu]rg[üu]n\b|\b(?:bu|her)\s+(?:ak[sş]am|sabah|hafta|ay)\b|` +
	// Generic "every day/morning/evening"
	`\bevery\s+(?:day|morning|evening|week|month|night)\b|` +
	// Turkish habit
	`\bher\s+(?:g[üu]n|sabah|ak[sş]am|hafta|ay|gece)\b|` +
	// Common future-intent markers in various languages
	`\b(?:plan|planla|schedule|appointment|meeting|randevu|toplant[ıi]|event|etkinlik)\b`,
)

// weekdayNames matches day-of-week names in Turkish and English. A message
// containing a weekday often implies a time-bound plan ("cuma akşamı sinema").
var weekdayNames = regexp.MustCompile(`(?i)\b(?:` +
	// English
	`monday|tuesday|wednesday|thursday|friday|saturday|sunday|` +
	// Turkish
	`pazartesi|sal[ıi]|[çc]ar[sş]amba|per[sş]embe|cuma|cumartesi|pazar` +
	`)\b`)

// timeDigit matches a digit in a time-context word ("öğlenden sonra 5",
// "saat 3", "akşamı 8", "morning 7").
var timeDigit = regexp.MustCompile(
	`(?i)\b(saat|at|by|h[oô]ra|um|a(?:\s+las?)?|alle|ora|ab|von|dalla|dopo\s+le|[öo][ğg]lenden\s+sonra|ak[sş]am[ıi]?|sabah[ıi]?|geceyi?|morning|afternoon|evening|night|noon|midnight|ö[ğg]len)\s+\d{1,2}\b|` +
		`\b\d{1,2}\s*(am|pm|h|o['\x{2019}]clock|['\x{2019}](?:de|te|da|ta)|['\x{2019}]den\s+sonra|['\x{2019}]ten\s+sonra)\b`,
)

// trivialPatterns match messages that are almost certainly not intents.
var trivialPatterns = regexp.MustCompile(`(?i)^\s*(ok|okay|tamam|yes|no|hay[ıi]r|thanks|te[sş]ekk[üu]r|sa[ğg]ol|eyvallah|hi|hello|selam|merhaba|bye|g[öo]r[üu][sş][üu]r[üu]z|g[üu]nayd[ıi]n|iyi\s+(?:ak[sş]amlar|geceler|g[üu]nler)|[:;][)(pPdD]|[\p{So}]{1,3})\s*[!.]*\s*$`)

// MightHaveIntent returns true when text looks like it could contain a
// time-bound intention. The LLM makes the definitive call.
//
// The filter is intentionally permissive (optimising for recall, not
// precision). It uses only universal patterns — digits, time prepositions,
// date separators, weekday names — that work across virtually every language.
// There is no language-specific keyword list to maintain.
func MightHaveIntent(text string) bool {
	// Empty or whitespace only — nothing to analyse.
	s := strings.TrimSpace(text)
	if s == "" {
		return false
	}

	// Very short single-emoji / reaction messages.
	if utf8.RuneCountInString(s) <= 3 && trivialPatterns.MatchString(s) {
		return false
	}

	// Trivial one-word responses — not intents.
	if trivialPatterns.MatchString(s) {
		return false
	}

	// Universal time patterns (digits + context).
	if universalTime.MatchString(s) {
		return true
	}

	// Date patterns.
	if universalDate.MatchString(s) {
		return true
	}

	// Relative time phrases.
	if relativeTime.MatchString(s) {
		return true
	}

	// Digit in time context ("öğlenden sonra 5", "saat 3").
	if timeDigit.MatchString(s) {
		return true
	}

	// Weekday mention — high chance of time-bound intent.
	if weekdayNames.MatchString(s) {
		return true
	}

	return false
}

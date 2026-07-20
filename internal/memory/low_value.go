package memory

import (
	"strings"
	"unicode"
)

// maxLowValueRunes is the per-side length cap: both user and assistant must
// be at or under this (after normalize) for the short-ack / short-noise
// heuristics to apply. Longer turns always save.
const maxLowValueRunes = 40

// maxVeryShortUserRunes: a user message this short with no digit may still
// skip even when it's not in the explicit ack set (e.g. "hmm", "aaa").
const maxVeryShortUserRunes = 12

// lowValueAcks is a small bilingual set of pure acknowledgements / greets /
// fillers that carry no durable fact for RAG. Matched after normalize
// (lower, trim, strip punctuation) as whole-message equality.
var lowValueAcks = map[string]struct{}{
	// Turkish
	"ok": {}, "tamam": {}, "tm": {}, "tmm": {}, "peki": {},
	"evet": {}, "hayir": {}, "hayır": {}, "yok": {}, "var": {},
	"tesekkurler": {}, "teşekkürler": {}, "tesekkur": {}, "teşekkür": {},
	"sagol": {}, "sağol": {}, "sagolun": {}, "sağolun": {},
	"tsk": {}, "tşk": {}, "eyv": {}, "eyvallah": {},
	"sa": {}, "as": {}, "selam": {}, "selamlar": {}, "merhaba": {},
	"naber": {}, "nbr": {}, "nasilsin": {}, "nasılsın": {},
	"iyi": {}, "iyiyim": {}, "guzel": {}, "güzel": {},
	"super": {}, "süper": {}, "harika": {}, "mukemmel": {}, "mükemmel": {},
	"anladim": {}, "anladım": {}, "tamamdir": {}, "tamamdır": {},
	"hadi": {}, "kolay gelsin": {}, "gorusuruz": {}, "görüşürüz": {},
	"bb": {}, "bye": {}, "gule gule": {}, "güle güle": {},
	"hmm": {}, "hı": {}, "hih": {}, "hıh": {}, "hhh": {},
	"he": {}, "ha": {}, "aa": {}, "aaa": {}, "ee": {},
	// English
	"yes": {}, "no": {}, "yep": {}, "nope": {}, "yeah": {}, "nah": {},
	"thanks": {}, "thank you": {}, "thx": {}, "ty": {},
	"hi": {}, "hello": {}, "hey": {}, "yo": {}, "sup": {},
	"howdy": {}, "good morning": {}, "good night": {}, "gn": {},
	"cool": {}, "nice": {}, "great": {}, "awesome": {}, "perfect": {},
	"lol": {}, "lmao": {}, "haha": {}, "hehe": {},
	"brb": {}, "gtg": {}, "np": {}, "sure": {}, "alright": {},
	"got it": {}, "gotcha": {}, "k": {}, "kk": {}, "mhm": {},
	"oh": {}, "ah": {}, "uh": {}, "um": {}, "erm": {},
}

// IsLowValueTurn reports whether a chat turn is pure acknowledgement /
// filler that should not be indexed into RAG. Explicit SaveExplicit is never
// routed through this — only the automatic SaveInteraction path (via
// app.saveMemorySync) should call it.
//
// Rules (all must hold for skip):
//  1. Both sides ≤ maxLowValueRunes after normalize (or empty assistant).
//  2. User matches a bilingual ack set, OR user is very short (≤12 runes),
//     has no digit, and is not a real multi-word question.
func IsLowValueTurn(userMsg, assistantMsg string) bool {
	user := normalizeLowValue(userMsg)
	reply := normalizeLowValue(assistantMsg)
	if user == "" {
		return true
	}
	if runeLen(user) > maxLowValueRunes {
		return false
	}
	if reply != "" && runeLen(reply) > maxLowValueRunes {
		return false
	}

	if _, ok := lowValueAcks[user]; ok {
		return true
	}

	// Very short user noise: "hmm?", "aaa", "👍" equivalents already stripped
	// of punctuation — keep anything with a digit (dates, phone, ages, etc.).
	if runeLen(user) <= maxVeryShortUserRunes && !containsDigit(user) {
		// A short question mark form like "ne?" / "neden?" may still be low
		// value when it's pure filler; "adım ne?" has a space + content word
		// and is longer path — still under 12 runes though. Require either a
		// single token (no space) or exact ack-like form.
		if !strings.Contains(user, " ") {
			return true
		}
	}
	return false
}

// normalizeLowValue lowercases, trims, and strips punctuation/symbols so
// "Tamam!", "tamam?", "OK..." all collapse to the same key. Whitespace is
// collapsed to single spaces. Turkish İ/I handled via unicode.ToLower.
func normalizeLowValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			prevSpace = false
			continue
		}
		// Keep single spaces between words; drop punctuation entirely.
		if unicode.IsSpace(r) {
			if !prevSpace && b.Len() > 0 {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		// Drop other runes (punctuation, emoji, symbols).
	}
	return strings.TrimSpace(b.String())
}

func runeLen(s string) int {
	return len([]rune(s))
}

func containsDigit(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

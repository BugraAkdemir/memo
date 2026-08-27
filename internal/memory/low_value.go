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

	// "What time is it?" / "saat kaç?" and friends. The answer is a live
	// clock/calendar reading — zero durable value, and actively harmful:
	// once "it's 14:32 on Wednesday" is in RAG, a later time question can
	// retrieve that stale line and the model reports the old time instead
	// of reading the fresh [Time context] block from its system prompt.
	// Checked before the reply-length gate on purpose — a full spoken
	// answer ("it's 14:32 on Wednesday, 27 August 2026") easily exceeds
	// maxLowValueRunes.
	if isTimeOrDateQuestion(user) {
		return true
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

// timeOrDateQuestions is a bilingual set of normalized whole-message forms
// (lower, trimmed, punctuation stripped — same shape as lowValueAcks) that
// are pure "tell me the current time/date" questions.
// Both the real diacritic forms (what normalizeLowValue actually produces
// for Turkish input — it lowercases but does NOT ASCII-fold) and the
// diacritic-free forms people often type. Same both-forms approach
// lowValueAcks already takes ("hayir"/"hayır").
var timeOrDateQuestions = map[string]struct{}{
	// Turkish — diacritic
	"saat kaç": {}, "saat kaçta": {}, "saat kaçı": {}, "saat kaç oldu": {},
	"şu an saat kaç": {}, "şimdi saat kaç": {}, "saat kaç şimdi": {},
	"saati söyler misin": {}, "saat kaç acaba": {}, "saat": {},
	"bugün günlerden ne": {}, "günlerden ne": {}, "bugün hangi gün": {},
	"hangi gün": {}, "hangi gündeyiz": {}, "bugün ayın kaçı": {},
	"ayın kaçı": {}, "tarih ne": {}, "bugünün tarihi ne": {}, "bugün ne": {},
	"hafta günü ne": {}, "gün ne": {},
	// Turkish — diacritic-free
	"saat kac": {}, "saat kacta": {}, "saat kaci": {}, "saat kac oldu": {},
	"su an saat kac": {}, "simdi saat kac": {}, "saat kac simdi": {},
	"saati soyler misin": {}, "saat kac acaba": {},
	"bugun gunlerden ne": {}, "gunlerden ne": {}, "bugun hangi gun": {},
	"hangi gun": {}, "hangi gundeyiz": {}, "bugun ayin kaci": {},
	"ayin kaci": {}, "bugunun tarihi ne": {}, "bugun ne": {},
	"hafta gunu ne": {}, "gun ne": {},
	// English
	"what time is it": {}, "what time is it now": {}, "whats the time": {},
	"what is the time": {}, "whats the time now": {}, "time now": {},
	"current time": {}, "the time": {}, "got the time": {},
	"whats the date": {}, "what is the date": {}, "current date": {},
	"whats todays date": {}, "what is todays date": {}, "todays date": {},
	"what day is it": {}, "what day is it today": {}, "whats today": {},
	"what is today": {}, "what is todays day": {},
}

// isTimeOrDateQuestion reports whether user (already normalized) is asking
// only for the current time or date. Exact-match against timeOrDateQuestions
// plus a couple of unambiguous substrings so a small amount of trailing
// politeness ("saat kaç acaba kanka") still counts.
func isTimeOrDateQuestion(user string) bool {
	if _, ok := timeOrDateQuestions[user]; ok {
		return true
	}
	for _, frag := range []string{
		"saat kaç", "saat kac", "what time is it", "whats the time",
		"whats the date", "what day is it",
	} {
		if strings.Contains(user, frag) {
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

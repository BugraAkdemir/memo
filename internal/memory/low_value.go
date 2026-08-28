package memory

import (
	"regexp"
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

	// "Who are you / what's your name / what model are you / what's running
	// under you" — the user probing the assistant's identity. Zero durable
	// value as a memory *about the user*, and once "User: model adın ne" is
	// in RAG it gets retrieved on unrelated future turns. Same class as the
	// time question above.
	if isIdentityMetaQuestion(user) {
		return true
	}

	// Language-agnostic backstop for the same bug: a short question (in ANY
	// language — the phrase lists above only cover TR/EN) whose answer is
	// essentially just a clock reading ("14:32", "Es ist 14:32", "saat
	// 14:32 civarı"). Gated hard to avoid eating durable facts: the user
	// message must be an actual question and must not be asking to set a
	// reminder/alarm/timer, and the answer must be short and contain an
	// HH:MM token. Uses the raw strings — normalizeLowValue strips the ":".
	if isQuestion(userMsg) && !hasSchedulingIntent(user) && looksLikeBareClockReading(assistantMsg) {
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

// identityMetaQuestions: normalized whole-message forms (same shape as
// timeOrDateQuestions) of the user asking who/what the assistant is, its
// name, its underlying model, or who made it. TR diacritic + diacritic-free
// + EN. These never carry a durable fact about the user.
var identityMetaQuestions = map[string]struct{}{
	// Turkish — diacritic
	"sen kimsin": {}, "kimsin": {}, "kimsin sen": {}, "sen nesin": {}, "nesin sen": {},
	"adın ne": {}, "adin ne": {}, "ismin ne": {}, "senin adın ne": {}, "adın ne senin": {},
	"sen kimsin adın ne": {}, "selam sen kimsin adın ne": {}, "kimsin adın ne": {},
	"model adın ne": {}, "modelin ne": {}, "hangi modelsin": {}, "hangi model": {},
	"altta çalışan ne": {}, "altta ne çalışıyor": {}, "altta ne var": {}, "altında ne var": {},
	"altta çalışan model ne": {}, "seni kim yaptı": {}, "seni kim yarattı": {},
	"kim yaptı seni": {}, "yapay zeka mısın": {}, "sen yapay zeka mısın": {},
	"hangi yapay zekasın": {}, "chatgpt misin": {}, "claude musun": {},
	// Turkish — diacritic-free
	"sen kimsin adin ne": {}, "selam sen kimsin adin ne": {}, "kimsin adin ne": {},
	"model adin ne": {}, "hangi modelsin sen": {},
	"altta calisan ne": {}, "altta ne calisiyor": {}, "altinda ne var": {},
	"altta calisan model ne": {}, "seni kim yapti": {}, "seni kim yaratti": {},
	"kim yapti seni": {}, "yapay zeka misin": {}, "hangi yapay zekasin": {},
	// English
	"who are you": {}, "what are you": {}, "whats your name": {}, "what is your name": {},
	"your name": {}, "what model are you": {}, "which model are you": {}, "what model is this": {},
	"whats running under you": {}, "what is running under you": {}, "whats under the hood": {},
	"who made you": {}, "who created you": {}, "who built you": {},
	"are you an ai": {}, "are you a robot": {}, "are you chatgpt": {}, "are you claude": {},
	"are you gpt": {}, "what llm are you": {}, "which llm are you": {},
}

// identityMetaFragments are unambiguous substrings so a little trailing
// politeness ("sen kimsin kanka") or a leading greet still counts. Kept
// tight to avoid eating real facts ("what model of car…" must NOT match, so
// "what model are you" is a fragment, bare "what model" is not).
var identityMetaFragments = []string{
	"sen kimsin", "adın ne", "adin ne", "ismin ne", "model adın", "model adin",
	"hangi modelsin", "seni kim yaptı", "seni kim yapti", "seni kim yarattı", "seni kim yaratti",
	"who are you", "what are you", "your name", "what model are you", "which model are you",
	"who made you", "who created you", "who built you",
}

// isIdentityMetaQuestion reports whether user (already normalized) is asking
// about the assistant's identity/name/model rather than stating a fact.
func isIdentityMetaQuestion(user string) bool {
	if _, ok := identityMetaQuestions[user]; ok {
		return true
	}
	for _, frag := range identityMetaFragments {
		if strings.Contains(user, frag) {
			return true
		}
	}
	return false
}

// clockToken matches an HH:MM / HH.MM wall-clock value.
var clockToken = regexp.MustCompile(`\b\d{1,2}[:.]\d{2}\b`)

// schedulingWords mark a "set a reminder/alarm/timer at X" turn — durable,
// must never be dropped by the clock-reading backstop. Diacritic + plain.
var schedulingWords = []string{
	"hatirlat", "hatırlat", "alarm", "zamanlayici", "zamanlayıcı", "kur ",
	"remind", "reminder", "set a", "set an", "schedule", "timer", "wake me", "alert me",
}

func hasSchedulingIntent(user string) bool {
	for _, w := range schedulingWords {
		if strings.Contains(user, w) {
			return true
		}
	}
	return false
}

// isQuestion reports whether raw looks like a question (any script's
// question mark). normalizeLowValue drops "?", so this checks the raw text.
func isQuestion(raw string) bool {
	return strings.ContainsAny(raw, "?？؟")
}

// looksLikeBareClockReading: a short raw reply that is essentially just a
// wall-clock value. Length-capped so "it's 14:32, and your 15:00 meeting
// is still on" (real content) doesn't match.
func looksLikeBareClockReading(rawReply string) bool {
	r := strings.TrimSpace(rawReply)
	if r == "" || runeLen(r) > 48 {
		return false
	}
	return clockToken.MatchString(r)
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

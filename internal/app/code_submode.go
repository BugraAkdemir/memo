package app

import (
	"regexp"
	"strings"
)

// planDecisionWordRe splits a reply into whole words (Unicode letters/
// digits) so classifyPlanDecisionReply never false-matches a substring
// inside an unrelated word (e.g. "autobiography" must not match "auto").
var planDecisionWordRe = regexp.MustCompile(`[\p{L}\p{N}]+`)

// planDecisionAffirmatives are plain "yes, go ahead" replies with no named
// sub-mode — these default to "auto" (today's baseline Code Mode behavior,
// the more conservative of the two named choices) per the design's decision
// 1: "if the user just says yes, Memo picks the mode itself."
var planDecisionAffirmatives = map[string]bool{
	"evet": true, "yes": true, "uygula": true, "devam": true,
	"tamam": true, "tamamdır": true, "olur": true, "ok": true, "okay": true, "yap": true,
}

// classifyPlanDecisionReply implements the one-shot text classification for
// the message immediately following a saved plan (Code Mode "plan"
// sub-mode, save_code_plan just succeeded, global auto-permission was off so
// the model's own reply asked in plain chat text whether to move to build or
// auto). Deliberately keyword-based, not an LLM call —
// sendMessageStreamCore runs this on literally the next message in any chat
// with AwaitingPlanDecision set, and it must stay instant and free.
//
// An explicit "build" or "auto" mention wins over a plain affirmative, so
// "evet build'e geçelim" picks build, not the affirmative default.
// Anything matching neither list — including an explicit negative like
// "hayır"/"no"/"dur"/"iptal" — reports no match: "no clear signal" means
// "don't change anything" (stay in plan sub-mode), never "assume a mode."
func classifyPlanDecisionReply(msg string) (subMode string, matched bool) {
	words := planDecisionWordRe.FindAllString(strings.ToLower(msg), -1)
	if len(words) == 0 {
		return "", false
	}
	for _, w := range words {
		if w == "build" {
			return "build", true
		}
	}
	for _, w := range words {
		if w == "auto" {
			return "auto", true
		}
	}
	for _, w := range words {
		if planDecisionAffirmatives[w] {
			return "auto", true
		}
	}
	return "", false
}

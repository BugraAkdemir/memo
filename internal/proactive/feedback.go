// SPDX-License-Identifier: AGPL-3.0-or-later

package proactive

import (
	"strings"
	"unicode"
)

// Outcome is how the user reacted to a proactive suggestion.
type Outcome string

const (
	OutcomeAccepted Outcome = "accepted" // "Evet", "Harika"
	OutcomeIgnored  Outcome = "ignored"  // no response within TTL
	OutcomeRejected Outcome = "rejected" // "Hayır", "Sonra"
	OutcomeStopped  Outcome = "stopped"  // "Artık yapmıyorum" → delete pattern
)

// Confidence deltas applied to a pattern per outcome (README §9).
const (
	deltaAccepted = +0.10
	deltaIgnored  = -0.03
	deltaRejected = -0.15
)

// FeedbackEntry is one round of the self-correcting loop: what was suggested and
// how the user reacted. The Chief sees the most recent few.
type FeedbackEntry struct {
	PatternID string  `json:"pattern_id"`
	Action    Action  `json:"action"`
	Message   string  `json:"message"`
	Outcome   Outcome `json:"outcome"`
}

// ConfidenceDelta maps an outcome to the confidence adjustment. OutcomeStopped
// returns 0 because the pattern is deleted outright, not nudged.
func ConfidenceDelta(o Outcome) float64 {
	switch o {
	case OutcomeAccepted:
		return deltaAccepted
	case OutcomeIgnored:
		return deltaIgnored
	case OutcomeRejected:
		return deltaRejected
	default:
		return 0
	}
}

// Single-word reaction tokens, matched on word boundaries so e.g. the Turkish
// "yok" ("no") is not mistaken for the affirmative "ok".
var (
	rejectWords = map[string]bool{
		"no": true, "nope": true, "hayır": true, "hayir": true, "yok": true,
		"sonra": true, "olmaz": true, "istemiyorum": true,
	}
	acceptWords = map[string]bool{
		"yes": true, "yep": true, "evet": true, "tamam": true, "olur": true,
		"harika": true, "başla": true, "basla": true, "devam": true,
		"ok": true, "okay": true, "tabii": true, "tabi": true,
	}
)

// OutcomeFromResponse maps a free-form user response to an Outcome. Multi-word
// phrases and emojis are matched as substrings; single affirmative/negative
// words are matched on word boundaries to avoid false positives (e.g. "yok"
// containing "ok").
func OutcomeFromResponse(resp string) Outcome {
	r := strings.ToLower(strings.TrimSpace(resp))
	if r == "" {
		return OutcomeIgnored
	}

	switch {
	case containsAny(r, "artık yapmıyorum", "artik yapmiyorum", "bir daha yapma",
		"bir daha sorma", "hiç yapma", "hic yapma", "vazgeç", "vazgec", "stop doing", "kapat şunu"):
		return OutcomeStopped
	case strings.Contains(r, "❌"):
		return OutcomeRejected
	case strings.Contains(r, "✅"), strings.Contains(r, "👍"):
		return OutcomeAccepted
	case containsAny(r, "şimdi değil", "simdi degil", "şu an değil", "su an degil"):
		return OutcomeRejected
	}

	// First decisive single word wins.
	for _, w := range strings.FieldsFunc(r, func(c rune) bool { return !unicode.IsLetter(c) }) {
		if rejectWords[w] {
			return OutcomeRejected
		}
		if acceptWords[w] {
			return OutcomeAccepted
		}
	}
	return OutcomeIgnored
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

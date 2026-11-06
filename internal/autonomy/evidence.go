package autonomy

import (
	"encoding/json"

	"memo/internal/twin"
)

// Every model input is capped (engine.call). Without trimming, a contact with a
// long history or a long incoming mail used to make the whole attempt fail as a
// provider error (found live). Old context is dropped before the newest is.
const (
	maxUntrustedRunes = 3200 // wrapped, HTML-escaped external message
	maxEvidenceRunes  = 3200 // all research results together
	maxEvidenceItems  = 12   // newest history messages kept
	maxEvidenceBody   = 240  // runes of each kept message
)

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// wrapBounded wraps an external message and shrinks the text until the wrapped,
// escaped result fits (escaping can grow quote-heavy text several times over).
func wrapBounded(sender, channel, text string, max int) string {
	text = truncateRunes(text, max)
	for {
		w := WrapUntrustedExternalMessage(sender, channel, text)
		r := []rune(text)
		if len([]rune(w)) <= max || len(r) < 40 {
			return w
		}
		text = string(r[:len(r)*3/4])
	}
}

type evidenceMessage struct {
	Channel  string `json:"channel"`
	Outgoing bool   `json:"outgoing"`
	SentAt   string `json:"sent_at"`
	Body     string `json:"body"`
}

// compactEvidence slims a tool result. Only history lists (newest first, as the
// store returns them) are reduced; everything else is already small.
func compactEvidence(result any) any {
	msgs, ok := result.([]twin.Message)
	if !ok {
		return result
	}
	out := make([]evidenceMessage, 0, maxEvidenceItems)
	for _, m := range msgs {
		if len(out) == maxEvidenceItems {
			break
		}
		out = append(out, evidenceMessage{Channel: m.Channel, Outgoing: m.Outgoing, SentAt: m.SentAt.Format("2006-01-02T15:04Z07:00"), Body: truncateRunes(m.Body, maxEvidenceBody)})
	}
	return out
}

// fitEvidence drops the OLDEST history messages until all evidence fits.
func fitEvidence(evidence map[string]any) {
	for {
		b, err := json.Marshal(evidence)
		if err != nil || len([]rune(string(b))) <= maxEvidenceRunes {
			return
		}
		trimmed := false
		for name, v := range evidence {
			if list, ok := v.([]evidenceMessage); ok && len(list) > 1 {
				evidence[name] = list[:len(list)-1]
				trimmed = true
			}
		}
		if !trimmed {
			return
		}
	}
}

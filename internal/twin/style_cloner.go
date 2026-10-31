package twin

import (
	"github.com/google/uuid"
	"strings"
	"time"
)

func newID() string                         { return uuid.NewString() }
func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

// StyleProfile is evidence from outgoing messages only, never from a sender's
// instructions. Low sample counts are surfaced instead of pretending certainty.
type StyleProfile struct {
	Samples    int    `json:"samples"`
	Tone       string `json:"tone"`
	Language   string `json:"language"`
	Greeting   string `json:"greeting"`
	Sufficient bool   `json:"sufficient"`
}

func InferStyle(messages []Message) StyleProfile {
	p := StyleProfile{Tone: "unknown", Language: "unknown"}
	informal, formal, tr, en := 0, 0, 0, 0
	greetings := map[string]int{}
	for _, m := range messages {
		if !m.Outgoing || strings.TrimSpace(m.Body) == "" {
			continue
		}
		p.Samples++
		text := strings.ToLower(strings.TrimSpace(m.Body))
		for _, w := range []string{"kanka", "dostum", "selam", "abi"} {
			if strings.Contains(text, w) {
				informal++
			}
		}
		for _, w := range []string{"sayın", "saygılarımla", "regards", "dear "} {
			if strings.Contains(text, w) {
				formal++
			}
		}
		if strings.ContainsAny(text, "çğıöşü") || strings.Contains(text, "selam") || strings.Contains(text, "merhaba") {
			tr++
		}
		if strings.Contains(text, "hello") || strings.Contains(text, "regards") || strings.Contains(text, "hi ") {
			en++
		}
		for _, g := range []string{"selam", "merhaba", "sayın", "hello", "hi", "dear"} {
			if strings.HasPrefix(text, g) {
				greetings[g]++
				break
			}
		}
	}
	p.Sufficient = p.Samples >= 3
	if informal > formal {
		p.Tone = "informal"
	} else if formal > informal {
		p.Tone = "formal"
	}
	if tr > en {
		p.Language = "tr"
	} else if en > tr {
		p.Language = "en"
	}
	best := 0
	for _, g := range []string{"selam", "merhaba", "sayın", "hello", "hi", "dear"} {
		if greetings[g] > best {
			best = greetings[g]
			p.Greeting = g
		}
	}
	return p
}

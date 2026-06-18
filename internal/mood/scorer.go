package mood

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

// Decider LLM çağrı arayüzü — app paketinden enjekte edilir.
type Decider func(ctx context.Context, systemPrompt, userPrompt string) (string, error)

// scorerSystemPrompt instructs the model to return only a bare number.
// Few-shot examples guide small models that otherwise add explanation text.
const scorerSystemPrompt = `You are a sentiment scoring system. Your ONLY job is to output a single number.

Analyze the emotional tone of the USER message and output a single float in [-10.0, 10.0].
Scale: -10 = extreme hostility/insult, 0 = neutral/factual, +10 = affection/praise/enthusiasm.

RULES — read carefully:
- Output ONLY the number. No words. No punctuation. No explanation. No prefix. Nothing else.
- Short factual messages ("ok", "yes", "send it") → 0.0
- Greetings / casual chat → slight positive (1.0 – 2.0)
- Complaints, frustration → negative (-2.0 to -5.0)
- Insults, aggression → strongly negative (-6.0 to -10.0)
- Excitement, compliments → positive (3.0 to 8.0)

EXAMPLES (message → number only):
"hey thanks!" → 3.5
"ok" → 0.0
"this is broken again" → -2.5
"you're useless" → -7.0
"amazing, love it!" → 7.5
"can you help me write a function?" → 0.0`

// scorePattern extracts the first float-like token (with optional leading minus)
// from a model response. Handles models that add prefixes ("Score: 3.2") or
// suffixes ("3.2 (slightly positive)") despite the system prompt instruction.
var scorePattern = regexp.MustCompile(`-?\d+(?:\.\d+)?`)

// Scorer LLM tabanlı duygu skoru hesaplar.
type Scorer struct {
	decide Decider
}

// NewScorer Decider ile Scorer oluşturur.
func NewScorer(decide Decider) *Scorer {
	return &Scorer{decide: decide}
}

// Score kullanıcı mesajını -10/+10 arasında puanlar.
// Hata durumunda 0.0 döner (nötr) — scoring hatası chat akışını kesmemeli.
func (s *Scorer) Score(ctx context.Context, userMsg string) float64 {
	raw, err := s.decide(ctx, scorerSystemPrompt, userMsg)
	if err != nil {
		log.Printf("mood.Scorer: LLM call failed: %v", err)
		return 0.0
	}
	score, err := parseScore(raw)
	if err != nil {
		log.Printf("mood.Scorer: parse failed (raw=%q): %v", raw, err)
		return 0.0
	}
	return clamp(score, -10, 10)
}

// parseScore extracts a float from the LLM's raw response.
// Fast path: response is already a bare number (model followed instructions).
// Fallback: regex extracts the first float-like token so models that add
// surrounding text ("Score: 3.2", "3.2 (positive)") still parse correctly.
func parseScore(raw string) (float64, error) {
	cleaned := strings.TrimSpace(raw)
	// Fast path — model returned exactly a number.
	if v, err := strconv.ParseFloat(cleaned, 64); err == nil {
		return v, nil
	}
	// Fallback — extract first number from free-form text.
	m := scorePattern.FindString(cleaned)
	if m == "" {
		return 0, fmt.Errorf("parseScore: no number found in %q", cleaned)
	}
	v, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0, fmt.Errorf("parseScore: %q → %w", m, err)
	}
	return v, nil
}

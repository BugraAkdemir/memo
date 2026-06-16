package mood

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
)

// Decider LLM çağrı arayüzü — app paketinden enjekte edilir.
type Decider func(ctx context.Context, systemPrompt, userPrompt string) (string, error)

const scorerSystemPrompt = `You are a sentiment scoring system.
Analyze the emotional tone of the user message and return a single float between -10.0 and +10.0.
Scale: -10 = extreme hostility or insult, 0 = neutral, +10 = affection or praise.
Rules: No explanation. No text. No punctuation. Only the number. Examples: -7.5 or 3.2 or 0.0`

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

// parseScore LLM'in döndürdüğü ham metni float'a çevirir.
func parseScore(raw string) (float64, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.Split(cleaned, "\n")[0]
	cleaned = strings.Trim(cleaned, "\"'.,;:")
	v, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("parseScore: %q → %w", cleaned, err)
	}
	return v, nil
}

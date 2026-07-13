package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/provider"
)

const importMemorySystemPrompt = `You extract personal memory items from arbitrary text a user pastes in — usually another AI assistant's answer describing what it knows about the user. The input is often organized into labeled categories (e.g. Demographics, Interests & Preferences, Relationships, Dated Events, Communication Style & Personality, Instructions), with each entry followed by an "Evidence:"/"Basis:" sub-line quoting or explaining where it came from, and a trailing "Source: <name>" line. Read it carefully and return ONLY a single JSON object, no other text, no markdown fences, in this exact shape:

{"facts": ["short atomic fact or preference about the user", "..."], "style_summary": "a cohesive paragraph of behavioral/tonal instructions for how to talk to this user, or empty string if none is present"}

Rules:
- Ignore category headers, "Evidence:"/"Basis:" sub-lines, and any trailing "Source: <name>" line — they are provenance, not content.
- Facts (Demographics, Interests & Preferences, Relationships, Dated Events/Projects/Plans, and anything else describing what the user IS/HAS/DID): turn each entry into one short, self-contained, concrete sentence — one fact per item, no bullet symbols. Preserve concrete specifics (names, dates, project names) rather than vague generalities.
- Style (Communication Style & Personality, plus any Instructions describing HOW to respond — tone, formatting rules, standing behavioral requests, a trigger-phrase-to-canned-response rule, etc.): fold ALL of these into one "style_summary" paragraph, since it gets injected into every future reply rather than retrieved probabilistically like facts. Preserve specific, actionable instructions rather than vaguely paraphrasing them away.
- Skip filler, meta-commentary, or anything that isn't actually about the user.
- If the input has no usable content at all, return {"facts": [], "style_summary": ""}.`

type importedMemory struct {
	Facts        []string `json:"facts"`
	StyleSummary string   `json:"style_summary"`
}

// extractJSON finds the first balanced JSON object in s, string-aware so
// braces inside string literals don't truncate it early. This codebase
// keeps one small per-package copy of this helper (internal/intent,
// internal/orchestra, internal/taskloop, internal/proactive) rather than
// sharing an exported one across packages — same pattern here.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// ImportMemoryFromText takes an arbitrary block of text pasted by the user
// (typically another AI's answer to a "tell me what you know about me"
// style prompt), asks the active provider to structure it into atomic
// memory facts plus a communication-style summary, saves each fact as an
// explicit memory (source="explicit", same path as the /remember chat
// command), and — if a style summary was found — persists it as the
// identity's LearnedStyleNotes so BuildSystemPrompt injects it into every
// future system prompt.
func (a *App) ImportMemoryFromText(ctx context.Context, rawText string) (factsSaved int, styleUpdated bool, err error) {
	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return 0, false, fmt.Errorf("empty text")
	}

	a.providerMu.RLock()
	router := a.providerRouter
	a.providerMu.RUnlock()
	if router == nil {
		return 0, false, fmt.Errorf("no provider router available")
	}

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	req := provider.ChatRequest{
		MaxTokens: 4000,
		Messages: []provider.Message{
			provider.TextMessage("system", importMemorySystemPrompt),
			provider.TextMessage("user", rawText),
		},
	}
	resp, err := router.ChatCompletion(ctx, req)
	if err != nil {
		return 0, false, fmt.Errorf("structuring LLM call: %w", err)
	}

	jsonStr := extractJSON(resp.Content)
	if jsonStr == "" {
		return 0, false, fmt.Errorf("model did not return usable JSON")
	}
	var parsed importedMemory
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return 0, false, fmt.Errorf("unmarshal structured memory: %w", err)
	}

	for _, fact := range parsed.Facts {
		fact = strings.TrimSpace(fact)
		if fact == "" {
			continue
		}
		if err := a.SaveExplicitMemory(fact, "imported"); err != nil {
			logx.Printf("WARN: ImportMemoryFromText: save fact: %v", err)
			continue
		}
		factsSaved++
	}

	if style := strings.TrimSpace(parsed.StyleSummary); style != "" {
		a.identity.SetLearnedStyleNotes(style)
		a.cfg.Identity.LearnedStyleNotes = style
		if saveErr := config.Save(a.cfg); saveErr != nil {
			logx.Printf("WARN: ImportMemoryFromText: save style config: %v", saveErr)
		} else {
			styleUpdated = true
		}
	}

	return factsSaved, styleUpdated, nil
}

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/truncate"
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

// maxImportedFactsPerCall bounds a single import — deliberately higher than
// extractAndPinFacts' per-turn maxExtractedFactsPerTurn, since one paste can
// legitimately cover a whole multi-category profile dump rather than one
// chat turn's worth of content, but still bounded: pinned facts bypass the
// RAG relevance decay a normal memory would age out under (same reasoning as
// maxExtractedFactsPerTurn/maxExtractedFactLength in memory.go), so an
// unbounded or hallucinating model response must not be able to flood the
// pinned-facts list.
const maxImportedFactsPerCall = 30

// ImportMemoryFromText takes an arbitrary block of text pasted by the user
// (typically another AI's answer to a "tell me what you know about me"
// style prompt), asks the active model to structure it into atomic memory
// facts plus a communication-style summary, saves each fact as an explicit
// memory (source="explicit", same path as the /remember chat command), and
// — if a style summary was found — persists it as the identity's
// LearnedStyleNotes so BuildSystemPrompt injects it into every future
// system prompt.
//
// Routes through callLLM (Orchestra → external provider → local model,
// same priority chain as regular chat, internal/app/llm.go) rather than
// reaching into a.providerRouter directly — an earlier version only ever
// tried the provider router, so a local-only setup silently never got
// tried at all, and "nothing connected" hung on a live network call
// instead of failing immediately with callLLM's existing "no model
// loaded" message.
//
// Fact saving shares extractAndPinFacts' two guards (memory.go) — this
// feature predates that mechanism and was never updated to match it: (1)
// dedup against already-pinned facts via pinnedFactTexts, since a user can
// paste the same "what do you know about me" export more than once (e.g.
// after asking the other AI again) and would otherwise get a fresh
// duplicate pinned entry each time; (2) a length cap per fact
// (maxExtractedFactLength) and a count cap for the whole call
// (maxImportedFactsPerCall), since parsed.Facts comes straight from the
// model's JSON output and pinned facts have no relevance decay to bound
// unchecked growth the way ordinary RAG memories do.
func (a *App) ImportMemoryFromText(ctx context.Context, rawText string) (factsSaved int, styleUpdated bool, err error) {
	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return 0, false, fmt.Errorf("empty text")
	}

	msgs := []api.Message{
		api.NewTextMessage("system", importMemorySystemPrompt),
		api.NewTextMessage("user", rawText),
	}
	reply := a.callLLM(ctx, msgs, categoryMemoryImport)
	if isLLMErrorReply(reply) {
		return 0, false, fmt.Errorf("%s", strings.TrimPrefix(reply, "⚠️ "))
	}

	jsonStr := extractJSON(reply)
	if jsonStr == "" {
		return 0, false, fmt.Errorf("model did not return usable JSON")
	}
	var parsed importedMemory
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return 0, false, fmt.Errorf("unmarshal structured memory: %w", err)
	}

	pinned := a.pinnedFactTexts(ctx)
	for _, fact := range parsed.Facts {
		if factsSaved >= maxImportedFactsPerCall {
			logx.Printf("MEMORY: ImportMemoryFromText: import fact cap (%d) reached, dropping remaining", maxImportedFactsPerCall)
			break
		}
		fact = strings.TrimSpace(fact)
		if fact == "" {
			continue
		}
		if len(fact) > maxExtractedFactLength {
			fact = fact[:maxExtractedFactLength]
		}
		key := normalizeFactText(fact)
		if _, dup := pinned[key]; dup {
			logx.Printf("MEMORY: ImportMemoryFromText: skipping duplicate imported fact: %q", truncate.Text(fact, 60))
			continue
		}
		if err := a.SaveExplicitMemory(fact, "imported"); err != nil {
			logx.Printf("WARN: ImportMemoryFromText: save fact: %v", err)
			continue
		}
		pinned[key] = struct{}{}
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

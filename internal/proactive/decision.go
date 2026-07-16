// SPDX-License-Identifier: AGPL-3.0-or-later

package proactive

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Action is the kind of proactive step the Chief decided on.
type Action string

const (
	ActionNone    Action = "none"    // do nothing
	ActionNotify  Action = "notify"  // mobile push notification
	ActionSuggest Action = "suggest" // write a chat message
	ActionAuto    Action = "auto"    // start the agent pipeline

	// ActionAmbient marks a suggestion that was never a separate banner/
	// notification at all — the main chat model was simply told a habit
	// might be worth mentioning and wove it into an ordinary reply on its
	// own judgment. See internal/app/proactive_ambient.go.
	ActionAmbient Action = "ambient"
)

// Valid reports whether a is a recognised action.
func (a Action) Valid() bool {
	switch a {
	case ActionNone, ActionNotify, ActionSuggest, ActionAuto, ActionAmbient:
		return true
	default:
		return false
	}
}

// Level is the user-controlled proactivity setting (README §7). Off means the
// engine observes but never acts.
type Level string

const (
	LevelOff       Level = "off"
	LevelSubtle    Level = "subtle"
	LevelNormal    Level = "normal"
	LevelAssertive Level = "assertive"
)

// ParseLevel normalises a free-form string into a Level, defaulting to Off for
// anything unrecognised (privacy-safe default).
func ParseLevel(s string) Level {
	switch Level(strings.ToLower(strings.TrimSpace(s))) {
	case LevelSubtle:
		return LevelSubtle
	case LevelNormal:
		return LevelNormal
	case LevelAssertive:
		return LevelAssertive
	default:
		return LevelOff
	}
}

// Decision is the Chief's structured output.
type Decision struct {
	Action    Action `json:"decision"`
	Message   string `json:"message"`
	PatternID string `json:"pattern_id"`
}

// ParseDecision extracts a Decision from a raw LLM response. LLMs often wrap
// JSON in prose or code fences, so the first balanced JSON object is located and
// parsed. An unknown or missing action falls back to ActionNone rather than
// erroring, so a malformed reply never causes an unwanted action.
func ParseDecision(raw string) (Decision, error) {
	jsonStr, ok := extractJSONObject(raw)
	if !ok {
		return Decision{Action: ActionNone}, fmt.Errorf("proactive.ParseDecision: no JSON object found")
	}
	var d Decision
	if err := json.Unmarshal([]byte(jsonStr), &d); err != nil {
		return Decision{Action: ActionNone}, fmt.Errorf("proactive.ParseDecision: %w", err)
	}
	d.Action = Action(strings.ToLower(strings.TrimSpace(string(d.Action))))
	if !d.Action.Valid() {
		d.Action = ActionNone
	}
	return d, nil
}

// extractJSONObject returns the first balanced {...} run in s, ignoring braces
// inside string literals.
func extractJSONObject(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
	}
	depth := 0
	inStr := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}

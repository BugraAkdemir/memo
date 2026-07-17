// SPDX-License-Identifier: AGPL-3.0-or-later

// Package jsonutil holds small, dependency-free JSON-adjacent helpers shared
// across packages that parse LLM output.
package jsonutil

import "strings"

// ExtractBalancedObject finds the first balanced {...} object in s and
// returns it, or ("", false) if none is found. LLMs sometimes wrap JSON in
// prose or markdown fences despite being told not to; this scans forward
// from the first "{" tracking brace depth (string/escape-aware, so a brace
// inside a quoted string doesn't affect depth) until it closes, rather than
// assuming the whole response is clean JSON.
//
// Consolidated from three near-identical private copies (internal/intent,
// internal/proactive, internal/routine — TD-1): a bug found in one used to
// have to be found and fixed again in the others independently.
func ExtractBalancedObject(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
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
				return s[start : i+1], true
			}
		}
	}
	return "", false
}

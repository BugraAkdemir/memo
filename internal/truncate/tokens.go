package truncate

// Text truncates s to at most n runes, appending "..." if it was cut.
// Rune-safe (operates on runes, not bytes) so a cut can't land mid-character
// and corrupt multi-byte UTF-8 (Turkish ı, ş, ğ, ü, ö, ç, or any non-ASCII
// text) — several earlier per-package copies of this exact helper sliced by
// byte instead and could do exactly that. Consolidated here so a fix only
// needs to happen once (TD-2).
func Text(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

// EstimateTokens provides a rough token count for a given text.
// Uses len/3 as a reasonable upper bound for mixed content (code, Turkish, English).
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(text) / 3
}

// TruncateMessages truncates a message list to fit within maxTokens.
// Preserves system prompt, drops oldest non-system messages first.
func TruncateMessages(messages []Message, maxTokens int) []Message {
	if maxTokens <= 0 || len(messages) <= 1 {
		return messages
	}

	systemTokens := 0
	sysIdx := -1

	for i, msg := range messages {
		if msg.Role == "system" && sysIdx < 0 {
			systemTokens = EstimateTokens(msg.Content)
			sysIdx = i
		}
	}

	budget := maxTokens - systemTokens
	if budget <= 0 {
		if sysIdx >= 0 {
			return []Message{messages[sysIdx]}
		}
		return messages[:1]
	}

	start := sysIdx + 1
	if start >= len(messages) {
		return messages
	}

	nonSystem := messages[start:]

	total := 0
	keepStart := len(nonSystem)
	for i := len(nonSystem) - 1; i >= 0; i-- {
		msgTokens := EstimateTokens(nonSystem[i].Content)
		if total+msgTokens > budget {
			break
		}
		total += msgTokens
		keepStart = i
	}

	// BUG-M6: a plain token-budget cutoff can land in the middle of an
	// assistant-with-tool_calls + its tool-result messages — this Message
	// projection doesn't carry ToolCalls/ToolCallID (see its own doc
	// comment), but Role alone is enough to detect the boundary: a "tool"
	// message's result always belongs to the nearest preceding "assistant"
	// message that requested it, and that pairing is never optional —
	// dropping the assistant message while keeping its tool results
	// produces an invalid message sequence (a tool-role message with no
	// preceding assistant declaring its tool_call_id), which providers
	// reject outright. Walk keepStart back to that assistant message so
	// the whole group survives as one unit, even if this pushes the kept
	// slice over budget — an over-budget-but-valid request beats an
	// under-budget-but-invalid one.
	for keepStart > 0 && keepStart < len(nonSystem) && nonSystem[keepStart].Role == "tool" {
		keepStart--
	}

	if keepStart >= len(nonSystem) {
		if sysIdx >= 0 {
			return []Message{messages[sysIdx]}
		}
		return messages[:1]
	}

	var result []Message
	if sysIdx >= 0 {
		result = append(result, messages[sysIdx])
	}
	result = append(result, nonSystem[keepStart:]...)
	return result
}

// Message is a minimal message representation for token-aware truncation.
// Index carries the message's position in the caller's original slice so
// callers that need to recover richer original messages (e.g. with
// ToolCalls/ToolCallID) can do so by position instead of matching on
// Content, which is not guaranteed to be unique (e.g. two tool results
// that both returned "OK").
type Message struct {
	Role    string
	Content string
	Index   int
}

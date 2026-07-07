package truncate

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

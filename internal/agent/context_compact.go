package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"memo/internal/provider"
	"memo/internal/truncate"
)

// In-turn context management for the agent loop.
//
// A long coding turn is dozens of model calls, and every one re-sends everything
// the turn has produced so far — each file read, each command output, each
// write_file body. The old guard dropped the OLDEST messages when its estimate
// passed the budget. That discarded exactly the wrong things: the user's own
// request and the earlier conversation are the oldest messages in the turn, so
// a long task slowly forgot what it had been asked to do. Its estimate also
// ignored the arguments of tool calls (a write_file body is often the largest
// thing in the turn), so it undercounted and a 55-step turn ran into the
// provider's hard limit and died with a 400.
//
// What happens now:
//   - the seed (system prompt, conversation history and the user's request) is
//     never touched here;
//   - the size is the provider's own prompt-token count from the last call plus
//     an estimate of what was added since, not a guess over the content alone;
//   - when it passes the budget, OLD TOOL OUTPUTS are replaced by a one-line
//     stub (and huge string arguments of old tool calls likewise), oldest first,
//     down to a lower target, so the next compaction is far off and the
//     provider's prompt cache is rebuilt once rather than on every step;
//   - only if that is not enough are whole old tool exchanges dropped.
// Messages keep their structure (every tool call still has its result), so the
// request stays valid for every provider.

const (
	// compactTargetFrac is where a compaction aims, as a share of the budget.
	// Well below the trigger so one compaction buys many steps and the cached
	// prefix is invalidated rarely.
	compactTargetFrac = 0.6
	// keepRecentToolResults are never elided by an ordinary compaction: the
	// model is usually still working with them.
	keepRecentToolResults = 6
	// minElidedChars: tool outputs/args shorter than this are not worth a stub.
	minElidedChars = 600
	// elidedArgChars: a string argument longer than this in an OLD tool call
	// (a file body, a long patch) is replaced by a stub.
	elidedArgChars = 400
)

// estimateMessageTokens is a deliberately generous size estimate for one
// message: text, plus the arguments of its tool calls, plus a provider-native
// replay payload (Claude thinking blocks).
func estimateMessageTokens(m provider.Message) int {
	n := 4 // role/format overhead
	switch c := m.Content.(type) {
	case string:
		n += truncate.EstimateTokens(c)
	case nil:
	default:
		if b, err := json.Marshal(c); err == nil {
			n += truncate.EstimateTokens(string(b))
		}
	}
	for _, tc := range m.ToolCalls {
		n += 8 + truncate.EstimateTokens(tc.Function.Name) + truncate.EstimateTokens(string(tc.Function.Arguments))
	}
	if m.Native != nil {
		if b, err := json.Marshal(m.Native); err == nil {
			n += truncate.EstimateTokens(string(b))
		}
	}
	return n
}

func estimateMessagesTokens(msgs []provider.Message) int {
	total := 0
	for _, m := range msgs {
		total += estimateMessageTokens(m)
	}
	return total
}

// estimateToolDefsTokens sizes the tool schema, which is re-sent on every call.
func estimateToolDefsTokens(defs []provider.ToolDefinition) int {
	b, err := json.Marshal(defs)
	if err != nil {
		return 0
	}
	return truncate.EstimateTokens(string(b))
}

func elidedOutputStub(chars int) string {
	return fmt.Sprintf("%s output elided to save context (%d chars). If you still need it, run the tool again.", contextTrimMarker, chars)
}

// elideLongStrings returns args with every string value longer than limit
// replaced by a stub, as valid JSON again. args may be a JSON object (Anthropic,
// Gemini) or a JSON string holding one (OpenAI-compatible, the shape the loop
// stores); anything else is returned unchanged.
func elideLongStrings(args json.RawMessage, limit int) (json.RawMessage, bool) {
	if len(args) <= limit {
		return args, false
	}
	var inner json.RawMessage = args
	wrapped := false
	var asString string
	if json.Unmarshal(args, &asString) == nil {
		inner = json.RawMessage(asString)
		wrapped = true
	}
	var obj map[string]any
	if json.Unmarshal(inner, &obj) != nil {
		return args, false
	}
	changed := false
	for k, v := range obj {
		if s, ok := v.(string); ok && len(s) > limit {
			obj[k] = fmt.Sprintf("[elided %d chars to save context]", len(s))
			changed = true
		}
	}
	if !changed {
		return args, false
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return args, false
	}
	if wrapped {
		out, err = json.Marshal(string(out))
		if err != nil {
			return args, false
		}
	}
	return out, true
}

// compactTurn shrinks the loop-generated part of msgs (index >= seedLen) until
// its estimated size is at most target tokens, or nothing more can be shrunk.
// keepRecent tool results are left whole unless the last resort (dropping whole
// exchanges) is needed. It returns a new slice; msgs is not modified. changed
// reports whether anything was shrunk.
func compactTurn(msgs []provider.Message, seedLen, target, toolDefTokens, keepRecent int) (out []provider.Message, changed bool) {
	if seedLen > len(msgs) {
		seedLen = len(msgs)
	}
	out = append([]provider.Message(nil), msgs...)
	size := func() int { return estimateMessagesTokens(out) + toolDefTokens }
	if size() <= target {
		return out, false
	}

	// Index the loop's tool results, newest last.
	var toolIdx []int
	for i := seedLen; i < len(out); i++ {
		if out[i].Role == "tool" {
			toolIdx = append(toolIdx, i)
		}
	}
	// 1. Elide old tool outputs, oldest first. First the ones older than the
	// keepRecent newest; if that is not enough, keep only the newest two whole.
	for _, keep := range []int{keepRecent, 2} {
		protectedFrom := len(toolIdx) - keep
		if protectedFrom < 0 {
			protectedFrom = 0
		}
		for n, i := range toolIdx {
			if n >= protectedFrom || size() <= target {
				break
			}
			s, ok := out[i].Content.(string)
			if !ok || len(s) < minElidedChars || strings.HasPrefix(s, contextTrimMarker) {
				continue
			}
			out[i].Content = elidedOutputStub(len(s))
			changed = true
		}
		if size() <= target || keepRecent <= 2 {
			break
		}
	}

	// 2. Elide huge arguments of old tool calls (write_file bodies, patches).
	if size() > target {
		recentFrom := len(out)
		seen := 0
		for i := len(out) - 1; i >= seedLen && seen < keepRecent; i-- {
			if out[i].Role == "assistant" && len(out[i].ToolCalls) > 0 {
				recentFrom = i
				seen++
			}
		}
		for i := seedLen; i < recentFrom && size() > target; i++ {
			if out[i].Role != "assistant" || len(out[i].ToolCalls) == 0 {
				continue
			}
			calls := append([]provider.ToolCall(nil), out[i].ToolCalls...)
			touched := false
			for j := range calls {
				if a, ok := elideLongStrings(calls[j].Function.Arguments, elidedArgChars); ok {
					calls[j].Function.Arguments = a
					touched = true
				}
			}
			if touched {
				out[i].ToolCalls = calls
				changed = true
			}
		}
	}

	// 3. Last resort: drop the oldest whole tool exchanges (an assistant message
	// with tool calls and the tool results that answer it), keeping the newest
	// one, and say so once.
	if size() > target {
		dropped := 0
		var names []string
		seenName := map[string]bool{}
		for size() > target {
			first := -1
			for i := seedLen; i < len(out); i++ {
				if out[i].Role == "assistant" && len(out[i].ToolCalls) > 0 {
					first = i
					break
				}
			}
			if first < 0 {
				break
			}
			end := first + 1
			for end < len(out) && out[end].Role == "tool" {
				end++
			}
			// Keep the newest exchange: dropping it leaves the model nothing to
			// continue from.
			hasLater := false
			for i := end; i < len(out); i++ {
				if out[i].Role == "assistant" && len(out[i].ToolCalls) > 0 {
					hasLater = true
					break
				}
			}
			if !hasLater {
				break
			}
			for _, tc := range out[first].ToolCalls {
				if n := tc.Function.Name; n != "" && !seenName[n] {
					seenName[n] = true
					names = append(names, n)
				}
			}
			out = append(out[:first], out[end:]...)
			dropped++
			changed = true
		}
		if dropped > 0 {
			// The note rides on the first tool result that is left, never as a
			// new message: a user-role note after the user's request would be
			// read as the latest instruction, and an assistant one breaks role
			// alternation. A note from an earlier compaction is replaced.
			note := fmt.Sprintf("%s Earlier steps of this turn were dropped to fit the context window (tools used: %s). If you need any of that information again, run the tool again rather than assuming it was never run.", contextTrimMarker, strings.Join(names, ", "))
			for k := seedLen; k < len(out); k++ {
				if out[k].Role != "tool" {
					continue
				}
				if body, ok := out[k].Content.(string); ok {
					out[k].Content = note + "\n\n" + stripDropNote(body)
				}
				break
			}
		}
	}
	return out, changed
}

// stripDropNote removes the note a previous compaction put at the front of a
// tool result, so notes are replaced instead of stacked.
func stripDropNote(body string) string {
	const head = contextTrimMarker + " Earlier steps"
	if strings.HasPrefix(body, head) {
		if _, rest, ok := strings.Cut(body, "\n\n"); ok {
			return rest
		}
	}
	return body
}

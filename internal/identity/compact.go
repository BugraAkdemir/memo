package identity

import "strings"

// BuildCompactSystemPrompt is the system prompt for a small context window
// (a local model below the compact limit, see app.isCompactWindow). The full
// prompt costs ~1.1k tokens before a single word of conversation, which on a
// 4k window is a quarter of everything the model can see; this one keeps only
// what a turn cannot do without: who is answering, the language rule, the
// honesty rule, and — with agent mode on — how tools are reached.
//
// Nothing personal goes in (no user name, persona text, origin story, style
// instructions, learned style notes): the person's memories still reach the
// model, budgeted, on the current user message. The text depends only on the
// assistant name and the agent flag, so it stays byte-identical turn to turn
// and the local server keeps reusing its KV cache.
func (id *Identity) BuildCompactSystemPrompt(agentEnabled bool) string {
	_, assistantName, _, _ := id.GetProfile()
	if strings.TrimSpace(assistantName) == "" {
		assistantName = "Memo"
	}
	var sb strings.Builder
	sb.WriteString("You are " + assistantName + ", a helpful assistant running on the user's own computer. ")
	sb.WriteString("Always reply in the language the user writes in. Be clear and concise. ")
	sb.WriteString("If you do not know something, say so; never invent facts, names, dates or results.")
	if agentEnabled {
		sb.WriteString("\n" + CompactAgentRule)
	}
	return sb.String()
}

// CompactAgentRule is the one line about tools the compact prompt carries when
// agent mode is on.
const CompactAgentRule = "When a task needs a tool, use the tools you are given. Never claim a tool result you did not receive."

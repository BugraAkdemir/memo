package app

import "context"

// currentChatIDCtxKey carries the chat ID whose agent turn is currently
// running. Set by callAgentStream/callAgentWithOrchestra before the executor
// runs, read by agent tools that must act on "this chat" rather than a chat
// the model names — start_self_driving_task binds its task list to the chat
// that asked for it, exactly like create_routine resolves its own delivery
// target from ctx (see selfChatSourceFromContext's doc comment for the same
// reasoning: an unattended, model-driven "do this to some other chat" call is
// a real risk that a hardcoded "always the conversation that asked" contract
// closes off).
type currentChatIDCtxKey struct{}

// withCurrentChatID attaches chatID to ctx. A "" chatID is a no-op so callers
// don't have to guard it.
func withCurrentChatID(ctx context.Context, chatID string) context.Context {
	if chatID == "" {
		return ctx
	}
	return context.WithValue(ctx, currentChatIDCtxKey{}, chatID)
}

// currentChatIDFromContext returns the chat ID attached by withCurrentChatID,
// or "" for a turn that never had one (e.g. a background/system call).
func currentChatIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(currentChatIDCtxKey{}).(string)
	return id
}

type taskMemoryDisabledCtxKey struct{}

// withTaskMemoryDisabled marks a turn as running with "task memory" off, so
// buildMessagesForSession skips the RAG/memory block for it.
func withTaskMemoryDisabled(ctx context.Context) context.Context {
	return context.WithValue(ctx, taskMemoryDisabledCtxKey{}, true)
}

func taskMemoryDisabled(ctx context.Context) bool {
	v, _ := ctx.Value(taskMemoryDisabledCtxKey{}).(bool)
	return v
}

// codingDirective is the entire system prompt for a Code Mode turn — it
// replaces the persona / origin / style / passive / capabilities / memory
// stack. Deliberately short (~110 tokens): the value is a lean, cacheable
// prefix and a clear behavioural contract, not a personality. Not shown to
// the user, so it is exempt from the L10n rule.
const codingDirective = `You are a coding agent working directly in the user's project through your tools. ` +
	`Read the relevant files before you edit. Make targeted, minimal edits rather than rewrites, and match the existing file's style, naming and conventions. ` +
	`After a non-trivial change, run the project's build / test / lint if you can, and fix what you broke. ` +
	`Keep prose short: a line or two on what you changed and why — no walkthroughs, no restating the code. ` +
	`Ask first before anything ambiguous, destructive, or outside what was requested.`

type codeModeCtxKey struct{}

// withCodeMode marks a turn as running in Code Mode — a coding-tuned preset
// that strips every chat-oriented block and background LLM call (persona,
// mood, personal memory, time context, intent/fact/title extraction,
// proactive nudging) while keeping the agent tool loop, the working-set
// digest and conversation compaction, and swapping the persona for one
// compact coding directive. Resolved per chat by App.resolveCodeMode
// (explicit per-chat flag, else "is this a project/agent chat") and attached
// in sendMessageStreamCore.
func withCodeMode(ctx context.Context) context.Context {
	return context.WithValue(ctx, codeModeCtxKey{}, true)
}

func codeModeActive(ctx context.Context) bool {
	v, _ := ctx.Value(codeModeCtxKey{}).(bool)
	return v
}

// resolveCodeMode decides whether a chat runs in Code Mode: an explicit
// per-chat pin (Session.CodeMode) wins; otherwise it defaults on for a
// project/agent chat (one with a ProjectPath) and off for a plain chat.
func (a *App) resolveCodeMode(chatID string) bool {
	if chatID == "" {
		return false
	}
	sm := a.getSessionManager()
	if sm == nil {
		return false
	}
	if v := sm.GetCodeMode(chatID); v != nil {
		return *v
	}
	return sm.IsAgentChat(chatID)
}

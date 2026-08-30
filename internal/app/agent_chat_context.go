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

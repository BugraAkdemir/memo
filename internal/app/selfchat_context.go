package app

import "context"

// selfChatSourceCtxKey carries which self-chat surface (if any) the current
// agent turn is running in — set by handleWhatsAppSelfChatMessage/
// handleTelegramMessage before calling SendMessageStreamTo, read by
// App.CreateRoutineFromChat (the create_routine agent tool's backing
// implementation, internal/agent/tools/routine.go) so a routine created
// conversationally always delivers back to the exact surface/chat it was
// asked from — never to a contact/chat the model itself supplies, since
// there deliberately is no such parameter in that tool's schema at all.
type selfChatSourceCtxKey struct{}

// SelfChatSource identifies the self-chat surface (if any) that originated
// the current agent turn. At most one of WhatsApp/Telegram is ever true —
// a turn only ever comes from one surface at a time.
type SelfChatSource struct {
	WhatsApp       bool
	WhatsAppJID    string
	Telegram       bool
	TelegramChatID int64
}

// withSelfChatSource attaches src to ctx.
func withSelfChatSource(ctx context.Context, src SelfChatSource) context.Context {
	return context.WithValue(ctx, selfChatSourceCtxKey{}, src)
}

// selfChatSourceFromContext retrieves the SelfChatSource attached by
// withSelfChatSource, if any. ok is false for a normal (non-self-chat)
// chat turn, where no single surface should be preferred.
func selfChatSourceFromContext(ctx context.Context) (src SelfChatSource, ok bool) {
	src, ok = ctx.Value(selfChatSourceCtxKey{}).(SelfChatSource)
	return src, ok
}

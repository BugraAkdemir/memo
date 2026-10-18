package agent

import "context"

// ClientOptions are settings ONE client asks for on ONE request, without
// touching anything the other clients see. The terminal REPL and the desktop app
// share a backend, and the backend's flags (auto-permission, web search, the
// active chat) are global: a REPL that flipped them silently changed the app's
// behaviour and vice versa. A client that wants its own setting sends it with
// the request instead; absent means "follow the backend", so every existing
// client behaves exactly as before.
type ClientOptions struct {
	// AutoPermission, when non-nil, decides whether this request's tool calls
	// are auto-approved — in BOTH directions: an explicit false prompts even if
	// the backend's global auto-permission is on.
	AutoPermission *bool
	// NoWebTools hides web_search and fetch_page from this request's tool list.
	NoWebTools bool
}

type clientOptionsCtxKey struct{}

// WithClientOptions attaches o to ctx for the next RunStream to apply.
func WithClientOptions(ctx context.Context, o ClientOptions) context.Context {
	return context.WithValue(ctx, clientOptionsCtxKey{}, o)
}

func clientOptionsFromCtx(ctx context.Context) ClientOptions {
	o, _ := ctx.Value(clientOptionsCtxKey{}).(ClientOptions)
	return o
}

// webToolNames are the tools NoWebTools hides.
var webToolNames = map[string]bool{"web_search": true, "fetch_page": true}

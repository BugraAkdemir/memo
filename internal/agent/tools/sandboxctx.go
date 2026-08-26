package tools

import "context"

// SandboxSetter is the narrow surface change_directory needs from the
// agent's per-call sandbox (*agent.Sandbox in practice) — kept as an
// interface here, not a concrete import, so this package doesn't have to
// import package agent (which already imports this package; a direct import
// back would be a cycle). *agent.Sandbox satisfies this structurally.
type SandboxSetter interface {
	SetBasePath(string)
}

type sandboxSetterKey struct{}

// WithSandboxSetter attaches the current turn's sandbox to ctx so
// change_directory can widen it for the rest of the turn — mirrors how
// per-turn state is threaded through elsewhere in this package (see
// fetch_page's domain budget). Seeded once per Pipeline.RunStream call.
func WithSandboxSetter(ctx context.Context, s SandboxSetter) context.Context {
	return context.WithValue(ctx, sandboxSetterKey{}, s)
}

// SandboxSetterFromContext retrieves the setter WithSandboxSetter attached,
// if any.
func SandboxSetterFromContext(ctx context.Context) (SandboxSetter, bool) {
	s, ok := ctx.Value(sandboxSetterKey{}).(SandboxSetter)
	return s, ok
}

// ProjectPathSetter is the narrow surface change_directory needs from
// *sessions.Manager to persist a directory switch past the current turn —
// same import-cycle-avoidance reasoning as SandboxSetter above.
type ProjectPathSetter interface {
	SetProjectPath(sessionID, path string) error
}

type projectPathSetterKey struct{}

type projectPathCtxValue struct {
	setter    ProjectPathSetter
	sessionID string
}

// WithProjectPathSetter attaches the session manager + current session id so
// change_directory's effect survives into later turns of the same
// conversation. Seeded once per Executor.RunStream call — WhatsApp/Telegram
// self-chat sessions get this exactly like the Flutter chat does, since they
// all go through the same Executor.RunStream.
func WithProjectPathSetter(ctx context.Context, setter ProjectPathSetter, sessionID string) context.Context {
	return context.WithValue(ctx, projectPathSetterKey{}, projectPathCtxValue{setter: setter, sessionID: sessionID})
}

// ProjectPathSetterFromContext retrieves what WithProjectPathSetter attached,
// if any. ok is false if nothing was seeded, the setter is nil, or the
// session id is empty (background/anonymous runs with no session to persist
// against).
func ProjectPathSetterFromContext(ctx context.Context) (setter ProjectPathSetter, sessionID string, ok bool) {
	v, present := ctx.Value(projectPathSetterKey{}).(projectPathCtxValue)
	if !present || v.setter == nil || v.sessionID == "" {
		return nil, "", false
	}
	return v.setter, v.sessionID, true
}

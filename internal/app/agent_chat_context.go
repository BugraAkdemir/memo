package app

import (
	"context"

	"memo/internal/config"
)

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

// taskToolReminder is appended to all three Code Mode directives below.
// get_task_status/pause_task/resume_task (internal/agent/tools.go) have
// always been registered and reachable from a Code Mode turn — a project/
// agent chat is exactly where a Self-Driving task list gets bound (see
// chatBoundTaskID) — but none of the three directives ever mentioned them,
// since they predate the task loop and were never revisited after it
// shipped. Without this, the model had no prompted reason to reach for a
// tool literally designed to stop it from fabricating a task's status (see
// TaskStatusForChat's BUG-PLAN10 doc comment) — live-tested finding, not
// hypothetical: asked about a running task's progress, the model guessed
// from its own turn's context instead of calling the tool that exists
// specifically to answer that.
const taskToolReminder = `If asked about a Self-Driving task list's status or progress, or to pause/resume one, call get_task_status/pause_task/resume_task — don't guess from your own turn's context.`

// codingDirective is the entire system prompt for a Code Mode turn — it
// replaces the persona / origin / style / passive / capabilities / memory
// stack. Deliberately short (~110 tokens): the value is a lean, cacheable
// prefix and a clear behavioural contract, not a personality. Not shown to
// the user, so it is exempt from the L10n rule.
const codingDirective = `You are a coding agent working directly in the user's project through your tools. ` +
	`Read the relevant files before you edit. Make targeted, minimal edits rather than rewrites, and match the existing file's style, naming and conventions. ` +
	`After a non-trivial change, run the project's build / test / lint if you can, and fix what you broke. ` +
	`Keep prose short: a line or two on what you changed and why — no walkthroughs, no restating the code. ` +
	`Ask first before anything ambiguous, destructive, or outside what was requested. ` +
	taskToolReminder

// codePlanDirective is the "plan" sub-mode's system prompt: investigate and
// produce a plan, don't edit. The restriction is deliberately soft/prompt-
// only, not enforced by removing tools from the model's registry (see
// pipeline.go's codeModeToolAutoApproveSet: plan sub-mode's auto-approve set
// is empty, so any edit attempt the model makes anyway still falls through
// to a normal permission prompt — friction without a hard lockout).
const codePlanDirective = `You are a coding agent in PLANNING mode. Investigate the codebase and produce a concrete, step-by-step implementation plan — do not edit, create, or delete any files, and do not run commands that change project state. ` +
	`When the plan is ready, call save_code_plan with the full plan as Markdown. ` +
	`Then ask the user in plain chat text whether to proceed in build mode (fast, auto-approved edits) or auto mode (today's normal confirm-as-you-go editing) — just ask normally, don't use a tool or special format for that question. ` +
	taskToolReminder

// codeBuildDirective is the "build" sub-mode's system prompt: same coding
// contract as codingDirective, but explicit that edits/commands proceed
// without waiting — pipeline.go's codeModeBuildAutoApproveTools is what
// actually makes that true for write/edit/run_command; this text just sets
// the model's own expectation to match.
const codeBuildDirective = `You are a coding agent working directly in the user's project through your tools, running fast and permissively: file edits and shell commands proceed without waiting for confirmation. ` +
	`Read relevant files before editing, make targeted minimal edits, match the existing file's style, naming and conventions. ` +
	`After a non-trivial change, run the project's build / test / lint if you can, and fix what you broke. Keep prose short. ` +
	`Still ask first for anything beyond normal editing/running commands — deleting files, changing directory, configuring providers — those always require confirmation regardless of this mode. ` +
	taskToolReminder

// codeSubModeDirective resolves the system prompt text for subMode ("plan",
// "auto", "build", or "" which is treated as "auto") — a user-edited
// Settings override (AgentModeConfig.CodePlanPrompt/CodeAutoPrompt/
// CodeBuildPrompt) wins when non-empty, else the built-in const above.
func codeSubModeDirective(cfg config.AgentModeConfig, subMode string) string {
	switch subMode {
	case "plan":
		if cfg.CodePlanPrompt != "" {
			return cfg.CodePlanPrompt
		}
		return codePlanDirective
	case "build":
		if cfg.CodeBuildPrompt != "" {
			return cfg.CodeBuildPrompt
		}
		return codeBuildDirective
	default: // "auto" and ""
		if cfg.CodeAutoPrompt != "" {
			return cfg.CodeAutoPrompt
		}
		return codingDirective
	}
}

type codeSubModeCtxKey struct{}

// withCodeSubMode attaches this turn's resolved Code Mode sub-mode
// ("plan"/"auto"/"build") to ctx — mirrors withCodeMode exactly, set once in
// sendMessageStreamCore alongside it.
func withCodeSubMode(ctx context.Context, subMode string) context.Context {
	return context.WithValue(ctx, codeSubModeCtxKey{}, subMode)
}

// codeSubModeFromCtx returns "" if unset. Callers in a Code Mode turn should
// treat "" the same as "auto" (codeSubModeDirective and
// codeModeToolAutoApproveSet both already do) — a non-Code-Mode ctx simply
// never had this attached at all.
func codeSubModeFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(codeSubModeCtxKey{}).(string)
	return v
}

// resolveCodeSubMode mirrors resolveCodeMode exactly: an explicit per-chat
// pin (Session.CodeSubMode) wins, else "auto" — so every existing Code Mode
// chat (nothing has ever pinned a sub-mode before this feature existed)
// resolves to "auto" and behaves identically to before.
func (a *App) resolveCodeSubMode(chatID string) string {
	if chatID == "" {
		return "auto"
	}
	sm := a.getSessionManager()
	if sm == nil {
		return "auto"
	}
	if v := sm.GetCodeSubMode(chatID); v != "" {
		return v
	}
	return "auto"
}

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

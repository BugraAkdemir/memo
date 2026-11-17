package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/agent/tools"
	"memo/internal/beta"
	"memo/internal/provider"
	"sort"
	"sync"
)

// DangerLevel indicates the potential risk of a tool.
// It is the canonical danger level type for the agent package.
// Use FromString to convert from other danger level types (e.g. skill.DangerLevel).
type DangerLevel string

const (
	Safe      DangerLevel = "safe"
	Medium    DangerLevel = "medium"
	Dangerous DangerLevel = "dangerous"
)

// FromString converts a string danger level to the agent DangerLevel type.
// Unknown values default to Medium.
func FromString(s string) DangerLevel {
	switch s {
	case "safe":
		return Safe
	case "medium":
		return Medium
	case "dangerous":
		return Dangerous
	default:
		return Medium
	}
}

// ToolDef defines a single tool available to the agent.
type ToolDef struct {
	Name        string
	Description string
	Parameters  json.RawMessage // JSON Schema describing the arguments
	DangerLevel DangerLevel
	ExecuteFn   func(ctx context.Context, args json.RawMessage, basePath string, createBackup func(string) error) (string, error)
	PreviewFn   func(args json.RawMessage, basePath string) (string, error)
	// SkillOwner is the name of the skill that contributed this tool, or ""
	// for a built-in tool. Tools are registered unconditionally (at skill
	// install/discover time, not at "activate" time) — SkillOwner is what
	// lets ToOpenAITools/dispatch gate a skill's tools to only the chats
	// that actually have that skill active, without needing a separate
	// registry per chat.
	SkillOwner string
	// Beta marks a tool that only exists while beta features are on
	// (Settings › Beta Features, internal/beta). With beta off the tool is
	// neither offered to the model (ToOpenAITools) nor found by Get, so a
	// model that calls it anyway gets the ordinary "unknown tool" answer — it
	// is absent, not disabled. Today: open_app and the browser_* session tools.
	Beta bool
}

// ToolRegistry manages all available tools.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]ToolDef
}

// NewRegistry creates a new tool registry and registers built-in tools.
func NewRegistry() *ToolRegistry {
	r := &ToolRegistry{
		tools: make(map[string]ToolDef),
	}
	r.registerBuiltins()
	r.registerChatSubAgentTools()
	if isKorteksOSModeEnabled() {
		r.RegisterOSTools("")
	}
	return r
}
func (r *ToolRegistry) registerBuiltins() {
	r.registerCommunicationTools()
	r.Register(ToolDef{
		Name:        "read_file",
		Description: "Reads a file (use offset/limit for large files). Read a file before editing it, so your edit matches its exact text and style.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Path to the file to read"},"offset":{"type":"integer","description":"1-based line to start from (optional)"},"limit":{"type":"integer","description":"max lines to return from offset (optional)"}},"required":["path"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.ReadFile,
	})

	r.Register(ToolDef{
		Name:        "write_file",
		Description: "Creates a new file, or fully replaces one. To change part of an existing file use edit_file instead — do not rewrite a whole file for a small change.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string", "description": "Path to the file to write"}, "content": {"type": "string", "description": "Content to write"}}, "required": ["path", "content"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.WriteFile,
	})

	r.Register(ToolDef{
		Name:        "delete_file",
		Description: "Deletes a file or directory",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string", "description": "Path to delete"}}, "required": ["path"]}`),
		DangerLevel: Dangerous,
		ExecuteFn:   tools.DeleteFile,
	})

	r.Register(ToolDef{
		Name:        "list_directory",
		Description: "Lists files and directories in a path",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string", "description": "Path to directory"}, "recursive": {"type": "boolean", "description": "Whether to list recursively"}}, "required": ["path", "recursive"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.ListDirectory,
	})

	r.Register(ToolDef{
		Name:        "change_directory",
		Description: "Changes the agent's working-directory sandbox for this conversation. Resolve relative paths and '~'; set create_if_missing=true to create a missing folder. Confirm with the user before switching away from the project root.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string", "description": "Directory path"}, "create_if_missing": {"type": "boolean", "description": "Create directory if missing"}}, "required": ["path"]}`),
		DangerLevel: Dangerous,
		ExecuteFn:   tools.ChangeDirectory,
		PreviewFn:   tools.ChangeDirectoryPreview,
	})

	r.Register(ToolDef{
		Name:        "run_command",
		Description: "Executes a terminal command using bash -c. Use it to build, run tests and linters and inspect git state; prefer read_file / search_content / edit_file over cat, grep and sed.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"command": {"type": "string", "description": "The command to run"}, "cwd": {"type": "string", "description": "Optional working directory"}}, "required": ["command"]}`),
		DangerLevel: Dangerous,
		ExecuteFn:   tools.RunCommand,
	})

	r.Register(ToolDef{
		Name:        "search_files",
		Description: "Finds files by NAME using a glob (e.g. *.go). It does not look inside files — to search the text of files use search_content.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"pattern": {"type": "string", "description": "Glob pattern (e.g. *.go)"}, "path": {"type": "string", "description": "Directory to search in"}}, "required": ["pattern", "path"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.SearchFiles,
	})

	r.Register(ToolDef{
		Name:        "search_content",
		Description: "Searches INSIDE files for a regular expression (grep) and returns path:line: text. Use it to find where a function, type or string is defined and every place it is used BEFORE you change it. Skips .git, node_modules, vendor, build output and binary files.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string","description":"Regular expression (Go/RE2 syntax). Escape ( ) [ ] { } . * + ? with a backslash to match them literally."},"path":{"type":"string","description":"Directory or file to search (default: the project root)"},"glob":{"type":"string","description":"Only search files whose NAME matches this glob, e.g. *.go"},"ignore_case":{"type":"boolean"},"context":{"type":"integer","description":"Lines of context around each match, 0-3 (default 0)"},"max_results":{"type":"integer","description":"Stop after this many matches (default 100, max 500)"}},"required":["pattern"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.SearchContent,
	})
	r.Register(ToolDef{
		Name:        "get_file_info",
		Description: "Gets metadata about a file or directory",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string", "description": "Path to the file/directory"}}, "required": ["path"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.GetFileInfo,
	})

	r.Register(ToolDef{
		Name:        "read_env",
		Description: "Reads non-sensitive environment variables",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {}}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.ReadEnv,
	})

	r.Register(ToolDef{
		Name:        "edit_file",
		Description: "Edits an existing file. Provide EITHER old_string and new_string (old_string must match the file text EXACTLY, including whitespace, and be unique in the file — add surrounding lines until it is; otherwise the edit fails), OR start_line, end_line and new_content.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string"}, "old_string": {"type": "string"}, "new_string": {"type": "string"}, "start_line": {"type": "integer"}, "end_line": {"type": "integer"}, "new_content": {"type": "string"}}, "required": ["path"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.EditFile,
		PreviewFn:   tools.EditFilePreview,
	})

	r.Register(ToolDef{
		Name:        "insert_line",
		Description: "Inserts content at a specific line number in a file.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string"}, "line_number": {"type": "integer"}, "content": {"type": "string"}}, "required": ["path", "line_number", "content"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.InsertLine,
		PreviewFn:   tools.InsertLinePreview,
	})

	r.Register(ToolDef{
		Name:        "delete_lines",
		Description: "Deletes a range of lines from a file.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string"}, "start_line": {"type": "integer"}, "end_line": {"type": "integer"}}, "required": ["path", "start_line", "end_line"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.DeleteLines,
		PreviewFn:   tools.DeleteLinesPreview,
	})

	r.registerWebSearchTool()
	r.registerFetchPageTool()
	r.registerOpenAppTool()
	r.registerBrowserTools()

	r.Register(ToolDef{
		Name:        "self_clone",
		Description: "Copies this entire project (source files + binary) to another local directory. Use for local replication or backup.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"dest":{"type":"string","description":"Destination directory path"}},"required":["dest"]}`),
		DangerLevel: Dangerous,
		ExecuteFn:   tools.SelfClone,
	})

	r.Register(ToolDef{
		Name:        "configure_provider",
		Description: "Adds or updates an AI provider configuration (type, base URL, API key, model). Use when the user explicitly asks to add/configure a provider in chat instead of Settings. Requires user confirmation before it runs.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"type":{"type":"string","description":"Provider type: openai, gemini, grok, groq, claude, openrouter, ollama, llama.cpp, or custom"},"name":{"type":"string","description":"Display name for this provider (defaults to type if omitted)"},"api_key":{"type":"string","description":"API key, if the provider needs one"},"base_url":{"type":"string","description":"Base URL (required for type=custom)"},"model":{"type":"string","description":"Model ID to use"},"enabled":{"type":"boolean","description":"Whether to enable it immediately (default true)"}},"required":["type","model"]}`),
		DangerLevel: Dangerous,
		ExecuteFn:   tools.ConfigureProvider,
	})

	r.Register(ToolDef{
		Name:        "get_calendar_events",
		Description: "Gerçek takvimden (events.db) kayıtlı etkinlikleri okur. Kullanıcı takviminde ne olduğunu sorduğunda tahmin etme, bu aracı çağır. from/to: YYYY-MM-DD veya ISO 8601 (varsayılan: dünden 7 gün sonrasına kadar)",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"from":{"type":"string","description":"Başlangıç tarihi (YYYY-MM-DD veya ISO 8601)"},"to":{"type":"string","description":"Bitiş tarihi (YYYY-MM-DD veya ISO 8601)"}}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.GetCalendarEvents,
	})

	r.registerWhatsAppTools()
	r.registerRoutineTool()
	r.registerFileSenderTool()
	r.registerSelfDrivingTaskTool()
	r.registerTaskMdTools()
	r.registerCodePlanTool()
	r.registerProjectSkillTool()
	r.registerUseSkillTool()

	r.Register(ToolDef{
		Name:        "get_task_status",
		Description: "Çalışan otonom görevin (Task.md döngüsü) durumunu okur: faz, adım ve kalan maddeler. Tahmin etme; görev yoksa bunu bildir.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.GetTaskStatus,
	})
	r.Register(ToolDef{
		Name:        "pause_task",
		Description: "Bu sohbete bağlı çalışan otonom görevi duraklatır (kaldığı adım korunur).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.PauseTask,
	})
	r.Register(ToolDef{
		Name:        "resume_task",
		Description: "Duraklatılmış otonom görevi kaldığı adımdan sürdürür.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.ResumeTask,
	})
}

// registerTaskMdTools adds create_task_md / edit_task_md — only to the
// main/full registry (like create_routine), reachable from any agent-enabled
// chat. They write/mutate a Task.md following taskloop.TaskMdSchemaDoc.
func (r *ToolRegistry) registerTaskMdTools() {
	r.Register(ToolDef{
		Name:        "create_task_md",
		Description: "Yeni bir Task.md dosyası oluşturur (mevcutsa hata verir; düzenlemek için edit_task_md kullan). Önce hedefi ve doğrulanabilir maddeleri netleştir.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Task.md path (default: project Task.md)"},"items":{"type":"array","items":{"type":"string"},"description":"Checkbox items"},"intro":{"type":"string","description":"Short goal summary"},"notify":{"type":"string","description":"sadece-bitince | önemli | her-şey"},"mode":{"type":"string","description":"worker | planlayıcı"},"planner_model":{"type":"string"},"coder_model":{"type":"string"},"verifier_model":{"type":"string"},"memory":{"type":"string","description":"açık | kapalı"},"auto_approve":{"type":"boolean"}},"required":["items"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.CreateTaskMd,
	})
	r.Register(ToolDef{
		Name:        "edit_task_md",
		Description: "Var olan Task.md dosyasını yerinde düzenler (add_item, split_item, set_header, check_item; item_index 1 tabanlıdır).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Task.md path"},"op":{"type":"string","enum":["add_item","split_item","set_header","check_item"]},"text":{"type":"string","description":"add_item text"},"item_index":{"type":"integer","description":"1-based index"},"sub_items":{"type":"array","items":{"type":"string"},"description":"split_item sub-items"},"header_key":{"type":"string","description":"e.g. bildirim, mod, kodlayıcı"},"header_value":{"type":"string"}},"required":["op"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.EditTaskMd,
	})
}

// registerCodePlanTool adds save_code_plan — only to the main/full registry
// (like create_routine), reachable from any agent-enabled chat. Always
// registered rather than filtered to Code Mode's "plan" sub-mode: the
// registry has no per-turn tool-subset mechanism (see the other
// NewXxxRegistry constructors below, each a completely separate fixed set
// built for a different call path, not a per-turn filter), and Code Mode's
// "plan" sub-mode restriction is deliberately soft/prompt-only (codePlanDirective)
// — harmless if called outside plan sub-mode, since "auto"/"build"'s own
// directives never mention it.
func (r *ToolRegistry) registerCodePlanTool() {
	r.Register(ToolDef{
		Name:        "save_code_plan",
		Description: "Saves the step-by-step Markdown plan to Memo's plan storage (only in Code Mode planning mode). Afterward, ask the user whether to proceed in build or auto mode.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"content":{"type":"string","description":"Markdown plan"}},"required":["content"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.SaveCodePlan,
	})
}

// registerProjectSkillTool adds install_project_skill. Available in both Code Mode
// (installs into .memo/skills) and general chats (desktop, web, Telegram,
// WhatsApp — installs into Memo's global skills library).
func (r *ToolRegistry) registerProjectSkillTool() {
	r.Register(ToolDef{
		Name:        "install_project_skill",
		Description: "Installs a plain-file skill from a user-provided GitHub source (owner/repo, URL, or npx skills add). Call only when the user explicitly gives a source; pass `--skill <name>` if multiple skills exist.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"source":{"type":"string","description":"GitHub source or npx command, optionally with --skill <name>"}},"required":["source"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.InstallProjectSkill,
	})
}

// registerUseSkill is how the model calls a skill itself ("use the necessary
// skills"): it switches the skill on for THIS chat only and returns its
// instructions for the rest of the turn.
func (r *ToolRegistry) registerUseSkillTool() {
	r.Register(ToolDef{
		Name:        "use_skill",
		Description: "Lists installed skills (when name is empty) or activates one for this chat and returns its instructions.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Skill name, or empty to list installed skills"}}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.UseSkill,
	})
}

// registerSelfDrivingTaskTool adds start_self_driving_task to this registry —
// only to the main/full registry (like create_routine), so it's reachable
// from any agent-enabled chat: normal chat, WhatsApp self-chat, Telegram. It
// deliberately has no chat/target parameter; the task binds to the chat that
// asked (see tools.SelfDrivingTasks).
func (r *ToolRegistry) registerSelfDrivingTaskTool() {
	r.Register(ToolDef{
		Name:        "start_self_driving_task",
		Description: "Task.md dosyasındaki onay kutulu maddeler için otonom görev döngüsünü bu sohbete bağlı olarak arka planda başlatır.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"task_md_path":{"type":"string","description":"Path to Task.md file"},"title":{"type":"string","description":"Optional display name"}},"required":["task_md_path"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.StartSelfDrivingTask,
	})
}

// registerFileSenderTool adds share_file to this registry — same
// availability as create_routine (any agent-enabled chat: normal chat,
// WhatsApp self-chat, Telegram).
func (r *ToolRegistry) registerFileSenderTool() {
	r.Register(ToolDef{
		Name:        "share_file",
		Description: "Bu konuşmada kullanıcıya bir dosya veya klasör (otomatik zip'lenir) gönderir.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Dosya veya klasör yolu"}},"required":["path"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.ShareFile,
	})
}

// registerRoutineTool adds create_routine to this registry — only to the
// main/full registry (not the scoped WhatsApp-only or web-search-only ones
// below), so it's reachable from any agent-enabled chat: normal chat,
// WhatsApp self-chat, Telegram.
func (r *ToolRegistry) registerRoutineTool() {
	r.Register(ToolDef{
		Name:        "create_routine",
		Description: "Kullanıcının serbest metin isteğinden (zaman, sıklık, içerik dahil) bu sohbete bağlı zamanlanmış bir rutin oluşturur.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"text":{"type":"string","description":"Kullanıcının rutin isteği"}},"required":["text"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.CreateRoutine,
	})
	r.Register(ToolDef{
		Name:        "list_routines",
		Description: "Kullanıcının zamanlanmış rutinlerini (id, prompt, zamanlama, durum) listeler.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.ListRoutines,
	})
	r.Register(ToolDef{
		Name:        "cancel_routine",
		Description: "list_routines'ten alınan gerçek id ile bir rutini siler.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Rutin id"}},"required":["id"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.DeleteRoutine,
	})
}

// registerWebSearchTool adds the web_search tool to this registry. Split out
// of registerBuiltins so NewWebSearchRegistry can build a registry with only
// web_search + fetch_page — used by App.routeStream's non-agent "web search
// mode" so plain chat can let the model decide, per message via native
// tool-calling, whether it actually needs to search, instead of the old
// blind-injection design that ran a search on every single message
// regardless of content.
func (r *ToolRegistry) registerWebSearchTool() {
	r.Register(ToolDef{
		Name:        "web_search",
		Description: "Searches the web (DuckDuckGo) for recent news, prices, or current facts. Do not call for greetings, general knowledge, or code questions, and do not use open_app for search requests.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Short keyword query (2-6 words)"},"max_results":{"type":"integer","description":"Number of results (default 5, max 10)"}},"required":["query"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.WebSearch,
	})
}

// registerFetchPageTool adds the fetch_page tool to this registry. Split out
// of registerBuiltins for the same reason as registerWebSearchTool — see its
// doc comment.
func (r *ToolRegistry) registerFetchPageTool() {
	r.Register(ToolDef{
		Name:        "fetch_page",
		Description: "Fetches a URL's readable content as Markdown (use after web_search or for read/summarize requests, instead of browser_navigate). Up to 5 domains per request; retrying another URL on the same domain is free.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"Exact URL to fetch"}},"required":["url"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.FetchPage,
	})
}

// registerOpenAppTool adds the open_app tool to this registry. Split out of
// registerBuiltins for the same reason as registerWebSearchTool — see its
// doc comment. The Description below is the entire disambiguation mechanism
// against web_search: without the explicit carve-out, "what's the latest
// news" and "open the browser" both mention a browser-adjacent concept, and
// nothing else in the prompt tells the model which tool actually answers
// which.
func (r *ToolRegistry) registerOpenAppTool() {
	r.Register(ToolDef{
		Name:        "open_app",
		Beta:        true,
		Description: "Launches a desktop application or blank browser tab ONLY on an explicit open/launch command (never for questions or searches).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"app_name":{"type":"string","description":"Application name (e.g. Spotify, browser, VS Code)"}},"required":["app_name"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.OpenApp,
	})
}

// registerBrowserTools adds the interactive-browser tools to this registry.
// Split out of registerBuiltins for the same reason as registerWebSearchTool
// — see its doc comment. Only in the main/full registry: not
// NewWebSearchRegistry (unrelated scope — search + static fetch only, no
// interactive session), not NewWhatsAppRegistry (unrelated), and not
// NewReadOnlyRegistry (this checkpoint's tools are all Medium/Safe but the
// whole set is interactive/stateful by nature — a read-only sub-agent
// wanting to verify UI state would go through the full-registry coder
// sub-agent instead, not get its own session).
//
// browser_navigate is Medium, not Dangerous: the session runs as a
// brand-new, sandboxed OS process with its own dedicated profile directory
// (see internal/browserengine/session.go's doc comment) — never attached
// to, or sharing cookies/accounts with, the user's real browser. That's
// categorically safer than open_app's "launch the user's actual browser"
// (also Medium). Making it Dangerous would deny "allow for this session" in
// the permission dialog, forcing a fresh prompt on every single call during
// a multi-step UI test — exactly the friction this tool exists to avoid.
func (r *ToolRegistry) registerBrowserTools() {
	r.Register(ToolDef{
		Name:        "browser_navigate",
		Beta:        true,
		Description: "Opens a URL in Memo's disposable sandboxed Chromium session for interactive testing (use fetch_page just to read text). Follow with browser_get_text for selectors or browser_screenshot to display to the user.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"The URL to open"}},"required":["url"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.BrowserNavigate,
	})
	r.Register(ToolDef{
		Name:        "browser_click",
		Beta:        true,
		Description: "Clicks an element in the active browser session by CSS selector (from browser_get_text; never guess) or viewport (x, y).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector"},"x":{"type":"number","description":"Viewport x"},"y":{"type":"number","description":"Viewport y"}}}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.BrowserClick,
	})
	r.Register(ToolDef{
		Name:        "browser_type",
		Beta:        true,
		Description: "Focuses a CSS selector in the active browser session and types text via key events.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector"},"text":{"type":"string","description":"Text to type"}},"required":["selector","text"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.BrowserType,
	})
	r.Register(ToolDef{
		Name:        "browser_scroll",
		Beta:        true,
		Description: "Scrolls the active browser session by (dx, dy) pixels (default dy=800).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"dx":{"type":"integer","description":"Horizontal px"},"dy":{"type":"integer","description":"Vertical px"}}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.BrowserScroll,
	})
	r.Register(ToolDef{
		Name:        "browser_screenshot",
		Beta:        true,
		Description: "Shows the active browser page screenshot to the user (you do not see the image; use browser_get_text to read the page).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.BrowserScreenshot,
	})
	r.Register(ToolDef{
		Name:        "browser_get_text",
		Beta:        true,
		Description: "Reads the active browser page's visible text and clickable elements with exact CSS selectors.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.BrowserGetText,
	})
	r.Register(ToolDef{
		Name:        "browser_close",
		Beta:        true,
		Description: "Closes the active interactive browser session.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.BrowserClose,
	})
}

// NewWebSearchRegistry creates a registry with only web_search + fetch_page.
// See registerWebSearchTool's doc comment for why this exists. The
// underlying pipeline (NewPipelineWithBudget, maxIters 40 — same as full
// agent mode) already supports many tool-call iterations per turn; scoping
// the registry to just these two tools is what keeps this mode "search
// mode" rather than "full agent mode with everything else disabled" — the
// model can search, read a result, decide it's irrelevant, and try another
// one, exactly like full agent mode, just without file/command/WhatsApp
// access.
func NewWebSearchRegistry() *ToolRegistry {
	r := &ToolRegistry{
		tools: make(map[string]ToolDef),
	}
	r.registerWebSearchTool()
	r.registerFetchPageTool()
	return r
}

// registerWhatsAppTools adds WhatsApp-specific tools to this registry.
func (r *ToolRegistry) registerWhatsAppTools() {
	r.Register(ToolDef{
		Name:        "whatsapp_send",
		Description: "WhatsApp üzerinden bir kişiye mesaj gönderir. jid: telefon numarası (örnek: 905551234567@s.whatsapp.net), text: mesaj içeriği",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"jid":{"type":"string","description":"Alıcının JID'si (ör: 905551234567@s.whatsapp.net)"},"text":{"type":"string","description":"Gönderilecek mesaj"}},"required":["jid","text"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.SendWhatsApp,
	})
	r.Register(ToolDef{
		Name:        "whatsapp_search",
		Description: "WhatsApp mesajlarında metin araması yapar. query: aranacak kelime, limit: maksimum sonuç sayısı",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Aranacak metin"},"limit":{"type":"integer","description":"Maksimum sonuç sayısı (varsayılan 10)"}},"required":["query"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.SearchWhatsApp,
	})
	r.Register(ToolDef{
		Name:        "whatsapp_latest",
		Description: "En son mesajlaşılan WhatsApp sohbetlerini listeler. limit: kaç sohbet gösterileceği (varsayılan 10)",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer","description":"Sohbet sayısı (varsayılan 10)"}}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.LatestWhatsAppChats,
	})
	r.Register(ToolDef{
		Name:        "whatsapp_messages",
		Description: "Belirli bir WhatsApp sohbetinin mesaj geçmişini getirir. jid: sohbet JID'si, limit: mesaj sayısı (varsayılan 20)",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"jid":{"type":"string","description":"Sohbet JID'si (ör: 905551234567@s.whatsapp.net)"},"limit":{"type":"integer","description":"Mesaj sayısı (varsayılan 20)"}},"required":["jid"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.GetWhatsAppMessages,
	})
}

// NewWhatsAppRegistry creates a registry with only WhatsApp tools.
func NewWhatsAppRegistry() *ToolRegistry {
	r := &ToolRegistry{
		tools: make(map[string]ToolDef),
	}
	r.registerWhatsAppTools()
	return r
}

// NewReadOnlyRegistry creates a registry with no mutating tools — read/list/
// search/inspect plus web_search, fetch_page, get_calendar_events and an
// allowlisted run_command_readonly. Used for the analyzer/reviewer/test-runner
// sub-agents of the Self-Driving loop, so exactly one "coder" sub-agent can
// write while the others run in parallel with no risk of clobbering it.
//
// "Read-only" here means no write/edit/delete/cd/run_command tools and a
// prefix-anchored command allowlist — it is not a syscall sandbox (a test run
// can still touch the filesystem). That trade-off is what removes the need for
// any conflict-merge logic.
func NewReadOnlyRegistry() *ToolRegistry {
	r := &ToolRegistry{tools: make(map[string]ToolDef)}
	r.Register(ToolDef{
		Name:        "read_file",
		Description: "Reads a file (use offset/limit for large files). Read a file before editing it, so your edit matches its exact text and style.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Path to the file to read"},"offset":{"type":"integer","description":"1-based line to start from (optional)"},"limit":{"type":"integer","description":"max lines to return from offset (optional)"}},"required":["path"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.ReadFile,
	})
	r.Register(ToolDef{
		Name:        "list_directory",
		Description: "Lists files and directories in a path",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string", "description": "Path to directory"}, "recursive": {"type": "boolean", "description": "Whether to list recursively"}}, "required": ["path", "recursive"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.ListDirectory,
	})
	r.Register(ToolDef{
		Name:        "search_files",
		Description: "Finds files by NAME using a glob (e.g. *.go). It does not look inside files — to search the text of files use search_content.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"pattern": {"type": "string", "description": "Glob pattern (e.g. *.go)"}, "path": {"type": "string", "description": "Directory to search in"}}, "required": ["pattern", "path"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.SearchFiles,
	})
	r.Register(ToolDef{
		Name:        "search_content",
		Description: "Searches INSIDE files for a regular expression (grep) and returns path:line: text. Use it to find where a function, type or string is defined and every place it is used BEFORE you change it. Skips .git, node_modules, vendor, build output and binary files.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string","description":"Regular expression (Go/RE2 syntax). Escape ( ) [ ] { } . * + ? with a backslash to match them literally."},"path":{"type":"string","description":"Directory or file to search (default: the project root)"},"glob":{"type":"string","description":"Only search files whose NAME matches this glob, e.g. *.go"},"ignore_case":{"type":"boolean"},"context":{"type":"integer","description":"Lines of context around each match, 0-3 (default 0)"},"max_results":{"type":"integer","description":"Stop after this many matches (default 100, max 500)"}},"required":["pattern"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.SearchContent,
	})
	r.Register(ToolDef{
		Name:        "get_file_info",
		Description: "Gets metadata about a file or directory",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string", "description": "Path to the file/directory"}}, "required": ["path"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.GetFileInfo,
	})
	r.Register(ToolDef{
		Name:        "read_env",
		Description: "Reads non-sensitive environment variables",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {}}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.ReadEnv,
	})
	r.Register(ToolDef{
		Name:        "get_calendar_events",
		Description: "Reads saved calendar events.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"from":{"type":"string"},"to":{"type":"string"}}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.GetCalendarEvents,
	})
	r.Register(ToolDef{
		Name:        "run_command_readonly",
		Description: "Runs a build/test/inspection command from a fixed allowlist (go test/build/vet, git status/diff/log, ls, cat, rg, grep, find, flutter analyze/test, npm test, pytest, ...). Anything not on the allowlist is rejected.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"command": {"type": "string"}, "cwd": {"type": "string"}}, "required": ["command"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.RunCommandReadOnly,
	})
	r.registerWebSearchTool()
	r.registerFetchPageTool()
	return r
}

// NewBriefingRegistry creates a registry with safe read-only file/project/calendar
// inspection tools plus web_search and fetch_page, excluding read_env and any
// mutating tools. Used when the morning briefing model answers the owner's custom
// briefing instructions (news, URLs, repo/file reports such as @handoff.md).
func NewBriefingRegistry() *ToolRegistry {
	r := NewReadOnlyRegistry()
	r.Unregister("read_env")
	return r
}

func (r *ToolRegistry) Register(tool ToolDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
}

func (r *ToolRegistry) Get(name string) (ToolDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	if ok && tool.Beta && !beta.Enabled() {
		return ToolDef{}, false
	}
	return tool, ok
}

// Unregister removes a previously registered tool. Used to tear down a
// skill's tools when the skill is deactivated or removed.
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// ToOpenAITools converts the registered tools into the format expected by the
// LLM providers, sorted by tool name. The order matters: this is re-sent on
// every one of a turn's up-to-N agent iterations, and providers only serve a
// prompt-cache hit on it (Anthropic explicit cache_control, OpenAI automatic)
// when the serialized bytes are identical each time — a Go map iteration
// order is not.
//
// activeSkills restricts skill-provided tools (ToolDef.SkillOwner != "") to
// those whose owning skill is in the set — a skill's tools are registered
// into this shared registry unconditionally (see skill.Manager.
// RegisterAllTools), so this is the boundary that actually makes a skill's
// tools chat-scoped: a chat whose active-skill set doesn't include the
// owner never has that tool sent to the LLM. Built-in tools
// (SkillOwner == "") are never gated and always included. Pass nil (or an
// empty map) to include only built-ins.
func (r *ToolRegistry) ToOpenAITools(activeSkills map[string]bool) []provider.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Read once so one call sees one answer even if the switch flips midway.
	betaOn := beta.Enabled()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	defs := make([]provider.ToolDefinition, 0, len(names))
	for _, name := range names {
		t := r.tools[name]
		if t.SkillOwner != "" && !activeSkills[t.SkillOwner] {
			continue
		}
		if t.Beta && !betaOn {
			continue
		}
		defs = append(defs, provider.ToolDefinition{
			Type: "function",
			Function: provider.ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return defs
}

// codeModeOmitted are tools that have nothing to do with editing a project:
// messaging, calendar, routines, provider/self management. A Code Mode turn
// does not offer them — about 1k tokens of schema re-sent on every iteration,
// and a chance for the model to reach for WhatsApp in the middle of a refactor.
// They stay registered, so the same tools still work in ordinary agent chats.
var codeModeOmitted = map[string]bool{
	"twin_contacts": true, "contact_timeline": true, "twin_commitments": true, "twin_pending_actions": true, "check_availability": true, "git_project_status": true, "dev_status": true,
	"mail_accounts": true, "mail_inbox": true, "mail_search": true, "mail_read": true, "mail_download_attachment": true, "mail_send": true, "mail_save_draft": true, "telegram_send": true,
	"whatsapp_send": true, "whatsapp_search": true, "whatsapp_latest": true, "whatsapp_messages": true,
	"get_calendar_events": true, "create_routine": true, "list_routines": true, "cancel_routine": true,
	"configure_provider": true, "self_clone": true,
}

// ForCodeMode drops the tools a Code Mode turn does not need from defs.
func ForCodeMode(defs []provider.ToolDefinition) []provider.ToolDefinition {
	out := make([]provider.ToolDefinition, 0, len(defs))
	for _, d := range defs {
		if codeModeOmitted[d.Function.Name] {
			continue
		}
		out = append(out, d)
	}
	return out
}

// Execute runs a registered tool by name.
func (r *ToolRegistry) Execute(ctx context.Context, name string, args json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	tool, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}
	if guard, ok := ctx.Value(mutationGuardKey{}).(MutationGuard); ok && mutationTool(tool) {
		release, err := guard(ctx, basePath)
		if err != nil {
			return "", err
		}
		defer release()
	}
	return tool.ExecuteFn(ctx, args, basePath, createBackup)
}

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/agent/tools"
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
	return r
}
func (r *ToolRegistry) registerBuiltins() {
	r.Register(ToolDef{
		Name:        "read_file",
		Description: "Reads the content of a file",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Path to the file to read"},"offset":{"type":"integer","description":"1-based line to start from (optional)"},"limit":{"type":"integer","description":"max lines to return from offset (optional)"}},"required":["path"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.ReadFile,
	})

	r.Register(ToolDef{
		Name:        "write_file",
		Description: "Writes content to a file. Overwrites if exists, creates if not.",
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
		Description: "Changes the agent's working-directory sandbox to a different existing directory for the rest of this conversation. Every other file tool (read_file, write_file, run_command, etc.) is scoped to this directory afterward. Call this when the user explicitly asks you to work somewhere else, AND proactively when a file/command tool call fails with an 'outside the project directory' error and the user's request implies they want you working at that other location (e.g. they asked for a file on their Desktop) — in that case, tell them where you'd switch to and call this once they confirm, rather than telling them to move the file into your current directory instead. Requires an existing directory — relative paths resolve against the current working directory, and '~' resolves to the user's home.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string", "description": "Path to the directory to switch to"}}, "required": ["path"]}`),
		DangerLevel: Dangerous,
		ExecuteFn:   tools.ChangeDirectory,
		PreviewFn:   tools.ChangeDirectoryPreview,
	})

	r.Register(ToolDef{
		Name:        "run_command",
		Description: "Executes a terminal command using bash -c",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"command": {"type": "string", "description": "The command to run"}, "cwd": {"type": "string", "description": "Optional working directory"}}, "required": ["command"]}`),
		DangerLevel: Dangerous,
		ExecuteFn:   tools.RunCommand,
	})

	r.Register(ToolDef{
		Name:        "search_files",
		Description: "Searches for files matching a pattern",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"pattern": {"type": "string", "description": "Glob pattern (e.g. *.go)"}, "path": {"type": "string", "description": "Directory to search in"}}, "required": ["pattern", "path"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.SearchFiles,
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
		Description: "Edits an existing file. Provide EITHER old_string and new_string, OR start_line, end_line and new_content.",
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

	r.Register(ToolDef{
		Name:        "get_task_status",
		Description: "Çalışan otonom görev (Self-Driving / Task.md döngüsü) durumunu OKUR: faz, adım N/M, madde a/b, o an işlenen adım, geçen süre. Kullanıcı \"görev ne durumda\", \"nerede kaldı\", \"bitti mi\" gibi bir şey sorduğunda TAHMİN ETME — bu aracı çağır. Araç \"çalışan görev yok\" derse, öyle söyle; asla durum uydurma.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.GetTaskStatus,
	})
	r.Register(ToolDef{
		Name:        "pause_task",
		Description: "Bu sohbete bağlı çalışan otonom görevi DURAKLATIR (kaldığı adım korunur). Kullanıcı görevi kastederek \"dur\", \"duraklat\", \"bekle\", \"stop\", \"pause\" dediğinde çağır. Duraklatınca kullanıcı serbestçe soru sorabilir; sonra resume_task ile aynı adımdan devam eder.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.PauseTask,
	})
	r.Register(ToolDef{
		Name:        "resume_task",
		Description: "Bu sohbete bağlı DURAKLATILMIŞ otonom görevi kaldığı adımdan SÜRDÜRÜR. Kullanıcı \"devam\", \"devam et\", \"kaldığın yerden devam\", \"continue\", \"resume\" ya da açıkça görevin sürmesini istediğini belirten bir şey dediğinde çağır. Duraklatmada kullanıcının yazdığı gerçek talimatlar (ekle/düzelt) bir sonraki adıma otomatik iletilir.",
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
		Description: "Yeni bir Task.md dosyası yazar (Memo'nun otonom görev listesi formatında). Kullanıcı \"benimle bir task listesi/Task.md hazırla\", \"şu işi maddelere böl\" gibi bir şey dediğinde: önce sohbette hedefi ve ayrık, tek tek doğrulanabilir teslimatları netleştir, sonra bu aracı çağır. path: opsiyonel (verilmezse bu sohbetin proje klasöründe Task.md). items: onay kutulu maddeler (zorunlu). intro: hedefi anlatan kısa paragraf. notify: sadece-bitince|önemli|her-şey. mode: worker|planlayıcı. planner_model/coder_model/verifier_model: rol başına model sabitlemek istersen (ör. \"local\", \"claude\"). memory: açık|kapalı. auto_approve: plan onay kapısını atla. Dosya zaten varsa hata verir; değiştirmek için edit_task_md kullan.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Optional file path; defaults to Task.md in this chat's project folder"},"items":{"type":"array","items":{"type":"string"},"description":"Checkbox items — each a concrete, independently verifiable deliverable"},"intro":{"type":"string","description":"Short paragraph stating the goal and any context"},"notify":{"type":"string","description":"sadece-bitince | önemli | her-şey"},"mode":{"type":"string","description":"worker | planlayıcı"},"planner_model":{"type":"string"},"coder_model":{"type":"string"},"verifier_model":{"type":"string"},"memory":{"type":"string","description":"açık | kapalı"},"auto_approve":{"type":"boolean"}},"required":["items"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.CreateTaskMd,
	})
	r.Register(ToolDef{
		Name:        "edit_task_md",
		Description: "Var olan bir Task.md'yi yerinde düzenler, mevcut onay kutusu durumlarını ve başlıkları koruyarak. op: add_item (yeni madde ekle — text), split_item (item_index'teki maddeyi sub_items alt maddelerine böl, maddeye [parallel] ekler), set_header (header_key + header_value), check_item (item_index'teki maddeyi [x] yap). item_index 1 tabanlıdır ve iç içe maddeler dahil dosya sırasına göredir.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Task.md path; defaults to this chat's project Task.md"},"op":{"type":"string","enum":["add_item","split_item","set_header","check_item"]},"text":{"type":"string","description":"add_item: the new item text"},"item_index":{"type":"integer","description":"1-based, for split_item / check_item"},"sub_items":{"type":"array","items":{"type":"string"},"description":"split_item: the sub-item texts"},"header_key":{"type":"string","description":"set_header: e.g. bildirim, mod, kodlayıcı"},"header_value":{"type":"string"}},"required":["op"]}`),
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
		Description: "Saves the current implementation plan as Markdown to Memo's own plan storage — NOT the user's project, nothing here touches the project directory. Call this once your investigation is done and you have a concrete, reviewable, step-by-step plan — only in Code Mode's planning mode, never mid-edit. After it succeeds, ask the user in plain chat text whether to proceed in build mode (fast, auto-approved edits) or auto mode (normal confirm-as-you-go editing).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"content":{"type":"string","description":"The full plan as Markdown"}},"required":["content"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.SaveCodePlan,
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
		Description: "Bir Task.md dosyasından otonom (kendi kendine ilerleyen) bir görev döngüsü başlatır. Kullanıcı \"şu Task.md'ye başla\", \"bu görev listesini çalıştır\", \"Task.md'yi otonom yap\" gibi bir şey dediğinde çağır — sen tek tek yapmak yerine görev döngüsü maddeleri sırayla, planlama + alt-ajan desteğiyle işler. task_md_path: onay kutulu maddeler (- [ ]) içeren Task.md dosyasının yolu (mutlak ya da çalışma dizinine göreli, ~ desteklenir). title: opsiyonel görünen ad (verilmezse dosya adı). Görev bu sohbete bağlanır, arka planda çalışır; ilerleme Görevler sekmesinden ve \"görev durumu\" diye sorulunca görülür. Bu aracı çağırmak, maddelerin gerektirdiği dosya değişikliklerine onay vermek demektir.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"task_md_path":{"type":"string","description":"Path to a Task.md file containing '- [ ]' checkbox items (absolute, or relative to the working directory; ~ is expanded)"},"title":{"type":"string","description":"Optional display name for the task list; defaults to the file name"}},"required":["task_md_path"]}`),
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
		Description: "Bir dosyayı veya klasörü kullanıcıya gönderir. path: gönderilecek dosya/klasör yolu (agent'ın erişebildiği yerlerden biri olmalı). Klasör verilirse otomatik zip'lenir, tek dosya ise olduğu gibi gönderilir. Nereye gönderileceği (WhatsApp, Telegram, ya da bu masaüstü/web sohbeti) hiçbir zaman burada belirtilmez — her zaman bu konuşmanın kendisine gönderilir.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Gönderilecek dosya veya klasörün yolu"}},"required":["path"]}`),
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
		Description: "Kullanıcının serbest metinle tarif ettiği zamanlanmış bir görev (rutin) oluşturur — örn. \"her gün saat 9'da yapay zeka haberlerini getir\" veya \"her pazartesi takvimimi özetle\". text: kullanıcının isteğini olduğu gibi (kendi cümleleriyle) aktar; zaman, gün, içerik ve hangi kanaldan (WhatsApp/Telegram) gönderileceği otomatik çıkarılır — teslimat hedefi hiçbir zaman burada belirtilmez, her zaman bu konuşmanın kendisine (ve o konuşmada açıkça istenen diğer bağlı kanallara) gönderilir.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"text":{"type":"string","description":"Kullanıcının rutin isteğinin serbest metin hali"}},"required":["text"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.CreateRoutine,
	})
	r.Register(ToolDef{
		Name:        "list_routines",
		Description: "Kullanıcının tüm zamanlanmış rutinlerini listeler (id, prompt, saat, günler, hangi kanal(lar)dan gönderildiği, aktif olup olmadığı). Kullanıcı \"rutinlerimi göster\", \"hangi rutinlerim var\" gibi bir şey sorduğunda, ya da bir rutini iptal etmeden önce gerçek id'sini öğrenmek için bunu çağır.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.ListRoutines,
	})
	r.Register(ToolDef{
		Name:        "cancel_routine",
		Description: "Bir rutini kalıcı olarak siler. id: silinecek rutinin gerçek id'si — bunu tahmin etme, önce list_routines çağırıp oradaki gerçek id'yi kullan.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"list_routines'ten alınan gerçek rutin id'si"}},"required":["id"]}`),
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
		Description: "Searches the web (DuckDuckGo) and returns relevant results. Only call this for current events, recent news, prices, or specific facts that may have changed after your training cutoff. Do NOT call it for greetings, small talk, general knowledge you already know, or coding/file/project questions — answer those directly instead. This tool answers information requests in place — it never opens a browser window; use open_app only for an explicit \"launch/open the browser\" command with no actual question attached.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Short keyword-style search query (2-6 words) — extract the subject, do NOT pass the user's raw message verbatim"},"max_results":{"type":"integer","description":"Number of results to return (default 5, max 10)"}},"required":["query"]}`),
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
		Description: "Fetches the full readable content of a URL as Markdown (headings, lists, code blocks, links preserved) — a search result's snippet is a short teaser, not the actual page. Use this after web_search to actually read a promising result, or directly when the user already gave you a URL — including a plain \"read/summarize this page\" or \"what does this site say\" request. This is the right tool for that, NOT browser_navigate: fetch_page is fast, lightweight, and exactly what reading content means. Only reach for browser_navigate (and its click/type/scroll siblings) when the user needs you to actually interact with the live page — click something, fill a form, test a button — or visually verify a UI, not just read what's on it. Judge relevance yourself from what comes back: if the content doesn't actually match what you're looking for, call this again with a different search result's URL instead of answering from an irrelevant page. You get up to 5 attempts at DIFFERENT domains per request — fetching another page on a domain you already tried (pagination, a different page of the same docs site) is free and does not count against that limit. If the budget runs out, tell the user you could not find a relevant source instead of guessing.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"The exact URL to fetch — from a web_search result, or given directly by the user"}},"required":["url"]}`),
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
		Description: "Launches a desktop application, or the default browser with a blank tab, on the user's computer (Windows, macOS, or Linux). Call this ONLY for an explicit launch/open command naming an application — e.g. \"Spotify'ı aç\", \"Steam'i başlat\", \"tarayıcıyı aç\", \"open VS Code\". Do NOT call this for a question, a search, or any information request, even if it mentions a browser or an app by name — \"en son haberler ne\", \"X hakkında bilgi ver\", \"YouTube'da ne var\" are answered with web_search or directly, never by opening anything. This tool only starts the program — it cannot search inside it, open a specific site, play a specific song, or answer any question.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"app_name":{"type":"string","description":"Just the application's name (e.g. \"Spotify\", \"Steam\", \"tarayıcı\"/\"browser\", \"Discord\", \"VS Code\") — extract it, do NOT pass the user's raw sentence"}},"required":["app_name"]}`),
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
		Description: "Opens a URL in Memo's own sandboxed, interactive browser session (a dedicated, disposable Chromium process — never the user's real browser or accounts) so you can click/type/scroll through and test a live page, e.g. a site you just built. Do NOT use this for a plain \"read/summarize this page\" or \"what does this site say\" request — that's fetch_page (much faster, no browser launch); reach for this tool only when the task genuinely needs interaction (click a button, fill a form, verify something visually) or the user explicitly asks you to open/use a browser. Starts a session on first use; a later call reuses it, loading a new URL in the same tab. Does not return a screenshot itself — call browser_screenshot afterward to see the page.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"The URL to open"}},"required":["url"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.BrowserNavigate,
	})
	r.Register(ToolDef{
		Name:        "browser_click",
		Description: "Clicks an element in the active interactive browser session — either by CSS selector, or by exact viewport coordinates if there's no stable selector to target (e.g. canvas content). Provide exactly one of selector or (x and y). Requires an existing session (call browser_navigate first).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector of the element to click"},"x":{"type":"number","description":"Viewport x coordinate (use together with y instead of selector)"},"y":{"type":"number","description":"Viewport y coordinate (use together with x instead of selector)"}}}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.BrowserClick,
	})
	r.Register(ToolDef{
		Name:        "browser_type",
		Description: "Focuses an element by CSS selector in the active interactive browser session and types text into it as real key events (triggers the page's own input handlers, not just a value assignment). Requires an existing session (call browser_navigate first).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string","description":"CSS selector of the input/textarea/editable element"},"text":{"type":"string","description":"The text to type"}},"required":["selector","text"]}`),
		DangerLevel: Medium,
		ExecuteFn:   tools.BrowserType,
	})
	r.Register(ToolDef{
		Name:        "browser_scroll",
		Description: "Scrolls the page in the active interactive browser session by (dx, dy) pixels from its current position. Omitting both scrolls down a bit by default. Requires an existing session (call browser_navigate first).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"dx":{"type":"integer","description":"Horizontal scroll in pixels (default 0)"},"dy":{"type":"integer","description":"Vertical scroll in pixels (default 800 if both dx and dy are omitted)"}}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.BrowserScroll,
	})
	r.Register(ToolDef{
		Name:        "browser_screenshot",
		Description: "Captures the current page of the active interactive browser session and shows it to the USER in a live pane — call it after navigate/click/type/scroll so the user can watch along. You yourself do NOT receive the image (no visual/multimodal channel is wired up) — call browser_get_text if you need to know what's actually on the page. Returns an error if no session is currently open.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.BrowserScreenshot,
	})
	r.Register(ToolDef{
		Name:        "browser_get_text",
		Description: "Reads the active interactive browser session's page as plain visible text. Since you cannot see screenshots yourself, this is your real way to know what's on a page — use it to find button/link text, form field labels, or confirm what actually rendered, before deciding what to click or type. Requires an existing session (call browser_navigate first).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.BrowserGetText,
	})
	r.Register(ToolDef{
		Name:        "browser_close",
		Description: "Closes the active interactive browser session, if one is open. Call this once you're done testing a page — a forgotten session closes itself automatically after a few minutes of inactivity, but closing it explicitly frees the resources sooner.",
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
		Description: "Reads the content of a file",
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
		Description: "Searches for files matching a pattern",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"pattern": {"type": "string", "description": "Glob pattern (e.g. *.go)"}, "path": {"type": "string", "description": "Directory to search in"}}, "required": ["pattern", "path"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.SearchFiles,
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

func (r *ToolRegistry) Register(tool ToolDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
}

func (r *ToolRegistry) Get(name string) (ToolDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
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
func (r *ToolRegistry) ToOpenAITools() []provider.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	defs := make([]provider.ToolDefinition, 0, len(names))
	for _, name := range names {
		t := r.tools[name]
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

// Execute runs a registered tool by name.
func (r *ToolRegistry) Execute(ctx context.Context, name string, args json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	tool, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}
	return tool.ExecuteFn(ctx, args, basePath, createBackup)
}

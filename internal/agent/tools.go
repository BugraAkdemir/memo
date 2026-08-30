package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/agent/tools"
	"memo/internal/provider"
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
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string", "description": "Path to the file to read"}}, "required": ["path"]}`),
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

	r.Register(ToolDef{
		Name:        "get_task_status",
		Description: "Çalışan otonom görev (Self-Driving / Task.md döngüsü) durumunu OKUR: faz, adım N/M, madde a/b, o an işlenen adım, geçen süre. Kullanıcı \"görev ne durumda\", \"nerede kaldı\", \"bitti mi\" gibi bir şey sorduğunda TAHMİN ETME — bu aracı çağır. Araç \"çalışan görev yok\" derse, öyle söyle; asla durum uydurma.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.GetTaskStatus,
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
		Description: "Searches the web (DuckDuckGo) and returns relevant results. Only call this for current events, recent news, prices, or specific facts that may have changed after your training cutoff. Do NOT call it for greetings, small talk, general knowledge you already know, or coding/file/project questions — answer those directly instead.",
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
		Description: "Fetches the full readable content of a URL as Markdown (headings, lists, code blocks, links preserved) — a search result's snippet is a short teaser, not the actual page. Use this after web_search to actually read a promising result, or directly when the user already gave you a URL. Judge relevance yourself from what comes back: if the content doesn't actually match what you're looking for, call this again with a different search result's URL instead of answering from an irrelevant page. You get up to 5 attempts at DIFFERENT domains per request — fetching another page on a domain you already tried (pagination, a different page of the same docs site) is free and does not count against that limit. If the budget runs out, tell the user you could not find a relevant source instead of guessing.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"The exact URL to fetch — from a web_search result, or given directly by the user"}},"required":["url"]}`),
		DangerLevel: Safe,
		ExecuteFn:   tools.FetchPage,
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
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string", "description": "Path to the file to read"}}, "required": ["path"]}`),
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

// ToOpenAITools converts the registered tools into the format expected by the LLM providers.
func (r *ToolRegistry) ToOpenAITools() []provider.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var defs []provider.ToolDefinition
	for _, t := range r.tools {
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

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

	r.registerWhatsAppTools()
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
			Danger: string(t.DangerLevel),
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

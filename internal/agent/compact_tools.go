package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"memo/internal/provider"
	"memo/internal/truncate"
)

// LoadToolsName is the one tool a compact turn starts with.
const LoadToolsName = "load_tools"

// ToolGroup is a set of tools a model on a small context window loads on
// demand through load_tools. The model picks the group itself from the short
// summaries — there is no keyword matching on the user's message, so it works
// in whatever language the person writes.
type ToolGroup struct {
	Name    string
	Summary string
	Tools   []string
}

// ToolGroups are what a compact turn can load. Everything not listed here
// (sub-agents, the Self-Driving task loop and Task.md editing, provider
// configuration, self_clone, installing skills, use_skill) is never offered on
// a small window: those tools alone cost several thousand tokens and need a
// model that can carry a long multi-step plan.
var ToolGroups = []ToolGroup{
	{"files_read", "read, list and search files", []string{"read_file", "list_directory", "search_files", "search_content", "get_file_info"}},
	{"files_write", "create, edit, delete files", []string{"write_file", "edit_file", "insert_line", "delete_lines", "delete_file"}},
	{"shell", "run commands, git status", []string{"run_command", "git_project_status", "read_env", "dev_status"}},
	{"plan", "save a Code Mode plan", []string{"save_code_plan"}},
	{"web", "web search, read pages", []string{"web_search", "fetch_page"}},
	{"calendar", "calendar, free time, routines", []string{"get_calendar_events", "check_availability", "create_routine", "list_routines", "cancel_routine"}},
	{"mail", "email: read, search, draft, send", []string{"mail_accounts", "mail_inbox", "mail_search", "mail_read", "mail_save_draft", "mail_send", "mail_download_attachment"}},
	{"messaging", "WhatsApp, Telegram, send a file", []string{"whatsapp_send", "whatsapp_messages", "whatsapp_search", "whatsapp_latest", "telegram_send", "share_file"}},
	{"contacts", "contacts, pending replies, promises", []string{"twin_contacts", "twin_pending_actions", "twin_commitments", "contact_timeline"}},
}

func toolGroupByName(name string) (ToolGroup, bool) {
	for _, g := range ToolGroups {
		if g.Name == name {
			return g, true
		}
	}
	return ToolGroup{}, false
}

// CompactTools is one chat's on-demand tool state on a small window. Loaded
// groups stay loaded for the rest of the chat (the tool list is part of the
// prompt prefix; changing it every turn would throw the local KV cache away
// every turn). Budget caps the tokens the offered tool definitions may take.
type CompactTools struct {
	Budget int
	// OnLoad, when set, is told about every group that gets loaded, so the
	// caller can keep it for the chat's next turns.
	OnLoad func(group string)

	mu     sync.Mutex
	loaded map[string]bool
}

// NewCompactTools starts from the groups the chat already loaded.
func NewCompactTools(budget int, loaded []string, onLoad func(string)) *CompactTools {
	c := &CompactTools{Budget: budget, OnLoad: onLoad, loaded: map[string]bool{}}
	for _, g := range loaded {
		if _, ok := toolGroupByName(g); ok {
			c.loaded[g] = true
		}
	}
	return c
}

// Loaded returns the loaded group names, sorted.
func (c *CompactTools) Loaded() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.loaded))
	for g := range c.loaded {
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

func (c *CompactTools) offers(tool string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for g := range c.loaded {
		grp, _ := toolGroupByName(g)
		for _, t := range grp.Tools {
			if t == tool {
				return true
			}
		}
	}
	return false
}

// groupOf names the group a tool belongs to ("" if none).
func groupOf(tool string) string {
	for _, g := range ToolGroups {
		for _, t := range g.Tools {
			if t == tool {
				return g.Name
			}
		}
	}
	return ""
}

// LoadToolsDefinition is load_tools as the model sees it. Static text (the
// loaded state is reported in its result, not here), so the definition itself
// never changes the prompt.
func LoadToolsDefinition() provider.ToolDefinition {
	var sb strings.Builder
	sb.WriteString("Load tool groups before using them; they are callable from your next step. Load only what the task needs. Groups: ")
	names := make([]string, 0, len(ToolGroups))
	for i, g := range ToolGroups {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(g.Name + " = " + g.Summary)
		names = append(names, g.Name)
	}
	sb.WriteString(".")
	enum, _ := json.Marshal(names)
	params := `{"type":"object","properties":{"groups":{"type":"array","items":{"type":"string","enum":` + string(enum) +
		`},"description":"Group names to load"}},"required":["groups"]}`
	return provider.ToolDefinition{
		Type: "function",
		Function: provider.ToolFunction{
			Name:        LoadToolsName,
			Description: sb.String(),
			Parameters:  json.RawMessage(params),
		},
	}
}

// Filter is the tool list a compact turn sends: load_tools, the tools of the
// loaded groups, and keep(d) == true extras (a skill's own tools, which the
// person switched on by calling the skill). Order follows defs.
func (c *CompactTools) Filter(defs []provider.ToolDefinition, keep func(provider.ToolDefinition) bool) []provider.ToolDefinition {
	out := []provider.ToolDefinition{LoadToolsDefinition()}
	for _, d := range defs {
		if c.offers(d.Function.Name) || (keep != nil && keep(d)) {
			out = append(out, d)
		}
	}
	return out
}

// DefinitionTokens estimates what a tool list costs in the prompt.
func DefinitionTokens(defs []provider.ToolDefinition) int {
	if len(defs) == 0 {
		return 0
	}
	raw, err := json.Marshal(defs)
	if err != nil {
		return 0
	}
	return truncate.EstimateTokens(string(raw))
}

// Load handles a load_tools call against all, the full tool list of the turn.
// It loads every named group that fits the budget and says plainly what did
// not, so the model is never left guessing.
func (c *CompactTools) Load(rawArgs json.RawMessage, all []provider.ToolDefinition, keep func(provider.ToolDefinition) bool) string {
	var args struct {
		Groups []string `json:"groups"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil || len(args.Groups) == 0 {
		return "Error: give the group names to load in \"groups\", e.g. {\"groups\":[\"files_read\"]}."
	}
	var loaded, full, unknown []string
	for _, name := range args.Groups {
		name = strings.TrimSpace(name)
		grp, ok := toolGroupByName(name)
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		c.mu.Lock()
		already := c.loaded[name]
		c.mu.Unlock()
		if already {
			loaded = append(loaded, name+" (already loaded)")
			continue
		}
		c.mu.Lock()
		c.loaded[name] = true
		c.mu.Unlock()
		if c.Budget > 0 && DefinitionTokens(c.Filter(all, keep)) > c.Budget {
			c.mu.Lock()
			delete(c.loaded, name)
			c.mu.Unlock()
			full = append(full, name)
			continue
		}
		var tools []string
		for _, d := range all {
			for _, t := range grp.Tools {
				if d.Function.Name == t {
					tools = append(tools, t)
				}
			}
		}
		if len(tools) == 0 {
			loaded = append(loaded, name+" (no tools of this group are available here)")
		} else {
			loaded = append(loaded, name+": "+strings.Join(tools, ", "))
		}
		if c.OnLoad != nil {
			c.OnLoad(name)
		}
	}
	var sb strings.Builder
	if len(loaded) > 0 {
		sb.WriteString("Loaded " + strings.Join(loaded, "; ") + ". Call them from your next step.")
	}
	if len(full) > 0 {
		if sb.Len() > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(fmt.Sprintf("Not loaded: %s — the tool space of this small model is full (loaded: %s). Do the task with the loaded tools or tell the user.",
			strings.Join(full, ", "), strings.Join(c.Loaded(), ", ")))
	}
	if len(unknown) > 0 {
		if sb.Len() > 0 {
			sb.WriteString(" ")
		}
		names := make([]string, 0, len(ToolGroups))
		for _, g := range ToolGroups {
			names = append(names, g.Name)
		}
		sb.WriteString("Unknown group: " + strings.Join(unknown, ", ") + ". Groups: " + strings.Join(names, ", ") + ".")
	}
	return sb.String()
}

// NotLoadedError is the answer to a call of a tool the compact turn did not
// offer: which group to load first, or that it is not available here at all.
func NotLoadedError(tool string) string {
	if g := groupOf(tool); g != "" {
		return fmt.Sprintf("tool %q is not loaded; call %s with {\"groups\":[%q]} first", tool, LoadToolsName, g)
	}
	return fmt.Sprintf("tool %q is not available with this small model", tool)
}

type compactToolsCtxKey struct{}

// WithCompactTools makes the next RunStream a compact turn using c.
func WithCompactTools(ctx context.Context, c *CompactTools) context.Context {
	return context.WithValue(ctx, compactToolsCtxKey{}, c)
}

func compactToolsFromCtx(ctx context.Context) *CompactTools {
	c, _ := ctx.Value(compactToolsCtxKey{}).(*CompactTools)
	return c
}

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

// ExtendedToolGroups are additional on-demand groups offered when OnDemandTools
// is active on normal/large windows (>= compactWindowLimit), omitted on <10k
// small windows so small models never receive heavy delegation/task schemas.
var ExtendedToolGroups = []ToolGroup{
	{"subagents", "delegate tasks to child agents", []string{"spawn_agent", "send_agent", "list_agents", "close_agent", "cancel_agent", "merge_agent"}},
	{"tasks", "Task.md and autonomous task loop", []string{"start_self_driving_task", "create_task_md", "edit_task_md", "get_task_status", "pause_task", "resume_task"}},
	{"skills", "list, load or install skills", []string{"use_skill", "install_project_skill"}},
	{"system", "working dir, provider config, clone", []string{"change_directory", "configure_provider", "self_clone"}},
	{"browser", "open apps and interactive browser", []string{"open_app", "browser_navigate", "browser_click", "browser_type", "browser_scroll", "browser_screenshot", "browser_get_text", "browser_close"}},
	{"os", "system snapshots, services, packages, GPU", []string{"sys_btrfs_snapshot", "sys_systemd_manage", "sys_package_env", "sys_kde_control", "sys_gpu_model_orchestrator", "sys_network_security"}},
}

var allToolGroups = append(append([]ToolGroup(nil), ToolGroups...), ExtendedToolGroups...)

func toolGroupByName(name string) (ToolGroup, bool) {
	return groupByNameIn(ToolGroups, name)
}

func groupByNameIn(groups []ToolGroup, name string) (ToolGroup, bool) {
	for _, g := range groups {
		if g.Name == name {
			return g, true
		}
	}
	return ToolGroup{}, false
}

// CompactTools is one chat's on-demand tool state. Loaded groups stay loaded
// for the rest of the chat (the tool list is part of the prompt prefix;
// changing it every turn would throw the KV/prompt cache away every turn).
// Budget caps the tokens the offered tool definitions may take (0 = unbounded).
// Pinned names core tools offered upfront alongside load_tools without
// requiring a round-trip first (used on normal/large windows).
type CompactTools struct {
	Budget   int
	Extended bool
	Pinned   map[string]bool
	// OnLoad, when set, is told about every group that gets loaded, so the
	// caller can keep it for the chat's next turns.
	OnLoad func(group string)

	mu     sync.Mutex
	loaded map[string]bool
}

// NormalHybridCoreTools and CodeHybridCoreTools are the upfront core tools
// kept loaded alongside load_tools in >=10k hybrid mode.
var (
	NormalHybridCoreTools = []string{"read_file", "list_directory", "search_content", "web_search"}
	CodeHybridCoreTools   = []string{"read_file", "list_directory", "search_content", "search_files", "write_file", "edit_file", "run_command"}
)

// NewCompactTools starts from the groups the chat already loaded (small-window mode).
func NewCompactTools(budget int, loaded []string, onLoad func(string)) *CompactTools {
	c := &CompactTools{Budget: budget, OnLoad: onLoad, loaded: map[string]bool{}}
	for _, g := range loaded {
		if _, ok := toolGroupByName(g); ok {
			c.loaded[g] = true
		}
	}
	return c
}

// NewOnDemandTools creates a hybrid on-demand loader for normal/large windows:
// pinned core tools are available immediately while all extended groups can be
// loaded on demand via load_tools.
func NewOnDemandTools(loaded []string, pinned []string, onLoad func(string)) *CompactTools {
	pinMap := make(map[string]bool, len(pinned))
	for _, name := range pinned {
		pinMap[name] = true
	}
	c := &CompactTools{Budget: 0, Extended: true, Pinned: pinMap, OnLoad: onLoad, loaded: map[string]bool{}}
	for _, g := range loaded {
		if _, ok := groupByNameIn(allToolGroups, g); ok {
			c.loaded[g] = true
		}
	}
	return c
}

func (c *CompactTools) groups() []ToolGroup {
	if c.Extended {
		return allToolGroups
	}
	return ToolGroups
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
	if c.Pinned[tool] {
		return true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	groups := c.groups()
	for g := range c.loaded {
		grp, ok := groupByNameIn(groups, g)
		if !ok {
			continue
		}
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
	return groupOfIn(ToolGroups, tool)
}

func groupOfIn(groups []ToolGroup, tool string) string {
	for _, g := range groups {
		for _, t := range g.Tools {
			if t == tool {
				return g.Name
			}
		}
	}
	return ""
}

// LoadToolsDefinition is load_tools as the model sees it on a small window.
// Static text so the definition itself never changes the prompt.
func LoadToolsDefinition() provider.ToolDefinition {
	return buildLoadToolsDef(ToolGroups)
}

// LoadToolsDefinitionFor builds load_tools advertising only groups from
// c.groups() that have at least one tool present in all. On small windows
// (!c.Extended) it returns the static LoadToolsDefinition().
func (c *CompactTools) LoadToolsDefinitionFor(all []provider.ToolDefinition) provider.ToolDefinition {
	if !c.Extended {
		return LoadToolsDefinition()
	}
	present := make(map[string]bool, len(all))
	for _, d := range all {
		present[d.Function.Name] = true
	}
	var avail []ToolGroup
	for _, g := range c.groups() {
		for _, t := range g.Tools {
			if present[t] && !c.Pinned[t] {
				avail = append(avail, g)
				break
			}
		}
	}
	if len(avail) == 0 {
		return LoadToolsDefinition()
	}
	return buildLoadToolsDef(avail)
}

func buildLoadToolsDef(groups []ToolGroup) provider.ToolDefinition {
	var sb strings.Builder
	sb.WriteString("Load tool groups before using them; they are callable from your next step. Load only what the task needs. Groups: ")
	names := make([]string, 0, len(groups))
	for i, g := range groups {
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

// Filter is the tool list an on-demand/compact turn sends: load_tools, the
// tools of the loaded groups, and keep(d) == true extras (a skill's own tools,
// which the person switched on by calling the skill). Order follows defs.
func (c *CompactTools) Filter(defs []provider.ToolDefinition, keep func(provider.ToolDefinition) bool) []provider.ToolDefinition {
	out := []provider.ToolDefinition{c.LoadToolsDefinitionFor(defs)}
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
	groups := c.groups()
	var loaded, full, unknown []string
	for _, name := range args.Groups {
		name = strings.TrimSpace(name)
		grp, ok := groupByNameIn(groups, name)
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
		names := make([]string, 0, len(groups))
		for _, g := range groups {
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

// NotLoadedError answers an unloaded tool call using c's active group list.
func (c *CompactTools) NotLoadedError(tool string) string {
	if g := groupOfIn(c.groups(), tool); g != "" {
		return fmt.Sprintf("tool %q is not loaded; call %s with {\"groups\":[%q]} first", tool, LoadToolsName, g)
	}
	return fmt.Sprintf("tool %q is not available with this model", tool)
}

type compactToolsCtxKey struct{}
type excludedToolsCtxKey struct{}

// WithCompactTools makes the next RunStream a compact turn using c.
func WithCompactTools(ctx context.Context, c *CompactTools) context.Context {
	return context.WithValue(ctx, compactToolsCtxKey{}, c)
}

func compactToolsFromCtx(ctx context.Context) *CompactTools {
	c, _ := ctx.Value(compactToolsCtxKey{}).(*CompactTools)
	return c
}

// WithExcludedTools hides tools in excluded from the turn's offered schemas
// and execution (e.g. mail/WhatsApp/Telegram when not connected).
func WithExcludedTools(ctx context.Context, excluded map[string]bool) context.Context {
	if len(excluded) == 0 {
		return ctx
	}
	cp := make(map[string]bool, len(excluded))
	for k, v := range excluded {
		if v {
			cp[k] = true
		}
	}
	return context.WithValue(ctx, excludedToolsCtxKey{}, cp)
}

func excludedToolsFromCtx(ctx context.Context) map[string]bool {
	m, _ := ctx.Value(excludedToolsCtxKey{}).(map[string]bool)
	return m
}

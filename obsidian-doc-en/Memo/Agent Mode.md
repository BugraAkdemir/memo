# 🤖 Agent Mode

> **Package:** `internal/agent/` (**40 tools** in the main registry as of v4.6.0 — counted from `NewRegistry()`, `internal/agent/tools.go`; 36 plus WhatsApp's four, see "WhatsApp tools" below)
> **Config file:** `data/permissions.json`
> **API endpoints:** `/api/agent/enabled`, `/api/agent/permission`, `/api/agent/permissions`
> **Requires:** An active external provider, or a running local llama.cpp model — `resolveAgentProvider()` wraps either one into the same `provider.Router`-based tool-calling request, so agent mode isn't external-provider-only. Real-world tool-calling *quality* on local models varies a lot by model (see Known Issues).

Agent Mode transforms Memo from a chat interface into an AI assistant that can interact with the user's computer — reading/writing files, executing commands, searching code, and more. It implements a Claude Code-like experience with a permission-based security model.

> **Don't confuse with:** the **Claude Code CLI** and **Codex CLI** providers in [[External Providers]] (`internal/agentcli/`, v3.3.4) are a completely different mechanism — they run the real `claude`/`codex` CLIs as a subprocess. When a chat's CLI provider is active, this page's Agent Mode pipeline (tool definitions, permission dialog, `internal/agent`) **never runs at all** — a deliberate design choice so the two tool-execution systems can't run at once.

---

## Architecture Overview

```
User Message
      │
      ▼
┌─────────────────────────────────────────┐
│  SendMessageStream()                     │
│  ┌─────────────────────────────────────┐ │
│  │  Agent enabled + active provider?   │ │
│  │  → callAgentStream()                │ │
│  │  → else: callLLMStream() (normal)   │ │
│  └─────────────────────────────────────┘ │
└──────────────────┬──────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────┐
│  Executor.RunStream()                    │
│  ┌─────────────────────────────────────┐ │
│  │  Pipeline                            │ │
│  │  1. LLM (with tool definitions)      │ │
│  │  2. Parse tool calls                 │ │
│  │  3. Permission check                 │ │
│  │  4. Execute tool (sandboxed)         │ │
│  │  5. Feed result back to LLM          │ │
│  │  6. Repeat until final response      │ │
│  │     (max 40 iterations)              │ │
│  └─────────────────────────────────────┘ │
└──────────────────┬──────────────────────┘
                   │
                   ▼
            ┌──────────┐
            │  Events   │──► user sees tool calls & results
            └──────────┘
```

---

## Tool System

**File:** `internal/agent/tools.go` (149 lines)

### Tool Definition Format

Each tool is defined with:
- **Name** — unique identifier
- **Description** — what the tool does (LLM uses this to decide)
- **Parameters** — JSON Schema format (OpenAI tool calling standard)
- **DangerLevel** — `safe` | `medium` | `dangerous`

```go
type ToolDef struct {
    Name        string
    Description string
    Parameters  map[string]interface{} // JSON Schema
    DangerLevel DangerLevel
    ExecuteFn   func(ctx context.Context, args map[string]interface{}) (string, error)
}
```

### Registry

`ToolRegistry` is a thread-safe registry that stores all available tools:

```go
type ToolRegistry struct {
    mu    sync.RWMutex
    tools map[string]ToolDef
}
```

Methods:
- `Register(tool)` — add a tool (panics on duplicate)
- `Get(name)` — retrieve by name
- `Execute(ctx, name, args)` — execute with validation
- `ToOpenAITools()` — convert to `[]provider.ToolDefinition` for LLM API

### Scoped Registries

The full 19-tool registry isn't the only one — `NewRegistry()` builds it via `registerBuiltins()`, but two narrower constructors build a registry with a hand-picked subset instead, each paired with a matching `New*Executor(existing *Executor)` (`executor.go`) that shares sandbox/permissions/backup/audit-log with an existing executor and only swaps the registry:

| Constructor | Tools | Used by |
|---|---|---|
| `NewWhatsAppRegistry()` / `NewWhatsAppExecutor()` | The 4 `whatsapp_*` tools | WhatsApp-triggered agent runs |
| `NewWebSearchRegistry()` / `NewWebSearchExecutor()` | `web_search` only | Plain chat's non-agent "web search mode" (see `web_search`'s entry below) |

This is how a feature gets native tool-calling's "model decides, single request, only pays for a second round-trip if it actually calls the tool" behavior without exposing the *entire* agent toolset (file writes, `run_command`, etc.) to a context that was never meant to be full Agent Mode.

---

## Built-in Tools

Registered in `internal/agent/tools.go`'s `registerBuiltins()` — **40 tools** in the main registry (counted by instantiating `NewRegistry()`; the numbered headings below were written when it held 27, and §28–36 add what came later), up from the original 8. On top of these, **skill tools are now real** (v3.3.3): a skill's `SKILL.md` can declare a `command:` field, and it executes through this exact same pipeline and permission UI — see [[Guides]].

### 1. `read_file` — Safe
```json
{
  "name": "read_file",
  "description": "Read the contents of a file",
  "parameters": {
    "type": "object",
    "properties": {
      "path": {"type": "string", "description": "Absolute or relative path"}
    },
    "required": ["path"]
  }
}
```
- **Implementation:** Reads file content, max 1MB size limit
- **Security:** Path validated against sandbox

### 2. `write_file` — Medium
- Creates parent directories automatically
- Creates `.bak` backup of existing files before overwriting
- Path validated against sandbox

### 3. `delete_file` — Dangerous
- Blocks deletion of `.git/` directory
- Supports both files and directories

### 4. `list_directory` — Safe
- Shows `[F]` (file) and `[D]` (directory) prefixes
- Recursive option available
- Max 1000 entries, skips `.git`, `node_modules`, etc.

### 5. `run_command` — Dangerous
```json
{
  "parameters": {
    "command": {"type": "string"},
    "cwd": {"type": "string", "description": "Working directory (default: project root)"}
  }
}
```
- Executes via `bash -c` with 60s timeout
- Output truncated at 10MB
- **Blacklist:** 23 dangerous patterns blocked

### 6. `search_files` — Safe
- Glob pattern matching (e.g., `**/*.go`)
- 30s timeout, max 100 results
- Skips `.git`, `node_modules`, `vendor`, `build`

### 7. `get_file_info` — Safe
- Returns: name, size, mode, modified time, is_dir

### 8. `read_env` — Medium
- Lists environment variables
- Masks sensitive keys (containing: KEY, TOKEN, SECRET, PASS, AUTH, CREDENTIAL)

### 9. `edit_file` — Medium
- Edits an existing file: either `old_string`/`new_string` (find-and-replace) or `start_line`/`end_line`/`new_content` (line-range replace)
- Has a `PreviewFn` for a diff-style preview before execution

### 10. `insert_line` — Medium
- Inserts content at a specific line number

### 11. `delete_lines` — Medium
- Deletes a range of lines (`start_line`–`end_line`)

### 12. `web_search` — Safe
- DuckDuckGo search; the model is instructed (via this tool's own `Description`) to reach for it only for current events/prices/facts that may have changed since training, not for greetings or general knowledge
- Fixed in v3.3.4: previously ran unconditionally on every message when the (separate, dumber) chat-level web search toggle was on, even with agent mode active — agent mode's own tool-based decision started handling it alone.
- **Redesigned (v3.5.6):** the chat-level toggle no longer has a separate blind-injection mechanism at all — when it's on and agent mode is off, `App.routeStream` (`chat.go`) now dispatches to `callWebSearchAgentStream` (`llm.go`), which runs this exact same tool through a *scoped* executor (`agent.NewWebSearchExecutor` / `NewWebSearchRegistry`, `executor.go`/`tools.go`) containing only `web_search` — nothing else from the full toolset. Same one-request, native-function-calling decision as full agent mode, just with a single tool registered, so plain chat gets "search only when the model actually decides it's needed" without an extra "should I search" LLM call and without exposing file/command tools. Gated off (no tool sent at all) when Orchestra mode is active — its own single-completion `RunSingle` path has nothing to plug tool-calling into — or when Minimal Mode is on, matching how the old blind-injection design was gated (Minimal Mode's whole promise is "zero injection beyond memory," and a tool definition riding along on every request is the same category of overhead). See the root `handoff.md`'s Session 17 entry for the live before/after repro (a bare "naber" now triggers zero web_search calls; a real info-seeking message triggers exactly one precisely-timed status update and a real search).

### 13. `self_clone` — Dangerous
- Copies the entire project (source + binary) to another local directory, for local replication/backup

### 14. `configure_provider` — Dangerous
- Adds/updates an external provider config (type, base URL, API key, model) from within a chat instead of Settings; requires user confirmation

### 15. `get_calendar_events` — Safe
- Reads real events from `events.db` for a date range — the model is instructed to call this instead of guessing when asked about the calendar

### 16. `change_directory` — Dangerous
- Switches the agent's whole file/command sandbox to a different existing directory for the rest of the conversation; every other file tool is scoped to the new base afterward

### 17. `get_task_status` — Safe
- Reads a running Self-Driving task's live status (phase, step N/M, item a/b, current step, elapsed) — the model is instructed to call this instead of guessing when asked "how's the task going"

### 18. `pause_task` — Safe / 19. `resume_task` — Safe
- Pause/resume the Self-Driving task tied to the current chat; on resume, whatever the user typed while paused is passed on to the next step

### 20. `create_task_md` — Medium / 21. `edit_task_md` — Medium
- Write a new `Task.md` (Memo's canonical checkbox-item task-list format) or edit one in place (add/split/check an item, set a header) — see [[Guides]] for the schema

### 22. `start_self_driving_task` — Medium
- Starts an autonomous, unattended run of a `Task.md` file's items — planning + optional sub-agents, progress visible in the Tasks tab or via `get_task_status`

### 23. `share_file` — Medium
- Sends a file or folder (auto-zipped) back to the user, always into the current conversation (never a different channel)

### 24. `create_routine` — Medium / 25. `list_routines` — Safe / 26. `cancel_routine` — Medium
- Create/list/cancel a scheduled routine from a free-text description (e.g. "every day at 9 send me AI news") — see [[Proactive Learning and Calendar]]

### 27. `fetch_page` — Safe
- Fetches a URL's full content as Markdown (not just `web_search`'s snippet) — up to 5 attempts across different domains per request

### 28. `save_code_plan` — see [[#Code Mode: Plan / Auto / Build sub-modes (new in v4.5.0)]]

### 29. `open_app` — Medium (v4.6.0)
- Launches a named desktop application, or the default browser with a blank tab, on Windows, macOS and Linux. Parameter: `app_name` — just the application's name ("Spotify", "Steam", "tarayıcı"/"browser", "VS Code"), never the raw sentence. Implementation: `internal/agent/tools/openapp.go` + `applaunch*.go`.
- A real side effect, hence **Medium** — one notch lighter than `run_command` (Dangerous) so it can still be allowed for the session.
- **Linux:** the name the model gives is a display name, not a binary; it is resolved through the Desktop Entry registry (so Flatpak/Snap apps work) and started with `startDetached` (own session, always reaped; an immediate non-zero exit is an error). `xdg-open about:blank` is *not* "open the browser" — `about:` has no handler on a normal desktop, so the default browser's own entry is started instead.
- **The description is the disambiguation mechanism against `web_search`:** it says to call this *only* for an explicit launch command and never for a question or information request, so "what's the latest news" searches the web instead of opening a browser.

### 30–36. The interactive browser tools — `browser_navigate` (Medium), `browser_click` (Medium), `browser_type` (Medium), `browser_scroll` / `browser_screenshot` / `browser_get_text` / `browser_close` (Safe) (v4.6.0)
- Drive one real but isolated Chromium tab (`internal/browserengine/session.go`, a dedicated profile — never the user's own browser, cookies or accounts). They live only in the main/full registry, not in the web-search, WhatsApp or read-only registries.
- `browser_click` takes a CSS `selector` or viewport `x`/`y`; `browser_type` takes a `selector` and `text`. `browser_get_text` returns the page's text **and a list of every clickable element with a selector that is guaranteed to work** (capped so a huge page cannot flood the prompt) — so Memo clicks one of those instead of guessing.
- `browser_navigate` is Medium rather than Dangerous on purpose: the session is a sandboxed throwaway process, and Dangerous would deny "allow for this session", forcing a fresh prompt on every call of a multi-step UI test.
- Only `http`, `https` and a blank page are accepted — `file://` would have let the browser read any file on the machine and bypass the file tools' restrictions.
- **Every page-changing tool pushes a frame itself** (`tools.pushFrame` + `framePushingTools` in `internal/app/browser_frame.go`) as a `browser_frame` SSE chunk, so the live pane updates without the model ever calling `browser_screenshot`. Frames go to the pane only and never enter the chat history. A new page-changing tool must be added to both.
- A session whose Chromium died is dropped, not reused (`Session.dead()`); each screenshot is exactly `ViewportWidth × ViewportHeight` because the viewport is emulated, so the pane maps taps through the PNG's real size.
- The user can drive the same session by hand from the pane through `/api/browser/session/*` (agent permission required, Agent Mode need not be on) — see [[API Documentation]].

---

### WhatsApp tools

`whatsapp_send` (Medium), `whatsapp_search` (Safe), `whatsapp_latest` (Safe), `whatsapp_messages` (Safe) are registered in the main registry too (`registerWhatsAppTools()` is called from `registerBuiltins()` — a count of `NewRegistry()` finds all four), and additionally exist in the separate scoped `NewWhatsAppRegistry()`/`NewWhatsAppExecutor()` used for WhatsApp-triggered agent runs (see "Scoped Registries" above, `internal/app/whatsapp.go`). An earlier version of this page claimed they were *only* in the scoped registry; that stopped being true.

---

## Permission System

**File:** `internal/agent/permissions.go` (241 lines)

### Permission Policies

| Policy | Behavior | Persistence |
|--------|----------|-------------|
| `PromptAlways` | Always ask user | Never stored |
| `AllowOnce` | Allow this one time | Cleared after execution |
| `AllowSession` | Allow for this session | In-memory, lost on restart |
| `AllowForever` | Allow permanently | Saved to `data/permissions.json` |
| `DenyOnce` | Deny this one time | Cleared after execution |
| `DenyForever` | Deny permanently | Saved to `data/permissions.json` |

### Permission Manager

```go
type PermissionManager struct {
    mu           sync.RWMutex
    sessionPerms map[string]PermissionPolicy  // key: "tool:args_hash"
    permFile     string                       // data/permissions.json
    records      []PermissionRecord
}
```

**Check flow:**

```
Tool call requested
      │
      ▼
Is tool Safe? ──Yes──► Auto-allow (no prompt)
      │
      No
      ▼
Check session perms ──Hit──► Return policy
      │
      Miss
      ▼
Check permanent perms ──Hit──► Return policy
      │
      Miss
      ▼
Return NeedPrompt → frontend must respond
```

### Permission Record (persisted)

```json
{
  "id": "1749052800123456000",
  "tool_name": "run_command",
  "args_hash": "a1b2c3d4e5f6...",
  "policy": "allow_forever",
  "created_at": "2026-06-05T12:00:00Z",
  "updated_at": "2026-06-05T12:00:00Z"
}
```

### UI Flow (when implemented)

```
┌──────────────────────────────────────────┐
│ ⚠️ Tool Execution Request                │
│                                          │
│ 🔧 run_command                           │
│ $ rm -rf /tmp/test                       │
│                                          │
│ ████████████ Dangerous ⚠️                │
│                                          │
│ [Allow Once] [Session] [Forever]         │
│ [Deny]       [Deny Forever]              │
└──────────────────────────────────────────┘
```

> **Note:** The permission dialog frontend UI is implemented (`frontend/lib/screens/agent_screen.dart`, `frontend/lib/widgets/agent/agent_chat_card.dart`) — the backend's `EventPermissionRequest` events are rendered as a real dialog, and agent mode has its own toggle in Chat's top bar (v3.3.3), not just the separate Agent tab.

---

## Code Mode: Plan / Auto / Build sub-modes (new in v4.5.0)

**Files:** `internal/app/code_submode.go` (state + prompts), `internal/agent/tools/plan.go` (`save_code_plan` tool), `internal/app/plan_tool.go` (its backend)

Code Mode used to be a single on/off switch — now it's three presets stored per-session (`Session.CodeSubMode`), cycled with **Ctrl+Tab** in the message box or a tap on the chip in the frontend's bottom engine strip:

| Sub-mode | Behavior | Tool auto-approve set |
|---|---|---|
| **Plan** | Investigates the codebase, writes a step-by-step plan, touches no files | Empty — Plan mode can't edit or run commands at all |
| **Auto** | Today's familiar Code Mode | The original 6 auto-approved tools |
| **Build** | Fast lane — edits and commands run without waiting | Auto's set **plus `run_command`** (still Dangerous-classified; blacklist/protected-path checks in the sandbox still apply) |

Each sub-mode has its own system prompt (`codePlanDirective`/`codingDirective`/`codeBuildDirective`, resolved by `codeSubModeDirective`) — like Memo's main system prompt, each is editable from Settings if the defaults don't fit (`GET/POST /api/code-mode/prompt`, `POST /api/code-mode/prompt/reset`).

### The Plan → Build hand-off

Plan mode's output is written to disk via a **dedicated tool**, `save_code_plan`, rather than the normal `write_file` — `write_file`'s sandbox (`validatePath`) never permits writing outside the project directory, but a plan is deliberately saved outside it at `data/plans/<project-slug>/plan.md`. This keeps the project directory itself untouched during planning.

Once a plan is ready, Memo asks in plain chat whether to proceed to Build or Auto. If the **global auto-permission toggle** is already on when the plan finishes, Memo skips the question entirely and chains straight into Build — **within the same reply**: since Memo's SSE protocol can only carry one `Done:true` per response (`streamSSE`, `handlers_flutter.go`), the auto-chain isn't a second stream — it's a second `RunStreamWithRouter` pass inside the *same* `callAgentStream` goroutine, without ever releasing the lock. `drainAgentStream` returns `(finishReason, chainable)` to make this decision.

> **Known gap:** on a local model + Plan mode + auto-permission (no system-role message on local models — the prompt folds into the user role instead), the chaining path deliberately falls back to the plain-text-ask flow instead of auto-chaining. Verified by code review only — no automated test exists for it since it requires a real local llama.cpp model; not yet live-verified.

---

## Security Sandbox

**File:** `internal/agent/sandbox.go` (137 lines)

### Sandbox Configuration

```go
type SandboxConfig struct {
    BasePath             string        // Project root directory
    MaxCommandTimeout    time.Duration // 60 seconds
    MaxOutputSize        int64         // 10 MB
    MaxToolCallsPerMin   int           // 30
    CommandCooldown      time.Duration // 5 seconds
    ProtectedPaths       []string      // /etc/, /usr/, /boot/, etc.
}
```

### Path Validation

```go
func (s *Sandbox) ValidatePath(path string) error {
    // 1. Resolve symlinks (prevent symlink attacks)
    // 2. Ensure resolved path is within BasePath
    // 3. Reject if path is in protected system directories
    // 4. Reject if path contains ".." traversal
}
```

### Command Blacklist (43 patterns as of this pass — `blacklistedPatterns`, `internal/agent/tools/command.go`; grown a lot since the "23" this doc used to say)

The following patterns are **blocked** in `run_command`:

| Category | Patterns |
|----------|----------|
| **Destructive** | `rm -rf /`, `rm -rf ~`, `rm -rf .` |
| **Disk operations** | `dd`, `mkfs`, `format`, `fdisk`, `parted` |
| **Permission changes** | `chmod 777`, `chown` (on system files) |
| **Privilege escalation** | `sudo`, `su`, `pkexec` |
| **Fork bombs** | `:(){ :\|:& };:`, `forkbomb` |
| **Network** | `nc -e`, `bash -i`, `mkfifo` (reverse shells) |
| **System** | `shutdown`, `reboot`, `halt`, `poweroff` |

### Rate Limiting

- **Global:** Max 30 tool calls per minute
- **Per command:** 5 second cooldown (same command can't be called more than once per 5s)
- **Enforcement:** `RateLimit()` returns error if limits exceeded
- **Cleanup:** `CleanOldState()` goroutine periodically purges stale entries

---

## Agent Pipeline

**File:** `internal/agent/pipeline.go` (226 lines)

### Execution Loop

```
1. Build messages (system + history + user)
2. Add tool definitions to LLM request
3. Call ChatCompletion (non-streaming, temp=0.2)
4. Parse response:
   ├── If tool_calls found:
   │   For each tool call:
   │     a. Check tool exists in registry
   │     b. Rate limit check
   │     c. Permission check
   │     d. If NeedPrompt → emit EventPermissionRequest → wait for user response
   │     e. Execute tool (measure duration)
   │     f. Emit EventToolResult or EventToolError
   │     g. Append tool result as role: "tool" to conversation
   │   Loop to step 3
   └── If no tool_calls:
       └── Emit EventFinalResponse → done
5. Repeat max 40 iterations (safety limit)
```

### Event Types

```go
type AgentEventType string

const (
    EventToolExecuting      AgentEventType = "tool_executing"
    EventToolResult         AgentEventType = "tool_result"
    EventToolError          AgentEventType = "tool_error"
    EventPermissionRequest  AgentEventType = "permission_request"
    EventPermissionDenied   AgentEventType = "permission_denied"
    EventFinalResponse      AgentEventType = "final_response"
)
```

### Event Structure

```go
type AgentEvent struct {
    Type       AgentEventType    `json:"type"`
    RequestID  string            `json:"request_id,omitempty"`
    ToolName   string            `json:"tool_name,omitempty"`
    Args       map[string]interface{} `json:"args,omitempty"`
    Result     string            `json:"result,omitempty"`
    Error      string            `json:"error,omitempty"`
    DangerLevel string           `json:"danger_level,omitempty"`
    DurationMs int64             `json:"duration_ms,omitempty"`
    Content    string            `json:"content,omitempty"`
}
```

---

## Executor

**File:** `internal/agent/executor.go` (171 lines)

The `Executor` is the top-level orchestrator that ties everything together:

```go
type Executor struct {
    registry    *ToolRegistry
    permissions *PermissionManager
    sandbox     *Sandbox
    provider    AgentProvider
    configMgr   *provider.ConfigManager
    mu          sync.Mutex
    pendingPerm map[string]chan PermissionResponse
    logs        []AgentLogEntry
}
```

### Key Methods

| Method | Description |
|--------|-------------|
| `NewExecutor(basePath, router, configMgr)` | Creates executor with default tools, permissions, sandbox |
| `IsAvailable()` | Returns true if external provider router exists |
| `RunStream(ctx, messages, onEvent)` | Starts the agent pipeline with event callback |
| `HandlePermissionResponse(requestID, policy)` | Routes user's allow/deny to waiting pipeline |
| `GetPermissions()` | Returns all permanent permission records |
| `RevokePermission(id)` | Removes a specific permanent permission |
| `ClearPermissions()` | Removes all permanent permissions |

### Audit Log

All tool executions are logged:

```go
type AgentLogEntry struct {
    Timestamp    time.Time
    SessionID    string
    ToolName     string
    Args         map[string]interface{}
    Result       string
    Error        string
    DurationMs   int64
    Permission   string // "allowed", "denied", "auto_allowed"
}
```

- In-memory buffer: last 1000 entries (oldest dropped when full) — nothing currently reads this slice; it's a ready-made source for a future in-app view.
- **Also durably persisted (fixed, BUG-H10):** every entry is additionally appended as one JSON line to `config.DataPath("agent-audit.jsonl")` (`openAuditLogFile()`, `executor.go`) — this file is the actual source of truth across restarts, the in-memory buffer is just a cache. If the file can't be opened (permissions, read-only fs), logging silently falls back to in-memory + `logx` only rather than failing tool execution.

---

## Integration with app.go

**File:** `app.go` (lines 132, 290, 453, 552)

```go
// Struct fields
agentExecutor *agent.Executor
agentEnabled  bool
agentMu       sync.RWMutex

// Initialization
basePath, _ := filepath.Abs(".")
a.agentExecutor = agent.NewExecutor(basePath, a.providerRouter, a.providerCfgMgr)
a.agentEnabled = false

// Routing in SendMessageStream
a.agentMu.RLock()
agentActive := a.agentEnabled
a.agentMu.RUnlock()

if agentActive && a.activeProvider != "" {
    return a.callAgentStream(ctx, messages, userMsg)
}
return a.callLLMStream(ctx, messages, userMsg, "", "")
```

**Bridge implementation** (`app_agent.go`, 48 lines):
- `GetAgentEnabled()` / `SetAgentEnabled()`
- `HandleAgentPermission()`
- `GetAgentPermissions()` / `RevokeAgentPermission()` / `ClearAgentPermissions()`

---

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/agent/enabled` | Returns `{"enabled": bool}` |
| `PUT` | `/api/agent/enabled` | Body: `{"enabled": bool}` |
| `POST` | `/api/agent/permission` | Body: `{"request_id": "...", "policy": "allow_once"}` |
| `GET` | `/api/agent/permissions` | Returns array of `PermissionRecord` |
| `DELETE` | `/api/agent/permissions?=id` | Revoke specific permission |
| `DELETE` | `/api/agent/permissions` | Clear all permissions |

---

## Known Issues & Limitations

| Issue | Detail |
|-------|--------|
| **No streaming** | Pipeline uses non-streaming ChatCompletion for tool calls — blocks UI |
| ~~No per-tool timeout~~ | Fixed — `Pipeline.toolTimeout` (120s default) wraps every tool execution in its own `context.WithTimeout` (`pipeline.go`), on top of the sandbox's own command-level timeout |
| ~~Audit log not persisted~~ | Fixed (BUG-H10) — every entry is appended to `agent-audit.jsonl` on disk, not just the in-memory buffer; see the Audit Log section above |
| **Max 40 iterations** | Hard limit prevents infinite loops but may cut off complex tasks |
| **Tool schema now budgeted against context (v3.3.4)** | Previously agent mode's tool definitions weren't counted against the model's context size, so even a one-word message could fail on a small-context local model with a confusing error. Fixed; default local context also raised 4096 → 8192. |
| **Local models** | llama.cpp's own tool-calling support varies by model; starting a model that doesn't support it now shows a warning instead of failing unexplained later (v3.3.4) |

---

### Linked Notes:
- [[External Providers]] — Required for agent mode to function
- [[Self-Driving Task Loop]] — Builds on this pipeline for unattended multi-step execution
- [[Desktop Mascot]] — Reflects agent/tool activity in real time
- [[Orchestra Mode]] — Alternative multi-model workflow
- [[Architecture]] — System integration
- [[API Documentation]] — Agent endpoint details

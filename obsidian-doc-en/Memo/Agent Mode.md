# 🤖 Agent Mode

> **Package:** `internal/agent/` (19 built-in tools, up from the original 8)
> **Config file:** `data/permissions.json`
> **API endpoints:** `/api/agent/enabled`, `/api/agent/permission`, `/api/agent/permissions`
> **Requires:** Active external provider (local llama.cpp does not support tool calling)

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
│  │     (max 20 iterations)              │ │
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

---

## Built-in Tools

Registered in `internal/agent/tools.go` — 19 tools total, up from the original 8 (`edit_file`/`insert_line`/`delete_lines`/`web_search`/`self_clone`/`configure_provider`/`get_calendar_events` and the 4 WhatsApp tools were added later, see [[WhatsApp Integration]]). On top of these, **skill tools are now real** (v3.3.3): a skill's `SKILL.md` can declare a `command:` field, and it executes through this exact same pipeline and permission UI — see [[Guides]].

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
- DuckDuckGo search; the model is instructed to reach for it only for current events/prices/facts that may have changed since training, not for greetings or general knowledge
- Fixed in v3.3.4: previously ran unconditionally on every message when the (separate, dumber) chat-level web search toggle was on, even with agent mode active — now agent mode's own tool-based decision handles it alone

### 13. `self_clone` — Dangerous
- Copies the entire project (source + binary) to another local directory, for local replication/backup

### 14. `configure_provider` — Dangerous
- Adds/updates an external provider config (type, base URL, API key, model) from within a chat instead of Settings; requires user confirmation

### 15. `get_calendar_events` — Safe
- Reads real events from `events.db` for a date range — the model is instructed to call this instead of guessing when asked about the calendar

### 16-19. WhatsApp tools — `whatsapp_send` (Medium), `whatsapp_search` (Safe), `whatsapp_latest` (Safe), `whatsapp_messages` (Safe)
- Send/search/list-chats/get-history against the paired WhatsApp account — see [[WhatsApp Integration]]

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

### Command Blacklist (23 patterns)

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

- Buffer size: 1000 entries (oldest dropped when full)
- Not persisted to disk (in-memory only)

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
| **No per-tool timeout** | Pipeline doesn't enforce individual timeouts (sandbox does for commands) |
| **Audit log not persisted** | 1000-entry in-memory buffer, lost on restart |
| **Max 40 iterations** | Hard limit prevents infinite loops but may cut off complex tasks |
| **Tool schema now budgeted against context (v3.3.4)** | Previously agent mode's tool definitions weren't counted against the model's context size, so even a one-word message could fail on a small-context local model with a confusing error. Fixed; default local context also raised 4096 → 8192. |
| **Local models** | llama.cpp's own tool-calling support varies by model; starting a model that doesn't support it now shows a warning instead of failing unexplained later (v3.3.4) |

---

### Linked Notes:
- [[External Providers]] — Required for agent mode to function
- [[Orchestra Mode]] — Alternative multi-model workflow
- [[Architecture]] — System integration
- [[API Documentation]] — Agent endpoint details

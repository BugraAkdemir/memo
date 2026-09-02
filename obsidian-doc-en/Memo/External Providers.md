# 🔌 External Providers

> **Package:** `internal/provider/` (10 files, ~1700 lines)
> **Config file:** `data/providers.json`
> **API endpoints:** `/api/providers`, `/api/providers/test`, `/api/providers/active`

Memo supports external LLM APIs alongside its local llama.cpp engine. This enables access to powerful models like GPT-4o, Claude, Gemini, and Grok without requiring a local GPU.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    app.go                               │
│  ┌──────────────────────────────────────────────────┐   │
│  │  callLLMStream()                                  │   │
│  │  1. Orchestra mode?    → orchestra.Conductor      │   │
│  │  2. External provider? → provider.Router          │   │
│  │  3. Local llama.cpp    → api.Client               │   │
│  └──────────────────────────────────────────────────┘   │
│                    │                                     │
│                    ▼                                     │
│  ┌──────────────────────────────────────────────────┐   │
│  │           provider.Router                         │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐         │   │
│  │  │ OpenAI   │ │ Gemini   │ │ Claude   │  ...     │   │
│  │  └──────────┘ └──────────┘ └──────────┘         │   │
│  │  Fallback: auto-disable after 3 failures          │   │
│  │  Health check: re-enable on recovery (5min)      │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

---

## Provider Interface

All providers implement a common interface defined in `internal/provider/provider.go`:

```go
type Provider interface {
    Name() ProviderType
    DisplayName() string
    ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
    ListModels(ctx context.Context) ([]string, error)
}
```

### Key Types

| Type | Description |
|------|-------------|
| `ChatRequest` | Messages, tools, temperature, top_p, max_tokens |
| `ChatResponse` | Content, tool_calls, usage (tokens in/out) |
| `StreamChunk` | Delta content, finish_reason, usage |
| `ToolDefinition` | Name, description, parameters (JSON Schema), danger level |
| `ToolCall` | ID, function name, arguments (JSON) |

---

## Supported Providers (15 registered `ProviderType` values, verified against `internal/provider/provider.go`)

7 detailed below (OpenAI, Gemini, Claude, Grok, Groq, OpenRouter, Ollama), plus: **Custom** (any OpenAI-compatible endpoint — LM Studio, vLLM, etc., wraps `openAIProvider`), **Custom (Anthropic-compatible)** (`custom-anthropic`, added this branch — any Anthropic Messages API-shaped endpoint, e.g. your own proxy; wraps `claudeProvider` the same way `custom` wraps `openAIProvider`, see §10 below), **OpenCode Zen**, **OpenCode Go**, and **Kilo Code** (app.kilo.ai, added v3.9.0 — three live-model-list gateways; Zen and Kilo are pay-as-you-go with some free models sorted to the top and marked with a green checkmark, Go is subscription-based), the two CLI-based providers (#8-9 below, v3.3.4), and `llama.cpp` — an enum placeholder only, **not actually implemented** as a provider (see the note below).

### 1. OpenAI (`openai.go`, 353 lines)
- **Compatible APIs:** OpenAI, any OpenAI-compatible endpoint
- **Auth:** Bearer token in `Authorization` header
- **Implementation:** Full chat completions, SSE streaming, model listing
- **Default models:** GPT-4o, GPT-4o-mini, o1, o3
- **Custom base URL:** Supports LM Studio, local proxies (e.g., `http://127.0.0.1:8046/v1`)

### 2. Google Gemini (`gemini.go`)
- **API Style:** Custom (not OpenAI-compatible)
- **Auth:** API key as query parameter (`?key=...`)
- **Implementation:** `:generateContent` (non-streaming), `:streamGenerateContent?alt=sse` (streaming)
- **Role mapping:** `system`/`developer` → `SystemInstruction`, `assistant` → `model`
- **Default model (`DefaultModels`, `provider.go`):** `gemini-2.0-flash` — a single fallback used when the user hasn't picked one; the actual model list is fetched live, never hardcoded beyond this fallback
- **Tool-calling (fixed this branch, was previously entirely missing):** requests send `tools:[{functionDeclarations:[...]}]`; parallel calls in one turn are merged into a single following `functionResponse` turn per Gemini's role-alternation rules

### 3. Anthropic Claude (`claude.go`)
- **API Style:** Custom (not OpenAI-compatible)
- **Auth:** `x-api-key` header (not Bearer), `anthropic-version: 2023-06-01`
- **Implementation:** POST `/messages` with custom SSE event parsing (`content_block_delta`, `message_stop`, `error`)
- **Role mapping:** `system` → top-level `system` field
- **Default model (`DefaultModels`, `provider.go`):** `claude-sonnet-4-20250514` — a single fallback, live-fetched list otherwise
- **Tool-calling (fixed this branch, was previously entirely missing):** sends `tools:[{name,description,input_schema}]`; assistant `tool_use` blocks and the following `tool_result` blocks are round-tripped correctly, including batching parallel calls into one following "user" turn
- **Extended thinking is requested but not yet surfaced** — see [[Agent Mode]]'s Known Issues / `BUG-THINK1` in the repo's `BUG_REPORT.md`: `thinking:{type:"adaptive"}` is sent whenever an effort level is picked, but the response's `"thinking"` content block is parsed nowhere yet, so it's silently dropped rather than shown

### 4. xAI Grok (`grok.go`, 29 lines)
- **API Style:** OpenAI-compatible (wraps `openAIProvider`)
- **Base URL:** `https://api.x.ai/v1`

### 5. Groq (`groq.go`, 62 lines)
- **API Style:** OpenAI-compatible with custom `ListModels`
- **Base URL:** `https://api.groq.com/openai/v1`
- **Override:** Custom model list endpoint parsing

### 6. OpenRouter (`openrouter.go`, 28 lines)
- **API Style:** OpenAI-compatible (wraps `openAIProvider`)
- **Base URL:** `https://openrouter.ai/api/v1`
- **Value:** Single API for 200+ models

### 7. Ollama (`ollama.go`, 28 lines)
- **API Style:** OpenAI-compatible (wraps `openAIProvider`)
- **Base URL:** `http://localhost:11434/v1`
- **Value:** Local open-source models

> **Note:** llama.cpp is NOT implemented as a provider. Local models are handled separately via `api.Client`. The `NewProvider()` factory returns an error for `ProviderLlamaCPP`.

### 8-9. Claude Code CLI and Codex CLI (`internal/agentcli/`, CLI-based — v3.3.4)

Architecturally unlike the other providers: instead of calling an HTTP API, it shells out to a locally installed command-line tool as a subprocess — `claude -p --output-format stream-json --dangerously-skip-permissions [--resume <id>]` for Claude Code, `codex exec --json --dangerously-bypass-approvals-and-sandbox [-C <dir>] [resume <thread-id>]` for Codex. Implements `provider.Provider` without creating an import cycle — `internal/provider` never imports `internal/agentcli` directly; `agentcli`'s own `init()` (in each of `claude_code.go` and `codex.go` separately) registers itself via `provider.RegisterConstructor` (the `database/sql` driver pattern). Codex's stream-json output differs from Claude Code's: it emits each turn's full text in one piece per `item.completed` event (`type:"agent_message"`) rather than incremental deltas; the session id comes back as `thread_id`, not `session_id`; and its `resume` subcommand (unlike a fresh run) rejects `-C` — it remembers the original session's working directory on its own.

- **Per-chat, not app-wide.** New `sessions.Session` fields `CLIProvider`/`CLISessionIDs`/`CLIWorkdir` — each chat carries its own CLI provider, its own CLI session id, and its own working directory.
- **Independent of `App.streamMu`.** The normal `SendMessageStreamTo` path uses one global stream lock (only one chat can stream at a time); CLI jobs instead lock per-chat via `App.cliJobs` (`map[chatID]context.CancelFunc`) — different chats never block each other.
- **Tied to `a.lifecycleCtx`, not the HTTP request.** `SendCLIMessageStream`'s ctx is derived from `App.lifecycleCtx`, not the request's — the subprocess survives the user switching chats or closing the window, and only a real backend shutdown (`lifecycleCancel`) kills it (cascades through `exec.CommandContext`).
- **`ChatRequest.ResumeSessionID`/`WorkDir` and `StreamChunk.CLISessionID`** — these two fields exist only for CLI providers; the other 7 ignore them.
- New endpoints: `GET /api/cli/status?type=`, `POST /api/chats/cli-provider`, `POST /api/chats/cli-workdir`, `POST /api/send/cli-stream`, `GET /api/cli/running`, `GET /api/cli/commands?type=&chat_id=`.
- **Slash commands** (`internal/agentcli/commands.go`): `ListCommands` reads each CLI's own command directories — `.claude/commands/*.md` + `.claude/skills/*/SKILL.md` for Claude Code, `.codex/prompts/*.md` for Codex — at both project (the chat's working directory) and user (home) level, with project winning a name clash. Descriptions come from the YAML frontmatter `description:`, falling back to the first prose line. Built-ins are deliberately a short curated list: most (`/clear`, `/model`, `/compact`, …) only mutate interactive-session state and mean nothing in Memo's chat.
- **Execution difference (verified against the real binaries, 2026-08-02):** `claude -p "/command"` genuinely runs the command (its init event even returns the full `slash_commands` list) — pass-through is enough. `codex exec "/command"` does **not**; codex only resolves `~/.codex/prompts` in its own TUI, and in exec mode the text reaches the model verbatim, which then improvises. So `CodexCLI.ChatCompletionStream` expands it itself (`ExpandCommand`): frontmatter stripped, `$ARGUMENTS`/`$1..$9` substituted, bare arguments appended when the file declares no placeholder (so the user's words are never silently dropped). An unknown command passes through unchanged.

### 10. Custom (Anthropic-compatible) (`custom_anthropic.go`, new this branch)

A thin wrapper (`customAnthropicProvider{*claudeProvider}`) — same pattern as `grok.go`/`openrouter.go` wrapping `openAIProvider`, but for the Anthropic Messages API shape instead. Exists so a user whose own proxy speaks Anthropic's wire format (not OpenAI's) can still point Memo at it and get the same tool-calling support `claude.go` has, without Memo assuming it's talking to `api.anthropic.com` itself — `BaseURL` is required (`Validate()` rejects an empty one, same rule as plain `custom`). Verified end-to-end (tool send, parse, round-trip) against a local `httptest` server standing in for a real proxy.

---

## Router & Fallback System

**File:** `internal/provider/router.go` (282 lines)

### How it works

```
Request → Router.getActiveEntries() → Try provider 1
                                         │
                                    Success? → Return response
                                         │
                                       No → recordFailure()
                                         │
                                    failCount ≥ 3? → auto-disable provider
                                         │
                                    Try provider 2
                                         │
                                    (repeat until exhausted)
```

### Features

| Feature | Implementation |
|---------|---------------|
| **Ordering** | Providers returned in insertion order (not by `Priority` field) |
| **Auto-disable** | After 3 consecutive failures, provider is skipped automatically |
| **Health check** | Background goroutine (5-minute interval) tests disabled providers and re-enables them on recovery |
| **Error classification** | Rate limiting (429), auth errors (401/403), timeouts — all trigger fallback |
| **Manual control** | `ReenableProvider()`, `ReenableAllProviders()` available via API |

### Auto-Disable Logic

```go
func (r *Router) recordFailure(pt ProviderType) {
    for i, e := range r.entries {
        if e.cfg.Type == pt {
            r.entries[i].failCount++
            if r.entries[i].failCount >= 3 {
                r.entries[i].disabled = true
                log.Printf("Provider %s auto-disabled after 3 failures", pt)
            }
            break
        }
    }
}
```

---

## Encrypted Config Management

**File:** `internal/provider/config.go` (369 lines)

### Storage Format

Provider configs are stored in `data/providers.json`. API keys are encrypted with AES-256-GCM.

```json
{
  "providers": [
    {
      "type": "openai",
      "name": "My OpenAI",
      "api_key": "AES-256-GCM:base64encrypted...",
      "base_url": "https://api.openai.com/v1",
      "model": "gpt-4o",
      "enabled": true,
      "priority": 1,
      "temperature": 0.7,
      "top_p": 1.0,
      "max_tokens": 4096
    }
  ]
}
```

### Encryption Details

| Parameter | Value |
|-----------|-------|
| Algorithm | AES-256-GCM |
| Key derivation | Machine ID (`/etc/machine-id` or persistent UUID fallback) |
| Nonce | 12-byte random per encryption |
| Storage format | `base64(nonce + ciphertext)` |
| Portability | **Not portable** — tied to machine ID |
| Fallback key | Hardcoded constant (if machine-id unavailable) |

### Default Configurations

```go
func defaultConfigs() []ProviderConfig {
    return []ProviderConfig{}
}
```

Used to return 7 disabled placeholder configs (one per built-in provider type) so a fresh install's Providers tab showed every known provider as "Disabled" before the user added anything. Changed in the v3.9.0 UI-fix round: that made the tab cluttered with providers the user would never use, so it now returns empty — only providers the user actually adds appear. A non-destructive frontend filter hides any leftover placeholder rows already on disk for installs from before this change.

---

## Frontend Integration

### Files

| File | Lines | Purpose |
|------|-------|---------|
| `provider_provider.dart` | 88 | Riverpod state (list, active provider) |
| `provider_config.dart` | 125 | Dart model matching Go `ProviderConfig` |
| `provider_config_dialog.dart` | 264 | Add/edit provider dialog |
| `settings_dialog.dart` | (tab) | Provider list with cards |

### Provider Config Dialog

```
┌─────────────────────────────────────┐
│  Add Provider                       │
│                                     │
│  Provider Type: [OpenAI        ▼]   │
│  Display Name: [My OpenAI       ]   │
│  API Key:      [****************]   │
│  Base URL:     [api.openai.com/v1]  │
│  Model:        [gpt-4o         ]   │
│                                     │
│  [✓] Enabled                        │
│                                     │
│  [Test Connection]  [Cancel] [Save] │
└─────────────────────────────────────┘
```

### Provider Card (Settings Tab)

```
┌──────────────────────────────────────┐
│ 🤖 OpenAI                            │
│ Model: GPT-4o                        │
│ Status: ✅ Connected                 │
│         [Configure] [Disable] [Delete]│
└──────────────────────────────────────┘
```

---

## LLM Routing Priority

In `callLLMStream()` (app.go), the routing order is:

1. **Orchestra mode** (if `orchestraConductor.Config().Enabled`) → [[Orchestra Mode]]
2. **External provider** (if `activeProvider != ""` and `providerRouter != nil`) → `provider.Router.ChatCompletionStream`
3. **Local llama.cpp** (fallback) → `api.Client` → local `llama-server`

If no provider is configured and no local model is running, an error is returned.

---

## Known Issues & Limitations

| Issue | Detail |
|-------|--------|
| **No llama.cpp provider** | Local engine handled separately, not through provider interface |
| **Priority field unused** | `Priority` field exists in config but router ignores it |
| **~~No test files~~ (fixed)** | `config_test.go` and `factory_test.go` now cover config storage and provider construction — was accurate through v3.8.x, no longer true as of v3.9.0 |
| **Orchestra bypasses router** | Orchestra creates providers directly, no fallback chain |
| **Machine-bound encryption** | `providers.json` not portable across machines |
| **CLI job cancellation is partial** | `App.streamMu`'s global "one stream at a time" protection doesn't apply to CLI jobs — a separate `cliJobs` lock prevents two messages racing into the same chat, but there's deliberately no cross-chat blocking |

---

### Linked Notes:
- [[Architecture]] — System module map
- [[Orchestra Mode]] — How providers are used in multi-model orchestration
- [[Agent Mode]] — Tool calling with provider models
- [[Developer API Gateway]] — Using these providers from an external Anthropic/OpenAI-compatible tool like Claude Code
- [[API Documentation]] — Provider API endpoints
- [[Backend (Go) Architecture]] — Package structure

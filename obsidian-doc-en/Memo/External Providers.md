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

## Supported Providers (7 types)

### 1. OpenAI (`openai.go`, 353 lines)
- **Compatible APIs:** OpenAI, any OpenAI-compatible endpoint
- **Auth:** Bearer token in `Authorization` header
- **Implementation:** Full chat completions, SSE streaming, model listing
- **Default models:** GPT-4o, GPT-4o-mini, o1, o3
- **Custom base URL:** Supports LM Studio, local proxies (e.g., `http://127.0.0.1:8046/v1`)

### 2. Google Gemini (`gemini.go`, 351 lines)
- **API Style:** Custom (not OpenAI-compatible)
- **Auth:** API key as query parameter (`?key=...`)
- **Implementation:** `:generateContent` (non-streaming), `:streamGenerateContent?alt=sse` (streaming)
- **Role mapping:** `system`/`developer` → `SystemInstruction`, `assistant` → `model`
- **Default models:** Gemini 2.0 Flash, Gemini 2.5 Pro

### 3. Anthropic Claude (`claude.go`, 368 lines)
- **API Style:** Custom (not OpenAI-compatible)
- **Auth:** `x-api-key` header (not Bearer), `anthropic-version: 2023-06-01`
- **Implementation:** POST `/messages` with custom SSE event parsing (`content_block_delta`, `message_stop`, `error`)
- **Role mapping:** `system` → top-level `system` field
- **Default models:** Claude 3.5 Sonnet, Claude 3 Opus, Claude 4 Sonnet

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

### 8. Claude Code CLI (`internal/agentcli/`, CLI-based — v3.3.4)

Architecturally unlike the other 7: instead of calling an HTTP API, it shells out to the locally installed `claude` command-line tool as a subprocess (`claude -p --output-format stream-json --dangerously-skip-permissions [--resume <id>]`). Implements `provider.Provider` without creating an import cycle — `internal/provider` never imports `internal/agentcli` directly; `agentcli`'s own `init()` registers itself via `provider.RegisterConstructor` (the `database/sql` driver pattern).

- **Per-chat, not app-wide.** New `sessions.Session` fields `CLIProvider`/`CLISessionIDs`/`CLIWorkdir` — each chat carries its own CLI provider, its own CLI session id, and its own working directory.
- **Independent of `App.streamMu`.** The normal `SendMessageStreamTo` path uses one global stream lock (only one chat can stream at a time); CLI jobs instead lock per-chat via `App.cliJobs` (`map[chatID]context.CancelFunc`) — different chats never block each other.
- **Tied to `a.lifecycleCtx`, not the HTTP request.** `SendCLIMessageStream`'s ctx is derived from `App.lifecycleCtx`, not the request's — the subprocess survives the user switching chats or closing the window, and only a real backend shutdown (`lifecycleCancel`) kills it (cascades through `exec.CommandContext`).
- **`ChatRequest.ResumeSessionID`/`WorkDir` and `StreamChunk.CLISessionID`** — these two fields exist only for CLI providers; the other 7 ignore them.
- New endpoints: `GET /api/cli/status?type=`, `POST /api/chats/cli-provider`, `POST /api/chats/cli-workdir`, `POST /api/send/cli-stream`, `GET /api/cli/running`.

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
    // Returns 7 disabled configs with sensible defaults:
    // - openai:    gpt-4o
    // - gemini:    gemini-2.0-flash
    // - grok:      grok-2
    // - groq:      mixtral-8x7b-32768
    // - claude:    claude-sonnet-4-20250514
    // - openrouter: openai/gpt-4o
    // - ollama:    llama3
}
```

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
| **No test files** | Zero tests in `internal/provider/` |
| **Orchestra bypasses router** | Orchestra creates providers directly, no fallback chain |
| **Machine-bound encryption** | `providers.json` not portable across machines |
| **No Codex CLI** | Only Claude Code CLI is implemented; Codex support is planned, not built yet |
| **CLI job cancellation is partial** | `App.streamMu`'s global "one stream at a time" protection doesn't apply to CLI jobs — a separate `cliJobs` lock prevents two messages racing into the same chat, but there's deliberately no cross-chat blocking |

---

### Linked Notes:
- [[Architecture]] — System module map
- [[Orchestra Mode]] — How providers are used in multi-model orchestration
- [[Agent Mode]] — Tool calling with provider models
- [[Developer API Gateway]] — Using these providers from an external Anthropic/OpenAI-compatible tool like Claude Code
- [[API Documentation]] — Provider API endpoints
- [[Backend (Go) Architecture]] — Package structure

# 🧩 Developer API Gateway

> **Package:** `internal/anthropicapi/` + `internal/openaiapi/` (wire-format translation), `internal/app/devgateway.go` (routing), `internal/webserver/devgateway_handlers.go` + `openai_handlers.go` (HTTP)
> **Config:** `config.DevGatewayConfig` (`require_api_key`, `use_memory`) — both default off
> **API endpoints:** `GET/PUT /api/dev-gateway/config`, `GET /api/dev-gateway/models`, `GET /api/dev-gateway/logs`, `POST /api/dev-gateway/token/rotate`, `POST /v1/messages` (Anthropic-compatible), `GET /v1/models` + `POST /v1/chat/completions` (OpenAI-compatible)
> **NavRail:** the "Developer" icon in the left sidebar (NOT inside Settings — its own top-level screen)

Makes Memo usable from tools that only speak an Anthropic- or OpenAI-compatible endpoint — most notably **Claude Code itself**, via `ANTHROPIC_BASE_URL`, or any OpenAI-SDK-based tool via a custom base URL. The point: point the tool at Memo, and have it actually run your own local model or your own OpenAI/Gemini/etc. API key behind the scenes.

Since v4.4.0 there are genuinely **two** gateway endpoints sharing the same config, routing, and live log: the original Anthropic-compatible one (`internal/anthropicapi/`, `POST /v1/messages`) and an OpenAI-compatible sibling (`internal/openaiapi/`, `GET /v1/models` + `POST /v1/chat/completions`) for tools that only know OpenAI's wire format.

---

## Why it exists

Tools like Claude Code only speak Anthropic's Messages API format (`POST /v1/messages`) — you can't just point them at a different provider directly. This gateway sits between the two: a request comes in Anthropic-shaped, Memo translates it to its own internal representation, decides which backend (local model or a configured provider) to route to, gets the reply, and translates it back to Anthropic's shape on the way out.

`internal/provider/claude.go` is where Memo talks to the real Anthropic API as a **client** — this package (`internal/anthropicapi`) is the mirror image: Memo itself acts as an Anthropic **server**.

---

## Model selection: `type/model-id`

The request's `"model"` field must be `<type>/<model-id>`:

| Example | What happens |
|---|---|
| `local/qwen2.5` | Routes to whatever local llama.cpp model is loaded (the model-id part is just a label — the real model is whatever's already running) |
| `openai/gpt-4o` | Uses the first **enabled** provider under Settings → API Providers whose type is `openai`, with model set to `gpt-4o` |
| `custom/qwen2.5` | The enabled provider of type `custom` (your own OpenAI-compatible endpoint — LM Studio, vLLM, etc.) |

If more than one provider shares a type, the **enabled** one wins — there's no separate UI to disambiguate further (a deliberate simplification).

`GET /api/dev-gateway/models` lists every currently available `type/model-id` — the Developer screen in the left sidebar shows this as a copyable list — that same screen also has a live request/response log.

---

## Authentication

`DevGateway.RequireAPIKey` **defaults to off** — matching how plain localhost access is already unauthenticated (see `remoteAuthOK`). When turned on:

- The request must carry an `x-api-key` header (what real Anthropic clients — including Claude Code — send automatically) or `Authorization: Bearer <token>` as a fallback.
- The token is the **same one** Remote Access uses (`RemoteAccess.Token`) — shown as copyable in the Developer screen, rotatable via `POST /api/dev-gateway/token/rotate`.
- This check is **independent** of the existing `remoteAuthMiddleware`: that one only kicks in once Memo is bound to `0.0.0.0`; this one applies based purely on `RequireAPIKey`, local or remote — the point is stopping another process on the same machine from using this port without permission.

**v4.5.0 security fix:** v4.4.0 closed the key-enforcement gap for `/v1/messages` (the Anthropic-compatible endpoint); this release closed the same gap for the OpenAI-compatible pair (`GET /v1/models`, `POST /v1/chat/completions`) — with "Require API Key" left off (its default) and remote access on, either endpoint could previously be reached by anything else able to reach the port, with no credential at all, using the configured provider for free. Both endpoints now enforce the key for any caller that isn't genuinely on this machine.

---

## Memory integration

`DevGateway.UseMemory` **defaults to off**. When turned on:

- The request gets enriched with a block from Memo's RAG memory (built from the user's last message via `retrieveMemory` + `memory.FormatMemoriesForPrompt`) — the rest of the external tool's own system prompt (persona, capability announcements) is **left untouched**, only remembered facts are added.
- The turn is saved to memory afterward via `saveMemoryAsync`.
- **But it never creates a visible chat session** — a coding conversation through Claude Code never shows up in Memo's own chat history. This is a deliberate design decision so gateway traffic and real chats never get mixed up.

When off (default): requests are completely isolated, memory is never touched.

---

## Tool use — full agentic support

Claude Code's actual power — tool calling (reading/writing files, running commands) — **works**: the request's `"tools"` field, Anthropic's `tool_use`/`tool_result` blocks, and multi-turn continuation (sending a tool's result back in the next message) are all translated.

**How it works:**
- Anthropic's `input_schema` already is the JSON Schema OpenAI's `parameters` field expects — a near-direct mapping.
- A prior assistant `tool_use` block becomes OpenAI's `tool_calls`; a user `tool_result` block becomes its own standalone `role: "tool"` message (OpenAI expects tool results as separate messages, not nested in a user turn).
- **A subtle but critical format detail:** Anthropic's `tool_use.input` is a real JSON object, while OpenAI's `function.arguments` is a JSON *string* carrying that object's text — conflating the two (e.g. passing the same bytes straight through) either double-encodes or produces the exact bug found and fixed via a live end-to-end test: Claude Code would receive a plain string in `input` instead of an object. `anthropicInputToOpenAIArguments`/`openAIArgumentsToJSONText` are the exact inverse of each other and handle this correctly.
- Tool-calling requests always go to the backend **non-streaming** (`DevGatewayChat`) — Memo's own agent pipeline (`internal/agent/pipeline.go`) already only ever decides tool calls via non-streaming `ChatCompletion`; no provider's streaming path decodes `tool_calls` deltas at all. If the client asked for streaming, the complete response is replayed as Anthropic's SSE event sequence in one shot.

**Known limitation:** `ollama` doesn't support tool calling yet — its `internal/provider` implementation doesn't decode/encode Tools/ToolCalls at all (a pre-existing gap, unrelated to the gateway itself). A tools-bearing request routed to it gets a clear error instead of silently dropping the tools. (`gemini` and `claude` **used to** be in this list too — both gained real tool-calling in v4.4.0.)

Token counts are also **estimates** (word-count based), not the real provider-reported numbers — the same approach the rest of the codebase's live counter already uses.

---

## Live Log

The Developer screen has a live list of every request passing through the gateway — so when wiring up Claude Code for the first time, "is this actually working, what did it send, what came back" has an answer without reading backend logs. Each row: time, resolved model, `stream`/`tools` badges, duration (ms), and a truncated request/response preview or error message.

- A separate, 200-entry in-memory buffer (`internal/app/devgatewaylog.go`) — independent of the app-wide shared event ring (64 slots, used at much higher frequency) so a busy chat session elsewhere can't evict the log entries you're actively watching.
- Auto-refreshes every 2 seconds while the screen is open; stops refreshing when you switch tabs (so it doesn't poll needlessly in the background).
- Resets on app restart — not persisted, purely for live observation.

---

## Related pages

- [[External Providers]] — the provider system the gateway routes into
- [[RAG and Semantic Memory]] — the system the memory integration relies on
- [[API Documentation]] — every REST endpoint

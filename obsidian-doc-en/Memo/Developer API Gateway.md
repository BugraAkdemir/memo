# 🧩 Developer API Gateway

> **Package:** `internal/anthropicapi/` (wire-format translation), `internal/app/devgateway.go` (routing), `internal/webserver/devgateway_handlers.go` (HTTP)
> **Config:** `config.DevGatewayConfig` (`require_api_key`, `use_memory`) — both default off
> **API endpoints:** `GET/PUT /api/dev-gateway/config`, `GET /api/dev-gateway/models`, `POST /v1/messages`
> **Settings tab:** Settings → Developer

Makes Memo usable from tools that only speak an Anthropic-compatible endpoint — most notably **Claude Code itself**, via `ANTHROPIC_BASE_URL`. The point: point Claude Code at Memo, and have it actually run your own local model or your own OpenAI/Gemini/etc. API key behind the scenes.

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

`GET /api/dev-gateway/models` lists every currently available `type/model-id` — the Settings → Developer tab shows this as a copyable list.

---

## Authentication

`DevGateway.RequireAPIKey` **defaults to off** — matching how plain localhost access is already unauthenticated (see `remoteAuthOK`). When turned on:

- The request must carry an `x-api-key` header (what real Anthropic clients — including Claude Code — send automatically) or `Authorization: Bearer <token>` as a fallback.
- The token is the **same one** Remote Access uses (`RemoteAccess.Token`) — shown as copyable in the Settings → Developer tab.
- This check is **independent** of the existing `remoteAuthMiddleware`: that one only kicks in once Memo is bound to `0.0.0.0`; this one applies based purely on `RequireAPIKey`, local or remote — the point is stopping another process on the same machine from using this port without permission.

---

## Memory integration

`DevGateway.UseMemory` **defaults to off**. When turned on:

- The request gets enriched with a block from Memo's RAG memory (built from the user's last message via `retrieveMemory` + `memory.FormatMemoriesForPrompt`) — the rest of the external tool's own system prompt (persona, capability announcements) is **left untouched**, only remembered facts are added.
- The turn is saved to memory afterward via `saveMemoryAsync`.
- **But it never creates a visible chat session** — a coding conversation through Claude Code never shows up in Memo's own chat history. This is a deliberate design decision so gateway traffic and real chats never get mixed up.

When off (default): requests are completely isolated, memory is never touched.

---

## Known limitation (v1)

Only **text** content blocks are translated. Anthropic's `tool_use`/`tool_result` blocks (and the request's `"tools"` field) are **not** — a request relying on Claude Code's tool-calling gets a plain text reply without the backend ever seeing the tool definitions. Full bidirectional translation between Anthropic's tool format and other providers' OpenAI-style function-calling format is a separate, much bigger piece of work — deliberately out of scope for v1.

Token counts are also **estimates** (word-count based), not the real provider-reported numbers — the same approach the rest of the codebase's live counter already uses.

---

## Related pages

- [[External Providers]] — the provider system the gateway routes into
- [[RAG and Semantic Memory]] — the system the memory integration relies on
- [[API Documentation]] — every REST endpoint

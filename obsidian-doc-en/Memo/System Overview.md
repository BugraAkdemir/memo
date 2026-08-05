# 📊 System Overview

End-to-end data flow and component interactions of Memo.

## Data Flow Diagram
When a user sends a message, the following happens in the background:

1. **Frontend:** Transmits the message to the Backend via the REST API (`/api/send/stream`).
2. **Backend (AppGo):**
    - Receives the message.
    - **Memory Module:** Vectorizes the message and finds the 5 relevant past memories.
    - **Identity Module:** Prepares the system prompt + memories + history + new message.
3. **LLM Routing (priority order):**
    - **Orchestra Mode** (if enabled) → multi-model workflow (chief plans, experts execute, chief synthesizes)
    - **External Provider** (if active) → `provider.Router` with fallback chain across configured providers
    - **Local llama.cpp** → `api.Client` pointed at local `llama-server`
4. **Streaming:** The response flows back to the Frontend token-by-token as it is generated.
5. **Persistence:** When the response is complete, both the message and response are permanently written to memory.
6. **Agent Mode** (optional): When enabled, the LLM can call tools (read_file, run_command, etc.) with user permission.
7. **Reliability (v3.3.4):** Every background task (memory saving, routines, WhatsApp, cloud sync, STT, notifications, tunnels, ...) now runs under panic recovery — an unexpected error in one is logged and contained instead of crashing the whole backend.

## Core Metrics
- **Latency:** The time until the first token arrives.
- **Throughput:** Words generated per second (Tokens per second).
- **Context Window:** How much information the model can "keep in mind" at once.

## Beyond v3.0.0: What Else Runs in the Background

The picture above is the core chat loop. On top of it, several always-on or on-demand subsystems now exist:

- **Routines** (`internal/routine/`) — scheduled automations, fire against the device's own timezone, either a simple prompt or a full agent run
- **Proactive Engine** (`internal/proactive/`, `internal/observer/`) — observes usage patterns and surfaces ambient nudges (a real suggestion banner on desktop)
- **Live Mode** (Beta) — hands-free voice conversation: local STT in, local Piper TTS (or an optional external provider) out, one-directional barge-in
- **Memo Swarm** (Beta) — pools several machines' compute for one model too large for any single one (llama.cpp RPC)
- **Developer API Gateway** — an Anthropic-compatible `/v1/messages` endpoint so tools like Claude Code can use Memo as their backend, with full agentic tool calling
- **Claude Code CLI / Codex CLI providers** (Beta) — a chat can instead shell out to a real, locally-installed coding agent CLI as a background job

### Linked Notes:
- [[Architecture]]
- [[RAG and Semantic Memory]]
- [[Llama.cpp Integration]]
- [[Proactive Learning and Calendar]]
- [[Memo Swarm]]
- [[Developer API Gateway]]

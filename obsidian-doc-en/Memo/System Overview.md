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

## Core Metrics
- **Latency:** The time until the first token arrives.
- **Throughput:** Words generated per second (Tokens per second).
- **Context Window:** How much information the model can "keep in mind" at once.

## New in v3.0.0
- **External Providers:** Connect to OpenAI, Gemini, Claude, Grok, etc.
- **Agent Mode:** LLM can read/write files, run commands (with permission).
- **Orchestra Mode:** Multiple models collaborate as a team.

### Linked Notes:
- [[Architecture]]
- [[RAG and Semantic Memory]]
- [[Llama.cpp Integration]]

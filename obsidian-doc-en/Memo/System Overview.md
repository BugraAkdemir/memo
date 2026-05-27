# 📊 System Overview

End-to-end data flow and component interactions of Memo.

## Data Flow Diagram
When a user sends a message, the following happens in the background:

1. **Frontend:** Transmits the message to the Backend via the REST API (`/api/send`).
2. **Backend (AppGo):**
    - Receives the message.
    - **Memory Module:** Vectorizes the message and finds the 5 relevant past memories.
    - **Identity Module:** Prepares a massive package with the system prompt + memories + history + new message.
3. **Llama Server:** Processes the package and begins generating the response.
4. **Streaming:** The response flows back to the Frontend token-by-token as it is generated.
5. **Persistence:** When the response is complete, both the message and response are permanently written to memory.

## Core Metrics
- **Latency:** The time until the first token arrives.
- **Throughput:** Words generated per second (Tokens per second).
- **Context Window:** How much information the model can "keep in mind" at once.

### Linked Notes:
- [[Architecture]]
- [[RAG and Semantic Memory]]
- [[Llama.cpp Integration]]

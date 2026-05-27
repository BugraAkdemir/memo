# 🕵️ Incognito Mode

Memo offers a dedicated "Incognito Mode" to ensure complete privacy when working on sensitive topics.

## Zero-Persistence
When Incognito Mode is activated:
1. **Memory Recording is Stopped:** Conversations are not saved to the semantic memory (`data/memory/`).
2. **Session History is Not Written:** Chat history is not saved to the disk and is deleted when the application is closed.
3. **Volatile Context:** Context lives only within that specific session, in RAM.

## Use Cases
- When working on code blocks containing passwords or secret keys.
- When conducting temporary research.
- In random chats where you don't want to clutter the assistant's permanent memory.

## How to Activate
It can be activated by clicking the "Eye" icon in the chat interface or by turning on the "Incognito Mode" option in settings. A clear visual warning appears in the interface when active.

### Linked Notes:
- [[RAG and Semantic Memory]]
- [[Data Layer and Persistence]]

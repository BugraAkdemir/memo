# 🧠 RAG and Semantic Memory

The heart of Memo is the **Retrieval-Augmented Generation (RAG)** mechanism, which turns every interaction into a permanent "memory."

## Contextual Resonance
Memo doesn't just read what you write; it indexes it semantically. This allows the AI to not just talk to you, but to remember your way of thinking, past decisions, and private information.

## How It Works
1. **Embedding:** The user's message is converted into a numerical vector by a local embedding model.
2. **Semantic Search:** This vector is compared against past memories in the local memory directory (Cosine Similarity).
3. **Context Construction:** The most relevant memories (Top-K) are retrieved and secretly placed inside the prompt sent to the LLM.
4. **Response Generation:** The LLM generates a response that "knows you," drawing from these past memories.
5. **Persistence:** The generated response and user message are asynchronously added to the memory as new memories.

## Technical Specifications
- **Binary-Atomic Persistence:** Data is stored in SQLite/vec0 format.
- **RAM Indexing:** Vectors are cached in RAM for search performance.
- **Lazy Loading:** Only matching details are read from the disk after a search, keeping RAM usage minimal.

### Linked Notes:
- [[Vector Search Logic]]
- [[Data Layer and Persistence]]
- [[Advanced Settings]] (Top-K and Threshold values)

# 🧠 RAG and Semantic Memory

The heart of Memo is the **Retrieval-Augmented Generation (RAG)** mechanism, which turns every interaction into a permanent "memory." This is no longer a pure vector-similarity system — it's now a **hybrid (vector + keyword) search**, compound-question splitting, and a separate guarantee layer for important facts (see [[Data Layer and Persistence]]).

## Contextual Resonance
Memo doesn't just read what you write; it indexes it semantically. This allows the AI to not just talk to you, but to remember your way of thinking, past decisions, and private information.

## How It Works

1. **Embedding:** The user's message is converted into a numerical vector by a local embedding model.
2. **Compound-question splitting:** The query is split on conjunctions ("and"/"ve"/"ile"/","), so each topic gets its own vector search instead of being blended into one averaged vector (`splitCompoundQuery`, `internal/memory/store.go`).
3. **Hybrid search:** For the whole query and each split segment, both vector similarity and FTS5 keyword search run, and the results are merged via Reciprocal Rank Fusion (RRF).
4. **Importance weighting:** Results are re-ranked by their `importance` field — durable/explicit facts get extra weight.
5. **Pinned facts:** On top of all this, facts tagged `source='explicit'` (name, birthday, pets, etc.) are injected into every prompt **unconditionally**, bypassing search/ranking entirely — see the "Pinned Facts" section in [[Data Layer and Persistence]].
6. **Context construction:** The combination of the above is placed inside the prompt sent to the LLM.
7. **Response generation:** The LLM generates a response that "knows you," drawing from both retrieved memories and pinned facts.
8. **Persistence:** The generated response and user message are asynchronously added to memory as a new entry. Additionally (as of 2026-07-15), a narrow background check runs after each turn: if the message contains a durable personal fact, it's automatically saved a second time into the pinned-facts set — previously only possible via the explicit `/remember` command.

## Why Hybrid (Not Pure Vector)?

Pure vector similarity blends every topic of a multi-part question ("do you know my name, birthday, and favorite color") into one averaged vector — as the memory store grows, a short, precise fact (like a favorite color) can end up not similar enough to that blended vector and get lost. FTS5 keyword search acts as a safety net against this, and the pinned-facts layer gives core information a guarantee that doesn't depend on search at all.

## Technical Specifications
- **Persistence:** Data is stored in a single SQLite file (`data/memory/memory.db`).
- **Disk-based search:** Every search queries SQLite directly — there's no separate in-RAM vector cache (see [[Data Layer and Persistence]]).

### Linked Notes:
- [[Vector Search Logic]]
- [[Data Layer and Persistence]]
- [[Advanced Settings]] (Top-K and Threshold values)

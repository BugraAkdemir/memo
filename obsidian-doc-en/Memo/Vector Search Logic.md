# 🔍 Vector Search Logic

Memo's "remembering" capability is now based on a **hybrid** search: vector (semantic) similarity + FTS5 (keyword) search + a fusion of the two (RRF).

## Cosine Similarity
Cosine Similarity is used for semantic search.
- The cosine of the angle between two vectors is calculated.
- The closer the result is to 1, the more semantically relevant the two texts are.

## Vector Search Flow

1. **Query:** User asks "What did we say in yesterday's meeting?"
2. **Embedding:** This sentence is converted into a vector.
3. **Search:** If `sqlite-vec` (`vec0`) is available, a k-nearest-neighbor query goes straight to SQLite via the virtual table's own ANN index. Otherwise, a Go-side fallback reads every embedding and computes cosine similarity directly (`goSearch`) — fast enough for small/medium memory stores, but not as fast as `vec0`'s ANN index at scale.
4. **Candidate pool:** Up to 5× `topK` (min 20, max 100) candidates are pulled — not just the final result count.

## Keyword (FTS5) Search

Running alongside the vector search, when FTS5 is available:
- The query's words are joined with `OR` (`escapeFTSQuery`) and matched against bm25 ranking.
- Using `OR` matters: joining words with a plain space makes FTS5 treat it as `AND`, and a multi-topic question ("do you know my name and birthday...") would then never match any row exactly. `OR` makes each word an independent candidate; bm25 already downweights common words automatically.

## Compound-Question Splitting

A long/multi-topic question is split into separate segments on conjunctions ("and"/"ve"/"ile"/",") (`splitCompoundQuery`), and each segment gets its own full-budget vector search. This stops a single blended vector from diluting away one specific topic.

## Fusion: Reciprocal Rank Fusion (RRF)

Vector results and FTS5 results are merged with an RRF formula that boosts a record's score if it appears in both lists (`reciprocalRankFusion`). A record found by both is marked `MatchType="hybrid"`.

## Importance Weighting and Pinned Facts

- The merged results are re-weighted and re-sorted by their `importance` field.
- Completely separate from all of the above, facts tagged `source='explicit'` ("pinned facts," see [[Data Layer and Persistence]]) are added without going through any search/ranking at all — these steps only apply to the general memory pool, not to pinned facts.

### Linked Notes:
- [[RAG and Semantic Memory]]
- [[Data Layer and Persistence]]
- [[Advanced Settings]]

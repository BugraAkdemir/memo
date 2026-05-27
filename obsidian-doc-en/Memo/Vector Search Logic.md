# 🔍 Vector Search Logic

Memo's "remembering" capability is based on advanced mathematical vector searches.

## Cosine Similarity
Cosine Similarity, the most common and effective method for semantic search, is used.
- The cosine of the angle between two vectors is calculated.
- The closer the result is to 1, the more semantically relevant the two texts are.

## Search Flow
1. **Query:** User asks "What did we say in yesterday's meeting?".
2. **Embedding:** This sentence is converted into a vector, for example, `[0.12, -0.55, 0.89, ...]`.
3. **RAM Scan:** This vector is compared against thousands of past memory vectors kept in RAM.
4. **Ranking:** Memories are ranked based on their similarity score.
5. **Threshold:** Memories below the determined minimum similarity score (e.g., 0.70) are filtered out.
6. **Top-K:** The first `K` (e.g., 5) memories with the highest scores are selected.

## Optimization: Worker Pool
To speed up the scanning process as the memory grows significantly:
- Vector comparisons are split across CPU cores.
- Thanks to Go's `goroutine` structure, thousands of memories are scanned within milliseconds.

### Linked Notes:
- [[RAG and Semantic Memory]]
- [[Advanced Settings]]

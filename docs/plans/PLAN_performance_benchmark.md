# Three-stage longevity benchmark

- [ ] Build and verify a reproducible, isolated fixture generator and bounded runner using real Memo startup/API/retrieval code, a deterministic embedding/LLM fixture, and existing Gemini subscription for answer evaluation.
- [ ] Execute sequential 1/2/3-year stages, collect raw timings/token/quality/UI evidence, and extend docs/PERFORMANCE_REVIEW.md with results and limitations.

No CPU busy loops, concurrent load generators, global cache eviction, or live credential exports. Fixture root: data/performance-benchmark (existing data outside it is preserved). All model calls are serial and bounded. Process-cold results are not OS-cache-cold or Windows/macOS launch measurements. Browser frame cadence is not native Flutter GPU frame timing.

Paused at user request: see docs/PERFORMANCE_BENCHMARK_CHECKPOINT.md. Harness checkpoint 36ba732a exists; corrected final stage 1 is saved, stage 2 was interrupted. Final matrix/report and full verification remain pending.

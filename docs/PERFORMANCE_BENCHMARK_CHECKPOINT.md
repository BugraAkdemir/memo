# Longevity benchmark — paused 29 October 2026

The user explicitly requested a pause and a durable resume point. This is a
checkpoint, **not a finished benchmark report**. No product optimization was
implemented. Branch: `codex/performance-longevity`. Initial harness commit:
`36ba732a`; follow-up tooling fixes are saved with this checkpoint.

## Authorization and scope

The user explicitly authorized replacing the repository's `data/` test contents
and using the existing Gemini subscription. We replaced sessions/memory/profile/
stats/calendar/providers/Telegram test state directly under `data/`. Subscription
credentials and existing model weights were retained. Clear fake GGUF placeholders
were added; these are not runnable models. No CPU busy loops or load workers.

## Where work stopped

- An exploratory pass completed all three nominal 365/730/1095-day stages.
- The corrected final pass completed **stage 1** and stopped during stage 2 at the
  user's request, after its Cedar Gemini answer. Stage-2 cold/API/fake measurements
  had run but were still in memory; they are not in the final results array.
- Current `data/` is stage 2: **7,303 session files, 73,015 memory rows, 730,006
  observations, 146,008 usage rows and 7,300 calendar events**. Small excesses over
  the seeded counts are actual benchmark turns/background records.
- Final `data/performance-benchmark/results.json` contains **stage 1 only**.
- `PAUSED.json` records the machine-readable state. The final run log is copied
  to `paused-final-run.log`; no required evidence depends on `/tmp` logs.
- All benchmark processes and the subscription manager were stopped. Listeners
  on 18080/18082/18090 were confirmed absent. Resume must restart the sidecar.

The final stage-1 process-to-API median was **457 ms** (3 process-cold launches;
50 ms readiness polling, no OS cache eviction). Post-turn backend RSS was about
390 MiB. Opening the three selected chats made **zero provider calls**. Measured
first visible fake text arrived after about 153/2695/2731 ms for empty/100-message/
5000-message chats. These are fixture measurements on Linux, not Windows/macOS
cold starts and not real local inference.

## Artifacts and interpretation

`data/performance-benchmark/` is locally excluded from git and intentionally left
on disk. It contains downloaded UltraChat seed/source metadata, raw prompts,
replies, timings, manifests, logs and screenshots. Never commit credentials or
`data/image.key`. Existing dirty AGENTS.md, .claude-flow files and
`data/telemetry_state.json` belong to the user and were not staged.

- `results.json`, stage-1 manifest/log/UI/supplement files: corrected stage 1.
- `exploratory/results.json`: completed exploratory stages 1–3, NOT final.
- `exploratory/quality.json`: three project-scoped Gemini evaluations, each 4/4
  (Cedar code, current Atlas region, Orchid currency, unknown-password abstention).
- `exploratory/stale-artifacts-on-pause/`: stale root stage-2 UI/RAG and stage-3
  files moved aside to prevent accidental use as final-pass evidence.
- `stage-1-ui.json`: response persisted; Canvas text requires screenshot review.
  Browser RAF cadence is a responsiveness proxy, not native rendered FPS.

Exploratory values (provisional): process-to-API medians 405/759/1112 ms;
post-turn RSS 404/724/946 MiB. Do not mix these with corrected final-pass rows.
The final third horizon was increased to **1096 days** to cover at least three
calendar years including the leap day; it has not yet been executed.

## Findings already supported

1. Startup/RAM grew with full-history loading. Long-history first-send prompt
   estimates stayed near 8k rather than including all archived messages. Fake
   usage is a character heuristic; real Gemini usage is stored separately.
2. A synthetic prior message with repeated commas/“and” produced about 35–37
   embedding calls per turn. `buildMemoryQuery` includes prior user text;
   `splitCompoundQuery` splits it repeatedly. Large-stage retrieval reached its
   5 s budget. No optimization has been applied.
3. The Python HTTP fixture's reused connections showed roughly 40 ms delays on
   some requests, while fresh embedding requests were ~0.6–8 ms. Calibrate this
   (TCP_NODELAY/control probes) before attributing absolute RAG times to Memo or
   real embeddings. Do not silently subtract an assumed overhead.
4. Ordinary chats intentionally read all project pools. Identical Orchid facts
   with conflicting currencies across pools made Gemini choose JPY instead of
   the intended CHF. This deliberately ambiguous normal-chat case is not proof
   of a project isolation defect. Standalone project-scoped retrieval/evaluation
   returned CHF and scored 4/4 on the tiny suite in all exploratory stages.
5. Initial SSE parsing counted `memory_used` content as first text. Fixed in the
   final runner and guarded by two protocol tests; exploratory TTFT/text is not
   usable uncorrected.
6. Initial UI measurements hit auth/engine dialogs or could not see Markdown in
   the accessibility DOM. Canvas replies were visible in reviewed debug images.
   The script now uses real UI controls, checks persisted response and saves a
   screenshot for visual review. Reject dialog/idle cadence as streaming FPS.

## Exact resume order

1. Read this checkpoint, `PAUSED.json`, PLAN_performance_benchmark.md and
   scripts/performance/README.md. Check branch/status and preserve user changes.
   **Do not blindly run the current full runner**: it resets stage 1 and overwrites
   the accepted result. First add a bounded `--resume-stage` / per-section save
   capability, or preserve the accepted artifacts and deliberately re-run after
   fixing the calibration issue. This implementation is pending.
2. Calibrate persistent fixture transport. Decide and record whether to retain
   a controlled delay or set TCP_NODELAY, then keep the same setting across all
   compared stages. If changed, remeasure all three performance stages; retain
   existing Gemini evidence but do not label it a measurement of a new setup.
3. Restart the checksum-verified subscription manager from repository CWD:
   `/tmp/memo-perf -mode subscriptions -data "$PWD/data/cliproxy"`.
   The isolated app adapter is `/tmp/memo-performance-run/memo-perf`; its bundle
   contains only vec0, CWD is `data/`. Source and rebuild command are in README.
   This separation avoids auto-starting another authenticated sidecar.
4. Finish corrected stages 2 and 3 (or all three after calibration), one process
   at a time. Use the current real Gemini model list; the tested model was
   `gemini-3.8-flash-high`. Run quality.py only AFTER the main runner, serially.
   Do not repeat real calls merely to get a better score.
5. Visually review all three screenshots and annotate evidence; report RAF
   cadence explicitly as web/headless responsiveness. Native Flutter FPS and
   Windows/macOS measurements remain unavailable. Review chat-open call counts,
   first visible TTFT, mean real token usage, scoped recall and unknown abstention.
6. Export a reviewed compact result artifact, extend PERFORMANCE_REVIEW.md,
   mark the two plan items, run outstanding verification, update handoff, commit.

## Verification so far

PASS: adapter build with `GOMAXPROCS=2 CGO_ENABLED=1 go build -p 1 -tags
sqlite_fts5`; targeted `go vet` for cmd/memo-perf; two Python protocol tests;
Python compile; node syntax check; Flutter web release build; SQLite integrity
and memory/vec/FTS row-count checks in completed seeds; git diff --check.
Full Go build/vet/race suite and Flutter analyze/test were **not run** in this
benchmark session. Run them with bounded resources after measurements so they
do not contaminate timings. No PR/push/merge for this benchmark yet.

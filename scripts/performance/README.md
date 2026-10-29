# Opt-in longevity benchmark

This replaces the designated **test** stores under the repository's `data/`:
sessions, memory, profile, stats, calendar, providers and Telegram configuration.
It keeps subscription authentication and existing model weights. Fake GGUF files
are clearly labelled placeholders and must never be selected for real inference.
Do not run against a live installation or concurrently with Memo.

The adapter invokes real App startup, REST/SSE handlers, prompt assembly and
memory retrieval. It disables unrelated background model calls and external
connectors. A lexical SHA-256 embedding fixture is deterministic; it does not
measure real embedding model quality, GPU inference, Windows launchers or native
Flutter FPS. Browser RAF cadence is a responsiveness proxy only, and is valid
only after the response screenshot has been visually reviewed. The script records
`response_persisted`; Flutter Canvas text is not reliably exposed to DOM text queries.

Build with bounded parallelism (no load workers or cache eviction):

```bash
mkdir -p /tmp/memo-performance-run/binaries/linux/cpu
ln -sfn "$PWD/binaries/linux/cpu/vec0.so" /tmp/memo-performance-run/binaries/linux/cpu/vec0.so
GOMAXPROCS=2 CGO_ENABLED=1 go build -p 1 -tags sqlite_fts5 -o /tmp/memo-performance-run/memo-perf ./cmd/memo-perf
```

Only the vec library belongs beside the isolated adapter. Do not put the full
engine/sidecar tree there: the benchmark must not launch the user's engines.
The adapter runs with CWD `data/`, so repository `.env` is not loaded.
The HTTP fixture and the adapter's unused engine process ports are deliberately
different: App shutdown can otherwise stop the fixture's owning process.
The runner checks ports 18080/18082/18083/18084/18090 before mutating data.

Start the authenticated sidecar separately through the checksum-verifying
manager, with CWD at the repository root and the adapter outside its bundle:

```bash
/tmp/memo-performance-run/memo-perf -mode subscriptions -data "$PWD/data/cliproxy"
```

Choose an actually listed Gemini model. Never print/copy `state.json` API keys or
OAuth credentials. The initial run used `gemini-3.8-flash-high` from the live list.
Stop that manager with SIGTERM after all quality calls are complete.

Build the Flutter web release, then run:

```bash
(cd frontend && /home/bugra/Documents/flutter/bin/flutter build web --release --no-pub)
python3 scripts/performance/longevity.py --replace-test-data \
  --browser-command 'python3 scripts/performance/probe.py && node scripts/performance/browser.cjs'
python3 scripts/performance/quality.py
```

Resume and fixture transport: `--start-stage N` re-runs only stages N..3 and keeps
the accepted earlier entries of `results.json` (it refuses if they used a different
`--fixture-nodelay`). Files of re-run stages are first copied to
`data/performance-benchmark/superseded/<timestamp>/`. Each section of a stage is
saved to `stage-N-partial.json`; that file is evidence only, an interrupted stage is
re-run from its seed. The HTTP fixture sets `TCP_NODELAY` by default (`--fixture-nodelay
on`). Calibration on 29 Oct 2026 (40 keep-alive embedding POSTs, fixture alone, curl):
median 0.25-0.36 ms with it, 40.9 ms without it (Nagle + delayed ACK between the
header and body writes). Rows measured without it, including the first stage 1,
inflate every retrieval/embedding-heavy number by roughly 41 ms per call.
Do not mix the two settings in one comparison.

The runner fetches 100 dialogues from HuggingFaceH4/ultrachat_200k (MIT) if absent.
The downloaded seed and source metadata live under data/performance-benchmark/.
Each stage is regenerated from the same seed with nested record IDs and dates
rebased to its 365/730/1096-day horizon. This prevents benchmark-created chat memories
and answers leaking into the next stage. Needle facts have fixed timestamps.
Three process-cold starts and 15 sequential samples per API are collected.
There is no OS cache eviction. Real answer calls are capped at 12 in the main
runner; the separate scoped quality test makes three additional real requests.
Do not run the quality test concurrently with the main runner.

`browser.cjs` needs Playwright and Chromium. Override `BENCH_NODE_ROOT` (a Node
module root) and `BENCH_CHROMIUM` for other installations. It performs local-only
auth setup and skips engine installation using actual UI controls. It checks a persisted response and saves a screenshot for visual verification
before the result can be used. The scripts neither install
engines nor pretend a hidden dialog is a streamed answer.

Raw artifacts include prompts, replies, stage manifests, logs, UI screenshots,
all timing samples and protocol events. Keep `data/performance-benchmark/` and
`data/image.key` out of git. Commit only the reviewed compact report/results.
Synthetic historical usage counters are **not** measured token consumption.
Fake reported usage uses a character heuristic; Gemini's actual usage is kept
separately. GET context estimates, first-send prompt size and provider-reported
usage are separate metrics. `memory_used`/heartbeat/tool SSE metadata is never a
visible first token (`test_protocol.py` guards this).

The main real chat intentionally includes conflicting facts across scopes;
ordinary chats read all scopes by design, so ambiguity is not a project leak.
`quality.py` replays the exact project-scoped retrieval outputs to Gemini with
independent questions and no answers in the prompt, then checks the three known
values and one abstention. This is a tiny grounded-recall suite, not a general
LLM quality benchmark.

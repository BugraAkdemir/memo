# Task worker efficiency — 2026-10-24

Evidence: user-supplied backend.log (20:48:51–20:58:39, UTC+03). The file is
not committed because it contains local paths, account identity and user text.
The supplied screenshot shows Task.md progress, not a task duration counter.

## What the log proves

The worker runs 20:51:04–20:57:35 (391 seconds), then the list is paused.
The selected activity label is `claude-sonnet-5-5-high`; the transport is the
Subscriptions CLIProxyAPI sidecar. This log does not contain a 300-minute run
or establish which upstream physical model actually served the alias.

The coder makes 31 agent iterations. Its reported input grows from 11,573 to
87,319 tokens; cumulative input is 1,826,050, including 1,715,493 cached tokens
(93.95%, rounded). Noncached input is 110,557 and output is 26,559. Cumulative
input counts the context again for each iteration; it is not unique content
or a measurement of price. No price is inferred from these counts.

Eight commands fail: four project-boundary denials and four shell-substitution
blacklist denials. The worker explores files, invokes Waydroid/UI scripts and
writes a probe script. Most provider calls initially finish in seconds; later
calls take roughly 16–32 seconds. The log does not reveal complete arguments,
outputs or model reasoning, so it cannot prove every action was necessary.

The analyzer/reviewer/tester have zero prompt tokens at cancellation: they were
waiting for the coder. They did not consume the coder's 1.83M input tokens.

## Reproduced code defects and fixes

1. Length alone (>200 bytes) enabled a four-role workflow. A verbose simple
   item now uses the ordinary worker; explicit `[parallel]` and multi-action
   structure still enable fan-out. A real HTTP/App/provider E2E test asserts
   one worker call versus four for the explicit parallel case.
2. Fan-out dropped the normal worker's rules/preamble and retry context.
   All roles now get the same current work instructions. Reviewers also get
   a bounded coder handoff instead of having to rediscover what was done.
   Task rule loading now includes `.memo/AGENTS.md`, avoiding an identical
   copy when root rules already contain the same content.
3. Executor clones discarded configured context/iteration/continuation limits.
   Task and child executors now inherit those settings independently.
4. Role models came from an arbitrary running task. Resolution now receives
   the owning list's context. Matching-model roles inherit its effort; a
   different Orchestra role model does not inherit incompatible effort.
5. Failed/canceled coders still started three readers. Fan-out now stops and
   preserves the cause for quota/retry/self-heal handling.
6. Failed sub-agent runs skipped usage persistence; stream error chunks lost
   their attached usage/content. Available paid usage is now retained on
   failure, verified with a real SSE first iteration followed by HTTP 429.

Worker instructions explain existing project/shell constraints and ask for
narrow reads and concrete blockers rather than disguising denied commands.
Tool permissions and project boundaries are unchanged.

## Limits

These regressions verify wiring and avoidable call multiplication; they do not
measure real Antigravity task quality or guarantee completion in five minutes.
The 300-minute history, Task.md and complete worker transcript were not supplied.
The default 200K context budget is unchanged; configured limits now take effect.
The existing read-only-command prefix validation finding remains outside this
fix (see SUBAGENT_REVIEW.md). Real model/Waydroid replay remains to be measured.

## Verification results

- `CGO_ENABLED=1 go build -tags sqlite_fts5 ./...`: exit 0.
- `CGO_ENABLED=1 go vet -tags sqlite_fts5 ./...`: exit 0.
- Full `CGO_ENABLED=1 go test -tags sqlite_fts5 ./... -race`: FAIL only in
  `app.TestRunComponentUpdate_TamperedDownloadChangesNothing` (expected checksum
  error, received manifest/download length mismatch). This flake was already
  documented in handoff.md before this change; it was not retried until green.
- Final `-race -count=1` package results: taskloop 34.905s, agent 1.302s,
  e2e 68.897s, webserver 1.809s, all PASS.
- App suite with only that known update test excluded: PASS 51.355s.
- New focused regressions: app 1.656s, e2e 1.377s, taskloop 1.180s,
  agent 1.086s, all PASS. Root-cause tests were observed failing before fixes.
- Flutter: 826 tests PASS. `flutter analyze lib/`: three existing
  `use_build_context_synchronously` info findings, no warning/error.
- No Dart source changed; touched-Dart UI-literal check empty; diff check clean.

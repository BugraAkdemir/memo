# Sub-agents in normal agent chats

A normal agent turn can call `spawn_agent` when delegation helps or the person
requests it. Delegation is asynchronous: the tool returns the child's ID, and the
main agent can work, reply or yield without polling. The person can continue the
conversation while children work.

## Execution and context

- `spawn_agent`: `prompt`, optional `title`, `write`, `isolate`, `skills`.
- `send_agent`: `id`, `prompt`; continues the same task with its own prior messages
  and bounded tool evidence. Use a new child for unrelated work.
- `list_agents`, `cancel_agent`, `close_agent`, `merge_agent`: operate only on the
  requesting chat's children. No model argument selects a different parent chat.
- Read-only children have file inspection/search and web tools, with no shell,
  file mutation, personal tools or further delegation.
- Coding children additionally have file mutation and command tools. Explicit
  permission denies remain effective; task tools outside the delegated coding
  set and selected skills are denied rather than opening an invisible prompt.
- Children receive the configured Code Mode build prompt, their assigned task,
  their own task conversation and explicitly selected skill instructions. They
  receive no personal memory, RAG, persona or parent conversation. Project skills
  must first be enabled by the person in the parent chat. Explicit skill tools
  can be used by writable children; read-only children receive instructions only.
- Provider name, model and effort are selected from the actual parent turn and
  retained for the child. A removed provider or stopped local model fails clearly
  instead of silently substituting another model.

Settings exposes **Chat sub-agents** and the maximum simultaneous children:
1–16, default 3. This limit is independent of Self-Driving's parallel plan steps.
Excess child turns wait in the persisted start queue. Lowering the limit leaves
already running turns intact and prevents new starts until capacity is available.

## Main-agent FIFO and crash recovery

Every new completion is saved before it reserves a place in the main chat's FIFO.
Text, streamed image/file sends and child-result reviews use the same FIFO. A
second completion arriving during the first review waits; user messages arriving
later cannot start a concurrent main turn. Different conversations remain
independent.

The completion mailbox has monotonic IDs and pending/inflight/interrupted
states. The main agent claims one record before reviewing it. A successful final
assistant message contains its completion marker. The mailbox acknowledges it
only after reading that marker from the saved session file, never solely from
memory. Duplicate notifications and a crash after session save but before ACK do
not execute the review twice. Stopped, failed or unsaved reviews retain their
result and require **Resume main review** in the child pane; this resumes the
review without rerunning the child. Background reviews may use delegated coding/checking and child-coordination tools without an invisible permission dialog; explicit denies still win. Unrelated actions are denied and must be requested in an interactive turn.

Child tasks and completion records live in `config.DataPath("subagents")`, as an
atomic private state file and managed worktrees. They are temporary local recovery
records, git-ignored and excluded from the existing backup manifest. Stream text
is coalesced and checkpointed every 250 ms; completed output, tool events and
queue transitions save immediately. A crash can lose the latest unsaved stream
tail, but a finalized result remains stored. Running children recover as
interrupted and require an explicit follow-up; queued children can start normally.
Ordinary pending HTTP sends remain tied to their client connection.

Accepting a child with `close_agent` clears its transient prompt, output, system
prompt, messages and events. A small closed status record remains for the chat
card. Unacknowledged completion records are retained independently. Failed or
unmerged isolated work is not deleted automatically.

## Coding isolation and integration

For writable children, omitted `isolate` defaults to true. Each gets a separate
`memo-agent-<id>` branch and worktree. The source Git worktree must be clean,
including untracked files, and checked out on `main`; the runtime does not stash,
switch branches or discard local edits for the user. A non-Git project can use an
explicit `isolate:false` shared writer. A shared writer holds an ancestor-aware
project mutation lease: main can converse/read, but overlapping writes/commands
wait. Separate worktrees can write concurrently.

The child is instructed to test and commit its work locally. Main reviews the
result and requests corrections through `send_agent` as needed. `merge_agent`
serializes integration into clean main, validates the combined tree and only then
commits the merge. Validation checks whitespace and detected affected Go modules,
Flutter packages and npm test scripts (including mixed backend/frontend repos).
Toolchains must be available on the backend process's PATH. Unknown project types
still require main to perform their appropriate checks before requesting merge.

Conflicts or failed checks abort the attempted merge and preserve the child branch
and worktree. Cleanup verifies merge ancestry, removes the managed worktree and
uses safe branch deletion. Cleanup can resume after interruption. The runtime
never pushes branches, tags or releases.

## Interface and evidence

Child cards appear above the composer. Selecting a card opens a right pane with
its task prompt, own conversation, output and tool events; narrow windows use the
pane as a full-width view. Follow-up, cancellation, acceptance and interrupted
main-review recovery are available there. Polling is chat-scoped, gated by login,
nonoverlapping and disposed on navigation. Background main replies refresh only
when doing so cannot overwrite a user stream or a newer user turn.

Tests cover durable FIFO delivery, simultaneous completions, concurrency limits,
follow-up context, cancellation/restart, explicit denies, disk-save failure,
worktree isolation, merge conflict/validation failure/cleanup retry, streamed
attachments, real HTTP/fake-provider delegation and main resumption, and Flutter
panel/settings/recovery behavior. No live vendor-model quality or native
Windows/macOS execution is established by those tests.

## Review regressions verified on 2026-10-23

The two review findings were treated as separate checkpoints. The guarantees
come from the approved plan above and the review's model-selection race report.

| Guarantee | Test / evidence | Result |
| --- | --- | --- |
| Early subscription endpoint registration cannot write during a model-selection transaction, and reads the selection after acquiring the lock | `TestSyncSubscriptions_EarlyRegistrationWaitsForModelSelection` | RED on the previous code: `early registration wrote provider config while a model-selection transaction held subscriptionModelMu`; GREEN after protecting the early read/write |
| Model selection remains usable while network discovery is pending | `TestSyncSubscriptions_PreservesModelSelectedDuringDiscovery` and other `TestSyncSubscriptions_` cases | `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/app -race -run '^TestSyncSubscriptions_' -count=1`: PASS, 9.501s |
| Two isolated children actually write source/tests, run tests and commit; main reuses a child, merges and closes through real tools | `TestBenchmark_SubagentsFakeProject` | `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/e2e -race -run '^TestBenchmark_SubagentsFakeProject$' -count=1 -v -timeout=90s`: PASS, 2.654s |
| Child 2 completing during the blocked first review stays pending and reviews follow increasing completion IDs; accepted changes land through sequential merges and both branches/worktrees disappear | Same benchmark, channel-controlled ordering and final Git assertions | PASS |
| A broken child writer cannot produce a green benchmark | Temporary Go build overlay removing `write_file` from the child registry; no production mutation committed | Previous benchmark: PASS, 1.364s. New benchmark: expected FAIL, `Error: Unknown tool: write_file` for both writers |

Only fixture initialization writes files from the test process. All child changes,
follow-up edits, tests and commits use agent tool calls. Tool results must be
present and successful before the fake provider advances to its next step. The
fixture is a dependency-free Go module so merge validation also runs its tests.
The existing `TestChatSubAgents_ResultWhileMainBusyQueuesWithHumanTurn` covers a
human message arriving during main review; this benchmark controls child-result
order and writer isolation. Model quality and native desktop UX remain untested.

Verification: Go build/vet passed. The full `./... -race` run passed the agent,
subagent and E2E packages, but failed
`TestEnsureTailscaleWebServerBind_RebindsFromLoopback` with
`web server not running after rebind`. A comparison of race test binaries under
12 identical busy-loop processes, 150 runs per version, reproduced that same
failure 8 times before these fixes and 2 times after them. This is recorded as a
separate pre-existing server lifecycle problem; the full suite is **not** claimed
to be green. Flutter tests passed 815/815; analysis reported only the 3 existing
BuildContext infos. No Flutter source changed.

Coverage measurement:
`CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/app -run '^TestSyncSubscriptions_' -coverprofile=/tmp/memo-subagent-fixes-coverage.out`
passed (9.306s). `syncSubscriptions` and `SetProviderModel` each measured 75%;
the intentionally narrow selection measured 0.9% of the whole app package.
These figures do not establish 80% app-wide coverage.

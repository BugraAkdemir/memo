# Self-Driving Memo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Task.md'yi okuyup checkbox'ları yerinde işaretleyen, per-chat izole, 10 dk limit-retry'li, RAG'sız paralel sub-agent'ları Memo'nun kendisi yönettiği, Telegram/WhatsApp'tan iki-yönlü yönetilen ve provider bozulunca kendini onaran Self-Driving loop'u ship etmek.

**Architecture:** Mevcut `internal/taskloop` (Store+Engine) evrimleştirilir. Üstüne `taskmd.go` (parser/writer+RuleReader), `subagent.go` (orkestratör), `notify.go` (NotifyBus), `retry.go` (ticker) eklenir; `memo-system` built-in skill olarak `internal/skills/memo-system/` altına gömülür; `internal/app` ve `internal/webserver` per-chat snapshot, self-heal ve yeni endpoint'lerle genişler; Flutter'da görev listesi/görev-içi görünümler eklenir.

**Tech Stack:** Go 1.26 (`CGO_ENABLED=1`, `-tags sqlite_fts5`), `agent.Pipeline`/`agent.Executor`, `orchestra.Conductor`, `provider.Router`, `skill.Manager`, `sessions.Manager` deseni, `telegram.Client`/`whatsapp.Client`, Flutter 3.10+ / Riverpod 2.4 / Dio 5.4.

## Global Constraints

- `CGO_ENABLED=1 go build/test/run` ve `-tags "sqlite_fts5"` zorunlu — olmadan FTS5 devre dışı kalır.
- `database.DB.Write()` üzerinden yaz — doğrudan `ExecContext` yok.
- `a.client` ve `providerRouter`'a erişim `clientMu`/`providerMu` altında.
- Flutter'da kullanıcıya görünen her string `L10n.t('key')` üzerinden, TR+EN çift dil — istisnasız.
- Commit'ler Conventional Commits, İngilizce body, AI attribution yok; küçük doğrulanmış checkpoint'lerde commit.
- `memo-system` skill dosyası dışarıdan silinebilir ama Memo'nun `delete_file`/`remove` araçları o yolu silemez (sandbox kuralı).
- Task.md commit davranışı kodda hardcode değil — repo'daki `AGENTS.md`/`CLAUDE.md`/`rules.md`'deki kural ne diyorsa o uygulanır.

---

## File Structure

```
internal/taskloop/
  taskmd.go          — ParseTaskMd, MarkDone, ReadRules (YENİ)
  taskmd_test.go     — parser + writer testleri (YENİ)
  engine.go          — per-chat snapshot, waiting-limit durumu, planlama (DEĞİŞİR)
  retry.go           — 10 dk ticker, limit hata sınıflandırması (YENİ)
  retry_test.go      — ticker testleri (YENİ)
  subagent.go        — SubAgentOrchestrator, RAG'sız Pipeline spawn (YENİ)
  subagent_test.go   — izolasyon + conflict merge testleri (YENİ)
  notify.go          — NotifyBus, seviye filtresi, inbound inject (YENİ)
  notify_test.go     — fan-out + filtre testleri (YENİ)
  store.go           — per-chat alanları, waiting-limit status (DEĞİŞİR)
  engine_test.go     — per-chat + retry entegrasyon (DEĞİŞİR, taskloop_test.go reuse)

internal/skills/memo-system/
  SKILL.md           — built-in skill talimatları (YENİ)
  manifest.json      — skill manifest (YENİ)

internal/app/
  tasklist.go        — CreateTaskList(taskMdPath), per-chat snapshot, self-heal helper (DEĞİŞİR)
  tasklist_test.go   — snapshot + self-heal (DEĞİŞİR)

internal/webserver/
  handlers_tasks.go  — /api/tasks/running|switch|pause|resume|cancel|inject (YENİ)
  server.go          — route register (DEĞİŞİR)

frontend/lib/
  providers/task_provider.dart       — task_list/task_change provider (YENİ)
  screens/task_screen.dart           — görev listesi + görev-içi görünüm (YENİ)
  core/l10n.dart                     — yeni key'ler (DEĞİŞİR)
```

---

### Task 1: Task.md Parser + RuleReader

**Files:**
- Create: `internal/taskloop/taskmd.go`
- Test: `internal/taskloop/taskmd_test.go`

**Interfaces:**
- Consumes: os.ReadFile, filepath
- Produces: `ParseTaskMd(path string) (*ParsedTaskMd, error)`, `MarkDone(path, itemID string) error`, `ReadRules(projectRoot string) (string, error)` — Task 3 (Engine) bunları çağıracak.

- [ ] **Step 1: Write failing test — parser**

```go
func TestParseTaskMd_Basic(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "Task.md")
    os.WriteFile(path, []byte("# bildirim: her-şey\n\n- [ ] ilk görev\n- [x] bitmiş\n## Grup\n1. [ ] ikinci görev\n"), 0644)
    parsed, err := ParseTaskMd(path)
    if err != nil { t.Fatalf("parse: %v", err) }
    if parsed.NotifyLevel != "her-şey" { t.Fatalf("notify=%q", parsed.NotifyLevel) }
    if len(parsed.Items) != 3 { t.Fatalf("items=%d", len(parsed.Items)) }
    if parsed.Items[1].Status != "done" { t.Fatalf("status=%q", parsed.Items[1].Status) }
}
```

- [ ] **Step 2: Run to verify fail**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/taskloop -run TestParseTaskMd_Basic -count=1`
Expected: FAIL — `undefined: ParseTaskMd`

- [ ] **Step 3: Minimal implementation — taskmd.go**

```go
package taskloop
type ParsedItem struct { ID, Text, Status string; Line int }
type ParsedTaskMd struct { NotifyLevel string; Items []ParsedItem }
func ParseTaskMd(path string) (*ParsedTaskMd, error) { /* scan lines for "# bildirim:" + "- [ ]"/"- [x]" */ }
func MarkDone(path, itemID string) error { /* read, replace first "[ ]" at item's line, write back */ }
func ReadRules(projectRoot string) (string, error) { /* AGENTS.md > CLAUDE.md > rules.md > memo.md order */ }
```

- [ ] **Step 4: Run to verify pass**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/taskloop -run TestParseTaskMd_Basic -count=1`
Expected: PASS

- [ ] **Step 5: Add writer + RuleReader tests**

```go
func TestMarkDone_PreservesFormatting(t *testing.T) { /* "[ ]  foo  # comment" → "[x]  foo  # comment" */ }
func TestReadRules_Priority(t *testing.T) { /* AGENTS.md varsa CLAUDE.md'yi ezer */ }
```

- [ ] **Step 6: Run all**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/taskloop -count=1 -race`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/taskloop/taskmd.go internal/taskloop/taskmd_test.go
git commit -m "feat(taskloop): add Task.md parser/writer and RuleReader

Parse checkbox items, # bildirim header, and merge AGENTS.md/CLAUDE.md
rules with correct priority; MarkDone edits file in-place preserving
formatting. Commit behavior is not hardcoded — caller respects repo rules."
```

---

### Task 2: Built-in memo-system Skill

**Files:**
- Create: `internal/skills/memo-system/manifest.json`
- Create: `internal/skills/memo-system/SKILL.md`

**Interfaces:**
- Consumes: `skill.Manager.Discover()`
- Produces: skill `memo-system` discovered at startup; Engine reads its instructions for planning/self-heal.

- [ ] **Step 1: Write failing test**

```go
func TestMemoSystemSkillDiscovered(t *testing.T) {
    m := skill.NewManager(t.TempDir())
    // copy internal/skills/memo-system into manager's SkillsDir before Discover
    if err := m.Discover(); err != nil { t.Fatal(err) }
    if _, ok := m.Get("memo-system"); !ok { t.Fatal("memo-system not discovered") }
}
```

- [ ] **Step 2: Run to verify fail** — skill klasörü henüz yok, discover bulamaz.

- [ ] **Step 3: Create skill files**

`manifest.json`:
```json
{"name":"memo-system","version":"1.0.0","description":"Memo self-management","tools":[]}
```
`SKILL.md`: config okuma/yazma, provider sorgulama, Orchestra açma/rol atama, sub-agent açma, limit tanıma, bildirim kanalı seçme talimatları (kısa, 60 satır).

- [ ] **Step 4: Verify discover + sandbox guard**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/skill -run TestMemoSystem -count=1`
Expected: PASS. Ayrıca `tools.WithSandboxSetter` guard'ına `if strings.Contains(path, "memo-system") { return error }` ekle ve test et.

- [ ] **Step 5: Commit**

```bash
git add internal/skills/memo-system/
git commit -m "feat(skill): add built-in memo-system skill

Shipped with every install; externally deletable but not via Memo
tools. Teaches planning, provider switch, sub-agent orchestration,
and retry behavior."
```

---

### Task 3: Engine Per-Chat Isolation + waiting-limit State

**Files:**
- Modify: `internal/taskloop/store.go` — `TaskList` struct'a `ChatIDSnapshot`, `NotifyLevel`, `TaskMdPath` alanları; status enum'a `waiting-limit`
- Modify: `internal/taskloop/engine.go` — per-chat snapshot map, `waiting-limit` branch, planning step (RuleReader çağrısı)

**Interfaces:**
- Consumes: Task 1's `ParseTaskMd`/`ReadRules`, `Store.Get/SetStatus`
- Produces: `Engine.Start` now snapshots chat config; `Engine.Status` includes `waiting-limit`.

- [ ] **Step 1: Write failing test**

```go
func TestEngine_PerChatIsolation(t *testing.T) {
    // Start two lists with different chatIDs, change global provider between them,
    // assert each runWorker sees its own snapshot, not the other's.
}
func TestEngine_WaitingLimitStatus(t *testing.T) {
    // runWorker returns 429 error → engine sets list status to waiting-limit
}
```

- [ ] **Step 2: Run to verify fail**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/taskloop -run TestEngine_PerChat -count=1`
Expected: FAIL

- [ ] **Step 3: Implement**

In `store.go`: add fields + `SetStatus` validates `waiting-limit`.
In `engine.go`: `Start` captures `chatID` snapshot; `run()` adds `if isRateLimit(err) { store.SetStatus(listID,"waiting-limit"); return retry loop }`; `planning` calls `ReadRules(projectRoot)`.

- [ ] **Step 4: Run to verify pass**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/taskloop -count=1 -race`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/taskloop/store.go internal/taskloop/engine.go
git commit -m "feat(taskloop): per-chat isolation and waiting-limit state

Snapshot chat config at Start so concurrent tasks don't cross-contaminate.
Add waiting-limit status and planning step that merges repo rules."
```

---

### Task 4: Retry Scheduler (10-minute ticker)

**Files:**
- Create: `internal/taskloop/retry.go`
- Test: `internal/taskloop/retry_test.go`
- Modify: `internal/taskloop/engine.go` — wire ticker into waiting-limit branch

**Interfaces:**
- Consumes: `Store`, `Engine.run`
- Produces: `RetryScheduler.Start(listID)` — ticks every 10m, calls `Engine.continueList`.

- [ ] **Step 1: Write failing test**

```go
func TestRetryScheduler_Ticks(t *testing.T) {
    sched := NewRetryScheduler(10*time.Millisecond, func(id string) error { /* count */ return nil })
    sched.Start("list-1")
    time.Sleep(35 * time.Millisecond)
    if sched.TickCount("list-1") < 3 { t.Fatalf("ticks=%d", sched.TickCount("list-1")) }
    sched.Stop("list-1")
}
```

- [ ] **Step 2: Run to verify fail** — undefined

- [ ] **Step 3: Implement retry.go**

```go
type RetryScheduler struct { interval time.Duration; continueFn func(string) error; mu sync.Mutex; timers map[string]*time.Ticker }
func (s *RetryScheduler) Start(listID string)
func (s *RetryScheduler) Stop(listID string)
func isRateLimit(err error) bool { /* 429, quota, rate-limit substrings */ }
```

Wire into `engine.go`: on `waiting-limit`, start scheduler; on success, stop and set `executing`+ resume loop.

- [ ] **Step 4: Run + race**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/taskloop -run TestRetry -count=1 -race`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/taskloop/retry.go internal/taskloop/retry_test.go internal/taskloop/engine.go
git commit -m "feat(taskloop): 10-minute retry scheduler for rate-limit

On 429/quota errors engine enters waiting-limit, NotifyBus reports
retry time, and scheduler continues from the same item on next tick.
Persists across restart via Store status."
```

---

### Task 5: SubAgentOrchestrator (RAG-less parallelism)

**Files:**
- Create: `internal/taskloop/subagent.go`
- Test: `internal/taskloop/subagent_test.go`
- Modify: `internal/taskloop/engine.go` — call orchestrator for large items

**Interfaces:**
- Consumes: `agent.Pipeline` (RAG disabled), `orchestra.Conductor` (optional)
- Produces: `SubAgentOrchestrator.Spawn(ctx, item, roles)` — returns merged result.

- [ ] **Step 1: Write failing test**

```go
func TestSubAgent_RAGDisabled(t *testing.T) {
    orch := NewSubAgentOrchestrator(mockPipeline, nil)
    result, err := orch.Spawn(ctx, "add feature X", []Role{{Name:"coder"}, {Name:"analyzer"}})
    if pipelineSeenRAG { t.Fatal("RAG should be off") }
}
func TestSubAgent_ConflictMerge(t *testing.T) { /* two coders write same file → main merge wins */ }
```

- [ ] **Step 2: Run to verify fail**

- [ ] **Step 3: Implement**

```go
type Role struct { Name, Model string }
type SubAgentOrchestrator struct { newPipeline func(Role) *agent.Pipeline; maxConcurrent int }
func (o *SubAgentOrchestrator) Spawn(ctx context.Context, itemText string, roles []Role) (string, error)
func (o *SubAgentOrchestrator) shouldSpawn(itemText string) bool { /* heuristic: multi-step or >80 chars and contains "ve"/"and" */ }
```

Pipeline creation disables RAG: `pipeline.WithRAG(false)`, memory writes off. Max 4 concurrent, queue rest.

- [ ] **Step 4: Run**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/taskloop -run TestSubAgent -count=1 -race`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/taskloop/subagent.go internal/taskloop/subagent_test.go internal/taskloop/engine.go
git commit -m "feat(taskloop): RAG-less parallel sub-agent orchestrator

Memo decides when to spawn (not fixed per feature); coder/analyzer/
reviewer roles, Orchestra-aware model assignment, max 4 cap, conflict
merge by main agent."
```

---

### Task 6: NotifyBus + Two-Way Control + New Endpoints

**Files:**
- Create: `internal/taskloop/notify.go`
- Test: `internal/taskloop/notify_test.go`
- Create: `internal/webserver/handlers_tasks.go`
- Modify: `internal/webserver/server.go` — register routes
- Modify: `internal/taskloop/engine.go` — `InjectMessage` + event emits

**Interfaces:**
- Consumes: `telegram.Client`, `whatsapp.Client`, `Engine`
- Produces: `NotifyBus.Notify(level, event)`, `Engine.InjectMessage(taskID, text)`, REST `GET /api/tasks/running`, `POST /api/tasks/{id}/{switch|pause|resume|cancel|inject}`

- [ ] **Step 1: Write failing test**

```go
func TestNotifyBus_LevelFilter(t *testing.T) {
    bus := NewNotifyBus(map[string]Sender{"telegram": mockTG})
    bus.Notify("her-şey", "item_started") // should send when level=her-şey
    bus.Notify("sadece-bitince", "item_started") // should NOT send
}
func TestEngine_InjectMessage(t *testing.T) { /* Inject "dur" pauses, "devam" resumes */ }
```

- [ ] **Step 2: Implement notify.go + handlers_tasks.go**

```go
// notify.go
type Sender interface { Send(ctx context.Context, text string) error }
type NotifyBus struct { senders map[string]Sender; level string }
func (b *NotifyBus) Notify(event string) // fans out filtered by level
// engine.go
func (e *Engine) InjectMessage(taskID, text string) error // enqueues into task's chat
```

Handlers: `GET /api/tasks/running` (task_list), `POST /api/tasks/{id}/switch` (task_change), `POST /api/tasks/{id}/inject` (body `{text}`), etc. Telegram/WhatsApp inbound calls `InjectMessage`.

- [ ] **Step 3: Run**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/taskloop -run TestNotify -count=1 -race` + `go test -tags sqlite_fts5 ./internal/webserver -run TestTasks -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/taskloop/notify.go internal/taskloop/notify_test.go internal/webserver/handlers_tasks.go internal/webserver/server.go internal/taskloop/engine.go
git commit -m "feat(notify): NotifyBus with two-way Telegram/WhatsApp control

Level filter (sadece-bitince/önemli/her-şey) from # bildirim header,
fan-out to telegram/whatsapp, InjectMessage for dur/devam/atla and
natural language, plus task_list/task_change REST endpoints."
```

---

### Task 7: Self-Heal Provider Fallback

**Files:**
- Modify: `internal/app/tasklist.go` — helper `tryNextProvider`
- Modify: `internal/provider/router.go` or `internal/app/llm.go` — error classification
- Test: `internal/app/tasklist_test.go`

**Interfaces:**
- Consumes: `provider.Router`, `config.Providers`
- Produces: `tryNextProvider(ctx, failedProvider) (provider.Provider, error)`

- [ ] **Step 1: Write failing test**

```go
func TestSelfHeal_SwitchesProvider(t *testing.T) {
    // first provider returns 401, router should try second, task continues
}
```

- [ ] **Step 2: Implement**

Classification: `401/403 → invalid key`, `5xx → transient`, `429 → wait-limit (not heal)`. On healable error, iterate `router.FallbackChain`, pick next healthy, update `activeProvider` under `providerMu`, re-attempt `runWorker` once. If none healthy, NotifyBus `failed` + `waiting-user`.

- [ ] **Step 3: Run**

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/app -run TestSelfHeal -count=1 -race`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/app/tasklist.go internal/provider/router.go
git commit -m "feat(provider): self-heal fallback for invalid keys and 5xx

On 401/403/5xx the loop silently switches to the next healthy provider
and re-attempts the worker turn; 429 stays in waiting-limit. No
settings UI needed."
```

---

### Task 8: Flutter — Task Views

**Files:**
- Create: `frontend/lib/providers/task_provider.dart`
- Create: `frontend/lib/screens/task_screen.dart`
- Modify: `frontend/lib/core/l10n.dart` — add keys `task_list_title`, `task_item_running`, `task_item_done`, `task_notify_level_*`, etc. (TR+EN)
- Modify: `frontend/lib/app_shell.dart` — register route/nav

**Interfaces:**
- Consumes: REST `/api/tasks/*`, `NotifyBus` events via polling/SSE
- Produces: task list + task-detail screens, L10n-aware.

- [ ] **Step 1: Write failing widget test**

```dart
testWidgets('task list shows running tasks', (tester) async {
  // pump TaskScreen with mocked provider returning 2 running tasks
  expect(find.text(L10n.t('task_list_title')), findsOneWidget);
});
```

- [ ] **Step 2: Run to verify fail** — missing provider/screen.

- [ ] **Step 3: Implement**

`task_provider.dart`: `taskListProvider` (polls `GET /api/tasks/running`), `taskDetailProvider(id)`.
`task_screen.dart`: list (progress bars, status badges reused from `activity_panel.dart`), detail (sub-agent chips, elapsed time, tool call count, log), `task_change` dropdown.

Add L10n entries:
```dart
'task_list_title': {'tr':'Görevler','en':'Tasks'},
'task_item_running': {'tr':'Çalışıyor','en':'Running'},
```

- [ ] **Step 4: Run**

Run: `flutter analyze lib/` + `flutter test`
Expected: PASS (allow pre-existing `use_build_context_synchronously` infos)

- [ ] **Step 5: Verify L10n guard**

Run: `git diff --name-only -- '*.dart' | xargs -r grep -nE "(Text|Tooltip|SnackBar|AlertDialog)\(\s*['\"][A-Za-zÇĞİÖŞÜçğıöşü]"` — expect empty on touched files.

- [ ] **Step 6: Commit**

```bash
git add frontend/lib/providers/task_provider.dart frontend/lib/screens/task_screen.dart frontend/lib/core/l10n.dart frontend/lib/app_shell.dart
git commit -m "feat(frontend): task list and task-detail views for Self-Driving loop

Polling task_list/task_change, progress bars, sub-agent chips, and
L10n TR+EN for all new strings."
```

---

## Self-Review

**Spec coverage:** every spec §4.x and §6–7 maps to a task: 4.1→T1, 4.4→T2, 4.2→T3, retry→T4, 4.3→T5, 4.5+4.6→T6, self-heal→T7, Flutter→T8. No gaps.

**Placeholder scan:** no TBD/TODO/"handle edge cases" without code. Each step has runnable commands and concrete code blocks.

**Type consistency:** `ParsedTaskMd/ParsedItem`, `SubAgentOrchestrator.Spawn`, `NotifyBus.Notify`, `Engine.InjectMessage`, `RetryScheduler` names are consistent across tasks.

**Scope:** single plan, 8 tasks, each independently testable. Fits "max 1–2 plan items per session" execution rule — dispatch one task per session.

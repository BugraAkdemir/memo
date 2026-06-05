# 🎵 Orchestra Mode

> **Package:** `internal/orchestra/` (3 files, ~1000 lines)
> **Config file:** `data/orchestra.json`
> **API endpoints:** `GET/PUT /api/orchestra/config`
> **Slash command:** `/orchestra`

Orchestra Mode enables multiple AI models to collaborate as an expert team. A **Chief** model decomposes the user's request into subtasks, assigns each to a specialized **Expert Role**, and synthesizes the results into a coherent response.

---

## Concept

```
User Prompt: "Build a React dashboard with a Go backend"
      │
      ▼
┌─────────────────────────────────────────┐
│         CHIEF (e.g., Claude)             │
│  "Analyze request, create task plan"    │
│  Output: JSON with tasks + dependencies │
└──────────────────┬──────────────────────┘
                   │
                   ▼ Plan (JSON)
┌─────────────────────────────────────────┐
│  Tasks:                                  │
│  [                                       │
│    {"role":"frontend","prompt":"..."},   │
│    {"role":"backend","prompt":"..."},    │
│    {"role":"security","prompt":"..."}    │
│  ]                                       │
│  parallel: true                          │
└─────────────────────────────────────────┘
                   │
        ┌──────────┼──────────┐
        ▼          ▼          ▼
┌──────────┐ ┌──────────┐ ┌──────────┐
│ Frontend │ │ Backend  │ │ Security │
│ (Grok)   │ │ (GPT-4o) │ │ (Claude) │
└──────────┘ └──────────┘ └──────────┘
        │          │          │
        └──────────┼──────────┘
                   ▼
┌─────────────────────────────────────────┐
│         CHIEF (Synthesis)                │
│  "Merge all results into one response"  │
└──────────────────┬──────────────────────┘
                   │
                   ▼
           Final Response to User
```

---

## Chief Model

**Role:** The "conductor" of the orchestra. The chief:

1. **Analyzes** the user's request
2. **Plans** by breaking work into discrete tasks
3. **Assigns** each task to the most appropriate expert role
4. **Synthesizes** all expert results into one unified response

### Chief System Prompt

The chief receives a system prompt that defines its role:

```
Sen bir Orkestra Şefi'sin. Kullanıcının isteğini analiz eder,
alt görevlere ayırır, her görevi en uygun uzmana atar ve
sonuçları sentezlersin. JSON formatında plan döndürürsün.

Kullanılabilir Uzman Roller:
- planner: Yazılım mimarı, analiz ve planlama
- frontend: UI geliştirme (React/Vue/Flutter)
- backend: API ve sunucu geliştirme
- bug_fixer: Hata ayıklama ve çözüm
- reviewer: Kod kalite incelemesi
- security: Güvenlik denetimi
- devops: Altyapı ve deploy
- general: Genel amaçlı

Yanıt SADECE JSON olmalıdır:
{
  "tasks": [
    {
      "role": "frontend",
      "context": "Proje bağlamı",
      "prompt": "Yapılacak iş",
      "depends_on": []
    }
  ],
  "parallel": true
}
```

### Plan Output Format

```json
{
  "tasks": [
    {
      "role": "frontend",
      "context": "We are building a task management dashboard...",
      "prompt": "Create a React component for the task list...",
      "depends_on": []
    },
    {
      "role": "backend",
      "context": "...",
      "prompt": "Create a Go HTTP handler for task CRUD...",
      "depends_on": []
    },
    {
      "role": "security",
      "context": "...",
      "prompt": "Review the authentication flow...",
      "depends_on": ["backend"]
    }
  ],
  "parallel": false
}
```

- **`depends_on`**: References task roles that must finish first
- **`parallel`**: If `true`, independent tasks run concurrently

---

## Expert Roles (8 Built-in)

### Role Configuration

Each role has:
- **Name** — unique identifier
- **Enabled** — toggle on/off
- **Model Type** — provider (openai, gemini, grok, claude, etc.)
- **Model Name** — specific model (gpt-4o, claude-sonnet-4, etc.)
- **System Prompt** — customized instructions for the role

### Default Assignments

| Role | Icon | Default Model | Default Enabled | Purpose |
|------|------|---------------|-----------------|---------|
| `planner` | 📋 | Claude | ✅ | Software architecture, task decomposition |
| `frontend` | 🎨 | Grok | ✅ | UI development (React, Flutter, CSS) |
| `backend` | ⚙️ | GPT-4o | ✅ | API, database, server logic |
| `bug_fixer` | 🔧 | Gemini | ✅ | Debugging, stack trace analysis |
| `reviewer` | 👁️ | Claude | ❌ | Code quality, security, performance |
| `security` | 🔒 | GPT-4o | ❌ | OWASP, penetration testing |
| `devops` | 🚀 | Grok | ❌ | CI/CD, Docker, Kubernetes |
| `general` | 💬 | GPT-4o | ✅ | General-purpose fallback |

### Default System Prompts

Each role has a specialized system prompt (in Turkish by default):

**Planner:** "Sen bir yazılım mimarısın. İşi analiz et, adımlara böl, detaylı plan çıkar."

**Frontend:** "Sen bir frontend uzmanısın. React/Vue/Flutter bileşenleri yazarsın..."

**Backend:** "Sen bir backend uzmanısın. API'ler, veritabanı, sunucu mantığı..."

**Bug Fixer:** "Sen bir hata ayıklama uzmanısın. Kod hatalarını bulur, çözüm önerirsin..."

**Reviewer:** "Sen bir kod inceleme uzmanısın. Kod kalitesi, güvenlik, performans..."

**Security:** "Sen bir güvenlik uzmanısın. OWASP, güvenlik açıkları, şifreleme..."

**DevOps:** "Sen bir DevOps uzmanısın. CI/CD, Docker, Kubernetes, cloud altyapı..."

**General:** "Sen genel amaçlı bir asistansın. Kullanıcıya her konuda yardımcı olursun."

### Custom Roles

Users can create custom roles with:
- Custom name
- Model assignment
- Custom system prompt
- Same execution behavior as built-in roles

---

## Execution Engine

**File:** `internal/orchestra/conductor.go` (680 lines)

### Phase 1: Plan

```
createPlan(ctx, userPrompt, roleInfo, progressFn)
  │
  ├── Build chief prompt (system + role info + user message)
  ├── Call chief provider (ChatCompletionStream if progress, ChatCompletion if not)
  ├── Temperature: 0.3 (deterministic planning)
  ├── MaxTokens: 4096
  ├── Timeout: 120 seconds
  │
  ├── Parse JSON from response:
  │   ├── Try ```json ... ``` block
  │   ├── Try ``` ... ``` block
  │   └── Fall back to bare {...} brace matching
  │
  └── Fill ModelType/ModelName from role config for each task
```

### Phase 2: Execute

```
executeTasks(ctx, plan, results, progressFn)
  │
  ├── parallel = true AND no dependencies?
  │   └── Run all independent tasks concurrently (goroutines + WaitGroup)
  │
  ├── sequential / has dependencies?
  │   └── Resolve dependency graph (DAG):
  │       ├── Track completed tasks in a set
  │       ├── Iterate: find tasks whose deps are all met
  │       └── Execute in order, max iterations = len(tasks)*2 (deadlock detection)
  │
  └── Each task:
      ├── createProviderForType(modelType, modelName)
      │   ├── Find enabled provider config via getConfigs()
      │   ├── Clone config, override Model
      │   └── Create provider via factory
      ├── ChatCompletion with 120s timeout
      ├── Retry up to 2 times (exponential backoff: 3s, 6s)
      ├── Rate-limit retry: callWithRetry (3 retries, 5s/10s/20s backoff)
      └── Token estimation: len(strings.Fields(content))
```

#### Parallel Execution

```go
var wg sync.WaitGroup
for i, task := range tasks {
    wg.Add(1)
    go func(idx int, t OrchestraTask) {
        defer wg.Done()
        result := c.executeTask(ctx, t, progressFn)
        results[idx] = result
    }(i, task)
}
wg.Wait()
```

#### Dependency Resolution

```go
completed := make(map[string]bool)
for iteration := 0; iteration < maxIter; iteration++ {
    for i, task := range tasks {
        if results[i] != nil { continue } // already done
        depsMet := true
        for _, dep := range task.DependsOn {
            if !completed[dep] { depsMet = false; break }
        }
        if depsMet {
            results[i] = c.executeTask(ctx, task, progressFn)
            completed[task.Role] = true
        }
    }
}
```

### Phase 3: Synthesize

```
synthesize(ctx, userPrompt, results, progressFn)
  │
  ├── All tasks failed?
  │   └── Return failure summary
  │
  ├── Build synthesis prompt:
  │   "Kullanıcı isteği: {prompt}
  │    Uzman yanıtları:
  │    --- {role} ({model}) ---
  │    {content}
  │    ...
  │    Bu yanıtları tek bir tutarlı cevapta birleştir."
  │
  ├── Call chief provider
  ├── Temperature: 0.5
  ├── MaxTokens: 2048
  └── Stream result via ProgressSynthChunk
```

---

## Retry & Error Handling

### Rate Limit Retry (`callWithRetry`)

```go
func callWithRetry(ctx context.Context, fn func() error) error {
    var lastErr error
    delays := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second}
    
    for i := 0; i <= 3; i++ {
        err := fn()
        if err == nil { return nil }
        if isRateLimitError(err) {
            // Parse "try again in Xs" from error for dynamic backoff
            select {
            case <-time.After(delays[i]):
            case <-ctx.Done(): return ctx.Err()
            }
            continue
        }
        return err // Non-rate-limit errors: return immediately
    }
    return lastErr
}
```

### Task Retry (`retryTask`)

- 2 retries with 3-second base delay
- Exponential backoff: 3s, 6s
- Rate-limit errors handled by `callWithRetry` (not retried again)

### Timeouts

| Phase | Timeout |
|-------|---------|
| Plan | 120 seconds |
| Per task | 120 seconds |
| Synthesis | 60 seconds |

---

## Provider Creation

**Important:** Orchestra creates providers **directly** via factory, bypassing `provider.Router`:

```go
func (c *Conductor) createProviderForType(modelType, modelName string) (provider.Provider, error) {
    // 1. Find enabled provider config matching modelType
    configs := c.getConfigs()
    for _, cfg := range configs {
        if string(cfg.Type) == modelType && cfg.Enabled {
            pCfg := cfg
            pCfg.Model = modelName // Override with role-specific model
            return c.pf(pCfg) // factory: provider.NewProvider(pCfg)
        }
    }
    return nil, fmt.Errorf("Provider konfigürasyonu bulunamadı: %s", modelType)
}
```

This means:
- **No fallback chain** — each role is pinned to its configured model
- **No auto-disable** — if a provider fails, the task fails
- **Direct connection** — less overhead, simpler error tracking

---

## Config & Data Model

**File:** `internal/orchestra/types.go` (150 lines)

### OrchestraConfig

```json
{
  "enabled": false,
  "chief_type": "claude",
  "chief_model": "claude-sonnet-4-20250514",
  "roles": [
    {
      "role": "frontend",
      "enabled": true,
      "model_type": "grok",
      "model_name": "grok-2",
      "system_prompt": "Sen bir frontend uzmanısın..."
    }
  ]
}
```

### Default Config

| Field | Default Value |
|-------|---------------|
| `chief_type` | `claude` |
| `chief_model` | `claude-sonnet-4-20250514` |
| Roles: planner | Claude, enabled |
| Roles: frontend | Grok, enabled |
| Roles: backend | OpenAI, enabled |
| Roles: bug_fixer | Gemini, enabled |
| Roles: reviewer | Claude, disabled |
| Roles: security | OpenAI, disabled |
| Roles: devops | Grok, disabled |
| Roles: general | OpenAI, enabled |

### Config Persistence

- File: `data/orchestra.json`
- Loaded at startup via `LoadConfig()`
- Saved after every update via `SaveConfig()`
- `MergeRoles()` ensures all built-in roles exist (missing ones added disabled)

---

## Progress Streaming

During execution, the conductor emits typed progress updates:

| Progress Type | UI Message | Example |
|---------------|------------|---------|
| `ProgressPlan` | `"🧠 Şef planlıyor..."` | Shown once at start |
| `ProgressPlanChunk` | Raw stream | Chief's planning reasoning |
| `ProgressTaskStart` | `"🎯 frontend (grok/grok-2) çalışıyor..."` | Per task |
| `ProgressTaskChunk` | (not used in V1) | Streaming task output (future) |
| `ProgressTaskDone` | `"✅ frontend | grok-2 (3421ms)"` | On success |
| `ProgressTaskDone` | `"❌ backend | gpt-4o ⚠️ timeout"` | On failure |
| `ProgressSynthChunk` | Raw content | Chief's synthesis output |
| `ProgressError` | Error message | Fatal errors |

### Frontend Rendering

When orchestra is active, the chat header shows:
```
🎵 Orchestra Mode Active
🧙 Şef: claude / claude-sonnet-4-20250514
```

And progress updates appear inline in the chat stream.

---

## Frontend Controls

### Settings Tab

Located in Settings → Orchestra tab:
- **Enable/Disable** toggle
- **Chief Model** selector (dropdown of all configured providers + local)
- **Active Roles** summary: `"3 rol aktif • frontend (grok), backend (gpt-4o), bug_fixer (gemini)"`
- **"Rolleri ve Modelleri Yapılandır"** button → opens config dialog

### Config Dialog

```
┌───────────────────────────────────────────┐
│ 🎵 Orchestra Mode                          │
│                                           │
│ [✓] Aktif                                 │
│                                           │
│ Chief Model: [claude / claude-sonnet-4 ▼] │
│                                           │
│ ┌─ Roles ──────────────────────────────┐  │
│ │ 📋 planner    [✓] [gpt-4o       ▼]  │  │
│ │ 🎨 frontend   [✓] [grok-2       ▼]  │  │
│ │ ⚙️ backend    [✓] [gpt-4o       ▼]  │  │
│ │ 🔧 bug_fixer  [✓] [gemini-2.0-flash]│  │
│ │ 👁️ reviewer   [✗] [claude-sonnet ▼] │  │
│ │ 🔒 security   [✗] [gpt-4o       ▼]  │  │
│ │ 🚀 devops     [✗] [grok-2       ▼]  │  │
│ │ 💬 general    [✓] [gpt-4o       ▼]  │  │
│ └─────────────────────────────────────┘  │
│                                           │
│ [+ Özel Rol Ekle]                         │
│                                           │
│ [Cancel]  [Save]                          │
└───────────────────────────────────────────┘
```

### Slash Commands

| Command | Action |
|---------|--------|
| `/orchestra` | Open config dialog |
| `/orchestra on` | Enable orchestra mode |
| `/orchestra off` | Disable orchestra mode |
| `/orchestra config` | Open config dialog |
| `/orchestra status` | Show current config summary |

### Chat Input Behavior

When orchestra mode is enabled:
- Normal send button is **disabled** (tooltip: "Orchestra aktifken normal sohbet devre dışı")
- Only slash commands work in the input field
- All messages go through the conductor

---

## Known Issues & Limitations

| Issue | Detail |
|-------|--------|
| **No provider fallback** | Bypasses `provider.Router` — task fails if provider errors |
| **No streaming per task** | Task results are returned whole (no token-level streaming) |
| **Chief must output JSON** | Non-JSON responses cause parse errors |
| **No config validation** | Invalid chief model or missing role model causes runtime error |
| **Non-streaming tasks** | User waits for all tasks to complete before seeing anything |
| **Turkish prompts only** | System prompts and errors are Turkish (hardcoded) |

---

### Linked Notes:
- [[External Providers]] — Provider types used by orchestra roles
- [[Agent Mode]] — Alternative single-model tool-calling mode
- [[Architecture]] — System integration
- [[API Documentation]] — Orchestra endpoint

# Proactive Learning and Calendar

> **Status:** ✅ Complete (backend + mobile + desktop settings) | **Version:** v3.3.3 added Routines, ambient nudges with a real suggestion banner, and Self-Insight on top of the v3.1.1 intent/calendar system below.
>
> Memo's learning system now understands not just *when* you do something, but *what you tell it you will do* — in both chat and WhatsApp conversations — and, as of v3.3.3, can act on its own initiative: scheduling itself (Routines), noticing patterns unprompted (proactive nudges), and reflecting on them when asked (`/insight`).

Related pages: [[WhatsApp Integration]] · [[Orchestra Mode]] · [[Mobile App]] · [[Backend (Go) Architecture]]

---

## ⏰ Routines — Scheduled Automations (`internal/routine/`, v3.3.3)

Sidebar → **Routines** lets you describe, in plain language, something Memo should do on its own on a schedule — a plain-language prompt is parsed into a `routine.Routine` (`internal/routine/types.go`) via `Extractor` (`extractor.go`).

- **Two execution paths**, chosen by `Routine.AgentMode`: a plain LLM call with deterministically pre-fetched context (`ContextSource` — `none`/`calendar`/`whatsapp`/`insight`, fetched in Go rather than trusting an unattended tool call), or the full agent/tool pipeline for tasks like "git pull and report status" — the latter requires `AutoApproveTools` set at creation time, since there's no human present to answer a permission prompt when it fires.
- **Per-device timezone.** `Schedule.UTCOffsetMinutes` captures the creating device's UTC offset at creation time (not the backend host's local time) — `ParseFireTime` interprets `TimeOfDay` against that offset, and the offset **automatically resyncs on every (re)connect** (`/api/routines/sync-offset`) so travel or a DST change corrects itself instead of staying frozen at whatever was true the day the routine was created. A routine created before this field existed (nil offset) falls back to the previous host-local-time behavior.
- **Works on desktop and mobile.** The mobile app has no push channel, so `RoutineLoop` pre-generates content ahead of the fire time and the mobile app polls `/api/routines/mobile-ready` to pre-schedule a real local notification — it still arrives even if the app isn't open.
- **Language-aware output.** `Routine.Language` (captured client-side at creation) drives the notification title, the "nothing due today" filler text, and the system prompt used for the non-agent path — previously this always came out in Turkish regardless of the user's setting.
- **Delivery:** in-chat, and optionally to a WhatsApp target JID (`DeliveryWhatsApp`) or as a mobile notification (`DeliveryMobile`).

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|--------------|
| `GET`/`POST` | `/api/routines` | List / create routines |
| `POST` | `/api/routines/parse` | Turn plain-language text into a draft routine |
| `GET`/`PUT`/`DELETE` | `/api/routines/{id}` | Get/update/delete a routine |
| `GET` | `/api/routines/mobile-ready` | Mobile polling endpoint for pre-scheduling local notifications |
| `POST` | `/api/routines/sync-offset` | Resync a client's UTC offset against its routines |

---

## 🧭 Self-Insight (`/insight`, v3.3.3)

`GenerateSelfInsight` (`internal/app/insight.go`) synthesizes a short "what did you notice about yourself" reflection from the user's own conversation memory (last 30 days by default, capped at 60 raw memory rows to stay inside the model's context budget) plus their mood-history trend. It's explicitly instructed not to invent a pattern if there isn't enough to go on — it says so instead of guessing.

Two entry points share the same generation code:
- **On-demand:** type `/insight` in chat → `POST /api/memory/insight`.
- **Scheduled:** a Routine with `ContextSource.Type = "insight"` (e.g. a weekly self-insight digest) pre-fetches the same window deterministically before the LLM call.

---

## 🔔 Proactive Ambient Nudges (v3.3.3)

On top of the classic observation-based learning system (below), Memo can now surface a pattern **unprompted** instead of only ever answering when asked — the `internal/proactive/` engine (`Engine.NewEngine`, a periodic heartbeat, default 30s interval) matches the current moment against learned patterns (`internal/observer/`) and asks the Chief model what to do with it (`Decider`), subject to a minimum score/confidence and a 30-minute cooldown between consultations.

- **On by default** at the "subtle" level. Settings → General has the master switch, plus (new in v3.3.3) **independent Minimal Mode overrides** — persona/system-prompt, capability disclosures, passive-feature disclosures, and proactive learning can each be re-enabled individually even while Minimal Mode is otherwise stripping everything else out.
- A habit **stated directly** in conversation ("I code every night around 9") is trusted immediately, unlike a passively-observed pattern which needs to show up statistically across sessions first.
- A nudge can appear **woven into a normal reply** ("isn't it about your coding time?") instead of only as a separate suggestion — whether it was actually raised, and whether the user accepted or brushed it off, is judged by asking the model itself (language-independent, not string-matched).
- **A real suggestion banner exists on desktop now** (Yes / Not now / Stop asking) — previously there was no UI to see or respond to a `PendingSuggestion` at all.
- **Fully disabled under Incognito Mode**, and under Minimal Mode unless specifically re-enabled per the granular toggles above.

---

---

## 🎯 Overview

The classic learning system (see `docs/learning-system`) was observation-based: it watched the user's *behaviour* and extracted time patterns using circular statistics. This version adds three new layers:

1. **Intent Extraction** — understanding plans and habits from messages
2. **Calendar System** — saving detected events and sending reminders
3. **Single Model Mode** — using one model for learning decisions instead of Orchestra

```
Message Sources
  ├── Chat (normal)
  └── WhatsApp (incoming + outgoing)
          │
          ▼
  IntentExtractor (internal/intent/)
    1. KeywordFilter (fast, no LLM)
    2. LLM Parse → IntentResult
          │
    ┌─────┴──────┐
    ▼            ▼
Calendar      Observer
(event)       (habit/declaration)
    │
    ▼
ReminderLoop → AppEvent → Mobile Local Notification
```

---

## 🧩 Intent Extraction — `internal/intent/`

Sending every message to the LLM would be expensive. A two-stage pipeline is used instead.

### Stage 1 — Keyword Pre-filter (`filter.go`)
`MightHaveIntent(text)` performs a fast substring scan. Messages that don't contain time/intent/habit keywords (yarın, saat, her gün, meeting, "i will", "every day"…) are filtered out **without any LLM call**.

### Stage 2 — LLM Extraction (`extractor.go`)
Messages that pass the filter are sent to the model via `Extractor.Extract()`. The model returns JSON only:

```json
{
  "has_intent": true,
  "is_calendar_event": true,
  "is_habit": false,
  "summary": "Playing football tomorrow at 11:00 (with Ahmet)",
  "event_title": "Football",
  "event_time_iso": "2026-06-16T11:00:00",
  "time_explicit": true,
  "habit_time_hhmm": "",
  "habit_days": []
}
```

- **`now` parameter is injected** — relative expressions like "tomorrow" are resolved against a testable reference; the extractor never calls its own `time.Now()`.
- **String-aware JSON extraction** — `extractJSON` ignores braces inside JSON string values (like `"tomorrow :)}"`); JSON is extracted even when wrapped in prose or code fences.

### Privacy
**The raw message text is never stored.** Only a short summary, topic, and (for habits) the declared time are persisted — consistent with the learning system's privacy-first philosophy.

---

## 📅 Calendar System — `internal/calendar/`

| File | Responsibility |
|------|---------------|
| `event.go` | `Event` type, `ReminderPayload`, `AddedPayload` |
| `store.go` | SQLite CRUD (`data/calendar/events.db`), `PendingReminders` |
| `reminder.go` | `ReminderLoop` running every minute, emits `calendar:reminder` events |
| `bridge.go` | `AddFromIntent` — converts intent to calendar event |

### Auto-Insertion
When an event is detected (e.g. WhatsApp "football tomorrow at 11") it is immediately added to the calendar. The user gets an **added notification**; they can delete it if wrong. No confirmation prompt — friction is minimised.

### Reminder Loop
`ReminderLoop` queries `PendingReminders(now, lead)` every minute. When an event enters the `ReminderLeadMinutes` window:
1. A `calendar:reminder` AppEvent is emitted (JSON payload)
2. `reminder_sent=1` is marked (never sent again)

Lead time is user-configurable: **10 min / 15 min / 30 min / 1 hour / 2 hours**.

### Mobile Calendar Tab
The mobile app ([[Mobile App]]) has a new **Calendar** tab:
- Monthly grid with event dots
- Tap a day to see that day's events
- Manual event creation
- Delete button for removal
- Local notifications via `flutter_local_notifications` on `calendar:reminder`

---

## 🤖 Single Model Mode

The learning system (intent extraction + proactive decisions) previously always used [[Orchestra Mode]], which assigns different models for different tasks. If the user wants a single model for everything:

- **Settings → Learning** tab has a "Single Model Mode" toggle and model ID selector.
- `intent.BuildDecider` factory produces a single `Decider` that is injected into both the `Extractor` and the proactive engine.
- **Instant switch** — changing the setting rebuilds the extractor without restart (`learningMu` guard).

```go
func BuildDecider(cfg config.LearningConfig, orchestra OrchestraFn, single SingleCallFn) Decider {
    if cfg.SingleModelEnabled && cfg.ModelID != "" && single != nil {
        return func(ctx, system, user) (string, error) {
            return single(ctx, cfg.ModelID, system, user)
        }
    }
    // otherwise route through Orchestra
}
```

---

## 🔌 API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/calendar/events?from=&to=` | List events in time range |
| `POST` | `/api/calendar/events` | Create manual event |
| `DELETE` | `/api/calendar/events/{id}` | Delete event |
| `GET`/`PUT` | `/api/calendar/settings` | Reminder lead time (minutes) |
| `GET`/`PUT` | `/api/learning/settings` | Single model mode + model ID |

See [[API Documentation]].

---

## 🗄️ Data Flow & Integration

| Layer | File | What it does |
|-------|------|-------------|
| WhatsApp | `internal/app/whatsapp.go` | `runWhatsAppIntentLoop` — processes every incoming/outgoing message |
| Chat | `internal/app/chat.go` | `processMessageIntent` on user messages |
| Glue | `internal/app/learning.go` | init, decider factory, calendar bridge methods |
| Observation | `internal/observer/recorder.go` | `RecordIntent`, `RecordWhatsAppMessage` |

> **Important design decision:** The habit's declared time (e.g. "every day at 21:00") is stored at that time, not at the time the message was sent. Since `Store.Record` derives DayOfWeek and TimeOfDaySeconds from the timestamp, `RecordIntent` encodes the habit time directly into the timestamp — so the analyzer learns the correct pattern.

---

## 🧪 Test Coverage

| Package | Tests |
|---------|-------|
| `intent` | Keyword filter precision, extract (event/habit), string-aware JSON, prose-wrapped JSON |
| `calendar` | CRUD, time range, `PendingReminders`, reminder loop emit + single-fire |
| `observer` | `RecordIntent` preserves habit time, metadata round-trip |

All pass with `-race`.

---

## 🔭 Next Steps (Vision)

- Natural proactive chat from WhatsApp-learned intents ("hey, did you start working out?")
- Connecting all social media sources to the same intent pipeline
- Calendar ↔ proactive engine integration (pre-event preparation suggestions)

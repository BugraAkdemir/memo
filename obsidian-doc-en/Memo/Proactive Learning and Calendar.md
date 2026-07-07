# Proactive Learning and Calendar

> **Status:** ✅ Complete (backend + mobile + desktop settings) | **Version:** v3.1.1
>
> Memo's learning system now understands not just *when* you do something, but *what you tell it you will do* — in both chat and WhatsApp conversations.

Related pages: [[WhatsApp Integration]] · [[Orchestra Mode]] · [[Mobile App]] · [[Backend (Go) Architecture]]

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

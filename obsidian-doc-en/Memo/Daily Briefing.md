# 🌅 Daily Briefing

> **Packages:** `internal/autonomy/briefing.go` (builder), `briefing_text.go` (text for outside channels), `internal/twin/briefing.go` (settings, cards, delivery ledger), `internal/app/briefing*.go` (scheduling, model, delivery)
> **Data:** `twin.db` tables `briefings` and `briefing_deliveries`; the settings live in the twin preferences (`briefing`), **not** in `config.yaml` (`autonomy.briefing.*` there is never read)
> **API:** `POST /api/twin/preferences`, `POST /api/twin/briefing_read`, `GET /api/twin/state`
> Details: `docs/DIGITAL_TWIN_AND_MAIL.md` (last section), `docs/plans/PLAN_briefing_delivery.md`

Part of the [[Digital Twin and Mail]] system. **Off by default.**

## What it is
- **The morning card:** today's meetings, what was prepared overnight, drafts waiting, overdue replies, promises due today, unfinished code work — built from your own data. No model is needed.
- **A note before a meeting:** from 5 to 120 minutes ahead (15 by default): promises to that person, replies you wait for, waiting drafts, recent messages — for people Memo knows by address.

## What you can shape
| Setting | Choices |
|---|---|
| Time | any half hour, read in your time zone |
| Sections | any mix of `meetings`, `overnight`, `drafts`, `waiting`, `promises`, `code` (a briefing saved before this existed keeps all of them) |
| Custom items | up to five `{title, prompt}` |
| Channels | the app always; optionally Telegram (your owner chat), WhatsApp (your own number) and/or mail (your own address, from the account you pick) |
| Model | optional **briefing model**, separate from the mail reply model |

With a model: a short summary plus answers to the custom items from the day's data (which goes to the provider you picked; mail subjects travel as untrusted data). Without one: no model is called and custom items appear exactly as written. A failed model never blocks the card.

## Delivery safety
Each (briefing, channel) send is **claimed before it starts**, so a tick, a restart or a crash cannot send it twice. A finished send is never repeated; one of unknown outcome (cut short by a restart) is marked `uncertain` and not repeated; a known failure is retried up to five times, five minutes apart, within six hours; content the outbound secret scan refused is `blocked` and not retried. A channel that is down does not stop the others. Only the morning briefing leaves the app.

## Where to set it
The [[Setup Wizard]] (a step right after Channels) and **Settings → Mail & Autonomy → Digital Twin** use the same editor.

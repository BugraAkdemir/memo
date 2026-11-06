# Plan: configurable briefing delivery

Request (7 Nov): the morning briefing becomes configurable from Settings and from
the setup wizard (a new step right after Channels): which platforms it is sent to
(several at once), at what time, what it contains (several built-in sections plus
optional custom items) and which model writes it (separate from the reply model).

Today the briefing is built locally with no model, shown only in the app, and its
settings live in `twin.db` (`twin_preferences.briefing`). `config.yaml`
`autonomy.briefing.*` is dead config (never read).

## Decisions (made without asking; change them if wrong)

- Settings stay in `twin.Preferences.Briefing` (new optional fields; old stored JSON keeps working).
- **Sections**: `meetings`, `overnight`, `drafts`, `waiting`, `promises`, `code`. Missing list = all (the old behaviour); an explicit empty list = none.
- **Custom items** (max 5): `{title, prompt}`. With a briefing model the model answers the prompt from the day's data; without one the item appears verbatim as a reminder.
- **Model**: `briefing.model {provider, model}`, optional. Empty = no model call, exactly as today. A model failure never blocks the briefing (falls back to the plain card).
- **Channels** (`telegram`, `whatsapp`, `mail`) plus the always-on in-app card. Recipient is always the owner: Telegram owner chat, WhatsApp own number, mail account's own address. No free-form recipient.
- Only the **morning** briefing is delivered outside the app. Pre-meeting notes stay in-app.
- A send is claimed before it starts; an uncertain result is never retried (same rule as mail).

## Units

- [ ] B1 `twin`: settings fields + validation + tests
- [ ] B2 `autonomy`/`app`: section filter, custom items, optional model composition + tests
- [ ] B3 `twin`/`app`: per-channel delivery with claim/retry, text rendering (tr/en) + tests
- [ ] F1 Flutter: shared briefing editor, generalised model picker, Settings panel
- [ ] F2 Flutter: wizard step after Channels, snapshot/summary, tests
- [ ] D docs: `docs/DIGITAL_TWIN_AND_MAIL.md`, `docs/SETUP_WIZARD.md`, remove dead `autonomy.briefing` config

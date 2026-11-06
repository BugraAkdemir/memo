# 🧭 Setup Wizard

> **Code:** `frontend/lib/widgets/setup_wizard_view.dart`, `frontend/lib/widgets/setup/` (`setup_progress.dart` holds the rules)
> Tests: `setup_progress_test.dart`, `setup_steps_test.dart`, `setup_wizard_test.dart`
> Full page: `docs/SETUP_WIZARD.md`

A guided flow for everything that used to need a trip to Settings. Steps marked `*` are required; "Next" stays off until a required step is done; the side list scrolls and every row opens that step; compact windows use one column.

## Steps
1. Language and appearance
2. **Model connection `*`** — a local model (opens the Model Store in a window), an API key, or a subscription
3. Embedding model (recommended; skipping says plainly that memory will not work)
4. Persona
5. Mail (optional) — the same dialogs as the Mail screen
6. **Mail reply model `*`** — only once a mail account exists; also a safe "prepare reply drafts" switch and your time zone
7. Decision model
8. Memory and learning
9. Voice and images
10. Channels — WhatsApp and Telegram pairing
11. **Daily briefing** — see [[Daily Briefing]]
12. Privacy and backup — usage sharing (see [[Telemetry and Privacy]]), notification permission, encrypted cloud backup
13. Summary and check — every step as Done / Skipped / Missing, plus a system check

## How it decides
The rules are pure functions of a snapshot of the real app state (active provider, downloaded chat models, embedding model, mail accounts, the twin reply model, whether the briefing is on). There is no separate progress record, so the wizard resumes where it was left and stays right when something is changed elsewhere.

## Running it again
Settings → General restarts it. A re-run starts from the theme in use and writes the persona prompt only if you touched it there.

Related: [[Digital Twin and Mail]], [[Developer Setup Guide]].

# Setup wizard

The wizard walks a new user through everything that used to need a trip to
Settings. Required steps carry a trailing `*`; "Next" is off on a required step
that is not done, and the final button is off while any required step is
missing. The side list scrolls (there are up to twelve steps) and every row can
be opened directly; compact windows use a single column. Plan and unit history:
`docs/plans/PLAN_setup_wizard_v2.md`.

## Steps (`*` required)

| Step | What it does |
|---|---|
| Language & appearance | Defaults are preselected. |
| **Model connection \*** | **Local**: opens the Model Store as a window (the same store as the app, with sizes and hardware fit); closing it re-reads what is installed. **API key**: the existing provider dialog. **Subscription**: the existing sign-in flow inline. Done when a provider is active **or** a local chat model is downloaded. An embedding model alone is not a chat model. |
| Embedding model | Recommended, skippable. Download the recommended one, pick another in the store, or skip; skipping says plainly that memory will not work. Independent of how the chat model is connected. |
| Persona | Name and persona prompt. |
| Mail (optional) | Gmail, Outlook or custom IMAP/SMTP, using the same dialogs as the Mail screen. |
| **Mail reply model \*** | Appears only once a mail account exists. The model that writes reply drafts (same setting as Settings). Also: a safe switch "prepare reply drafts" (always draft-only mode, nothing is sent on its own) and the time zone, suggested from the device's UTC offset. |
| Decision model | The Settings section itself; recommended when mail is connected because it performs the incoming-mail security review. |
| Memory & learning | Memory, the nightly dream consolidation, proactive suggestions, minimal mode. |
| Voice & images | Speech to text, text to speech and voice, image model (Settings sections). |
| Channels | WhatsApp and Telegram pairing (Settings tabs, folded). |
| Privacy & backup | Usage sharing, OS notification permission where available, encrypted cloud backup. |
| Summary & check | Every step as Done / Skipped / Missing; a row goes back to that step. Also the system check. |

## How it decides

The rules are pure functions of a snapshot of the real application state
(`frontend/lib/widgets/setup/setup_progress.dart`): active provider, downloaded
chat models, embedding model, mail accounts, the twin reply model. There is no
separate progress record, so the wizard resumes where it was left and stays right
when something is changed elsewhere. The model step is always required; the reply
model step is required only while a mail account exists; everything else is
optional.

## Running it again

Settings -> General can restart the wizard. A re-run starts from the theme in use
and writes the persona prompt only if it was touched in the wizard, so a custom
prompt survives (`memo_setup_rerun` flag, cleared when the run finishes).

## Tests

`frontend/test/widgets/setup_progress_test.dart` (rules),
`setup_steps_test.dart` (step widgets, time zone and drafts writes, both
languages) and `setup_wizard_test.dart` (gating, mail requirement, store window,
summary, re-run, the full flow in Turkish and English at 360x900, 1100x900 and
1100x600).

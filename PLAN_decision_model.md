# Optional decision model

- [x] Add persisted, optional provider/model selection and bounded decision calls; use them for automatic memory retention/extraction and agent necessity without overriding explicit agent/code/task turns. Failures preserve existing behavior.
- [x] Expose settings in Learning with TR/EN labels, verify backend and frontend, rotate handoff and commit locally.

## Decision and verification extension
- [x] Add optional structured completion review with satisfaction/risk scores and configurable thresholds; return feedback to the same tool loop, cap at three reviews and perform one main-model self-check on rejection/error exhaustion.
- [x] Move decision settings to a dedicated tab, add verification controls and TR/EN labels, verify the real wiring and bounded fallback, rotate handoff and commit checkpoints.

## Provider model management redesign
- [x] Persist per-provider model IDs with explicit chat/decision roles; restrict decision calls and chat model switches accordingly, verify and commit backend.
- [x] Manage multiple models in API Providers, embed decision/verification model picker there, remove separate tab and expand chat dropdown; localize, verify and commit with handoff.

## Unrestricted selection and usable model menus
- [x] Remove manual model-role restrictions; retain and merge configured/catalog models, verify backend and commit.
- [x] Show provider model groups even for legacy records, simplify model editing and decision controls, verify UI and rotate handoff; import local Claude skills.

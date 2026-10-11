# Optional decision model

- [x] Add persisted, optional provider/model selection and bounded decision calls; use them for automatic memory retention/extraction and agent necessity without overriding explicit agent/code/task turns. Failures preserve existing behavior.
- [x] Expose settings in Learning with TR/EN labels, verify backend and frontend, rotate handoff and commit locally.

## Decision and verification extension
- [x] Add optional structured completion review with satisfaction/risk scores and configurable thresholds; return feedback to the same tool loop, cap at three reviews and perform one main-model self-check on rejection/error exhaustion.
- [ ] Move decision settings to a dedicated tab, add verification controls and TR/EN labels, verify the real wiring and bounded fallback, rotate handoff and commit checkpoints.

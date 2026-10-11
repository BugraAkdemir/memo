# Optional decision model

Settings → API Providers contains the decision and verification controls. Add or edit a provider once, add model IDs from its catalog or enter them, and save them. Select any connected API model from the dropdown and save. Local OpenAI-compatible servers and Ollama can be used through their configured providers; CLI coding agents cannot. With this option disabled, existing routing remains unchanged. When enabled it takes priority over Learning's older single-model setting.

The decision model checks whether an automatic conversation-memory write is useful, extracts durable personal facts, analyses learning intents and supplies proactive decisions. Explicit memory writes are unaffected. When the global agent toggle is on, it also checks whether a normal chat needs tools; only an exact NO skips the agent loop. It never enables an agent that the user disabled and never skips explicit project-agent chats, Code Mode, task workers or turns with attachments. Recent user/assistant messages provide context for follow-ups.

Requests go directly to the selected provider/model, without tools or changes to the active chat model. Yes/no calls request at most eight output tokens, structured extraction requests at most 512, with a five-second timeout and an 8,000-character (Unicode rune) input bound. Provider errors and ambiguous classifications preserve the existing routing; failed extraction/learning calls fall back to their original paths. Reported usage appears under Learning in Usage Stats. Switching settings takes effect without restart.

A separate model is not automatically cheaper or faster: that depends on the model selected. Agent classification adds an extra request before the main answer. Memory selection is a best-effort model judgment and may omit a useful turn; explicit “remember” actions remain available. The chosen provider receives the text needed for its decisions.

## Completion verification

Settings → API Providers contains the configuration card alongside default image settings; there is no separate decision tab. Decision routing and result verification have separate switches and share the selected provider/model. Verification is off by default. The default acceptance threshold is 85% requirement satisfaction; the default maximum estimated bug and regression risk is 20%. Both thresholds are configurable.

For Memo agent turns, the verifier sees the original request, a bounded excerpt of actual user/assistant/tool messages and the proposed final answer. It returns structured satisfaction, bug-risk and other-feature/regression-risk assessments plus concrete concerns and whether sufficient evidence exists. Missing, invalid or out-of-range scores cannot pass. Missing evidence also rejects the result even when scores are high. These percentages are subjective model assessments, not measured failure probabilities or proof that tests passed.

A rejected draft is withheld from the answer stream. The same main-model tool loop receives the concerns and may inspect code, run tests and correct the work under the existing permission rules. There are at most three verifier calls for the turn. After three rejections/errors, the main model gets one independent self-check phase with tool access; the verifier is not called again. Existing iteration and time limits still apply. This does not force a successful result: unavailable tools, failing tests, cancellation or exhausted budgets must remain visible. Scores and the self-check transition appear as verification events in the chat.

This hook covers Memo's agent pipeline (including Code Mode); plain text chat streams and external coding agents' own internal workflows do not gain an independent file/test inspection engine. The reviewer itself never executes tools or reads arbitrary files. Its transcript excerpt may omit relevant code, so it must request missing checks from the main agent rather than approve unsupported claims.

## Multiple models per API connection

Provider models are stored as IDs; existing single-model configurations are also shown as model rows under their provider. There is no manual Chat/Decision type selector. Any connected API model can serve chat, decision and verification work. The chat picker first shows saved/current models immediately, then merges vendor catalog models in the background. Catalog failures leave saved choices available. Changing the active model saves both the prior and selected ID; older clients updating only the active model also preserve earlier choices. Model-role restrictions from the previous implementation are removed.

Subscriptions retain their dynamic live catalog. Model IDs survive provider edits and restart alongside the existing encrypted credential. Availability still depends on the vendor; an unavailable decision or verification model follows the existing bounded fallback. Saved selection is separate from whether the model is cheaper, faster or good at evaluation.

The provider editor shows a single model list with direct selection and remove actions. Choosing from its catalog adds the model to that list. API Providers includes a direct model-management button and a compact decision card with optional acceptance-limit controls. No separate decision tab is used.

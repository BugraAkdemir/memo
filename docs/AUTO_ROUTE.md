# Auto Route

Select **Auto Route** in the chat model picker to choose a model for each request. This is a routing mode, not a downloaded model or an API connection. The actual provider/model appears above the streaming answer and remains attached to the saved assistant message. When token streaming is disabled, routing metadata still arrives over SSE while answer text is buffered until completion. Selecting Auto Route leaves a chat's CLI provider mode; explicit API, local and CLI selections remain available.

## Candidate pool and selection

- Enabled API connections: only their saved model IDs and current model. A vendor's full catalog is never implicitly added.
- Subscriptions: models exposed by the connected account's sidecar, with exact account-model context from the existing CLIProxyAPI capability catalog.
- Local: running chat model and downloaded chat GGUFs, excluding embedding models. GGUF task/vision/tool flags and the configured runtime context limit are used. A cold GGUF runs in a private llama-server process, on a separate loopback port, and is stopped after the turn. It does not replace the user's running local model or client. Private processes never kill an unrelated listener by port.
- CLI agents are excluded from the automatic pool because they maintain a separate session/tool protocol.

Selection happens before history truncation. The input estimate includes the current message, extracted file text, existing conversation history and a tool/output margin. Known context windows that fit are preferred. If none fit, the largest known window is used with Memo's existing history compaction. Vision requests require known vision support; known models without tool support are excluded from tool-bearing turns. Coding identifier/description hints and local coding flags favor coding models; general requests favor general models. Provider priority breaks suitability ties, then an already available endpoint and a sufficient context window. These hints are not measured quality or speed benchmarks.

If the optional decision model is enabled, it can choose an index from the eligible candidate list using bounded task text and capability facts. Invalid indices, errors and timeouts fall back immediately to the deterministic selection. There is no classifier retry loop. Decision and verification features remain optional; their disabled behavior is unchanged.

All models use the same saved chat history and system/memory/skill inputs. Each request is assembled against its chosen window; switching models does not create a second session. Different context limits can require summarizing older turns. Token estimates cannot guarantee an exact tokenizer count. Unknown capacities stay unknown in the gauge; the existing internal safety budget is used for request assembly.

Explicit drawing requests retain the existing default image routing and identify the image model. A normal question does not automatically become an image-generation call. The existing image routing setting and `/image` behavior remain in effect.

## Internet and offline fallback

API capability facts come from [Models.dev's published API](https://models.dev). Only exact IDs are matched, preferring the exact provider endpoint. Conflicting capabilities across endpoints are not guessed. Known embedding/reranking and image-output models are excluded from the text pool. The catalog is metadata, never proof that an account has access to a model.

1. Use a fresh in-memory or disk snapshot (six-hour refresh interval).
2. Refresh over HTTPS with a ten-second bound; save only successful JSON snapshots atomically at `config.DataPath("model-routing-capabilities.json")`.
3. If the site is unavailable, returns an HTTP error or invalid JSON, retain the last successful snapshot. Retry is limited; an empty cache retries after five minutes.
4. If no snapshot exists, use explicit provider context settings, the existing subscription capability cache and local GGUF/runtime facts. Unknown capability values are not presented as measured facts. Unknown-only text pools use a stable provider-priority fallback; unknown vision support is not claimed.

No prompt, key or account information is sent to the catalog host. A cached catalog permits local model selection offline; it cannot make a remote API work without an internet connection. If no suitable connected model exists, the turn ends with an explicit error instead of silently choosing an unconfigured vendor model.

## Verification

Real HTTP regression coverage exercises general → coding → long-context switches in one chat, shared earlier replies, saved/live attribution, actual context gauge, unchanged manual selection, explicit image generation, a valid classifier selection and invalid/failed classifier fallbacks. Catalog tests exercise an HTTP outage with and without a disk cache and exact/ambiguous IDs. Flutter tests exercise live marker parsing, malformed markers, serialized messages and narrow TR/EN layouts. Cold-setup tests verify that SSE headers/heartbeats are available before model loading completes; private-port tests verify that unrelated listeners remain untouched.

Real vendor model quality, subscription entitlement/quota behavior and cold loading of a real GGUF have not been measured in this change. Runtime loading/inference errors remain visible; Auto Route does not silently retry a possibly billable answer on another provider. TestSprite is not linked to this repository; verification uses the existing Go HTTP and Flutter suites.
